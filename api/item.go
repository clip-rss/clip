package api

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/i18n"
	"github.com/clip-rss/clip/internal/reader"
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
	// fetcher 用于按需抓取文章原文页面。可为 nil：
	// 传 nil 的实例只做本地查询，FetchFullContent 会返回明确的「不可用」错误，
	// 而不是崩在 nil 解引用上。
	fetcher *fetcher.Fetcher
}

// NewItemService 创建 ItemService。ft 可为 nil（不启用「获取全文」）。
//
// fetcher 走构造参数而不是导出 SetFetcher：wails3 会把 Service 上的**所有**导出方法
// 绑定成前端 API，一个接收 *fetcher.Fetcher 的 setter 是不该存在的公开接口。
func NewItemService(st *store.Store, ft *fetcher.Fetcher) *ItemService {
	return &ItemService{store: st, fetcher: ft}
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

// ListNotedItemsLight 列出有笔记的文章（轻量版本）。
func (s *ItemService) ListNotedItemsLight(limit, offset int) ([]store.ItemLight, error) {
	items, err := s.store.ListNotedItemsLight(limit, offset)
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

// FetchFullContent 按需抓取文章原文并提取正文，成功时返回正文 HTML。
//
// 「RSS 只给摘要」的源在阅读界面只有摘要，这个方法是那类文章
// 的补充路径。结果落库到 items.full_content，**不覆盖 items.content** ——
// RSS 原文必须留着，提取错了才有回退余地；同一篇第二次调用直接读库、不再联网。
//
// 已提取过的文章直接返回库里的结果，不再联网。
//
// 提取失败一律不写库、不碰 feeds.error_count —— 那是订阅源的字段，混用会让
// 「已失效」分组误判。错误按可行动程度分级返回给前端做提示。
func (s *ItemService) FetchFullContent(id int64) (string, error) {
	lang := backendLanguage(s.store)

	item, err := s.store.GetItem(id)
	if err != nil {
		return "", backendError(s.store, err)
	}
	// 命中库里的提取结果，直接返回，不联网。
	if item.FullContent != "" {
		return cleanUFFFD(item.FullContent), nil
	}
	if strings.TrimSpace(item.URL) == "" {
		return "", i18n.Error(lang, "fulltext.urlMissing", nil)
	}
	if s.fetcher == nil {
		return "", i18n.Error(lang, "fulltext.unavailable", nil)
	}

	// 单次提取的独立超时：HTTP 客户端自带的超时管的是单个连接，这里兜住整条
	// 「抓取 + 提取」链路，避免前端按钮一直转圈。
	ctx, cancel := context.WithTimeout(context.Background(), fullContentTimeout)
	defer cancel()

	page, err := s.fetcher.FetchArticle(ctx, item.URL)
	if err != nil {
		// 失败原因（超时/404/断网）对用户没有可操作性上的区别，统一成一句
		// 「抓取原文失败」，原始错误留在链路上供日志与 errors.Is 使用。
		log.Printf("fulltext: fetch failed for item %d (%s): %v", id, item.URL, err)
		return "", i18n.Error(lang, "fulltext.fetchFailed", err)
	}

	// 提取前先剥掉 RSS 正文，否则 readability 会把它当成页面上的额外内容。
	content, err := reader.Extract(page, item.URL)
	if err != nil {
		// 「提不出正文」与「解析出错」对用户是同一件事：这个页面拿不到正文。
		// 原始错误留在链路上，errors.Is(err, reader.ErrNoContent) 仍可判别。
		log.Printf("fulltext: extract failed for item %d (%s): %v", id, item.URL, err)
		return "", i18n.Error(lang, "fulltext.extractFailed", err)
	}
	// readability 返回的片段按未清洗 HTML 看待，落库前先过 fetcher 的清洗器，
	// 与 RSS 正文走同一条防线（前端只 sanitize 一次，别指望它兜底）。
	content = fetcher.Sanitize(content)
	if strings.TrimSpace(content) == "" {
		return "", i18n.Error(lang, "fulltext.extractFailed", fmt.Errorf("%w: sanitized to empty", reader.ErrNoContent))
	}

	if err := s.store.SaveFullContent(id, content); err != nil {
		return "", backendError(s.store, err)
	}
	return content, nil
}

// fullContentTimeout 是单次全文提取的整体上限（含抓取与解析）。
const fullContentTimeout = 30 * time.Second

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

// CleanReadByFeed 删除指定订阅源中已读且未星标的文章，返回删除条数。
func (s *ItemService) CleanReadByFeed(feedID int64) (int64, error) {
	return s.store.PruneReadItemsByFeed(feedID)
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
