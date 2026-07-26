package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
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
	ShowUnreadBadge       bool   `json:"showUnreadBadge"`       // 是否展示未读角标：macOS 在 Dock 显示数字，Windows 在任务栏显示红点
	AutoMarkReadDelay     int    `json:"autoMarkReadDelay"`     // 自动标记已读延迟（毫秒）：-1 关闭 / 0 立即 / >0 延迟
	LaunchMinimized       bool   `json:"launchMinimized"`       // 启动时最小化窗口
	WindowWidth           int    `json:"windowWidth"`           // 主窗口上次关闭时的宽度
	WindowHeight          int    `json:"windowHeight"`          // 主窗口上次关闭时的高度
	ProxyHost             string `json:"proxyHost"`             // HTTP 代理 IP / 主机名
	ProxyPort             int    `json:"proxyPort"`             // HTTP 代理端口
}

// DefaultSettings 返回出厂默认设置。
func DefaultSettings() Settings {
	return Settings{
		Theme:                 "system",
		Language:              detectDefaultLanguage(),
		DefaultUpdateInterval: 30,
		DefaultMaxItems:       100,
		NotificationMode:      NotifyEach,
		ShowUnreadBadge:       true,
		AutoMarkReadDelay:     0,
		LaunchMinimized:       false,
		WindowWidth:           1200,
		WindowHeight:          800,
	}
}

// GetSettings 读取全局设置，未持久化或解析失败时回退到默认值。
// 以默认值为基底反序列化，保证新增字段对旧数据有合理缺省。
func (s *Store) GetSettings() (Settings, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, settingsKey).Scan(&value)
	if err == sql.ErrNoRows {
		defaults := DefaultSettings()
		if err := s.UpdateSettings(defaults); err != nil {
			return Settings{}, err
		}
		return defaults, nil
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

// CacheStats 缓存统计：可清理的文章条数及预计可释放空间（字节）。
type CacheStats struct {
	CacheCount  int64 `json:"cacheCount"`  // 可清理文章数（已读且未星标）
	EstimatedBytes int64 `json:"estimatedBytes"` // 预计可释放字节数
}

// GetCacheStats 统计可清理缓存的规模：文章数 + 按比例估算的可释放字节数。
//
// 估算方法：可清理条数 / 全部条数 × 数据库文件大小。
// 该估算偏保守（content 字段是大头，已读文章的 content 占比通常更高），
// 但不需要逐行 sizeof，足够用于展示"预计可释放 xx MB"。
func (s *Store) GetCacheStats() (CacheStats, error) {
	var cacheCount, totalCount int64
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM items WHERE is_read = 1 AND is_starred = 0`,
	).Scan(&cacheCount); err != nil {
		return CacheStats{}, fmt.Errorf("failed to count cache items: %w", err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&totalCount); err != nil {
		return CacheStats{}, fmt.Errorf("failed to count total items: %w", err)
	}

	var estimatedBytes int64
	if totalCount > 0 {
		if fi, err := os.Stat(s.dbPath); err == nil {
			estimatedBytes = fi.Size() * cacheCount / totalCount
		}
	}
	return CacheStats{CacheCount: cacheCount, EstimatedBytes: estimatedBytes}, nil
}

// PruneReadItems 清理缓存：删除已读且未收藏的文章，随后 VACUUM 回收空间。
// 返回被删除的文章数。未读与收藏的文章一律保留。
func (s *Store) PruneReadItems() (int64, error) {
	res, err := s.db.Exec(`DELETE FROM items WHERE is_read = 1 AND is_starred = 0`)
	if err != nil {
		return 0, fmt.Errorf("failed to prune read items: %w", err)
	}
	removed, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to read prune count: %w", err)
	}
	// VACUUM 不能在事务中执行；删除后单独回收空间。
	if _, err := s.db.Exec(`VACUUM`); err != nil {
		return removed, fmt.Errorf("failed to vacuum: %w", err)
	}
	return removed, nil
}

// BackupTo 将当前数据库一致性备份到 dest 路径。
// 使用 VACUUM INTO，生成一份合并了 WAL 的干净副本，可独立打开。
func (s *Store) BackupTo(dest string) error {
	if _, err := s.db.Exec(`VACUUM INTO ?`, dest); err != nil {
		return fmt.Errorf("failed to backup database: %w", err)
	}
	return nil
}
