package store

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateFeed 创建新订阅源
func (s *Store) CreateFeed(feed *Feed) error {
	query := `
		INSERT INTO feeds (url, title, description, link, icon, category_id, update_interval, max_items, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := s.db.Exec(query,
		feed.URL,
		feed.Title,
		feed.Description,
		feed.Link,
		feed.Icon,
		feed.CategoryID,
		feed.UpdateInterval,
		feed.MaxItems,
		feed.Status,
	)
	if err != nil {
		return fmt.Errorf("failed to create feed: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get feed id: %w", err)
	}

	feed.ID = id
	return nil
}

// GetFeed 根据 ID 获取订阅源
func (s *Store) GetFeed(id int64) (*Feed, error) {
	query := `
		SELECT id, url, title, description, link, icon, category_id, update_interval, max_items,
		       last_updated, error_count, last_error, status, created_at, updated_at
		FROM feeds WHERE id = ?
	`
	feed := &Feed{}
	err := s.db.QueryRow(query, id).Scan(
		&feed.ID,
		&feed.URL,
		&feed.Title,
		&feed.Description,
		&feed.Link,
		&feed.Icon,
		&feed.CategoryID,
		&feed.UpdateInterval,
		&feed.MaxItems,
		&feed.LastUpdated,
		&feed.ErrorCount,
		&feed.LastError,
		&feed.Status,
		&feed.CreatedAt,
		&feed.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("feed not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get feed: %w", err)
	}
	return feed, nil
}

// GetFeedByURL 根据 URL 获取订阅源
func (s *Store) GetFeedByURL(url string) (*Feed, error) {
	query := `
		SELECT id, url, title, description, link, icon, category_id, update_interval, max_items,
		       last_updated, error_count, last_error, status, created_at, updated_at
		FROM feeds WHERE url = ?
	`
	feed := &Feed{}
	err := s.db.QueryRow(query, url).Scan(
		&feed.ID,
		&feed.URL,
		&feed.Title,
		&feed.Description,
		&feed.Link,
		&feed.Icon,
		&feed.CategoryID,
		&feed.UpdateInterval,
		&feed.MaxItems,
		&feed.LastUpdated,
		&feed.ErrorCount,
		&feed.LastError,
		&feed.Status,
		&feed.CreatedAt,
		&feed.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("feed not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get feed: %w", err)
	}
	return feed, nil
}

// ListFeeds 获取所有订阅源
func (s *Store) ListFeeds() ([]Feed, error) {
	query := `
		SELECT id, url, title, description, link, icon, category_id, update_interval, max_items,
		       last_updated, error_count, last_error, status, created_at, updated_at
		FROM feeds ORDER BY created_at DESC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list feeds: %w", err)
	}
	defer rows.Close()

	feeds := []Feed{}
	for rows.Next() {
		feed := Feed{}
		err := rows.Scan(
			&feed.ID,
			&feed.URL,
			&feed.Title,
			&feed.Description,
			&feed.Link,
			&feed.Icon,
			&feed.CategoryID,
			&feed.UpdateInterval,
			&feed.MaxItems,
			&feed.LastUpdated,
			&feed.ErrorCount,
			&feed.LastError,
			&feed.Status,
			&feed.CreatedAt,
			&feed.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan feed: %w", err)
		}
		feeds = append(feeds, feed)
	}
	return feeds, rows.Err()
}

// ListFeedsWithUnread 获取所有订阅源及未读计数
func (s *Store) ListFeedsWithUnread() ([]FeedWithUnread, error) {
	query := `
		SELECT f.id, f.url, f.title, f.description, f.link, f.icon, f.category_id,
		       f.update_interval, f.max_items, f.last_updated, f.error_count, f.last_error,
		       f.status, f.created_at, f.updated_at,
		       COALESCE(COUNT(CASE WHEN i.is_read = 0 THEN 1 END), 0) as unread_count
		FROM feeds f
		LEFT JOIN items i ON f.id = i.feed_id
		GROUP BY f.id
		ORDER BY f.created_at DESC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list feeds with unread: %w", err)
	}
	defer rows.Close()

	feeds := []FeedWithUnread{}
	for rows.Next() {
		fwu := FeedWithUnread{}
		err := rows.Scan(
			&fwu.ID,
			&fwu.URL,
			&fwu.Title,
			&fwu.Description,
			&fwu.Link,
			&fwu.Icon,
			&fwu.CategoryID,
			&fwu.UpdateInterval,
			&fwu.MaxItems,
			&fwu.LastUpdated,
			&fwu.ErrorCount,
			&fwu.LastError,
			&fwu.Status,
			&fwu.CreatedAt,
			&fwu.UpdatedAt,
			&fwu.UnreadCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan feed with unread: %w", err)
		}
		feeds = append(feeds, fwu)
	}
	return feeds, rows.Err()
}

// UpdateFeed 更新订阅源
func (s *Store) UpdateFeed(feed *Feed) error {
	query := `
		UPDATE feeds
		SET url = ?, title = ?, description = ?, link = ?, icon = ?, category_id = ?,
		    update_interval = ?, max_items = ?, last_updated = ?, error_count = ?,
		    last_error = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	result, err := s.db.Exec(query,
		feed.URL,
		feed.Title,
		feed.Description,
		feed.Link,
		feed.Icon,
		feed.CategoryID,
		feed.UpdateInterval,
		feed.MaxItems,
		feed.LastUpdated,
		feed.ErrorCount,
		feed.LastError,
		feed.Status,
		feed.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update feed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("feed not found")
	}

	return nil
}

// DeleteFeed 删除订阅源（级联删除文章）
func (s *Store) DeleteFeed(id int64) error {
	query := `DELETE FROM feeds WHERE id = ?`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete feed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("feed not found")
	}

	return nil
}

// UpdateFeedStatus 更新订阅源状态
func (s *Store) UpdateFeedStatus(id int64, status string) error {
	query := `UPDATE feeds SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	result, err := s.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update feed status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("feed not found")
	}

	return nil
}

// UpdateFeedError 记录订阅源错误
func (s *Store) UpdateFeedError(id int64, errMsg string) error {
	query := `
		UPDATE feeds
		SET error_count = error_count + 1, last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	_, err := s.db.Exec(query, errMsg, id)
	if err != nil {
		return fmt.Errorf("failed to update feed error: %w", err)
	}
	return nil
}

// ResetFeedError 重置订阅源错误计数
func (s *Store) ResetFeedError(id int64) error {
	query := `
		UPDATE feeds
		SET error_count = 0, last_error = '', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	_, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to reset feed error: %w", err)
	}
	return nil
}

// UpdateFeedLastUpdated 更新订阅源最后更新时间
func (s *Store) UpdateFeedLastUpdated(id int64, t time.Time) error {
	query := `UPDATE feeds SET last_updated = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := s.db.Exec(query, t, id)
	if err != nil {
		return fmt.Errorf("failed to update feed last_updated: %w", err)
	}
	return nil
}

// GetFeedsForUpdate 获取需要更新的订阅源（根据 update_interval）
func (s *Store) GetFeedsForUpdate() ([]Feed, error) {
	query := `
		SELECT id, url, title, description, link, icon, category_id, update_interval, max_items,
		       last_updated, error_count, last_error, status, created_at, updated_at
		FROM feeds
		WHERE status = 'active'
		  AND (last_updated IS NULL OR datetime(last_updated, '+' || update_interval || ' minutes') <= datetime('now'))
		ORDER BY last_updated ASC NULLS FIRST
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get feeds for update: %w", err)
	}
	defer rows.Close()

	feeds := []Feed{}
	for rows.Next() {
		feed := Feed{}
		err := rows.Scan(
			&feed.ID,
			&feed.URL,
			&feed.Title,
			&feed.Description,
			&feed.Link,
			&feed.Icon,
			&feed.CategoryID,
			&feed.UpdateInterval,
			&feed.MaxItems,
			&feed.LastUpdated,
			&feed.ErrorCount,
			&feed.LastError,
			&feed.Status,
			&feed.CreatedAt,
			&feed.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan feed: %w", err)
		}
		feeds = append(feeds, feed)
	}
	return feeds, rows.Err()
}
