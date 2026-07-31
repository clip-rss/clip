package store

import (
	"database/sql"
	"fmt"
	"strings"
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

// 列字段，搜索两种查询路径共用。
const itemColumns = `i.id, i.feed_id, i.title, i.author, i.published_at, i.updated_at, i.url,
	       i.content, i.summary, i.enclosure, i.categories, i.is_read, i.is_starred,
	       i.read_at, i.note, i.created_at`

// SearchItems 全文搜索文章（标题/摘要/笔记）。
//
// 分词器为 trigram，仅能匹配 ≥3 字符的子串；当关键词被空白切分后存在长度
// 不足 3 的 token（如中文「周刊」、英文「Go」）时，FTS 无法命中，回退到
// LIKE 子串匹配。多个 token 之间按 AND 处理（全部命中才算匹配）。
func (s *Store) SearchItems(keyword string, limit, offset int) ([]Item, error) {
	tokens := strings.Fields(keyword)
	if len(tokens) == 0 {
		return []Item{}, nil
	}

	if hasShortToken(tokens) {
		return s.searchByLike(tokens, limit, offset)
	}
	return s.searchByFTS(tokens, limit, offset)
}

// searchByFTS 走 FTS5 MATCH，按相关度排序。
func (s *Store) searchByFTS(tokens []string, limit, offset int) ([]Item, error) {
	query := `
		SELECT ` + itemColumns + `
		FROM items i
		INNER JOIN items_fts fts ON i.id = fts.rowid
		WHERE items_fts MATCH ?
		ORDER BY rank
		LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(query, buildFTSMatch(tokens), limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to search items: %w", err)
	}
	defer rows.Close()

	return s.scanItems(rows)
}

// searchByLike 子串匹配兜底：每个 token 需在 title/summary/note 任一字段出现，
// token 之间 AND。按发布时间倒序。
func (s *Store) searchByLike(tokens []string, limit, offset int) ([]Item, error) {
	var clauses []string
	var args []any
	for _, tok := range tokens {
		clauses = append(clauses, `(i.title LIKE ? ESCAPE '\' OR i.summary LIKE ? ESCAPE '\' OR i.note LIKE ? ESCAPE '\')`)
		pat := "%" + escapeLike(tok) + "%"
		args = append(args, pat, pat, pat)
	}

	query := `
		SELECT ` + itemColumns + `
		FROM items i
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY i.published_at DESC
		LIMIT ? OFFSET ?
	`
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search items (like): %w", err)
	}
	defer rows.Close()

	return s.scanItems(rows)
}

// hasShortToken 报告是否存在长度不足 3 个字符（rune）的 token——trigram 无法命中。
func hasShortToken(tokens []string) bool {
	for _, tok := range tokens {
		if len([]rune(tok)) < 3 {
			return true
		}
	}
	return false
}

// buildFTSMatch 将关键词 token 构造为安全的 FTS5 MATCH 串：
// 每个 token 用双引号包裹为短语（内部 " 转义为 ""），token 间空格即隐式 AND。
// 避免用户输入中的 * - : 等被当作 FTS 语法。
func buildFTSMatch(tokens []string) string {
	quoted := make([]string, len(tokens))
	for i, tok := range tokens {
		quoted[i] = `"` + strings.ReplaceAll(tok, `"`, `""`) + `"`
	}
	return strings.Join(quoted, " ")
}

// escapeLike 转义 LIKE 通配符，配合 ESCAPE '\' 使用。
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
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

// 列表轻量查询列字段（不含 content），供 Light 系列查询复用。
const itemColumnsLight = `i.id, i.feed_id, i.title, i.author, i.published_at, i.updated_at, i.url,
	       i.summary, i.enclosure, i.categories, i.is_read, i.is_starred,
	       i.read_at, i.note, i.created_at`

// ListAllItemsLight 获取所有文章列表（轻量版本，不含 content）
func (s *Store) ListAllItemsLight(limit, offset int) ([]ItemLight, error) {
	query := `
		SELECT ` + itemColumnsLight + `
		FROM items i
		ORDER BY i.published_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list all items (light): %w", err)
	}
	defer rows.Close()

	return s.scanItemsLight(rows)
}

// ListItemsByFeedLight 获取指定订阅源的文章列表（轻量版本）
func (s *Store) ListItemsByFeedLight(feedID int64, limit, offset int) ([]ItemLight, error) {
	query := `
		SELECT ` + itemColumnsLight + `
		FROM items i
		WHERE i.feed_id = ?
		ORDER BY i.published_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(query, feedID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list items by feed (light): %w", err)
	}
	defer rows.Close()

	return s.scanItemsLight(rows)
}

// ListUnreadItemsLight 获取未读文章列表（轻量版本）
func (s *Store) ListUnreadItemsLight(limit, offset int) ([]ItemLight, error) {
	query := `
		SELECT ` + itemColumnsLight + `
		FROM items i
		WHERE i.is_read = 0
		ORDER BY i.published_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list unread items (light): %w", err)
	}
	defer rows.Close()

	return s.scanItemsLight(rows)
}

// ListStarredItemsLight 获取星标文章列表（轻量版本）
func (s *Store) ListStarredItemsLight(limit, offset int) ([]ItemLight, error) {
	query := `
		SELECT ` + itemColumnsLight + `
		FROM items i
		WHERE i.is_starred = 1
		ORDER BY i.published_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list starred items (light): %w", err)
	}
	defer rows.Close()

	return s.scanItemsLight(rows)
}

// scanItemsLight 辅助函数：扫描轻量文章行（不含 content）
func (s *Store) scanItemsLight(rows *sql.Rows) ([]ItemLight, error) {
	items := []ItemLight{}
	for rows.Next() {
		item := ItemLight{}
		err := rows.Scan(
			&item.ID,
			&item.FeedID,
			&item.Title,
			&item.Author,
			&item.PublishedAt,
			&item.UpdatedAt,
			&item.URL,
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
			return nil, fmt.Errorf("failed to scan item (light): %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
