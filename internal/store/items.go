package store

import (
	"database/sql"
	"fmt"
)

// CreateItem 创建新文章条目
func (s *Store) CreateItem(item *Item) error {
	query := `
		INSERT INTO items (feed_id, title, author, published_at, updated_at, url, content, summary,
		                   enclosure, categories, is_read, is_starred, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := s.db.Exec(query,
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
		return fmt.Errorf("failed to create item: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get item id: %w", err)
	}

	item.ID = id
	return nil
}

// CreateItemIfNotExists 如果文章不存在则创建（根据 feed_id + url 判重）
func (s *Store) CreateItemIfNotExists(item *Item) (bool, error) {
	// 先检查是否存在
	var existingID int64
	checkQuery := `SELECT id FROM items WHERE feed_id = ? AND url = ?`
	err := s.db.QueryRow(checkQuery, item.FeedID, item.URL).Scan(&existingID)

	if err == nil {
		// 已存在，不插入
		item.ID = existingID
		return false, nil
	}

	if err != sql.ErrNoRows {
		return false, fmt.Errorf("failed to check item existence: %w", err)
	}

	// 不存在，插入
	if err := s.CreateItem(item); err != nil {
		return false, err
	}
	return true, nil
}

// GetItem 根据 ID 获取文章
func (s *Store) GetItem(id int64) (*Item, error) {
	query := `
		SELECT id, feed_id, title, author, published_at, updated_at, url, content, summary,
		       enclosure, categories, is_read, is_starred, read_at, note, created_at
		FROM items WHERE id = ?
	`
	item := &Item{}
	err := s.db.QueryRow(query, id).Scan(
		&item.ID,
		&item.FeedID,
		&item.Title,
		&item.Author,
		&item.PublishedAt,
		&item.UpdatedAt,
		&item.URL,
		&item.Content,
		&item.Summary,
		&item.Enclosure,
		&item.Categories,
		&item.IsRead,
		&item.IsStarred,
		&item.ReadAt,
		&item.Note,
		&item.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("item not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}
	return item, nil
}

// ListItemsByFeed 获取指定订阅源的文章列表
func (s *Store) ListItemsByFeed(feedID int64, limit, offset int) ([]Item, error) {
	query := `
		SELECT id, feed_id, title, author, published_at, updated_at, url, content, summary,
		       enclosure, categories, is_read, is_starred, read_at, note, created_at
		FROM items
		WHERE feed_id = ?
		ORDER BY published_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(query, feedID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list items: %w", err)
	}
	defer rows.Close()

	return s.scanItems(rows)
}

// ListAllItems 获取所有文章列表
func (s *Store) ListAllItems(limit, offset int) ([]Item, error) {
	query := `
		SELECT id, feed_id, title, author, published_at, updated_at, url, content, summary,
		       enclosure, categories, is_read, is_starred, read_at, note, created_at
		FROM items
		ORDER BY published_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list all items: %w", err)
	}
	defer rows.Close()

	return s.scanItems(rows)
}

// ListUnreadItems 获取未读文章列表
func (s *Store) ListUnreadItems(limit, offset int) ([]Item, error) {
	query := `
		SELECT id, feed_id, title, author, published_at, updated_at, url, content, summary,
		       enclosure, categories, is_read, is_starred, read_at, note, created_at
		FROM items
		WHERE is_read = 0
		ORDER BY published_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list unread items: %w", err)
	}
	defer rows.Close()

	return s.scanItems(rows)
}

// ListStarredItems 获取星标文章列表
func (s *Store) ListStarredItems(limit, offset int) ([]Item, error) {
	query := `
		SELECT id, feed_id, title, author, published_at, updated_at, url, content, summary,
		       enclosure, categories, is_read, is_starred, read_at, note, created_at
		FROM items
		WHERE is_starred = 1
		ORDER BY published_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list starred items: %w", err)
	}
	defer rows.Close()

	return s.scanItems(rows)
}

// UpdateItem 更新文章
func (s *Store) UpdateItem(item *Item) error {
	query := `
		UPDATE items
		SET title = ?, author = ?, published_at = ?, updated_at = ?, content = ?, summary = ?,
		    enclosure = ?, categories = ?, is_read = ?, is_starred = ?, read_at = ?, note = ?
		WHERE id = ?
	`
	result, err := s.db.Exec(query,
		item.Title,
		item.Author,
		item.PublishedAt,
		item.UpdatedAt,
		item.Content,
		item.Summary,
		item.Enclosure,
		item.Categories,
		item.IsRead,
		item.IsStarred,
		item.ReadAt,
		item.Note,
		item.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("item not found")
	}

	return nil
}

// DeleteItem 删除文章
func (s *Store) DeleteItem(id int64) error {
	query := `DELETE FROM items WHERE id = ?`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("item not found")
	}

	return nil
}

// MarkItemAsRead 标记文章为已读
func (s *Store) MarkItemAsRead(id int64) error {
	query := `UPDATE items SET is_read = 1, read_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to mark item as read: %w", err)
	}
	return nil
}

// MarkItemAsUnread 标记文章为未读
func (s *Store) MarkItemAsUnread(id int64) error {
	query := `UPDATE items SET is_read = 0, read_at = NULL WHERE id = ?`
	_, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to mark item as unread: %w", err)
	}
	return nil
}

// MarkItemsAsRead 批量标记文章为已读
func (s *Store) MarkItemsAsRead(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE items SET is_read = 1, read_at = CURRENT_TIMESTAMP WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			return fmt.Errorf("failed to mark item %d as read: %w", id, err)
		}
	}

	return tx.Commit()
}

// MarkItemsAsUnread 批量标记文章为未读
func (s *Store) MarkItemsAsUnread(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE items SET is_read = 0, read_at = NULL WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			return fmt.Errorf("failed to mark item %d as unread: %w", id, err)
		}
	}

	return tx.Commit()
}

// MarkAllAsReadByFeed 标记指定订阅源的所有文章为已读
func (s *Store) MarkAllAsReadByFeed(feedID int64) error {
	query := `UPDATE items SET is_read = 1, read_at = CURRENT_TIMESTAMP WHERE feed_id = ? AND is_read = 0`
	_, err := s.db.Exec(query, feedID)
	if err != nil {
		return fmt.Errorf("failed to mark all items as read: %w", err)
	}
	return nil
}

// ToggleItemStar 切换文章星标状态
func (s *Store) ToggleItemStar(id int64) error {
	query := `UPDATE items SET is_starred = NOT is_starred WHERE id = ?`
	_, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to toggle item star: %w", err)
	}
	return nil
}

// UpdateItemNote 更新文章笔记
func (s *Store) UpdateItemNote(id int64, note string) error {
	query := `UPDATE items SET note = ? WHERE id = ?`
	_, err := s.db.Exec(query, note, id)
	if err != nil {
		return fmt.Errorf("failed to update item note: %w", err)
	}
	return nil
}

// SearchItems 全文搜索文章（使用 FTS5）
func (s *Store) SearchItems(keyword string, limit, offset int) ([]Item, error) {
	query := `
		SELECT i.id, i.feed_id, i.title, i.author, i.published_at, i.updated_at, i.url,
		       i.content, i.summary, i.enclosure, i.categories, i.is_read, i.is_starred,
		       i.read_at, i.note, i.created_at
		FROM items i
		INNER JOIN items_fts fts ON i.id = fts.rowid
		WHERE items_fts MATCH ?
		ORDER BY rank
		LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(query, keyword, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to search items: %w", err)
	}
	defer rows.Close()

	return s.scanItems(rows)
}

// CleanupOldItems 清理旧文章（保留每个订阅源的 maxItems 条）
func (s *Store) CleanupOldItems(feedID int64, maxItems int) error {
	query := `
		DELETE FROM items
		WHERE feed_id = ?
		  AND id NOT IN (
		      SELECT id FROM items
		      WHERE feed_id = ?
		      ORDER BY published_at DESC
		      LIMIT ?
		  )
	`
	_, err := s.db.Exec(query, feedID, feedID, maxItems)
	if err != nil {
		return fmt.Errorf("failed to cleanup old items: %w", err)
	}
	return nil
}

// GetUnreadCount 获取未读文章总数
func (s *Store) GetUnreadCount() (int, error) {
	query := `SELECT COUNT(*) FROM items WHERE is_read = 0`
	var count int
	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}
	return count, nil
}

// GetUnreadCountByFeed 获取指定订阅源的未读文章数
func (s *Store) GetUnreadCountByFeed(feedID int64) (int, error) {
	query := `SELECT COUNT(*) FROM items WHERE feed_id = ? AND is_read = 0`
	var count int
	err := s.db.QueryRow(query, feedID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count by feed: %w", err)
	}
	return count, nil
}

// scanItems 辅助函数：扫描文章行
func (s *Store) scanItems(rows *sql.Rows) ([]Item, error) {
	items := []Item{}
	for rows.Next() {
		item := Item{}
		err := rows.Scan(
			&item.ID,
			&item.FeedID,
			&item.Title,
			&item.Author,
			&item.PublishedAt,
			&item.UpdatedAt,
			&item.URL,
			&item.Content,
			&item.Summary,
			&item.Enclosure,
			&item.Categories,
			&item.IsRead,
			&item.IsStarred,
			&item.ReadAt,
			&item.Note,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
