// Package scheduler 负责定时调度 Feed 更新任务。
//
// 调度器周期性扫描到期的订阅源并发抓取更新，写入数据库，
// 处理连续失败的智能退避，并向前端推送新文章事件。
package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/store"
)

const (
	// ItemsUpdatedEvent 新文章到达时推送给前端的事件名。
	ItemsUpdatedEvent = "items:updated"

	defaultPollInterval = time.Minute
	defaultInterval     = 30 * time.Minute
	defaultConcurrency  = 5
)

// FeedStore 调度器所需的数据库能力（便于测试替换）。
type FeedStore interface {
	GetFeedsForUpdate() ([]store.Feed, error)
	ListFeeds() ([]store.Feed, error)
	GetFeed(id int64) (*store.Feed, error)
	CreateItemIfNotExists(item *store.Item) (bool, error)
	UpdateFeedLastUpdated(id int64, t time.Time) error
	UpdateFeedError(id int64, errMsg string) error
	ResetFeedError(id int64) error
	UpdateFeedStatus(id int64, status string) error
	CleanupOldItems(feedID int64, maxItems int) error
}

// FeedFetcher 调度器所需的抓取能力（便于测试替换）。
type FeedFetcher interface {
	FetchFeed(ctx context.Context, url string) (*fetcher.ParsedFeed, *fetcher.FetchResult, error)
	FetchFeedForce(ctx context.Context, url string) (*fetcher.ParsedFeed, *fetcher.FetchResult, error)
}

// Emitter 向前端推送事件的能力（生产环境由 Wails 应用实现）。
type Emitter interface {
	Emit(name string, data any)
}

// nopEmitter 默认空实现。
type nopEmitter struct{}

func (nopEmitter) Emit(string, any) {}

// Config 调度器配置。
type Config struct {
	PollInterval    time.Duration // 扫描到期源的轮询间隔（默认 1 分钟）
	DefaultInterval time.Duration // 全局默认更新间隔（默认 30 分钟）
	Concurrency     int           // 并发更新上限（默认 5）
}

func (c Config) withDefaults() Config {
	if c.PollInterval <= 0 {
		c.PollInterval = defaultPollInterval
	}
	if c.DefaultInterval <= 0 {
		c.DefaultInterval = defaultInterval
	}
	if c.Concurrency <= 0 {
		c.Concurrency = defaultConcurrency
	}
	return c
}

// RefreshResult 单个 Feed 的刷新结果。
type RefreshResult struct {
	FeedID      int64
	NewItems    int
	NotModified bool
	Err         error
}

// Scheduler 定时调度器。
type Scheduler struct {
	store   FeedStore
	fetcher FeedFetcher
	emitter Emitter
	cfg     Config

	// now 提供当前时间，测试中可替换以控制退避判定。
	now func() time.Time

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// Option 配置 Scheduler。
type Option func(*Scheduler)

// WithEmitter 设置事件发射器。
func WithEmitter(e Emitter) Option {
	return func(s *Scheduler) {
		if e != nil {
			s.emitter = e
		}
	}
}

// WithConfig 设置调度配置。
func WithConfig(cfg Config) Option {
	return func(s *Scheduler) { s.cfg = cfg.withDefaults() }
}

// withClock 替换时间源（仅供测试）。
func withClock(now func() time.Time) Option {
	return func(s *Scheduler) {
		if now != nil {
			s.now = now
		}
	}
}

// New 创建调度器。
func New(st FeedStore, ft FeedFetcher, opts ...Option) *Scheduler {
	s := &Scheduler{
		store:   st,
		fetcher: ft,
		emitter: nopEmitter{},
		cfg:     Config{}.withDefaults(),
		now:     time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start 启动后台调度循环。重复调用无副作用。
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	loopCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true

	s.wg.Add(1)
	go s.loop(loopCtx)
}

// Stop 停止调度循环并等待其退出。
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	cancel := s.cancel
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

// loop 周期性触发到期源更新。
func (s *Scheduler) loop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	// 启动后立即执行一次。
	s.Tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Tick(ctx)
		}
	}
}

// Tick 扫描并刷新所有到期（且未处于退避期）的订阅源。
func (s *Scheduler) Tick(ctx context.Context) []RefreshResult {
	feeds, err := s.store.GetFeedsForUpdate()
	if err != nil {
		return nil
	}

	due := make([]store.Feed, 0, len(feeds))
	for _, f := range feeds {
		if s.isDue(f) {
			due = append(due, f)
		}
	}
	return s.refreshConcurrently(ctx, due, false)
}

// isDue 在 store 的间隔判定之上叠加退避判定。
//   - update_interval <= 0 视为“手动”，不自动刷新；
//   - 连续失败时，需等待退避窗口结束才再次尝试。
func (s *Scheduler) isDue(f store.Feed) bool {
	if f.UpdateInterval <= 0 {
		return false
	}
	if f.ErrorCount > 0 && f.LastUpdated != nil {
		base := time.Duration(f.UpdateInterval) * time.Minute
		next := f.LastUpdated.Add(fetcher.Backoff(f.ErrorCount, base))
		if s.now().Before(next) {
			return false
		}
	}
	return true
}

// RefreshFeed 手动刷新单个订阅源（条件 GET）。
func (s *Scheduler) RefreshFeed(ctx context.Context, id int64) (RefreshResult, error) {
	feed, err := s.store.GetFeed(id)
	if err != nil {
		return RefreshResult{FeedID: id}, err
	}
	return s.refreshFeed(ctx, *feed, false), nil
}

// ForceRefreshFeed 强制刷新单个订阅源，忽略条件 GET。
func (s *Scheduler) ForceRefreshFeed(ctx context.Context, id int64) (RefreshResult, error) {
	feed, err := s.store.GetFeed(id)
	if err != nil {
		return RefreshResult{FeedID: id}, err
	}
	return s.refreshFeed(ctx, *feed, true), nil
}

// RefreshAll 手动刷新全部非暂停订阅源。
func (s *Scheduler) RefreshAll(ctx context.Context) ([]RefreshResult, error) {
	return s.refreshAll(ctx, false)
}

// ForceRefreshAll 强制刷新全部非暂停订阅源，忽略条件 GET。
func (s *Scheduler) ForceRefreshAll(ctx context.Context) ([]RefreshResult, error) {
	return s.refreshAll(ctx, true)
}

func (s *Scheduler) refreshAll(ctx context.Context, force bool) ([]RefreshResult, error) {
	feeds, err := s.store.ListFeeds()
	if err != nil {
		return nil, err
	}
	active := make([]store.Feed, 0, len(feeds))
	for _, f := range feeds {
		if f.Status != "paused" {
			active = append(active, f)
		}
	}
	return s.refreshConcurrently(ctx, active, force), nil
}

// PauseFeed 暂停某订阅源的自动更新。
func (s *Scheduler) PauseFeed(id int64) error {
	return s.store.UpdateFeedStatus(id, "paused")
}

// ResumeFeed 恢复某订阅源的自动更新。
func (s *Scheduler) ResumeFeed(id int64) error {
	return s.store.UpdateFeedStatus(id, "active")
}

// refreshConcurrently 以受限并发刷新一组订阅源，结果顺序与输入对应。
func (s *Scheduler) refreshConcurrently(ctx context.Context, feeds []store.Feed, force bool) []RefreshResult {
	results := make([]RefreshResult, len(feeds))
	if len(feeds) == 0 {
		return results
	}

	sem := make(chan struct{}, s.cfg.Concurrency)
	var wg sync.WaitGroup
	for i, f := range feeds {
		wg.Add(1)
		go func(i int, f store.Feed) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[i] = RefreshResult{FeedID: f.ID, Err: ctx.Err()}
				return
			}
			defer func() { <-sem }()
			results[i] = s.refreshFeed(ctx, f, force)
		}(i, f)
	}
	wg.Wait()
	return results
}

// refreshFeed 抓取并持久化单个订阅源，更新其状态与退避计数。
func (s *Scheduler) refreshFeed(ctx context.Context, feed store.Feed, force bool) RefreshResult {
	res := RefreshResult{FeedID: feed.ID}

	var parsed *fetcher.ParsedFeed
	var fetchRes *fetcher.FetchResult
	var err error
	if force {
		parsed, fetchRes, err = s.fetcher.FetchFeedForce(ctx, feed.URL)
	} else {
		parsed, fetchRes, err = s.fetcher.FetchFeed(ctx, feed.URL)
	}

	if err != nil {
		res.Err = err
		_ = s.store.UpdateFeedError(feed.ID, err.Error())
		return res
	}

	now := s.now()

	// 304：内容未变化，仅刷新检查时间并重置退避。
	if fetchRes != nil && fetchRes.NotModified {
		res.NotModified = true
		_ = s.store.UpdateFeedLastUpdated(feed.ID, now)
		_ = s.store.ResetFeedError(feed.ID)
		return res
	}
	if parsed == nil {
		res.Err = errors.New("scheduler: empty feed response")
		_ = s.store.UpdateFeedError(feed.ID, res.Err.Error())
		return res
	}

	newItems := 0
	for _, p := range parsed.Items {
		item := toStoreItem(feed.ID, p, now)
		if item.URL == "" {
			continue // 无可用标识，跳过
		}
		created, e := s.store.CreateItemIfNotExists(item)
		if e != nil {
			continue
		}
		if created {
			newItems++
		}
	}

	if feed.MaxItems > 0 {
		_ = s.store.CleanupOldItems(feed.ID, feed.MaxItems)
	}
	_ = s.store.UpdateFeedLastUpdated(feed.ID, now)
	_ = s.store.ResetFeedError(feed.ID)

	res.NewItems = newItems
	if newItems > 0 {
		s.emitter.Emit(ItemsUpdatedEvent, map[string]any{
			"feedId":   feed.ID,
			"newItems": newItems,
		})
	}
	return res
}

// toStoreItem 将解析后的条目映射为数据库条目。
func toStoreItem(feedID int64, p fetcher.ParsedItem, fallbackPublished time.Time) *store.Item {
	url := p.Link
	if url == "" {
		url = p.GUID
	}

	published := p.Published
	if published.IsZero() {
		published = fallbackPublished // items.published_at 为 NOT NULL
	}

	var updated *time.Time
	if !p.Updated.IsZero() {
		u := p.Updated
		updated = &u
	}

	return &store.Item{
		FeedID:      feedID,
		Title:       p.Title,
		Author:      p.Author,
		PublishedAt: published,
		UpdatedAt:   updated,
		URL:         url,
		Content:     p.Content,
		Summary:     p.Summary,
		Enclosure:   p.Enclosure,
		Categories:  encodeCategories(p.Categories),
	}
}

// encodeCategories 将分类切片编码为 JSON 数组字符串，空则返回空串。
func encodeCategories(cats []string) string {
	if len(cats) == 0 {
		return ""
	}
	b, err := json.Marshal(cats)
	if err != nil {
		return ""
	}
	return string(b)
}
