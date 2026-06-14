package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store 数据库存储层
type Store struct {
	db *sql.DB
}

// dbPathFunc 可在测试中替换的数据库路径函数
var dbPathFunc = getDBPath

// New 创建新的 Store 实例
func New() (*Store, error) {
	dbPath, err := dbPathFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to get db path: %w", err)
	}

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

	store := &Store{db: db}

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
	FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE,
	UNIQUE(feed_id, url)
);

CREATE INDEX IF NOT EXISTS idx_items_feed ON items(feed_id);
CREATE INDEX IF NOT EXISTS idx_items_published ON items(published_at DESC);
CREATE INDEX IF NOT EXISTS idx_items_read ON items(is_read);
CREATE INDEX IF NOT EXISTS idx_items_starred ON items(is_starred);

-- FTS5 全文搜索索引（文章标题、摘要、笔记）
CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(
	title,
	summary,
	note,
	content='items',
	content_rowid='id',
	tokenize='porter unicode61'
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
	`

	_, err := s.db.Exec(schema)
	return err
}
