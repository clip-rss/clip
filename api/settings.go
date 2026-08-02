package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/scheduler"
	"github.com/clip-rss/clip/internal/store"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// SettingsService 应用设置相关的绑定方法。
type SettingsService struct {
	store      *store.Store
	sched      *scheduler.Scheduler
	httpClient *fetcher.Client
}

// NewSettingsService 创建 SettingsService。
func NewSettingsService(st *store.Store, sch *scheduler.Scheduler, client *fetcher.Client) *SettingsService {
	return &SettingsService{store: st, sched: sch, httpClient: client}
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
	if s.httpClient != nil {
		s.httpClient.SetProxy(settings.ProxyHost, settings.ProxyPort)
	}
	return nil
}

// TestProxy 测试代理连通性：用指定代理请求一个测试 URL，成功返回 nil。
func (s *SettingsService) TestProxy(host string, port int) error {
	if host == "" || port <= 0 {
		return errors.New("代理地址或端口无效")
	}
	proxyURL := fmt.Sprintf("http://%s:%d", host, port)
	u, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("代理地址格式错误: %w", err)
	}
	transport := &http.Transport{Proxy: http.ProxyURL(u)}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}

	// 使用一个可靠的测试地址检测连通性
	req, _ := http.NewRequest(http.MethodGet, "https://www.google.com", nil)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("代理连接失败: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("代理返回异常状态: %s", resp.Status)
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
		return false, errors.New("application not available")
	}
	dest, err := app.Dialog.SaveFile().
		SetMessage("备份数据库").
		SetFilename("clip-backup.db").
		AddFilter("Clip 数据库", "*.db").
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
		return false, errors.New("application not available")
	}
	src, err := app.Dialog.OpenFile().
		SetTitle("恢复数据库").
		AddFilter("Clip 数据库", "*.db").
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
