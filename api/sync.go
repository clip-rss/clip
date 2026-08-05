package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/clip-rss/clip/internal/secret"
	"github.com/clip-rss/clip/internal/store"
	"github.com/clip-rss/clip/internal/syncer"
	"github.com/clip-rss/clip/internal/webdav"
)

// syncTimeout 单次同步操作的整体超时。
//
// 高于 webdav.DefaultTimeout（单请求 30s）：一次同步最多是
// GET → MKCOL → PUT 三个来回，给足余量，又不至于让「立即同步」按钮转到用户以为卡死。
const syncTimeout = 90 * time.Second

// SyncService 配置同步的绑定方法。
type SyncService struct {
	store    *store.Store
	settings *SettingsService
	vault    *syncer.Vault
	engine   *syncer.Syncer

	// mu 保护 debounce 与 suppress。engine 自带互斥，vault 无状态，都不需要这把锁。
	mu       sync.Mutex
	debounce *syncer.Debouncer

	// suppress 抑制变更回调。同步引擎拉取时会经 settings.UpdateSettings 写库，
	// 那会触发变更回调，进而安排一次推送 —— 而刚拉下来的配置正是远端的那份，
	// 推回去毫无意义。计数而非布尔：虽然引擎已串行化，但计数不会因日后
	// 出现嵌套调用而错误地提前解除抑制。
	suppress int
}

// NewSyncService 创建同步服务。
//
// cipher 为 nil 表示凭据加密不可用（密钥文件所在路径被占等）。此时服务照常构造，
// 但所有涉及凭据的方法返回明确错误 —— 同步坏了不该拖垮整个应用启动。
//
// ⚠️ settings 必须是注册给 Wails 的**同一个** *SettingsService 实例：
// 拉取要经它的 UpdateSettings 才会把新的更新间隔下发到订阅源与调度器。
// 直接传 store 的话，拉下来的间隔只会落库，直到下次重启才生效。
func NewSyncService(st *store.Store, settings *SettingsService, cipher *secret.Cipher) *SyncService {
	s := &SyncService{store: st, settings: settings}
	if cipher != nil {
		s.vault = syncer.NewVault(st, cipher)
	}
	// remote 先给 nil：此刻还不知道用户配没配。每次操作前由 refreshRemote 现装。
	s.engine = syncer.New(settingsWriter{svc: settings, owner: s}, st, nil)
	return s
}

/* ---------- 前端绑定方法 ---------- */

// GetWebDAVConfig 读取同步配置。不含密码，只带 hasPassword。
func (s *SyncService) GetWebDAVConfig() (syncer.WebDAVView, error) {
	if s.vault == nil {
		return syncer.WebDAVView{}, errCredentialStoreUnavailable
	}
	return s.vault.View()
}

// SaveWebDAVConfig 保存同步配置。密码传空串表示保持原密码不变。
func (s *SyncService) SaveWebDAVConfig(cfg syncer.WebDAVInput) error {
	if s.vault == nil {
		return errCredentialStoreUnavailable
	}

	// ⚠️ 先校验地址，再落库。
	//
	// 反过来（先存后校验）的后果：用户看到一条错误、以为什么都没保存，
	// 而库里已经是那份坏配置且 enabled=true —— 下次启动 StartSync 读到它，
	// 之后每次同步都失败，而当初那条错误早已消失，用户无从关联。
	//
	// 地址规则的唯一裁判仍是 webdav.New，这里只是提前把它请出来判一次。
	if cfg.Enabled {
		creds, err := s.vault.CredentialsFor(cfg)
		if err != nil {
			return err
		}
		if _, err := s.newClient(creds); err != nil {
			return err
		}
	}

	if err := s.vault.Save(cfg); err != nil {
		return err
	}
	// 立刻重装远端，让「保存后马上点同步」用的是新配置而不是上一份。
	return s.refreshRemote()
}

// ClearWebDAVConfig 删除全部同步配置（含密码）。
func (s *SyncService) ClearWebDAVConfig() error {
	if s.vault == nil {
		return errCredentialStoreUnavailable
	}
	if err := s.vault.Clear(); err != nil {
		return err
	}
	s.engine.SetRemote(nil)
	return nil
}

// ConnectionTestResult 「测试连接」的逐步结果。
//
// 有意用结构体而非 error 返回：Wails 把 error 压成一个字符串，前端无法用
// errors.Is 判别，也就没法针对「认证失败」渲染服务商特定建议（如坚果云需用
// 应用密码）。把「哪一步失败」与「建议」作为数据回去，前端才能照 errors.Is
// 的判别结果那样分支。
type ConnectionTestResult struct {
	OK bool `json:"ok"`

	// Step 失败的步骤：connect / mkcol / write / delete；成功时为空。
	Step string `json:"step"`

	// Message 失败原因（面向用户）。
	Message string `json:"message"`

	// Hint 可操作的建议，可能为空。与 Message 分开，便于前端用不同字号展示。
	Hint string `json:"hint"`
}

// TestWebDAVConnection 建目录 + 写探针文件 + 删除，逐项报错。
//
// 传入的是**尚未保存**的表单配置：用户改完就点测试，不该先被迫保存一份
// 可能连不上的配置。密码留空时回落到已存的密码。
func (s *SyncService) TestWebDAVConnection(cfg syncer.WebDAVInput) (ConnectionTestResult, error) {
	if s.vault == nil {
		return ConnectionTestResult{}, errCredentialStoreUnavailable
	}
	creds, err := s.vault.CredentialsFor(cfg)
	if err != nil {
		return failedTest("connect", err), nil
	}
	client, err := s.newClient(creds)
	if err != nil {
		return failedTest("connect", err), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
	defer cancel()

	// 逐步来：每一步的失败原因不同，混在一次请求里报错等于让用户猜。
	if err := client.MkcolAll(ctx, syncer.RemoteDir()); err != nil {
		return failedTest("mkcol", err), nil
	}
	probe := syncer.RemoteDir() + "clip-probe.tmp"
	if _, err := client.Put(ctx, probe, []byte("clip connection probe\n"), ""); err != nil {
		return failedTest("write", err), nil
	}
	if err := client.Delete(ctx, probe); err != nil {
		// 写成功但删不掉：同步本身能用，只是留了个探针文件。
		// 不算失败，否则会把一个可用的配置判成不可用。
		log.Printf("sync: 探针文件清理失败（不影响同步）: %v", err)
	}
	return ConnectionTestResult{OK: true}, nil
}

// SyncNow 立即同步一次。
func (s *SyncService) SyncNow() (syncer.Result, error) {
	if err := s.refreshRemote(); err != nil {
		return syncer.Result{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
	defer cancel()

	res, err := s.engine.Sync(ctx)
	if err != nil {
		return syncer.Result{}, describeSyncError(err)
	}
	return res, nil
}

// ResolveConflict 按用户的选择解决冲突：keepLocal 为真用本地覆盖远端。
func (s *SyncService) ResolveConflict(keepLocal bool) (syncer.Result, error) {
	if err := s.refreshRemote(); err != nil {
		return syncer.Result{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
	defer cancel()

	res, err := s.engine.Resolve(ctx, keepLocal)
	if err != nil {
		return syncer.Result{}, describeSyncError(err)
	}
	return res, nil
}

// GetSyncStatus 上次同步时间 / 上次错误 / 是否有待推送改动。
func (s *SyncService) GetSyncStatus() (syncer.Status, error) {
	return s.engine.Status()
}

/* ---------- 生命周期 ---------- */
//
// ⚠️ 这几个方法一律不导出，改由下方的包级函数供 main 调用。
// wails3 会把 Service 上每一个**导出方法**都生成成前端可调用的绑定 ——
// 注释写「不暴露给前端」是拦不住的。而它们一旦能被前端调到就是真的有害：
// 前端调 stop 会静默关掉本次会话的自动推送；反复调 start 会泄漏 Debouncer
// 并叠出多次启动拉取。

// StartSync 装好远端并安排启动后的延迟拉取。由 main 在装配期调用。
func StartSync(s *SyncService) { s.start() }

// StopSync 取消尚未执行的推送。由 main 在退出钩子里调用。
func StopSync(s *SyncService) { s.stop() }

// start 装好远端并安排启动后的延迟拉取。不阻塞：拉取在后台进行。
//
// 未配置同步时静默返回 —— 这是绝大多数用户的状态，不该在日志里刷错误。
func (s *SyncService) start() {
	if s.vault == nil {
		return
	}
	enabled, err := s.vault.Enabled()
	if err != nil {
		log.Printf("sync: 读取同步配置失败: %v", err)
		return
	}
	if !enabled {
		return
	}

	s.mu.Lock()
	s.debounce = syncer.NewDebouncer(syncer.PushDebounce, s.pushNow)
	s.mu.Unlock()

	// 延迟拉取：让界面先可用。计时器而非 goroutine+Sleep，便于 Stop 取消。
	time.AfterFunc(syncer.StartupDelay, func() {
		if _, err := s.SyncNow(); err != nil {
			// 启动时同步失败很常见（还没连上网），记一行就够，
			// 状态已经存进 State.LastError，设置页会展示。
			log.Printf("sync: 启动同步失败: %v", err)
		}
	})
}

// stop 取消尚未执行的推送。
func (s *SyncService) stop() {
	s.mu.Lock()
	d := s.debounce
	s.debounce = nil
	s.mu.Unlock()
	if d != nil {
		d.Stop()
	}
}

// notifySettingsChanged 配置变更后安排一次延迟推送。
// 实现 SettingsObserver —— 同包内未导出方法即可满足接口，故不必导出。
func (s *SyncService) notifySettingsChanged() {
	s.mu.Lock()
	suppressed := s.suppress > 0
	d := s.debounce
	s.mu.Unlock()

	if suppressed || d == nil {
		return
	}
	d.Trigger()
}

// pushNow 由 debounce 触发的推送。
func (s *SyncService) pushNow() {
	if _, err := s.SyncNow(); err != nil {
		log.Printf("sync: 自动推送失败: %v", err)
	}
}

/* ---------- 内部 ---------- */

// settingsWriter 把 *SettingsService 适配成 syncer.SettingsStore，
// 并在同步引擎写设置期间抑制变更回调。
//
// 不能直接把 *SettingsService 交给引擎：引擎拉取后写库会触发变更回调，
// 于是刚拉下来的配置又被安排推回远端 —— 一次无谓的往返，且在两台机器间
// 可能来回打转。
type settingsWriter struct {
	svc   *SettingsService
	owner *SyncService
}

func (w settingsWriter) GetSettings() (store.Settings, error) {
	return w.svc.GetSettings()
}

func (w settingsWriter) UpdateSettings(v store.Settings) error {
	w.owner.mu.Lock()
	w.owner.suppress++
	w.owner.mu.Unlock()
	defer func() {
		w.owner.mu.Lock()
		w.owner.suppress--
		w.owner.mu.Unlock()
	}()
	return w.svc.UpdateSettings(v)
}

// refreshRemote 按当前凭据与代理设置重装远端客户端。
//
// 每次操作前都重装：用户可能刚改了地址、密码或代理。构造成本只是解析一个 URL。
func (s *SyncService) refreshRemote() error {
	if s.vault == nil {
		return errCredentialStoreUnavailable
	}
	enabled, err := s.vault.Enabled()
	if err != nil {
		return err
	}
	if !enabled {
		s.engine.SetRemote(nil) // 让引擎回到 ErrNotConfigured
		return syncer.ErrNotConfigured
	}

	creds, err := s.vault.Credentials()
	if err != nil {
		return err
	}
	client, err := s.newClient(creds)
	if err != nil {
		return err
	}
	s.engine.SetRemote(client)
	return nil
}

// newClient 用凭据与当前代理设置构造 WebDAV 客户端。
func (s *SyncService) newClient(creds syncer.WebDAVCredentials) (*webdav.Client, error) {
	opts := []webdav.Option{}
	// 代理与 RSS 抓取共用同一份设置（Settings.ProxyHost/ProxyPort）。
	// 读取失败不阻断同步：直连也可能通，让网络层给出可诊断的错误。
	if cfg, err := s.store.GetSettings(); err == nil && cfg.ProxyHost != "" && cfg.ProxyPort > 0 {
		opts = append(opts, webdav.WithProxy(cfg.ProxyHost, cfg.ProxyPort))
	}
	return webdav.New(webdav.Config{
		URL:      creds.URL,
		Username: creds.Username,
		Password: creds.Password,
	}, opts...)
}

// errCredentialStoreUnavailable 凭据加密不可用时的统一错误。
var errCredentialStoreUnavailable = errors.New(
	"凭据存储不可用，无法读写同步密码；请检查配置目录权限后重启")

// failedTest 把错误转成一步失败的测试结果。
func failedTest(step string, err error) ConnectionTestResult {
	return ConnectionTestResult{
		OK:      false,
		Step:    step,
		Message: err.Error(),
		Hint:    hintFor(err),
	}
}

// hintFor 按错误类别给出可操作建议。
//
// 服务商特定的建议放在这里而不是 webdav 包：那一层只负责判定「认证失败」
// 这个协议事实，具体建议依赖用户所用的服务，且日后要随界面语言切换。
func hintFor(err error) string {
	switch {
	case errors.Is(err, webdav.ErrUnauthorized):
		return "若使用坚果云，请在「安全选项」里生成应用密码，不要用登录密码。" +
			"Nextcloud 开启两步验证后同样需要应用专用密码。"
	case errors.Is(err, webdav.ErrNotCollection):
		return "服务器地址指向的上级目录不存在。请确认地址填到了 WebDAV 根目录，" +
			"例如 Nextcloud 形如 https://<域名>/remote.php/dav/files/<用户名>/"
	case errors.Is(err, webdav.ErrNotFound):
		return "地址不存在。常见原因是只填了域名而漏掉了 WebDAV 路径。"
	case errors.Is(err, webdav.ErrInvalidConfig):
		return "请检查地址格式，必须以 https:// 开头。"
	case errors.Is(err, webdav.ErrInsufficientStorage):
		return "网盘空间不足，清理后重试。"
	case errors.Is(err, secret.ErrCredentialsLost):
		return "本机的凭据密钥已失效（常见于换机器或清理过配置目录），请重新输入密码。"
	case errors.Is(err, syncer.ErrNoPassword):
		return "请输入密码后再测试。"
	case errors.Is(err, webdav.ErrNetwork):
		return "请检查网络连接与代理设置。"
	}
	return ""
}

// describeSyncError 给同步错误补上建议，供前端直接展示。
//
// Wails 会把 error 压成字符串，前端拿不到哨兵 —— 建议必须在这一层拼进去。
func describeSyncError(err error) error {
	if hint := hintFor(err); hint != "" {
		return fmt.Errorf("%w（%s）", err, hint)
	}
	return err
}
