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
//
// ⚠️ 所有字段必须是标量类型：本结构体依赖 Go 的可比较性（测试与变更检测均用 ==
// 直接比较整个结构体）。加入 slice / map / 函数字段会使其不可比较，编译期即失败。
type Settings struct {
	Theme                 string `json:"theme"`                 // system / light / dark / sepia
	Language              string `json:"language"`              // zh / zh-TW / en
	DefaultUpdateInterval int    `json:"defaultUpdateInterval"` // 全局更新间隔（分钟；字段名为兼容旧设置保留）
	DefaultMaxItems       int    `json:"defaultMaxItems"`       // 默认每源最大保留条目数
	NotificationMode      string `json:"notificationMode"`      // each / summary / off
	ShowUnreadBadge       bool   `json:"showUnreadBadge"`       // 是否展示未读角标：macOS 在 Dock 显示数字，Windows 在任务栏显示红点
	AutoMarkReadDelay     int    `json:"autoMarkReadDelay"`     // 自动标记已读延迟（毫秒）：-1 关闭 / 0 立即 / >0 延迟
	ReduceMotion          bool   `json:"reduceMotion"`          // 减弱动画效果（无障碍）
	ShowFocusIndicator    bool   `json:"showFocusIndicator"`    // 显示焦点指示器（无障碍）
	WindowWidth           int    `json:"windowWidth"`           // 主窗口上次关闭时的宽度
	WindowHeight          int    `json:"windowHeight"`          // 主窗口上次关闭时的高度
	ProxyHost             string `json:"proxyHost"`             // HTTP 代理 IP / 主机名
	ProxyPort             int    `json:"proxyPort"`             // HTTP 代理端口

	// 阅读视图排版偏好。原先存于前端 localStorage（clip-reader），
	// 收归后端以便随配置一起备份同步。
	ReaderFontFamily string  `json:"readerFontFamily"` // sans / serif / mono
	ReaderFontSize   int     `json:"readerFontSize"`   // 14 / 16 / 18
	ReaderLineHeight float64 `json:"readerLineHeight"` // 1.5 / 1.8 / 2.0
	ReaderWidth      string  `json:"readerWidth"`      // 640 / 800 / full
	ReaderBackground string  `json:"readerBackground"` // default / light / sepia / dark
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
		ShowFocusIndicator:    false,
		WindowWidth:           1200,
		WindowHeight:          800,
		ReaderFontFamily:      "sans",
		ReaderFontSize:        16,
		ReaderLineHeight:      1.8,
		ReaderWidth:           "640",
		ReaderBackground:      "default",
	}
}

func validGlobalUpdateInterval(interval int) bool {
	switch interval {
	case 0, 30, 60, 120:
		return true
	default:
		return false
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
	if !validGlobalUpdateInterval(out.DefaultUpdateInterval) {
		out.DefaultUpdateInterval = DefaultSettings().DefaultUpdateInterval
		if err := s.UpdateSettingsAndFeedIntervals(out); err != nil {
			return Settings{}, fmt.Errorf("failed to migrate global update interval: %w", err)
		}
	}
	return out, nil
}

// UpdateSettings 持久化全局设置（整体覆盖写入单行）。
func (s *Store) UpdateSettings(settings Settings) error {
	if !validGlobalUpdateInterval(settings.DefaultUpdateInterval) {
		return fmt.Errorf("unsupported global update interval: %d", settings.DefaultUpdateInterval)
	}
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

// UpdateSettingsAndFeedIntervals 在同一事务中保存设置，并把全局更新间隔应用到全部订阅源。
func (s *Store) UpdateSettingsAndFeedIntervals(settings Settings) error {
	if !validGlobalUpdateInterval(settings.DefaultUpdateInterval) {
		return fmt.Errorf("unsupported global update interval: %d", settings.DefaultUpdateInterval)
	}
	b, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to encode settings: %w", err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin global interval update: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, settingsKey, string(b)); err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE feeds
		SET update_interval = ?, updated_at = CURRENT_TIMESTAMP
	`, settings.DefaultUpdateInterval); err != nil {
		return fmt.Errorf("failed to apply global feed interval: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit global interval update: %w", err)
	}
	return nil
}

// CacheStats 缓存统计：可清理的文章条数及预计可释放空间（字节）。
type CacheStats struct {
	CacheCount     int64 `json:"cacheCount"`     // 可清理文章数（已读且未星标）
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
