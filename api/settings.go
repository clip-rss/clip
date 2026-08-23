package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/clip-rss/clip/internal/i18n"
	"github.com/clip-rss/clip/internal/scheduler"
	"github.com/clip-rss/clip/internal/store"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// SettingsService 应用设置相关的绑定方法。
type SettingsService struct {
	store *store.Store
	sched *scheduler.Scheduler

	// proxyAppliers 是代理设置的下游接收者：抓取用的 fetcher.Client，以及软件更新
	// 用的 updatesrc.Proxy。两者都要收到变更，否则会出现「抓 feed 走代理、下更新包
	// 不走」这种一半生效的状态。
	proxyAppliers []ProxyApplier

	// onChanged 设置变更后的观察者，由 SyncService 注入以触发 debounce 推送。
	// 为 nil 表示没人关心（未配置同步、或单测里不需要）。
	//
	// 用观察者而不是让 SettingsService 直接依赖 SyncService：后者要依赖前者
	// （拉取得经 UpdateSettings 才能下发间隔到订阅源与调度器），双向依赖会成环。
	//
	// ⚠️ 用接口而非 func()：wails3 generate bindings 会扫描 Service 的全部字段，
	// 遇到函数类型就报「function types are not supported by encoding/json」。
	// 该字段既不导出也不参与序列化，警告纯属噪音，但会出现在每次生成里。
	onChanged SettingsObserver
}

// ProxyApplier 接收代理设置变更的一方。
//
// *fetcher.Client 与 *updatesrc.Proxy 都以这个签名暴露 SetProxy，因此无需适配层。
type ProxyApplier interface {
	SetProxy(host string, port int)
}

// SettingsObserver 关心设置变更的一方。由 SyncService 实现。
//
// 方法有意不导出：同包内未导出方法即可满足接口，而导出方法会被 wails3
// 生成成前端绑定（见 sync.go 生命周期一节的说明）。
type SettingsObserver interface {
	notifySettingsChanged()
}

// NewSettingsService 创建 SettingsService。
//
// appliers 是代理设置的接收者（抓取客户端、更新下载源），可传 nil 值，逐个跳过。
func NewSettingsService(st *store.Store, sch *scheduler.Scheduler, appliers ...ProxyApplier) *SettingsService {
	return &SettingsService{store: st, sched: sch, proxyAppliers: appliers}
}

// ObserveSettings 把 o 注册为 svc 的设置变更观察者。
//
// ⚠️ 写成包级函数而非 SettingsService 的方法，是因为 wails3 会把 Service 上
// **每一个导出方法**都生成成前端可调用的绑定。这是装配期的接线，只该由 main 调用，
// 不该出现在前端 API 里（而且它的接口参数也无法序列化，会一直报警告）。
// 包级函数不参与 Service 绑定，同时仍能访问未导出字段。
//
// 只应在应用装配阶段调用，不是线程安全的。
func ObserveSettings(svc *SettingsService, o SettingsObserver) {
	svc.onChanged = o
}

// GetSettings 读取全局设置（未持久化时返回默认值）。
func (s *SettingsService) GetSettings() (store.Settings, error) {
	return s.store.GetSettings()
}

// UpdateSettings 保存全局设置；间隔变化时同步应用到全部现有订阅源和调度器。
func (s *SettingsService) UpdateSettings(settings store.Settings) error {
	current, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	intervalChanged := current.DefaultUpdateInterval != settings.DefaultUpdateInterval
	if intervalChanged {
		err = s.store.UpdateSettingsAndFeedIntervals(settings)
	} else {
		err = s.store.UpdateSettings(settings)
	}
	if err != nil {
		return err
	}
	if s.sched != nil && settings.DefaultUpdateInterval >= 0 {
		s.sched.SetDefaultInterval(time.Duration(settings.DefaultUpdateInterval) * time.Minute)
	}
	for _, applier := range s.proxyAppliers {
		if applier != nil {
			applier.SetProxy(settings.ProxyHost, settings.ProxyPort)
		}
	}
	// 放在最后：只有设置确实落库并生效了才通知，否则会推送一份没保存成功的配置。
	if s.onChanged != nil {
		s.onChanged.notifySettingsChanged()
	}
	return nil
}

// TestProxy 测试代理连通性：用指定代理请求一个测试 URL，成功返回 nil。
func (s *SettingsService) TestProxy(host string, port int) error {
	if host == "" || port <= 0 {
		return errors.New(i18n.T(backendLanguage(s.store), "proxy.invalid"))
	}
	proxyURL := fmt.Sprintf("http://%s:%d", host, port)
	u, err := url.Parse(proxyURL)
	if err != nil {
		return i18n.Error(backendLanguage(s.store), "proxy.invalidAddress", err)
	}
	transport := &http.Transport{Proxy: http.ProxyURL(u)}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}

	// 使用一个可靠的测试地址检测连通性
	req, _ := http.NewRequest(http.MethodGet, "https://www.google.com", nil)
	resp, err := client.Do(req)
	if err != nil {
		return i18n.Error(backendLanguage(s.store), "proxy.connectionFailed", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return errors.New(i18n.T(backendLanguage(s.store), "proxy.badStatus", resp.Status))
	}
	return nil
}

// DatabasePath 返回数据库文件路径，供设置面板「数据管理」展示。
func (s *SettingsService) DatabasePath() string {
	return s.store.Path()
}

// ClearCache 清理缓存：删除已读且未收藏的文章并回收空间，返回删除条数。
func (s *SettingsService) ClearCache() (int64, error) {
	return s.store.PruneReadItems()
}

// GetCacheStats 返回当前可清理缓存的统计信息（文章数 + 预计可释放字节数）。
func (s *SettingsService) GetCacheStats() (store.CacheStats, error) {
	return s.store.GetCacheStats()
}

// BackupDatabase 弹出保存对话框，让用户选择位置后备份数据库。
// 用户取消时返回 (false, nil)；成功返回 (true, nil)。
func (s *SettingsService) BackupDatabase() (bool, error) {
	app := application.Get()
	if app == nil {
		return false, errors.New(i18n.T(backendLanguage(s.store), "app.unavailable"))
	}
	dest, err := app.Dialog.SaveFile().
		SetMessage(i18n.T(backendLanguage(s.store), "database.backup")).
		SetFilename("clip-backup.db").
		AddFilter(i18n.T(backendLanguage(s.store), "database.fileFilter"), "*.db").
		PromptForSingleSelection()
	if err != nil {
		return false, err
	}
	if dest == "" {
		return false, nil // 用户取消
	}
	if err := s.store.BackupTo(dest); err != nil {
		return false, err
	}
	return true, nil
}

// RestoreDatabase 弹出打开对话框选择备份文件，校验后暂存为待恢复库，
// 实际换库在下次启动生效（前端据此提示用户重启）。
// 用户取消时返回 (false, nil)；暂存成功返回 (true, nil)。
func (s *SettingsService) RestoreDatabase() (bool, error) {
	app := application.Get()
	if app == nil {
		return false, errors.New(i18n.T(backendLanguage(s.store), "app.unavailable"))
	}
	src, err := app.Dialog.OpenFile().
		SetTitle(i18n.T(backendLanguage(s.store), "database.restore")).
		AddFilter(i18n.T(backendLanguage(s.store), "database.fileFilter"), "*.db").
		CanChooseFiles(true).
		PromptForSingleSelection()
	if err != nil {
		return false, err
	}
	if src == "" {
		return false, nil // 用户取消
	}
	if err := s.store.StageRestore(src); err != nil {
		return false, err
	}
	return true, nil
}
