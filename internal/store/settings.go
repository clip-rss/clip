package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// settingsKey 全局应用设置在 settings 表中的固定键名。
const settingsKey = "app"

// 通知模式取值。
const (
	NotifyEach    = "each"    // 每篇新文章一条通知
	NotifySummary = "summary" // 每次刷新一条摘要通知
	NotifyOff     = "off"     // 关闭通知
)

// Settings 应用全局设置。
type Settings struct {
	Theme                 string `json:"theme"`                 // system / light / dark
	Language              string `json:"language"`              // zh / en
	DefaultUpdateInterval int    `json:"defaultUpdateInterval"` // 默认更新间隔（分钟）
	DefaultMaxItems       int    `json:"defaultMaxItems"`       // 默认每源最大保留条目数
	NotificationMode      string `json:"notificationMode"`      // each / summary / off
}

// DefaultSettings 返回出厂默认设置。
func DefaultSettings() Settings {
	return Settings{
		Theme:                 "system",
		Language:              "zh",
		DefaultUpdateInterval: 30,
		DefaultMaxItems:       100,
		NotificationMode:      NotifyEach,
	}
}

// GetSettings 读取全局设置，未持久化或解析失败时回退到默认值。
// 以默认值为基底反序列化，保证新增字段对旧数据有合理缺省。
func (s *Store) GetSettings() (Settings, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, settingsKey).Scan(&value)
	if err == sql.ErrNoRows {
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("failed to get settings: %w", err)
	}

	out := DefaultSettings()
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return DefaultSettings(), nil
	}
	return out, nil
}

// UpdateSettings 持久化全局设置（整体覆盖写入单行）。
func (s *Store) UpdateSettings(settings Settings) error {
	b, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to encode settings: %w", err)
	}
	query := `
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`
	if _, err := s.db.Exec(query, settingsKey, string(b)); err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}
	return nil
}
