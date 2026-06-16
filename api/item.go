package api

import (
	"strings"

	"github.com/clip-rss/clip/internal/store"
)

// ItemService 文章查询与操作相关的绑定方法。
type ItemService struct {
	store *store.Store
}

// NewItemService 创建 ItemService。
func NewItemService(st *store.Store) *ItemService {
	return &ItemService{store: st}
}

// ListItems 列出文章：feedID > 0 时按源过滤，否则返回全部。
func (s *ItemService) ListItems(feedID int64, limit, offset int) ([]store.Item, error) {
	if feedID > 0 {
		return s.store.ListItemsByFeed(feedID, limit, offset)
	}
	return s.store.ListAllItems(limit, offset)
}

// ListUnreadItems 列出未读文章。
func (s *ItemService) ListUnreadItems(limit, offset int) ([]store.Item, error) {
	return s.store.ListUnreadItems(limit, offset)
}

// ListStarredItems 列出星标文章。
func (s *ItemService) ListStarredItems(limit, offset int) ([]store.Item, error) {
	return s.store.ListStarredItems(limit, offset)
}

// GetItem 按 ID 获取文章。
func (s *ItemService) GetItem(id int64) (*store.Item, error) {
	return s.store.GetItem(id)
}

// SearchItems 全文搜索文章（标题/摘要/笔记）。
func (s *ItemService) SearchItems(keyword string, limit, offset int) ([]store.Item, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []store.Item{}, nil
	}
	return s.store.SearchItems(keyword, limit, offset)
}

// MarkRead 标记文章为已读。
func (s *ItemService) MarkRead(id int64) error {
	return s.store.MarkItemAsRead(id)
}

// MarkUnread 标记文章为未读。
func (s *ItemService) MarkUnread(id int64) error {
	return s.store.MarkItemAsUnread(id)
}

// BatchMarkRead 批量标记文章为已读。
func (s *ItemService) BatchMarkRead(ids []int64) error {
	return s.store.MarkItemsAsRead(ids)
}

// MarkAllReadByFeed 标记某订阅源全部文章为已读。
func (s *ItemService) MarkAllReadByFeed(feedID int64) error {
	return s.store.MarkAllAsReadByFeed(feedID)
}

// ToggleStar 切换文章星标状态。
func (s *ItemService) ToggleStar(id int64) error {
	return s.store.ToggleItemStar(id)
}

// AddNote 更新（新增/覆盖）文章笔记。
func (s *ItemService) AddNote(id int64, note string) error {
	return s.store.UpdateItemNote(id, note)
}

// GetUnreadCount 获取全局未读总数。
func (s *ItemService) GetUnreadCount() (int, error) {
	return s.store.GetUnreadCount()
}
