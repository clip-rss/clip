package store

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store 数据库存储层
type Store struct {
	db     *sql.DB
	dbPath string // 数据库文件绝对路径
}

// Begin 开启一个事务。调用方负责 Commit 或 Rollback。
func (s *Store) Begin() (*sql.Tx, error) {
	return s.db.Begin()
}

// pendingRestoreSuffix 暂存的待恢复数据库文件后缀。
// 运行时数据库被占用无法直接覆盖，恢复操作先写入该暂存文件，
// 下次启动时由 applyPendingRestore 换库。
const pendingRestoreSuffix = ".pending"

// dbPathFunc 可在测试中替换的数据库路径函数
var dbPathFunc = getDBPath

// New 创建新的 Store 实例，数据库路径由 dbPathFunc 解析（用户配置目录）。
// 打开前先尝试应用上次会话暂存的数据库恢复（见 applyPendingRestore）。
func New() (*Store, error) {
	dbPath, err := dbPathFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to get db path: %w", err)
	}
	if err := applyPendingRestore(dbPath); err != nil {
		return nil, fmt.Errorf("failed to apply pending restore: %w", err)
	}
	return NewWithPath(dbPath)
}

// NewWithPath 在指定路径创建 Store 实例，便于自定义存储位置与测试。
func NewWithPath(dbPath string) (*Store, error) {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 配置连接池
	db.SetMaxOpenConns(1) // SQLite 推荐单连接
	db.SetMaxIdleConns(1)

	// 启用 WAL 模式和外键约束
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma: %w", err)
		}
	}

	store := &Store{db: db, dbPath: dbPath}

	// 执行数据库迁移
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return store, nil
}

// Close 关闭数据库连接
func (s *Store) Close() error {
	return s.db.Close()
}

// Path 返回数据库文件的绝对路径，供设置面板展示与备份默认目录。
func (s *Store) Path() string {
	return s.dbPath
}

// StageRestore 校验 src 为合法的 Clip 数据库后，将其暂存为待恢复文件，
// 实际换库发生在下次启动（见 applyPendingRestore）。
func (s *Store) StageRestore(src string) error {
	if err := validateClipDB(src); err != nil {
		return fmt.Errorf("invalid backup file: %w", err)
	}
	if err := copyFile(src, s.dbPath+pendingRestoreSuffix); err != nil {
		return fmt.Errorf("failed to stage restore: %w", err)
	}
	return nil
}

// validateClipDB 以只读方式打开 path，确认其为含 feeds 表的 SQLite 数据库。
func validateClipDB(path string) error {
	_, err := validateClipDBFile(path)
	return err
}

// applyPendingRestore 若存在暂存的待恢复数据库（dbPath+".pending"），
// 在打开数据库前用它覆盖现有库，并清理 WAL/SHM 旁文件，最后删除暂存文件。
// 没有暂存文件时为无操作。任一步失败均返回错误以避免半成品库。
func applyPendingRestore(dbPath string) error {
	pending := dbPath + pendingRestoreSuffix
	if _, err := os.Stat(pending); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := copyFile(pending, dbPath); err != nil {
		return err
	}
	// WAL/SHM 旁文件属于旧库，必须移除，否则会与新库内容不一致。
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	return os.Remove(pending)
}

// copyFile 将 src 内容复制到 dst（覆盖写入）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// getDBPath 获取数据库文件路径
func getDBPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "clip", "clip.db"), nil
}

// migrate 执行数据库迁移
func (s *Store) migrate() error {
	schema := `
-- 订阅源分类表
CREATE TABLE IF NOT EXISTS feed_categories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	parent_id INTEGER,
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (parent_id) REFERENCES feed_categories(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_categories_parent ON feed_categories(parent_id);

-- 订阅源表
CREATE TABLE IF NOT EXISTS feeds (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	url TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	description TEXT,
	link TEXT,
	icon TEXT,
	category_id INTEGER,
	update_interval INTEGER NOT NULL DEFAULT 30,
		max_items INTEGER NOT NULL DEFAULT 100,
		last_updated DATETIME,
		last_attempted DATETIME,
		error_count INTEGER NOT NULL DEFAULT 0,
	last_error TEXT,
	status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'paused', 'error')),
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (category_id) REFERENCES feed_categories(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_feeds_category ON feeds(category_id);
CREATE INDEX IF NOT EXISTS idx_feeds_status ON feeds(status);
CREATE INDEX IF NOT EXISTS idx_feeds_last_updated ON feeds(last_updated);

-- 文章条目表
-- 注意：无 UNIQUE(feed_id, url) 约束。去重完全依赖 seen_items 表的指纹键，
-- 因为 digest 型订阅源（如安妮薇看看）的所有条目共享同一链接，该约束会导致
-- 仅第一条被入库。
CREATE TABLE IF NOT EXISTS items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	feed_id INTEGER NOT NULL,
	title TEXT NOT NULL,
	author TEXT,
	published_at DATETIME NOT NULL,
	updated_at DATETIME,
	url TEXT NOT NULL,
	content TEXT,
	summary TEXT,
	enclosure TEXT,
	categories TEXT,
	is_read BOOLEAN NOT NULL DEFAULT 0,
	is_starred BOOLEAN NOT NULL DEFAULT 0,
	read_at DATETIME,
	note TEXT,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_items_feed ON items(feed_id);
CREATE INDEX IF NOT EXISTS idx_items_published ON items(published_at DESC);
CREATE INDEX IF NOT EXISTS idx_items_read ON items(is_read);
CREATE INDEX IF NOT EXISTS idx_items_starred ON items(is_starred);

-- 永久记录已见过的文章标识；文章因 max_items 被清理后也不会再次被当作新文章。
CREATE TABLE IF NOT EXISTS seen_items (
	feed_id INTEGER NOT NULL,
	item_key TEXT NOT NULL,
	seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (feed_id, item_key),
	FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE
);

-- FTS5 全文搜索索引（文章标题、摘要、笔记）
-- 使用 trigram 分词器：英文不区分大小写，且支持中文子串匹配（≥3 字符）；
-- 1~2 字短词由查询层用 LIKE 兜底（见 items.go: SearchItems）。
CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(
	title,
	summary,
	note,
	content='items',
	content_rowid='id',
	tokenize='trigram'
);

-- FTS5 触发器：插入
CREATE TRIGGER IF NOT EXISTS items_fts_insert AFTER INSERT ON items BEGIN
	INSERT INTO items_fts(rowid, title, summary, note)
	VALUES (new.id, new.title, new.summary, new.note);
END;

-- FTS5 触发器：更新（外部内容表需先删除旧索引再插入新索引）
CREATE TRIGGER IF NOT EXISTS items_fts_update AFTER UPDATE ON items BEGIN
	INSERT INTO items_fts(items_fts, rowid, title, summary, note)
	VALUES ('delete', old.id, old.title, old.summary, old.note);
	INSERT INTO items_fts(rowid, title, summary, note)
	VALUES (new.id, new.title, new.summary, new.note);
END;

-- FTS5 触发器：删除（外部内容表需使用 'delete' 命令）
CREATE TRIGGER IF NOT EXISTS items_fts_delete AFTER DELETE ON items BEGIN
	INSERT INTO items_fts(items_fts, rowid, title, summary, note)
	VALUES ('delete', old.id, old.title, old.summary, old.note);
END;

-- 应用设置表（键值对，全局设置以 JSON 存于单行）
CREATE TABLE IF NOT EXISTS settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	if _, err := s.db.Exec(`PRAGMA application_id = 1129072976`); err != nil {
		return fmt.Errorf("failed to set application id: %w", err)
	}

	if err := s.migrateSchedulingMetadata(); err != nil {
		return err
	}
	if err := s.migrateFTSTokenizer(); err != nil {
		return err
	}
	return s.migrateDropFeedURLConstraint()
}

// migrateSchedulingMetadata 为旧数据库补充抓取尝试时间与已见文章表。
// 不使用 user_version 门控，避免与独立的 FTS 迁移相互耦合。
func (s *Store) migrateSchedulingMetadata() error {
	rows, err := s.db.Query(`PRAGMA table_info(feeds)`)
	if err != nil {
		return fmt.Errorf("failed to inspect feeds schema: %w", err)
	}
	hasLastAttempted := false
	for rows.Next() {
		var cid, notNull, pk int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan feeds schema: %w", err)
		}
		if name == "last_attempted" {
			hasLastAttempted = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !hasLastAttempted {
		if _, err := s.db.Exec(`ALTER TABLE feeds ADD COLUMN last_attempted DATETIME`); err != nil {
			return fmt.Errorf("failed to add feeds.last_attempted: %w", err)
		}
	}

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS seen_items (
			feed_id INTEGER NOT NULL,
			item_key TEXT NOT NULL,
			seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (feed_id, item_key),
			FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE
		)`,
		`UPDATE feeds SET last_attempted = last_updated
		 WHERE last_attempted IS NULL AND last_updated IS NOT NULL`,
		`INSERT OR IGNORE INTO seen_items (feed_id, item_key)
		 SELECT feed_id, 'url:' || url FROM items`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("failed to migrate scheduling metadata: %w", err)
		}
	}
	return nil
}

// migrateFTSTokenizer 将既有用户库的 FTS 索引从旧分词器（porter unicode61）
// 重建为 trigram，以支持中文子串搜索。通过 PRAGMA user_version 做一次性门控。
//
// 新库：上面的 CREATE ... IF NOT EXISTS 已用 trigram 建好，这里仅把版本推进到 1，
// 不会真正重建。旧库：DROP 重建 items_fts 与触发器，再从 items 回填索引。
func (s *Store) migrateFTSTokenizer() error {
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("failed to read user_version: %w", err)
	}
	if version >= 1 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin fts migration: %w", err)
	}
	defer tx.Rollback()

	// 重建 FTS 表与触发器为 trigram 分词器。旧库此处会真正切换分词器；
	// 新库 items_fts 已是 trigram，DROP/CREATE 等价于无害重建。
	stmts := []string{
		`DROP TRIGGER IF EXISTS items_fts_insert`,
		`DROP TRIGGER IF EXISTS items_fts_update`,
		`DROP TRIGGER IF EXISTS items_fts_delete`,
		`DROP TABLE IF EXISTS items_fts`,
		`CREATE VIRTUAL TABLE items_fts USING fts5(
			title, summary, note,
			content='items', content_rowid='id',
			tokenize='trigram'
		)`,
		`CREATE TRIGGER items_fts_insert AFTER INSERT ON items BEGIN
			INSERT INTO items_fts(rowid, title, summary, note)
			VALUES (new.id, new.title, new.summary, new.note);
		END`,
		`CREATE TRIGGER items_fts_update AFTER UPDATE ON items BEGIN
			INSERT INTO items_fts(items_fts, rowid, title, summary, note)
			VALUES ('delete', old.id, old.title, old.summary, old.note);
			INSERT INTO items_fts(rowid, title, summary, note)
			VALUES (new.id, new.title, new.summary, new.note);
		END`,
		`CREATE TRIGGER items_fts_delete AFTER DELETE ON items BEGIN
			INSERT INTO items_fts(items_fts, rowid, title, summary, note)
			VALUES ('delete', old.id, old.title, old.summary, old.note);
		END`,
		// 从外部内容表 items 重建全部索引。
		`INSERT INTO items_fts(items_fts) VALUES('rebuild')`,
		`PRAGMA user_version = 1`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("fts migration step failed (%.40s...): %w", stmt, err)
		}
	}

	return tx.Commit()
}


// migrateDropFeedURLConstraint removes the UNIQUE(feed_id, url) constraint from items.
//
// Digest format feeds (e.g. Anyway.Now) have all items sharing the same URL,
// so this constraint causes only the first item to be stored. Dedup is already
// handled by seen_items with fingerprint keys.
//
// SQLite does not support ALTER TABLE DROP CONSTRAINT, so we recreate the table.
// The FTS5 external content table references items by name, which stays the same.
// Gated on user_version < 2.
func (s *Store) migrateDropFeedURLConstraint() error {
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("failed to read user_version: %w", err)
	}
	if version >= 2 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin drop-constraint migration: %w", err)
	}
	defer tx.Rollback()

	stmts := []string{
		`PRAGMA foreign_keys=OFF`,
		`CREATE TABLE items_v2 (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feed_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			author TEXT,
			published_at DATETIME NOT NULL,
			updated_at DATETIME,
			url TEXT NOT NULL,
			content TEXT,
			summary TEXT,
			enclosure TEXT,
			categories TEXT,
			is_read BOOLEAN NOT NULL DEFAULT 0,
			is_starred BOOLEAN NOT NULL DEFAULT 0,
			read_at DATETIME,
			note TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE
		)`,
		`INSERT INTO items_v2 SELECT * FROM items`,
		`DROP TABLE items`,
		`ALTER TABLE items_v2 RENAME TO items`,
		`CREATE INDEX IF NOT EXISTS idx_items_feed ON items(feed_id)`,
		`CREATE INDEX IF NOT EXISTS idx_items_published ON items(published_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_items_read ON items(is_read)`,
		`CREATE INDEX IF NOT EXISTS idx_items_starred ON items(is_starred)`,
		`CREATE TRIGGER IF NOT EXISTS items_fts_insert AFTER INSERT ON items BEGIN
			INSERT INTO items_fts(rowid, title, summary, note)
			VALUES (new.id, new.title, new.summary, new.note);
		END`,
		`CREATE TRIGGER IF NOT EXISTS items_fts_update AFTER UPDATE ON items BEGIN
			INSERT INTO items_fts(items_fts, rowid, title, summary, note)
			VALUES ('delete', old.id, old.title, old.summary, old.note);
			INSERT INTO items_fts(rowid, title, summary, note)
			VALUES (new.id, new.title, new.summary, new.note);
		END`,
		`CREATE TRIGGER IF NOT EXISTS items_fts_delete AFTER DELETE ON items BEGIN
			INSERT INTO items_fts(items_fts, rowid, title, summary, note)
			VALUES ('delete', old.id, old.title, old.summary, old.note);
		END`,
		// Rebuild FTS index: the external content table's schema is preserved
		// through DROP/RENAME, but the FTS index needs explicit rebuild.
		`INSERT INTO items_fts(items_fts) VALUES('rebuild')`,
		`PRAGMA foreign_key_check`,
		`PRAGMA foreign_keys=ON`,
		`PRAGMA user_version = 2`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("drop-constraint migration step failed (%.60s...): %w", stmt, err)
		}
	}

	return tx.Commit()
}
