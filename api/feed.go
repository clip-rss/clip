package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/scheduler"
	"github.com/clip-rss/clip/internal/store"
)

// FeedService 订阅源管理与刷新相关的绑定方法。
type FeedService struct {
	store   *store.Store
	fetcher *fetcher.Fetcher
	sched   *scheduler.Scheduler
}

// NewFeedService 创建 FeedService。
func NewFeedService(st *store.Store, ft *fetcher.Fetcher, sch *scheduler.Scheduler) *FeedService {
	return &FeedService{store: st, fetcher: ft, sched: sch}
}

// FeedPreview 添加订阅前的检测预览信息（不入库）。
type FeedPreview struct {
	URL          string `json:"url"`          // 实际可订阅的 Feed URL（网页输入时为发现到的地址）
	Title        string `json:"title"`        // 解析到的源标题
	Description  string `json:"description"`  // 源描述
	Link         string `json:"link"`         // 站点主页链接
	Icon         string `json:"icon"`         // favicon URL
	ItemCount    int    `json:"itemCount"`    // 当前抓取到的文章数
	AlreadyAdded bool   `json:"alreadyAdded"` // 该源是否已订阅
}

// PreviewFeed 检测并预览一个订阅地址，但不写入数据库。
// 统一处理两类输入：
//   - 直接的 RSS/Atom Feed 地址；
//   - 普通网页地址（解析其 <link rel="alternate"> 自动发现首个 Feed）。
func (s *FeedService) PreviewFeed(rawURL string) (*FeedPreview, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errors.New("feed url is empty")
	}

	ctx := context.Background()

	// 先尝试直接当作 Feed 解析。
	// 用 Force 全量抓取：检测是用户主动行为，必须拿到完整内容；
	// 普通 FetchFeed 会携带条件 GET 头，若该源此前抓过会命中 304（parsed 为 nil），
	// 被误判为「非 Feed」而走到下面的网页发现分支并失败。
	if parsed, _, err := s.fetcher.FetchFeedForce(ctx, rawURL); err == nil && parsed != nil {
		return s.buildPreview(rawURL, parsed), nil
	}

	// 解析失败多半是普通网页，尝试自动发现其中声明的 Feed。
	discovered, derr := s.fetcher.Discover(ctx, rawURL)
	if derr != nil || len(discovered) == 0 {
		return nil, errors.New("未在该地址找到可订阅的源")
	}
	feedURL := discovered[0].URL
	parsed, _, err := s.fetcher.FetchFeedForce(ctx, feedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %w", err)
	}
	if parsed == nil {
		return nil, errors.New("empty feed response")
	}
	return s.buildPreview(feedURL, parsed), nil
}

// buildPreview 由解析结果构造预览，并标注是否已订阅。
func (s *FeedService) buildPreview(feedURL string, parsed *fetcher.ParsedFeed) *FeedPreview {
	existing, _ := s.store.GetFeedByURL(feedURL)
	return &FeedPreview{
		URL:          feedURL,
		Title:        firstNonEmpty(parsed.Title, feedURL),
		Description:  parsed.Description,
		Link:         parsed.Link,
		Icon:         fetcher.DiscoverFavicon(nil, parsed.Link),
		ItemCount:    len(parsed.Items),
		AlreadyAdded: existing != nil,
	}
}

// AddFeed 新增订阅源：抓取并解析以补全元信息，入库后立即拉取一次文章。
// categoryID 为 0 表示归入「未分类」。
func (s *FeedService) AddFeed(feedURL string, categoryID int64) (*store.Feed, error) {
	feedURL = strings.TrimSpace(feedURL)
	if feedURL == "" {
		return nil, errors.New("feed url is empty")
	}
	if existing, _ := s.store.GetFeedByURL(feedURL); existing != nil {
		return nil, fmt.Errorf("feed already exists: %s", feedURL)
	}

	ctx := context.Background()
	// 用 Force 全量抓取，避免命中条件 GET 缓存返回 304（如检测时已缓存过 ETag）。
	parsed, _, err := s.fetcher.FetchFeedForce(ctx, feedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %w", err)
	}
	if parsed == nil {
		return nil, errors.New("empty feed response")
	}

	settings, _ := s.store.GetSettings()
	feed := &store.Feed{
		URL:            feedURL,
		Title:          firstNonEmpty(parsed.Title, feedURL),
		Description:    parsed.Description,
		Link:           parsed.Link,
		Icon:           fetcher.DiscoverFavicon(nil, parsed.Link),
		CategoryID:     nullableID(categoryID),
		UpdateInterval: settings.DefaultUpdateInterval,
		MaxItems:       settings.DefaultMaxItems,
		Status:         "active",
	}
	if err := s.store.CreateFeed(feed); err != nil {
		return nil, err
	}

	// 初次抓取入库（失败不影响订阅源已创建，错误会记录在 feed 上）。
	_, _ = s.sched.ForceRefreshFeed(ctx, feed.ID)

	return s.store.GetFeed(feed.ID)
}

// GetFeed 按 ID 获取订阅源。
func (s *FeedService) GetFeed(id int64) (*store.Feed, error) {
	return s.store.GetFeed(id)
}

// ListFeeds 列出全部订阅源。
func (s *FeedService) ListFeeds() ([]store.Feed, error) {
	return s.store.ListFeeds()
}

// ListFeedsWithUnread 列出全部订阅源及未读计数（用于侧栏）。
func (s *FeedService) ListFeedsWithUnread() ([]store.FeedWithUnread, error) {
	return s.store.ListFeedsWithUnread()
}

// UpdateFeed 更新订阅源（标题、分类、间隔、上限等）。
func (s *FeedService) UpdateFeed(feed store.Feed) error {
	return s.store.UpdateFeed(&feed)
}

// DeleteFeed 删除订阅源（级联删除其文章）。
func (s *FeedService) DeleteFeed(id int64) error {
	return s.store.DeleteFeed(id)
}

// PauseFeed 暂停订阅源自动更新。
func (s *FeedService) PauseFeed(id int64) error {
	return s.sched.PauseFeed(id)
}

// ResumeFeed 恢复订阅源自动更新。
func (s *FeedService) ResumeFeed(id int64) error {
	return s.sched.ResumeFeed(id)
}

// RefreshFeed 手动刷新单个订阅源（条件 GET）。
func (s *FeedService) RefreshFeed(id int64) (RefreshOutcome, error) {
	res, err := s.sched.RefreshFeed(context.Background(), id)
	if err != nil {
		return RefreshOutcome{FeedID: id}, err
	}
	return toOutcome(res), nil
}

// ForceRefreshFeed 强制刷新单个订阅源（忽略条件 GET）。
func (s *FeedService) ForceRefreshFeed(id int64) (RefreshOutcome, error) {
	res, err := s.sched.ForceRefreshFeed(context.Background(), id)
	if err != nil {
		return RefreshOutcome{FeedID: id}, err
	}
	return toOutcome(res), nil
}

// RefreshAll 手动刷新全部订阅源（条件 GET）。
func (s *FeedService) RefreshAll() ([]RefreshOutcome, error) {
	res, err := s.sched.RefreshAll(context.Background())
	if err != nil {
		return nil, err
	}
	return toOutcomes(res), nil
}

// ForceRefreshAll 强制刷新全部订阅源（忽略条件 GET）。
func (s *FeedService) ForceRefreshAll() ([]RefreshOutcome, error) {
	res, err := s.sched.ForceRefreshAll(context.Background())
	if err != nil {
		return nil, err
	}
	return toOutcomes(res), nil
}
