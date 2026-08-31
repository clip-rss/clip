package store

import (
	"database/sql"
	"fmt"
)

// TxCreateCategory 在事务中创建新分类。
func TxCreateCategory(tx *sql.Tx, category *Category) error {
	query := `
		INSERT INTO feed_categories (name, parent_id, sort_order)
		VALUES (?, ?, ?)
	`
	result, err := tx.Exec(query,
		category.Name,
		category.ParentID,
		category.SortOrder,
	)
	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get category id: %w", err)
	}

	category.ID = id
	return nil
}

// CreateCategory 创建新分类
func (s *Store) CreateCategory(category *Category) error {
	query := `
		INSERT INTO feed_categories (name, parent_id, sort_order)
		VALUES (?, ?, ?)
	`
	result, err := s.db.Exec(query,
		category.Name,
		category.ParentID,
		category.SortOrder,
	)
	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get category id: %w", err)
	}

	category.ID = id
	return nil
}

// GetCategory 根据 ID 获取分类
func (s *Store) GetCategory(id int64) (*Category, error) {
	query := `
		SELECT id, name, parent_id, sort_order, created_at, updated_at
		FROM feed_categories WHERE id = ?
	`
	category := &Category{}
	err := s.db.QueryRow(query, id).Scan(
		&category.ID,
		&category.Name,
		&category.ParentID,
		&category.SortOrder,
		&category.CreatedAt,
		&category.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("category not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get category: %w", err)
	}
	return category, nil
}

// ListCategories 获取所有分类（按 sort_order 排序）
func (s *Store) ListCategories() ([]Category, error) {
	query := `
		SELECT id, name, parent_id, sort_order, created_at, updated_at
		FROM feed_categories
		ORDER BY sort_order ASC, created_at ASC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}
	defer rows.Close()

	categories := []Category{}
	for rows.Next() {
		category := Category{}
		err := rows.Scan(
			&category.ID,
			&category.Name,
			&category.ParentID,
			&category.SortOrder,
			&category.CreatedAt,
			&category.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

// ListRootCategories 获取根分类（parent_id 为 NULL）
func (s *Store) ListRootCategories() ([]Category, error) {
	query := `
		SELECT id, name, parent_id, sort_order, created_at, updated_at
		FROM feed_categories
		WHERE parent_id IS NULL
		ORDER BY sort_order ASC, created_at ASC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list root categories: %w", err)
	}
	defer rows.Close()

	categories := []Category{}
	for rows.Next() {
		category := Category{}
		err := rows.Scan(
			&category.ID,
			&category.Name,
			&category.ParentID,
			&category.SortOrder,
			&category.CreatedAt,
			&category.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

// ListChildCategories 获取指定分类的子分类
func (s *Store) ListChildCategories(parentID int64) ([]Category, error) {
	query := `
		SELECT id, name, parent_id, sort_order, created_at, updated_at
		FROM feed_categories
		WHERE parent_id = ?
		ORDER BY sort_order ASC, created_at ASC
	`
	rows, err := s.db.Query(query, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list child categories: %w", err)
	}
	defer rows.Close()

	categories := []Category{}
	for rows.Next() {
		category := Category{}
		err := rows.Scan(
			&category.ID,
			&category.Name,
			&category.ParentID,
			&category.SortOrder,
			&category.CreatedAt,
			&category.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

// UpdateCategory 更新分类
func (s *Store) UpdateCategory(category *Category) error {
	query := `
		UPDATE feed_categories
		SET name = ?, parent_id = ?, sort_order = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	result, err := s.db.Exec(query,
		category.Name,
		category.ParentID,
		category.SortOrder,
		category.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update category: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("category not found")
	}

	return nil
}

// DeleteCategory 删除分类（级联删除子分类，订阅源的 category_id 设为 NULL）
func (s *Store) DeleteCategory(id int64) error {
	query := `DELETE FROM feed_categories WHERE id = ?`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("category not found")
	}

	return nil
}

// GetCategoryWithFeeds 获取分类及其订阅源列表
func (s *Store) GetCategoryWithFeeds(categoryID int64) (*CategoryWithFeeds, error) {
	category, err := s.GetCategory(categoryID)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT f.id, f.url, f.title, f.description, f.link, f.icon, f.category_id,
		       f.update_interval, f.max_items, f.last_updated, f.last_attempted, f.error_count, f.last_error,
		       f.status, f.created_at, f.updated_at,
		       COALESCE(COUNT(CASE WHEN i.is_read = 0 THEN 1 END), 0) as unread_count
		FROM feeds f
		LEFT JOIN items i ON f.id = i.feed_id
		WHERE f.category_id = ?
		GROUP BY f.id
		ORDER BY f.created_at DESC
	`
	rows, err := s.db.Query(query, categoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get feeds for category: %w", err)
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
			&fwu.LastAttempted,
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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &CategoryWithFeeds{
		Category: *category,
		Feeds:    feeds,
	}, nil
}

// GetUncategorizedFeeds 获取未分类的订阅源（category_id 为 NULL）
func (s *Store) GetUncategorizedFeeds() ([]FeedWithUnread, error) {
	query := `
		SELECT f.id, f.url, f.title, f.description, f.link, f.icon, f.category_id,
		       f.update_interval, f.max_items, f.last_updated, f.last_attempted, f.error_count, f.last_error,
		       f.status, f.created_at, f.updated_at,
		       COALESCE(COUNT(CASE WHEN i.is_read = 0 THEN 1 END), 0) as unread_count
		FROM feeds f
		LEFT JOIN items i ON f.id = i.feed_id
		WHERE f.category_id IS NULL
		GROUP BY f.id
		ORDER BY f.created_at DESC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get uncategorized feeds: %w", err)
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
			&fwu.LastAttempted,
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

// MoveFeedToCategory 移动订阅源到指定分类
func (s *Store) MoveFeedToCategory(feedID int64, categoryID *int64) error {
	query := `UPDATE feeds SET category_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	result, err := s.db.Exec(query, categoryID, feedID)
	if err != nil {
		return fmt.Errorf("failed to move feed to category: %w", err)
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
