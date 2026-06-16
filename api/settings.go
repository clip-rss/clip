package api

import "github.com/clip-rss/clip/internal/store"

// SettingsService 应用设置相关的绑定方法。
type SettingsService struct {
	store *store.Store
}

// NewSettingsService 创建 SettingsService。
func NewSettingsService(st *store.Store) *SettingsService {
	return &SettingsService{store: st}
}

// GetSettings 读取全局设置（未持久化时返回默认值）。
func (s *SettingsService) GetSettings() (store.Settings, error) {
	return s.store.GetSettings()
}

// UpdateSettings 保存全局设置。
func (s *SettingsService) UpdateSettings(settings store.Settings) error {
	return s.store.UpdateSettings(settings)
}
