// Package scheduler 负责定时调度 Feed 更新任务。
//
// 调度器周期性扫描到期的订阅源并发抓取更新，写入数据库，
// 处理连续失败的智能退避，并向前端推送新文章事件。
package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/store"
)

const (
	// ItemsUpdatedEvent 新文章到达时推送给前端的事件名。
	ItemsUpdatedEvent = "items:updated"
	// FeedErrorEvent 订阅源抓取失败时推送给前端的事件名。
	FeedErrorEvent = "feed:error"

	defaultPollInterval = time.Minute
	defaultInterval     = 30 * time.Minute
	defaultConcurrency  = 5
)

// ErrOffline 表示调度器已进入离线模式，当前不应发起网络请求。
var ErrOffline = errors.New("scheduler: offline")

// FeedStore 调度器所需的数据库能力（便于测试替换）。
type FeedStore interface {
	ListActiveFeeds() ([]store.Feed, error)
	ListFeeds() ([]store.Feed, error)
	GetFeed(id int64) (*store.Feed, error)
	RecordFeedFailure(id int64, attemptedAt time.Time, errMsg string) error
	MarkFeedNotModified(id int64, checkedAt time.Time) error
	ApplyFeedRefresh(id int64, checkedAt time.Time, items []store.RefreshItem, maxItems int) ([]store.Item, error)
	UpdateFeedStatus(id int64, status string) error
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

// NewItem 通知层需要的单篇文章信息。
type NewItem struct {
	ID    int64
	Title string
}

// Notifier 通知接口——每次刷新后，调度器将新发现的文章传入。
// 判断是否实际发送、内容格式与节流由通知层负责。
type Notifier interface {
	Notify(ctx context.Context, feed store.Feed, items []NewItem)
}

// nopNotifier 默认空实现。
type nopNotifier struct{}

func (nopNotifier) Notify(context.Context, store.Feed, []NewItem) {}

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
	// DefaultInterval 允许为 0（表示全局手动模式），不在此处覆盖。
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
	store    FeedStore
	fetcher  FeedFetcher
	emitter  Emitter
	notifier Notifier
	cfg      Config

	// now 提供当前时间，测试中可替换以控制退避判定。
	now func() time.Time

	mu          sync.Mutex
	running     bool
	offlineMode bool // 离线模式：为 true 时 tick 被跳过，不发起任何网络请求
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	slots       chan struct{}           // 所有入口共享的全局抓取并发额度
	feedLocks   map[int64]chan struct{} // 按 Feed 串行化后台与手动刷新
	wake        chan struct{}           // 恢复在线时立即触发到期扫描
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

// WithNotifier 设置通知发送器。
func WithNotifier(n Notifier) Option {
	return func(s *Scheduler) {
		if n != nil {
			s.notifier = n
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
		store:     st,
		fetcher:   ft,
		emitter:   nopEmitter{},
		notifier:  nopNotifier{},
		cfg:       Config{DefaultInterval: defaultInterval}.withDefaults(),
		now:       time.Now,
		feedLocks: make(map[int64]chan struct{}),
		wake:      make(chan struct{}, 1),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.slots = make(chan struct{}, s.cfg.Concurrency)
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

// SetDefaultInterval 运行时更新全局默认更新间隔，影响 UpdateInterval <= 0 的订阅源。
func (s *Scheduler) SetDefaultInterval(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d >= 0 {
		s.cfg.DefaultInterval = d
	}
}

// SetOfflineMode 设置离线模式：离线时暂停所有网络请求，在线时恢复正常调度。
func (s *Scheduler) SetOfflineMode(offline bool) {
	s.mu.Lock()
	if s.offlineMode == offline {
		s.mu.Unlock()
		return
	}
	s.offlineMode = offline
	s.mu.Unlock()
	if offline {
		log.Println("scheduler: entering offline mode, pausing updates")
	} else {
		log.Println("scheduler: exiting offline mode, resuming updates")
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

// tickSafe 调用 Tick 并捕获 panic，防止单次异常导致调度器静默退出。
func (s *Scheduler) tickSafe(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("scheduler: panic recovered in Tick: %v", r)
		}
	}()
	// 离线模式下跳过 tick，不发起网络请求
	s.mu.Lock()
	offline := s.offlineMode
	s.mu.Unlock()
	if offline {
		return
	}
	s.Tick(ctx)
}

// loop 周期性触发到期源更新，启动时也仅执行一次常规到期扫描。
func (s *Scheduler) loop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	// 启动时仅刷新真正到期的源，遵守手动模式、更新间隔、退避和离线状态。
	s.tickSafe(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tickSafe(ctx)
		case <-s.wake:
			s.tickSafe(ctx)
		}
	}
}

// Tick 扫描并刷新所有到期（且未处于退避期）的订阅源。
func (s *Scheduler) Tick(ctx context.Context) []RefreshResult {
	if s.isOffline() {
		return nil
	}
	feeds, err := s.store.ListActiveFeeds()
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

// isDue 统一判定单源间隔、全局回退间隔与失败退避。
//   - update_interval <= 0 时回退到全局默认间隔；
//   - 连续失败时，需等待退避窗口结束才再次尝试。
func (s *Scheduler) isDue(f store.Feed) bool {
	interval := time.Duration(f.UpdateInterval) * time.Minute
	if interval <= 0 {
		s.mu.Lock()
		interval = s.cfg.DefaultInterval
		s.mu.Unlock()
		if interval <= 0 {
			return false
		}
	}
	anchor := f.LastAttempted
	if anchor == nil {
		anchor = f.LastUpdated // 兼容尚未迁移或测试构造的 Feed
	}
	if anchor == nil {
		return true
	}
	wait := interval
	if f.ErrorCount > 0 {
		wait = fetcher.Backoff(f.ErrorCount, interval)
	}
	return !s.now().Before(anchor.Add(wait))
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
	if s.isOffline() {
		return nil, ErrOffline
	}
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

	var wg sync.WaitGroup
	for i, f := range feeds {
		wg.Add(1)
		go func(i int, f store.Feed) {
			defer wg.Done()
			results[i] = s.refreshFeed(ctx, f, force)
		}(i, f)
	}
	wg.Wait()
	return results
}

// refreshFeed 抓取并持久化单个订阅源，更新其状态与退避计数。
func (s *Scheduler) refreshFeed(ctx context.Context, feed store.Feed, force bool) RefreshResult {
	res := RefreshResult{FeedID: feed.ID}
	releaseFeed, err := s.acquireFeed(ctx, feed.ID)
	if err != nil {
		res.Err = err
		return res
	}
	defer releaseFeed()
	if s.isOffline() {
		res.Err = ErrOffline
		return res
	}
	releaseSlot, err := s.acquireSlot(ctx)
	if err != nil {
		res.Err = err
		return res
	}
	slotReleased := false
	defer func() {
		if !slotReleased {
			releaseSlot()
		}
	}()
	// 任务等待全局额度期间可能刚刚切换为离线；额度到手后必须再次确认。
	if s.isOffline() {
		releaseSlot()
		slotReleased = true
		res.Err = ErrOffline
		return res
	}

	var parsed *fetcher.ParsedFeed
	var fetchRes *fetcher.FetchResult
	if force {
		parsed, fetchRes, err = s.fetcher.FetchFeedForce(ctx, feed.URL)
	} else {
		parsed, fetchRes, err = s.fetcher.FetchFeed(ctx, feed.URL)
	}
	releaseSlot()
	slotReleased = true
	checkedAt := s.now()

	if err != nil {
		if errors.Is(err, context.Canceled) {
			res.Err = err
			return res
		}
		return s.recordFailure(feed.ID, checkedAt, err)
	}

	// 304：内容未变化，仅刷新检查时间并重置退避。
	if fetchRes != nil && fetchRes.NotModified {
		res.NotModified = true
		if err := s.store.MarkFeedNotModified(feed.ID, checkedAt); err != nil {
			return s.recordFailure(feed.ID, checkedAt, err)
		}
		return res
	}
	if parsed == nil {
		return s.recordFailure(feed.ID, checkedAt, errors.New("scheduler: empty feed response"))
	}

	refreshItems := make([]store.RefreshItem, 0, len(parsed.Items))
	for _, p := range parsed.Items {
		item := toStoreItem(feed.ID, p, checkedAt)
		if item.URL == "" {
			continue // 无可用标识，跳过
		}
		refreshItems = append(refreshItems, store.RefreshItem{
			Item: item,
			Keys: itemKeys(p, item.URL),
		})
	}

	created, err := s.store.ApplyFeedRefresh(feed.ID, checkedAt, refreshItems, feed.MaxItems)
	if err != nil {
		return s.recordFailure(feed.ID, checkedAt, err)
	}
	createdItems := make([]NewItem, len(created))
	for i, item := range created {
		createdItems[i] = NewItem{ID: item.ID, Title: item.Title}
	}
	res.NewItems = len(createdItems)
	if res.NewItems > 0 {
		s.emitter.Emit(ItemsUpdatedEvent, map[string]any{
			"feedId":   feed.ID,
			"newItems": res.NewItems,
		})
		// 首次抓取（last_updated 为空）不通知，避免订阅时刷屏。
		if feed.LastUpdated != nil {
			s.notifier.Notify(ctx, feed, createdItems)
		}
	}
	return res
}

func (s *Scheduler) recordFailure(feedID int64, attemptedAt time.Time, cause error) RefreshResult {
	if err := s.store.RecordFeedFailure(feedID, attemptedAt, cause.Error()); err != nil {
		cause = errors.Join(cause, err)
	}
	s.emitter.Emit(FeedErrorEvent, map[string]any{
		"feedId": feedID,
		"error":  cause.Error(),
	})
	return RefreshResult{FeedID: feedID, Err: cause}
}

func (s *Scheduler) acquireSlot(ctx context.Context) (func(), error) {
	select {
	case s.slots <- struct{}{}:
		return func() { <-s.slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Scheduler) acquireFeed(ctx context.Context, feedID int64) (func(), error) {
	s.mu.Lock()
	lock := s.feedLocks[feedID]
	if lock == nil {
		lock = make(chan struct{}, 1)
		s.feedLocks[feedID] = lock
	}
	s.mu.Unlock()
	select {
	case lock <- struct{}{}:
		return func() { <-lock }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Scheduler) isOffline() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.offlineMode
}

func itemKeys(item fetcher.ParsedItem, itemURL string) []string {
	keys := []string{"url:" + strings.TrimSpace(itemURL)}
	if fingerprint := fetcher.Fingerprint(item); fingerprint != "" {
		keys = append(keys, "source:"+fingerprint)
	}
	return keys
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
