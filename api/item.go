package api

import (
	"strings"

	"github.com/clip-rss/clip/internal/store"
)

// cleanUFFFD 从字符串中移除 U+FFFD 替换字符，避免渲染为「��」方块。
// 该字符表示数据损坏或编码转换失败，不应显示给用户。
func cleanUFFFD(s string) string {
	if strings.ContainsRune(s, '�') {
		return strings.ReplaceAll(s, "�", "")
	}
	return s
}

// cleanItemFields 移除 Item 所有文本字段中的 U+FFFD。
func cleanItemFields(item *store.Item) {
	item.Title = cleanUFFFD(item.Title)
	item.Content = cleanUFFFD(item.Content)
	item.Summary = cleanUFFFD(item.Summary)
	item.Author = cleanUFFFD(item.Author)
	item.Enclosure = cleanUFFFD(item.Enclosure)
	item.Categories = cleanUFFFD(item.Categories)
	item.Note = cleanUFFFD(item.Note)
}

// cleanItemLightFields 移除 ItemLight 所有文本字段中的 U+FFFD。
func cleanItemLightFields(item *store.ItemLight) {
	item.Title = cleanUFFFD(item.Title)
	item.Summary = cleanUFFFD(item.Summary)
	item.Author = cleanUFFFD(item.Author)
	item.Enclosure = cleanUFFFD(item.Enclosure)
	item.Categories = cleanUFFFD(item.Categories)
	item.Note = cleanUFFFD(item.Note)
}

// cleanItems 移除一批 Item 中的 U+FFFD。
func cleanItems(items []store.Item) {
	for i := range items {
		cleanItemFields(&items[i])
	}
}

// cleanLightItems 移除一批 ItemLight 中的 U+FFFD。
func cleanLightItems(items []store.ItemLight) {
	for i := range items {
		cleanItemLightFields(&items[i])
	}
}

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
	var items []store.Item
	var err error
	if feedID > 0 {
		items, err = s.store.ListItemsByFeed(feedID, limit, offset)
	} else {
		items, err = s.store.ListAllItems(limit, offset)
	}
	if err != nil {
		return nil, err
	}
	for i := range items {
		cleanItemFields(&items[i])
	}
	return items, nil
}

// ListItemsLight 列出文章（轻量版本，不含 content）：feedID > 0 时按源过滤，否则返回全部。
func (s *ItemService) ListItemsLight(feedID int64, limit, offset int) ([]store.ItemLight, error) {
	var items []store.ItemLight
	var err error
	if feedID > 0 {
		items, err = s.store.ListItemsByFeedLight(feedID, limit, offset)
	} else {
		items, err = s.store.ListAllItemsLight(limit, offset)
	}
	if err != nil {
		return nil, err
	}
	cleanLightItems(items)
	return items, nil
}

// ListUnreadItems 列出未读文章。
func (s *ItemService) ListUnreadItems(limit, offset int) ([]store.Item, error) {
	items, err := s.store.ListUnreadItems(limit, offset)
	if err != nil {
		return nil, err
	}
	cleanItems(items)
	return items, nil
}

// ListUnreadItemsLight 列出未读文章（轻量版本）。
func (s *ItemService) ListUnreadItemsLight(limit, offset int) ([]store.ItemLight, error) {
	items, err := s.store.ListUnreadItemsLight(limit, offset)
	if err != nil {
		return nil, err
	}
	cleanLightItems(items)
	return items, nil
}

// ListStarredItems 列出星标文章。
func (s *ItemService) ListStarredItems(limit, offset int) ([]store.Item, error) {
	items, err := s.store.ListStarredItems(limit, offset)
	if err != nil {
		return nil, err
	}
	cleanItems(items)
	return items, nil
}

// ListStarredItemsLight 列出星标文章（轻量版本）。
func (s *ItemService) ListStarredItemsLight(limit, offset int) ([]store.ItemLight, error) {
	items, err := s.store.ListStarredItemsLight(limit, offset)
	if err != nil {
		return nil, err
	}
	cleanLightItems(items)
	return items, nil
}

// GetItem 按 ID 获取文章。
func (s *ItemService) GetItem(id int64) (*store.Item, error) {
	item, err := s.store.GetItem(id)
	if err != nil {
		return nil, err
	}
	cleanItemFields(item)
	return item, nil
}

// SearchItems 全文搜索文章（标题/摘要/笔记）。
func (s *ItemService) SearchItems(keyword string, limit, offset int) ([]store.Item, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []store.Item{}, nil
	}
	items, err := s.store.SearchItems(keyword, limit, offset)
	if err != nil {
		return nil, err
	}
	cleanItems(items)
	return items, nil
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
