package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// CurrentSchemaVersion 是当前可安全恢复的最高数据库版本。现有 user_version=2
	// 表示 items 表的 UNIQUE(feed_id, url) 约束已被移除；版本 1 表示 FTS trigram
	// 迁移已完成；更高版本来自未来客户端，旧客户端不得覆盖打开。
	CurrentSchemaVersion = 2

	// applicationID 是 ASCII "CLIP"。旧数据库该值为 0，仍通过核心表检查兼容；
	// 非零且不同则明确拒绝，避免把别的 SQLite 文件当作 Clip 备份。
	applicationID = 0x434C4950
)

// BackupForCloud 生成只含应用数据的云端快照。
//
// settings 表会被清空并再次 VACUUM，确保 WebDAV 密文、同步基线、代理和窗口尺寸
// 不残留在空闲页里。配置已有独立同步通道，云端数据库恢复时保留目标机器当前设置。
func (s *Store) BackupForCloud(dest string) (int, error) {
	if err := s.BackupTo(dest); err != nil {
		return 0, err
	}

	db, err := sql.Open("sqlite", dest)
	if err != nil {
		return 0, fmt.Errorf("failed to open cloud backup: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA journal_mode=DELETE`); err != nil {
		return 0, fmt.Errorf("failed to prepare cloud backup journal: %w", err)
	}
	if _, err := db.Exec(`DELETE FROM settings`); err != nil {
		return 0, fmt.Errorf("failed to remove local settings from cloud backup: %w", err)
	}
	// DELETE 不会保证旧值从空闲页消失；再次 VACUUM 才能真正清掉凭据密文。
	if _, err := db.Exec(`VACUUM`); err != nil {
		return 0, fmt.Errorf("failed to compact cloud backup: %w", err)
	}
	if err := db.Close(); err != nil {
		return 0, fmt.Errorf("failed to close cloud backup: %w", err)
	}

	version, err := validateClipDBFile(dest)
	if err != nil {
		return 0, fmt.Errorf("failed to validate cloud backup: %w", err)
	}
	return version, nil
}

// StageCloudRestore 校验云端快照、创建当前数据库的本地回滚副本，并把快照暂存为
// 下次启动待恢复数据库。目标机器当前 settings 会写入待恢复库，避免跨机器覆盖代理、
// 窗口尺寸、WebDAV 凭据与同步状态。
func (s *Store) StageCloudRestore(src string) (string, error) {
	if _, err := validateClipDBFile(src); err != nil {
		return "", fmt.Errorf("invalid cloud backup: %w", err)
	}

	settings, err := s.readRawSettings()
	if err != nil {
		return "", err
	}

	rollback := filepath.Join(filepath.Dir(s.dbPath), "clip-before-cloud-restore.db")
	rollbackTemp := rollback + ".tmp"
	_ = os.Remove(rollbackTemp)
	if err := s.BackupTo(rollbackTemp); err != nil {
		return "", fmt.Errorf("failed to create restore rollback: %w", err)
	}
	if _, err := validateClipDBFile(rollbackTemp); err != nil {
		_ = os.Remove(rollbackTemp)
		return "", fmt.Errorf("failed to validate restore rollback: %w", err)
	}
	if err := replaceFile(rollbackTemp, rollback); err != nil {
		return "", fmt.Errorf("failed to publish restore rollback: %w", err)
	}

	pending := s.dbPath + pendingRestoreSuffix
	if err := copyFile(src, pending); err != nil {
		return "", fmt.Errorf("failed to stage cloud restore: %w", err)
	}
	if err := writeRawSettings(pending, settings); err != nil {
		_ = os.Remove(pending)
		return "", err
	}
	if _, err := validateClipDBFile(pending); err != nil {
		_ = os.Remove(pending)
		return "", fmt.Errorf("failed to validate staged cloud restore: %w", err)
	}
	return rollback, nil
}

// SetPendingJSONSetting 在已暂存的云恢复库中更新一个 JSON 设置。仅供恢复流程把
// “已暂存、需重启”的状态写入待恢复库，避免当前库与下次启动换入的库状态分叉。
func (s *Store) SetPendingJSONSetting(key string, value any) error {
	pending := s.dbPath + pendingRestoreSuffix
	if _, err := os.Stat(pending); err != nil {
		return fmt.Errorf("pending restore not found: %w", err)
	}
	return setJSONSettingFile(pending, key, value)
}

// DiscardPendingRestore 删除尚未生效的云恢复文件。用于恢复准备的最后一步失败时
// 回滚，避免下次启动意外应用一份调用方已收到“恢复失败”的数据库。
func (s *Store) DiscardPendingRestore() error {
	err := os.Remove(s.dbPath + pendingRestoreSuffix)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

type rawSetting struct {
	Key   string
	Value string
}

func (s *Store) readRawSettings() ([]rawSetting, error) {
	rows, err := s.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("failed to read local settings before restore: %w", err)
	}
	defer rows.Close()

	var out []rawSetting
	for rows.Next() {
		var row rawSetting
		if err := rows.Scan(&row.Key, &row.Value); err != nil {
			return nil, fmt.Errorf("failed to scan local setting: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read local settings: %w", err)
	}
	return out, nil
}

func writeRawSettings(path string, settings []rawSetting) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("failed to open staged cloud restore: %w", err)
	}
	defer db.Close()

	// 快照继承主库的 WAL 模式。暂存文件只复制主文件，故写入前切到 DELETE，
	// 确保设置变更不会留在一个稍后丢失的 .pending-wal 中。
	if _, err := db.Exec(`PRAGMA journal_mode=DELETE`); err != nil {
		return fmt.Errorf("failed to prepare staged restore journal: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin staged restore settings merge: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM settings`); err != nil {
		return fmt.Errorf("failed to clear staged restore settings: %w", err)
	}
	for _, row := range settings {
		if _, err := tx.Exec(
			`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
			row.Key,
			row.Value,
		); err != nil {
			return fmt.Errorf("failed to preserve setting %q: %w", row.Key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit staged restore settings: %w", err)
	}
	return db.Close()
}

func setJSONSettingFile(path, key string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to encode pending setting %q: %w", key, err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("failed to open pending restore: %w", err)
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA journal_mode=DELETE`); err != nil {
		return fmt.Errorf("failed to prepare pending restore: %w", err)
	}
	if _, err := db.Exec(`
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, key, string(b)); err != nil {
		return fmt.Errorf("failed to update pending setting %q: %w", key, err)
	}
	return db.Close()
}

func validateClipDBFile(path string) (int, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var appID int
	if err := db.QueryRow(`PRAGMA application_id`).Scan(&appID); err != nil {
		return 0, err
	}
	if appID != 0 && appID != applicationID {
		return 0, fmt.Errorf("not a clip database (application id %d)", appID)
	}

	required := []string{"feed_categories", "feeds", "items", "settings"}
	for _, table := range required {
		var found int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`,
			table,
		).Scan(&found); err != nil {
			return 0, err
		}
		if found != 1 {
			return 0, fmt.Errorf("not a clip database (missing %s table)", table)
		}
	}

	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return 0, err
	}
	if version > CurrentSchemaVersion {
		return 0, fmt.Errorf(
			"database schema %d is newer than supported version %d",
			version,
			CurrentSchemaVersion,
		)
	}

	rows, err := db.Query(`PRAGMA quick_check`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return 0, err
		}
		if result != "ok" {
			return 0, fmt.Errorf("database integrity check failed: %s", result)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return version, nil
}

func replaceFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(src, dst)
}
