package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// RecordFeedFailure 原子记录一次失败的抓取尝试，供调度器计算下一次退避时间。
func (s *Store) RecordFeedFailure(id int64, attemptedAt time.Time, errMsg string) error {
	result, err := s.db.Exec(`
		UPDATE feeds
		SET last_attempted = ?, error_count = error_count + 1,
		    last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, attemptedAt, errMsg, id)
	if err != nil {
		return fmt.Errorf("failed to record feed failure: %w", err)
	}
	return requireAffectedFeed(result)
}

// MarkFeedNotModified 原子完成一次 304 检查并清除之前的错误状态。
func (s *Store) MarkFeedNotModified(id int64, checkedAt time.Time) error {
	result, err := s.db.Exec(`
		UPDATE feeds
		SET last_updated = ?, last_attempted = ?, error_count = 0,
		    last_error = '', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, checkedAt, checkedAt, id)
	if err != nil {
		return fmt.Errorf("failed to mark feed not modified: %w", err)
	}
	return requireAffectedFeed(result)
}

// ApplyFeedRefresh 在一个事务内完成文章去重插入、数量清理及 Feed 成功状态更新。
// 返回本次新建且清理后仍保留的文章，供事件和系统通知使用。
func (s *Store) ApplyFeedRefresh(
	feedID int64,
	checkedAt time.Time,
	items []RefreshItem,
	maxItems int,
) ([]Item, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin feed refresh: %w", err)
	}
	defer tx.Rollback()

	created := make([]Item, 0, len(items))
	for _, candidate := range items {
		if candidate.Item == nil {
			continue
		}
		keys := uniqueKeys(candidate.Keys)
		if len(keys) == 0 {
			continue
		}

		seen, err := hasSeenItem(tx, feedID, keys)
		if err != nil {
			return nil, err
		}
		if err := rememberItemKeys(tx, feedID, keys); err != nil {
			return nil, err
		}
		if seen {
			continue
		}

		inserted, err := insertRefreshItem(tx, candidate.Item)
		if err != nil {
			return nil, err
		}
		if inserted {
			created = append(created, *candidate.Item)
		}
	}

	if maxItems > 0 {
		if _, err := tx.Exec(`
			DELETE FROM items
			WHERE feed_id = ?
			  AND id NOT IN (
			      SELECT id FROM items
			      WHERE feed_id = ?
			      ORDER BY published_at DESC, id DESC
			      LIMIT ?
			  )
		`, feedID, feedID, maxItems); err != nil {
			return nil, fmt.Errorf("failed to cleanup old items: %w", err)
		}
	}

	result, err := tx.Exec(`
		UPDATE feeds
		SET last_updated = ?, last_attempted = ?, error_count = 0,
		    last_error = '', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, checkedAt, checkedAt, feedID)
	if err != nil {
		return nil, fmt.Errorf("failed to complete feed refresh: %w", err)
	}
	if err := requireAffectedFeed(result); err != nil {
		return nil, err
	}

	created, err = retainedCreatedItems(tx, feedID, created, maxItems)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit feed refresh: %w", err)
	}
	return created, nil
}

// hasSeenItem 检查某篇文章此前是否已经入库。
//
// 优先使用 source:（指纹）键做决定性判定：只有 source: 键匹配时才认为 "已见"。
// 仅当条目没有 source: 键（即缺少 GUID 且无法计算指纹）时，才退而使用 url: 键匹配。
// 这避免了 digest 型订阅源（所有条目共享同一链接）被误判为重复。
func hasSeenItem(tx *sql.Tx, feedID int64, keys []string) (bool, error) {
	// 第一遍：查 source: 键（指纹）。
	for _, key := range keys {
		if !strings.HasPrefix(key, "source:") {
			continue
		}
		seen, err := checkKey(tx, feedID, key)
		if err != nil {
			return false, err
		}
		if seen {
			return true, nil
		}
	}
	// 第二遍仅当条目没有任何 source: 键时，才回退到 url: 键。
	hasSource := false
	for _, key := range keys {
		if strings.HasPrefix(key, "source:") {
			hasSource = true
			break
		}
	}
	if !hasSource {
		for _, key := range keys {
			if !strings.HasPrefix(key, "url:") {
				continue
			}
			seen, err := checkKey(tx, feedID, key)
			if err != nil {
				return false, err
			}
			if seen {
				return true, nil
			}
		}
	}
	return false, nil
}

func checkKey(tx *sql.Tx, feedID int64, key string) (bool, error) {
	var one int
	err := tx.QueryRow(
		`SELECT 1 FROM seen_items WHERE feed_id = ? AND item_key = ?`,
		feedID,
		key,
	).Scan(&one)
	if err == nil {
		return true, nil
	}
	if err != sql.ErrNoRows {
		return false, fmt.Errorf("failed to check seen item: %w", err)
	}
	return false, nil
}

func rememberItemKeys(tx *sql.Tx, feedID int64, keys []string) error {
	for _, key := range keys {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO seen_items (feed_id, item_key) VALUES (?, ?)`,
			feedID,
			key,
		); err != nil {
			return fmt.Errorf("failed to remember seen item: %w", err)
		}
	}
	return nil
}

func insertRefreshItem(tx *sql.Tx, item *Item) (bool, error) {
	result, err := tx.Exec(`
		INSERT INTO items (feed_id, title, author, published_at, updated_at, url, content, summary,
		                   enclosure, categories, is_read, is_starred, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		item.FeedID,
		item.Title,
		item.Author,
		item.PublishedAt,
		item.UpdatedAt,
		item.URL,
		item.Content,
		item.Summary,
		item.Enclosure,
		item.Categories,
		item.IsRead,
		item.IsStarred,
		item.Note,
	)
	if err != nil {
		return false, fmt.Errorf("failed to insert refreshed item: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to read refreshed item result: %w", err)
	}
	if rows == 0 {
		return false, nil
	}
	id, err := result.LastInsertId()
	if err != nil {
		return false, fmt.Errorf("failed to get refreshed item id: %w", err)
	}
	item.ID = id
	return true, nil
}

func retainedCreatedItems(tx *sql.Tx, feedID int64, created []Item, maxItems int) ([]Item, error) {
	if len(created) == 0 || maxItems <= 0 {
		return created, nil
	}
	rows, err := tx.Query(`SELECT id FROM items WHERE feed_id = ?`, feedID)
	if err != nil {
		return nil, fmt.Errorf("failed to list retained items: %w", err)
	}
	defer rows.Close()
	retained := make(map[int64]struct{}, maxItems)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan retained item: %w", err)
		}
		retained[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]Item, 0, len(created))
	for _, item := range created {
		if _, ok := retained[item.ID]; ok {
			out = append(out, item)
		}
	}
	return out, nil
}

func uniqueKeys(keys []string) []string {
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func requireAffectedFeed(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read affected feed: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("feed not found")
	}
	return nil
}
