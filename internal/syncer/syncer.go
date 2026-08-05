package syncer

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/clip-rss/clip/internal/store"
	"github.com/clip-rss/clip/internal/webdav"
)

// 远端布局。目录由本包独占，故其中的文件名不必再加前缀防重名。
//
// 用户填的地址只需是一个**已存在**的目录（网盘的 dav 根目录即可，
// Nextcloud 与坚果云都开箱存在），clip/ 由我们自己建。
const (
	remoteDir  = "clip/"
	remoteFile = "clip/settings.json"
)

// RemoteDir 返回同步目录（相对用户填写的地址）。
//
// 导出给 api 层做「测试连接」用：那一步要建目录并写探针文件，位置必须与
// 真正同步用的目录完全一致 —— 否则测的是另一个地方的写权限，
// 通过了也不代表同步能用。故只有这一个出口，不在 api 层再抄一遍路径。
func RemoteDir() string { return remoteDir }

// RemoteFile 返回同步文件的完整相对路径，供设置页向用户交代配置存到了哪里。
func RemoteFile() string { return remoteFile }

// 触发时机的两个延时常量。有意不提供后台轮询间隔：配置同步没有实时性要求，
// 轮询只会白耗电量与网盘的请求配额（坚果云对频率有限制）。
const (
	// PushDebounce 配置改动后延迟推送的时长。用户在设置页连续调几项时合并成一次上传。
	PushDebounce = 8 * time.Second

	// StartupDelay 启动后延迟拉取的时长。不阻塞启动，让界面先可用。
	StartupDelay = 5 * time.Second
)

// Remote 同步引擎用到的远端能力，由 *webdav.Client 实现。
//
// 抽成接口是为了让同步判定能脱离网络测试 —— 冲突判定是本阶段最容易出错的
// 部分，用假远端把五种组合逐一钉住，比起真连服务器可靠得多。
//
// 没有 Stat：Get 一次就同时拿到内容与 ETag，而变化判定用的是内容哈希
// （见 State 的说明），先 Stat 再 Get 等于多一趟往返换一个更弱的信号。
type Remote interface {
	Get(ctx context.Context, path string) ([]byte, string, error)
	Put(ctx context.Context, path string, data []byte, ifMatch string) (string, error)
	MkcolAll(ctx context.Context, path string) error
}

// SettingsStore 设置的读写能力。
//
// ⚠️ 生产代码应传 *api.SettingsService 而非 *store.Store：前者的 UpdateSettings
// 会把更新间隔应用到全部订阅源与调度器、并刷新代理。直接传 store 的话，
// 拉取到的新更新间隔只会落库，不会真正生效，直到下次重启。
type SettingsStore interface {
	GetSettings() (store.Settings, error)
	UpdateSettings(store.Settings) error
}

// Action 一次同步的结果动作。
type Action string

const (
	ActionNoop     Action = "noop"     // 两侧一致，什么都没做
	ActionPushed   Action = "pushed"   // 本地配置已上传
	ActionPulled   Action = "pulled"   // 远端配置已应用到本地
	ActionConflict Action = "conflict" // 两侧都改过，等用户裁决
)

// Result 一次同步的结果。
type Result struct {
	Action Action `json:"action"`

	// Conflict 仅 ActionConflict 时非空。
	Conflict *ConflictInfo `json:"conflict"`

	// Settings 仅 ActionPulled 时非空：把应用后的完整设置一并回给前端，
	// 省去「同步完再查一次设置」的往返，也避免两次调用之间的状态错位。
	Settings *store.Settings `json:"settings"`
}

// Syncer 配置同步引擎。零值不可用，须经 New 构造。
type Syncer struct {
	settings SettingsStore
	state    StateStore
	remote   Remote

	// mu 串行化整个同步过程。手动按钮与 debounce 推送可能同时触发，
	// 两者交错会各自基于同一份旧状态做判定，后写的那次覆盖前一次的基线。
	mu sync.Mutex
}

// New 创建同步引擎。remote 为 nil 表示用户尚未配置同步，此时所有操作返回
// ErrNotConfigured —— 让「没配置」成为一个明确状态，而不是 nil 解引用崩溃。
func New(settings SettingsStore, state StateStore, remote Remote) *Syncer {
	return &Syncer{settings: settings, state: state, remote: remote}
}

// SetRemote 替换远端客户端。用户在设置页改了地址或密码后调用；
// 传 nil 表示关闭同步。
func (s *Syncer) SetRemote(remote Remote) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.remote = remote
}

// Status 返回当前同步状态，供设置页展示。
func (s *Syncer) Status() (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := loadState(s.state)
	if err != nil {
		return Status{}, err
	}
	local, err := s.settings.GetSettings()
	if err != nil {
		return Status{}, err
	}

	out := Status{
		LastError:  st.LastError,
		HasPending: From(local).Hash() != st.LocalHash,
		Conflict:   st.Conflict,
		DeviceName: DeviceName(),
	}
	if !st.LastSyncAt.IsZero() {
		at := st.LastSyncAt
		out.LastSyncAt = &at
	}
	return out, nil
}

// Sync 执行一次同步，按两侧的变化情况自行决定推送 / 拉取 / 冲突。
//
// 判定表（remote 指远端载荷里的配置，local 指本机配置）：
//
//	远端不存在        → push（首次上传）
//	远端未变 & 本地未变 → noop
//	远端未变 & 本地变了 → push
//	远端变了 & 本地未变 → pull
//	远端变了 & 本地变了 → conflict（本机仍是出厂默认时除外，见 decide）
func (s *Syncer) Sync(ctx context.Context) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, err := s.newRun(ctx)
	if err != nil {
		return Result{}, err
	}

	data, etag, err := r.get()
	if errors.Is(err, webdav.ErrNotFound) {
		return r.push("", 1, nil)
	}
	if err != nil {
		return Result{}, r.fail(err)
	}

	remote, err := Decode(data, r.base)
	if err != nil {
		// 格式不兼容 / 文件不是我们的：既不应用也不推送，交给用户处理。
		// 推上去会覆盖掉高版本客户端的配置或用户的其他文件。
		return Result{}, r.fail(err)
	}
	return r.decide(remote, etag)
}

// Resolve 按用户的选择解决冲突：keepLocal 为真用本地覆盖远端，否则用远端覆盖本地。
//
// 不做字段级自动合并。配置项之间没有可靠的合并语义（把 A 机的主题和 B 机的
// 字号拼起来得到的是第三种、谁都没选过的配置），二选一虽然粗但结果可预期。
func (s *Syncer) Resolve(ctx context.Context, keepLocal bool) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, err := s.newRun(ctx)
	if err != nil {
		return Result{}, err
	}
	if r.st.Conflict == nil {
		return Result{}, ErrNoConflict
	}

	// 无论哪个方向都先重读远端：弹窗可能开了很久，其间远端可能又被改过，
	// 也可能已被删除。用最新状态执行用户的选择。
	data, etag, err := r.get()
	notFound := errors.Is(err, webdav.ErrNotFound)
	if err != nil && !notFound {
		return Result{}, r.fail(err)
	}

	if notFound {
		// 远端已不存在：无论用户选哪边，能做的只有把本地推上去重建。
		return r.push("", 1, nil)
	}

	remote, err := Decode(data, r.base)
	if err != nil {
		return Result{}, r.fail(err)
	}
	if keepLocal {
		return r.push(etag, remote.Revision+1, &remote)
	}
	return r.pull(remote, etag)
}

/* ---------- 单次同步的执行上下文 ---------- */

// run 承载一次同步调用期间的不变量，免得每个内部方法都重复传五六个参数。
type run struct {
	s   *Syncer
	ctx context.Context

	st    State            // 载入时的同步状态
	base  store.Settings   // 本机完整设置
	local SyncableSettings // 本机可同步子集
	hash  string           // local 的内容哈希
}

// newRun 载入状态与本机设置。
func (s *Syncer) newRun(ctx context.Context) (*run, error) {
	if s.remote == nil {
		return nil, ErrNotConfigured
	}
	st, err := loadState(s.state)
	if err != nil {
		return nil, err
	}
	base, err := s.settings.GetSettings()
	if err != nil {
		return nil, err
	}
	local := From(base)
	return &run{s: s, ctx: ctx, st: st, base: base, local: local, hash: local.Hash()}, nil
}

func (r *run) get() ([]byte, string, error) {
	return r.s.remote.Get(r.ctx, remoteFile)
}

// decide 在远端载荷已就绪时挑选动作。
func (r *run) decide(remote Payload, etag string) (Result, error) {
	remoteHash := remote.Settings.Hash()
	remoteChanged := remoteHash != r.st.RemoteHash
	localChanged := r.hash != r.st.LocalHash

	switch {
	case !remoteChanged && !localChanged:
		// 两侧都没动。仍要落一次状态：ETag 可能变了（有的服务器会重算），
		// 首次遇到「远端内容恰好与本地一致」时也得把基线补上，
		// 否则每次同步都会重复走一遍判定。
		return r.noop(remoteHash, remote.Revision, etag)

	case localChanged && !remoteChanged:
		return r.push(etag, remote.Revision+1, &remote)

	case remoteChanged && !localChanged:
		return r.pull(remote, etag)

	default:
		// 两侧都变了。但若本机配置仍是出厂默认，说明用户在这台机器上没改过
		// 任何配置（典型场景：新机器接入已有的同步），拿一份没人动过的默认配置
		// 去打扰用户做二选一没有意义 —— 直接拉取。
		if r.hash == From(store.DefaultSettings()).Hash() {
			return r.pull(remote, etag)
		}
		return r.conflict(remote)
	}
}

// push 上传本机配置。
//
// prevRemote 为已知的远端载荷，仅在 PUT 撞上 412 后用于展示冲突来源；
// 首次推送时无远端可言，传 nil。
func (r *run) push(ifMatch string, revision int, prevRemote *Payload) (Result, error) {
	data, err := Encode(newPayload(r.local, revision, time.Now()))
	if err != nil {
		return Result{}, r.fail(err)
	}

	etag, err := r.put(data, ifMatch)
	if errors.Is(err, webdav.ErrConflict) {
		// 412：GET 与 PUT 之间远端被另一台机器改写。这正是 If-Match 要挡的
		// 竞态窗口 —— 不能重试推送（会覆盖对方刚写的内容），转入冲突分支。
		return r.conflictAfterPreconditionFailed(prevRemote)
	}
	if err != nil {
		return Result{}, r.fail(err)
	}

	return Result{Action: ActionPushed}, r.commit(State{
		RemoteETag:     etag,
		RemoteHash:     r.hash, // 远端现在就是我们刚推上去的这份
		RemoteRevision: revision,
		LocalHash:      r.hash,
		LastSyncAt:     time.Now(),
	})
}

// put 上传，并在目录缺失时建目录重试一次。
//
// 首次同步时 clip/ 必然不存在，而 409（父目录不存在）在不同服务器上的表现
// 并不统一 —— 有的回 409，有的直接 404。两者都当作「该建目录了」。
func (r *run) put(data []byte, ifMatch string) (string, error) {
	etag, err := r.s.remote.Put(r.ctx, remoteFile, data, ifMatch)
	if err == nil {
		return etag, nil
	}
	if !errors.Is(err, webdav.ErrNotCollection) && !errors.Is(err, webdav.ErrNotFound) {
		return "", err
	}
	if mkErr := r.s.remote.MkcolAll(r.ctx, remoteDir); mkErr != nil {
		// 建目录也失败：报建目录的错误，它更接近根因（通常是用户填的
		// 地址本身不存在，或没有写权限）。
		return "", mkErr
	}
	return r.s.remote.Put(r.ctx, remoteFile, data, ifMatch)
}

// pull 应用远端配置到本机。
func (r *run) pull(remote Payload, etag string) (Result, error) {
	merged := Apply(r.base, remote.Settings)
	if err := r.s.settings.UpdateSettings(merged); err != nil {
		return Result{}, r.fail(err)
	}

	// 基线记两个哈希，不能只记一个：远端若含本端不认识的取值，Apply 会把该字段
	// 回落成本机原值，应用后的本机配置与远端载荷并不相等。详见 State 的说明。
	err := r.commit(State{
		RemoteETag:     etag,
		RemoteHash:     remote.Settings.Hash(),
		RemoteRevision: remote.Revision,
		LocalHash:      From(merged).Hash(),
		LastSyncAt:     time.Now(),
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Action: ActionPulled, Settings: &merged}, nil
}

// noop 两侧一致，只刷新基线。
func (r *run) noop(remoteHash string, revision int, etag string) (Result, error) {
	return Result{Action: ActionNoop}, r.commit(State{
		RemoteETag:     etag,
		RemoteHash:     remoteHash,
		RemoteRevision: revision,
		LocalHash:      r.hash,
		LastSyncAt:     time.Now(),
	})
}

// conflict 记录待裁决的冲突。
//
// 不动任何基线哈希：冲突意味着还没同步成功，把基线前移会让下次同步
// 误认为两侧已对齐，从而静默丢掉一侧的改动。
func (r *run) conflict(remote Payload) (Result, error) {
	info := &ConflictInfo{
		RemoteDeviceName: remote.DeviceName,
		RemoteUpdatedAt:  remote.UpdatedAt,
		RemoteRevision:   remote.Revision,
		DetectedAt:       time.Now(),
	}
	st := r.st
	st.Conflict = info
	st.LastError = "" // 冲突不是错误，是等用户决定
	if err := saveState(r.s.state, st); err != nil {
		return Result{}, err
	}
	return Result{Action: ActionConflict, Conflict: info}, nil
}

// conflictAfterPreconditionFailed 处理 412：重读远端以取得准确的来源信息，
// 读不到就退回已知的那份 —— 冲突本身是确定的，展示信息差一点不影响结论。
func (r *run) conflictAfterPreconditionFailed(prevRemote *Payload) (Result, error) {
	if data, _, err := r.get(); err == nil {
		if fresh, decErr := Decode(data, r.base); decErr == nil {
			return r.conflict(fresh)
		}
	}
	if prevRemote != nil {
		return r.conflict(*prevRemote)
	}
	return r.conflict(Payload{})
}

// commit 写入同步成功后的新状态：清空冲突与错误。
func (r *run) commit(st State) error {
	st.Conflict = nil
	st.LastError = ""
	return saveState(r.s.state, st)
}

// fail 记下失败原因后原样返回错误。
//
// 只改 LastError，保留基线与待裁决的冲突：一次网络失败不该让下次同步
// 对「谁改过」的判断发生偏移。
//
// 状态写入失败时静默忽略：此处返回的必须是原始错误，用一个次要的落库失败
// 把它替换掉，会让用户看到「保存状态失败」而不是真正的「连不上服务器」。
func (r *run) fail(err error) error {
	st := r.st
	st.LastError = err.Error()
	_ = saveState(r.s.state, st)
	return err
}
