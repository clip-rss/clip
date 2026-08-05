package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// settings 表是一张通用键值表：全局设置存在 key='app' 单行（见 settings.go），
// 其余功能模块各自占用独立键（如同步状态 key='sync_state'）。
// 本文件提供按键读写 JSON 值的通用入口，免得每个模块重复一遍
// INSERT ... ON CONFLICT 与 marshal 样板。

// GetJSONSetting 按键读取 JSON 值并反序列化到 out。
//
// 返回的 bool 表示该键是否存在：不存在时返回 (false, nil) 且不改动 out，
// 调用方据此走「首次使用」分支，不必把「没有」当错误处理。
//
// ⚠️ 值存在但无法解析时返回错误，不静默当作不存在。二者含义完全不同：
// 「不存在」是正常初始状态，「解析失败」意味着数据损坏或被外部改写，
// 静默退化成零值会让调用方拿着空状态继续跑，可能覆盖掉本该保留的数据。
func (s *Store) GetJSONSetting(key string, out any) (bool, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to get setting %q: %w", key, err)
	}
	if err := json.Unmarshal([]byte(value), out); err != nil {
		return false, fmt.Errorf("failed to decode setting %q: %w", key, err)
	}
	return true, nil
}

// SetJSONSetting 按键写入 JSON 值（整体覆盖单行）。
func (s *Store) SetJSONSetting(key string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to encode setting %q: %w", key, err)
	}
	query := `
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`
	if _, err := s.db.Exec(query, key, string(b)); err != nil {
		return fmt.Errorf("failed to update setting %q: %w", key, err)
	}
	return nil
}

// DeleteSetting 删除指定键；键不存在时返回 nil（幂等）。
func (s *Store) DeleteSetting(key string) error {
	if _, err := s.db.Exec(`DELETE FROM settings WHERE key = ?`, key); err != nil {
		return fmt.Errorf("failed to delete setting %q: %w", key, err)
	}
	return nil
}
