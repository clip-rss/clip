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

// AddFeed 新增订阅源：抓取并解析以补全元信息，入库后立即拉取一次文章。
func (s *FeedService) AddFeed(feedURL string) (*store.Feed, error) {
	feedURL = strings.TrimSpace(feedURL)
	if feedURL == "" {
		return nil, errors.New("feed url is empty")
	}
	if existing, _ := s.store.GetFeedByURL(feedURL); existing != nil {
		return nil, fmt.Errorf("feed already exists: %s", feedURL)
	}

	ctx := context.Background()
	parsed, _, err := s.fetcher.FetchFeed(ctx, feedURL)
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
