package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"changeme/internal/fetcher"
	"changeme/internal/store"
)

// ---- 测试替身：FeedStore ----

type fakeStore struct {
	mu sync.Mutex

	feeds     map[int64]*store.Feed
	forUpdate []store.Feed

	items    []store.Item
	existing map[string]bool

	errorUpdates  map[int64]int
	resets        map[int64]int
	lastUpdated   map[int64]time.Time
	cleanups      map[int64]int
	statusUpdates map[int64]string
	listCalls     int32
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		feeds:         map[int64]*store.Feed{},
		existing:      map[string]bool{},
		errorUpdates:  map[int64]int{},
		resets:        map[int64]int{},
		lastUpdated:   map[int64]time.Time{},
		cleanups:      map[int64]int{},
		statusUpdates: map[int64]string{},
	}
}

func (f *fakeStore) addFeed(feed store.Feed) {
	f.feeds[feed.ID] = &feed
}

func (f *fakeStore) GetFeedsForUpdate() ([]store.Feed, error) {
	atomic.AddInt32(&f.listCalls, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.Feed, len(f.forUpdate))
	copy(out, f.forUpdate)
	return out, nil
}

func (f *fakeStore) ListFeeds() ([]store.Feed, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.Feed, 0, len(f.feeds))
	for _, fd := range f.feeds {
		out = append(out, *fd)
	}
	return out, nil
}

func (f *fakeStore) GetFeed(id int64) (*store.Feed, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fd, ok := f.feeds[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *fd
	return &cp, nil
}

func (f *fakeStore) CreateItemIfNotExists(item *store.Item) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := keyOf(item.FeedID, item.URL)
	if f.existing[key] {
		return false, nil
	}
	f.existing[key] = true
	f.items = append(f.items, *item)
	return true, nil
}

func (f *fakeStore) UpdateFeedLastUpdated(id int64, t time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastUpdated[id] = t
	return nil
}

func (f *fakeStore) UpdateFeedError(id int64, msg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errorUpdates[id]++
	if fd := f.feeds[id]; fd != nil {
		fd.ErrorCount++
	}
	return nil
}

func (f *fakeStore) ResetFeedError(id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resets[id]++
	if fd := f.feeds[id]; fd != nil {
		fd.ErrorCount = 0
	}
	return nil
}

func (f *fakeStore) UpdateFeedStatus(id int64, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusUpdates[id] = status
	if fd := f.feeds[id]; fd != nil {
		fd.Status = status
	}
	return nil
}

func (f *fakeStore) CleanupOldItems(feedID int64, maxItems int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanups[feedID]++
	return nil
}

func (f *fakeStore) itemCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.items)
}

func keyOf(feedID int64, url string) string {
	return fmt.Sprintf("%d@%s", feedID, url)
}

// ---- 测试替身：FeedFetcher ----

type fakeFetcher struct {
	mu          sync.Mutex
	feeds       map[string]*fetcher.ParsedFeed
	errs        map[string]error
	notModified map[string]bool

	condCalls  int32
	forceCalls int32

	delay      time.Duration
	current    int32
	maxConcurr int32
}

func newFakeFetcher() *fakeFetcher {
	return &fakeFetcher{
		feeds:       map[string]*fetcher.ParsedFeed{},
		errs:        map[string]error{},
		notModified: map[string]bool{},
	}
}

func (f *fakeFetcher) FetchFeed(ctx context.Context, url string) (*fetcher.ParsedFeed, *fetcher.FetchResult, error) {
	atomic.AddInt32(&f.condCalls, 1)
	return f.common(url)
}

func (f *fakeFetcher) FetchFeedForce(ctx context.Context, url string) (*fetcher.ParsedFeed, *fetcher.FetchResult, error) {
	atomic.AddInt32(&f.forceCalls, 1)
	return f.common(url)
}

func (f *fakeFetcher) common(url string) (*fetcher.ParsedFeed, *fetcher.FetchResult, error) {
	cur := atomic.AddInt32(&f.current, 1)
	for {
		old := atomic.LoadInt32(&f.maxConcurr)
		if cur <= old || atomic.CompareAndSwapInt32(&f.maxConcurr, old, cur) {
			break
		}
	}
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	atomic.AddInt32(&f.current, -1)

	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.errs[url]; err != nil {
		return nil, nil, err
	}
	if f.notModified[url] {
		return nil, &fetcher.FetchResult{NotModified: true}, nil
	}
	return f.feeds[url], &fetcher.FetchResult{StatusCode: 200}, nil
}

// ---- 测试替身：Emitter ----

type fakeEmitter struct {
	mu     sync.Mutex
	events []emitted
}

type emitted struct {
	name string
	data any
}

func (e *fakeEmitter) Emit(name string, data any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, emitted{name, data})
}

func (e *fakeEmitter) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.events)
}

// ---- 测试 ----

func TestRefreshFeedPersistsAndEmits(t *testing.T) {
	st := newFakeStore()
	st.addFeed(store.Feed{ID: 1, URL: "u1", Status: "active", UpdateInterval: 30, MaxItems: 100})

	ft := newFakeFetcher()
	ft.feeds["u1"] = &fetcher.ParsedFeed{Items: []fetcher.ParsedItem{
		{GUID: "g1", Title: "a", Link: "https://x/1"},
		{GUID: "g2", Title: "b", Link: "https://x/2"},
	}}

	em := &fakeEmitter{}
	s := New(st, ft, WithEmitter(em))

	res, err := s.RefreshFeed(context.Background(), 1)
	if err != nil {
		t.Fatalf("RefreshFeed: %v", err)
	}
	if res.NewItems != 2 {
		t.Errorf("new items = %d, want 2", res.NewItems)
	}
	if st.itemCount() != 2 {
		t.Errorf("persisted items = %d, want 2", st.itemCount())
	}
	if st.cleanups[1] != 1 {
		t.Errorf("cleanup calls = %d, want 1", st.cleanups[1])
	}
	if st.resets[1] != 1 {
		t.Errorf("reset calls = %d, want 1", st.resets[1])
	}
	if _, ok := st.lastUpdated[1]; !ok {
		t.Error("lastUpdated should be set")
	}
	if em.count() != 1 {
		t.Fatalf("emitted events = %d, want 1", em.count())
	}
	data, ok := em.events[0].data.(map[string]any)
	if em.events[0].name != ItemsUpdatedEvent || !ok || data["feedId"] != int64(1) || data["newItems"] != 2 {
		t.Errorf("event payload wrong: %+v", em.events[0])
	}

	// 二次刷新：全部重复，无新文章、无事件。
	res2, _ := s.RefreshFeed(context.Background(), 1)
	if res2.NewItems != 0 {
		t.Errorf("second refresh new items = %d, want 0", res2.NewItems)
	}
	if em.count() != 1 {
		t.Errorf("no new event expected, got %d", em.count())
	}
}

func TestRefreshNotModified(t *testing.T) {
	st := newFakeStore()
	st.addFeed(store.Feed{ID: 1, URL: "u1", Status: "active", UpdateInterval: 30})
	ft := newFakeFetcher()
	ft.notModified["u1"] = true

	s := New(st, ft)
	res, err := s.RefreshFeed(context.Background(), 1)
	if err != nil {
		t.Fatalf("RefreshFeed: %v", err)
	}
	if !res.NotModified || res.NewItems != 0 {
		t.Errorf("expected NotModified with 0 items: %+v", res)
	}
	if st.itemCount() != 0 {
		t.Errorf("no items should be stored, got %d", st.itemCount())
	}
	if st.resets[1] != 1 {
		t.Errorf("304 should reset backoff, resets = %d", st.resets[1])
	}
	if _, ok := st.lastUpdated[1]; !ok {
		t.Error("304 should still refresh lastUpdated")
	}
}

func TestRefreshErrorRecordsFailure(t *testing.T) {
	st := newFakeStore()
	st.addFeed(store.Feed{ID: 1, URL: "u1", Status: "active", UpdateInterval: 30})
	ft := newFakeFetcher()
	ft.errs["u1"] = errors.New("boom")

	s := New(st, ft)
	res, _ := s.RefreshFeed(context.Background(), 1)
	if res.Err == nil {
		t.Fatal("expected error result")
	}
	if st.errorUpdates[1] != 1 {
		t.Errorf("error updates = %d, want 1", st.errorUpdates[1])
	}
	if st.feeds[1].ErrorCount != 1 {
		t.Errorf("error count = %d, want 1", st.feeds[1].ErrorCount)
	}
	if st.resets[1] != 0 {
		t.Errorf("failed refresh must not reset backoff, resets = %d", st.resets[1])
	}
}

func TestTickFiltersManualAndBackoff(t *testing.T) {
	fixed := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	recent := fixed.Add(-1 * time.Minute)
	old := fixed.Add(-200 * time.Hour)

	st := newFakeStore()
	st.forUpdate = []store.Feed{
		{ID: 1, URL: "u1", UpdateInterval: 30, ErrorCount: 0},                       // 到期
		{ID: 2, URL: "u2", UpdateInterval: 0},                                       // 手动 -> 跳过
		{ID: 3, URL: "u3", UpdateInterval: 30, ErrorCount: 5, LastUpdated: &recent}, // 退避中 -> 跳过
		{ID: 4, URL: "u4", UpdateInterval: 30, ErrorCount: 2, LastUpdated: &old},    // 退避已过 -> 到期
	}
	for _, fd := range st.forUpdate {
		st.addFeed(fd)
	}

	ft := newFakeFetcher()
	ft.feeds["u1"] = &fetcher.ParsedFeed{}
	ft.feeds["u4"] = &fetcher.ParsedFeed{}

	s := New(st, ft, withClock(func() time.Time { return fixed }))
	results := s.Tick(context.Background())

	if len(results) != 2 {
		t.Fatalf("due feeds = %d, want 2 (%+v)", len(results), results)
	}
	got := map[int64]bool{}
	for _, r := range results {
		got[r.FeedID] = true
	}
	if !got[1] || !got[4] {
		t.Errorf("expected feeds 1 and 4 to be refreshed, got %+v", got)
	}
	if got[2] || got[3] {
		t.Errorf("manual/backoff feeds should be skipped, got %+v", got)
	}
}

func TestForceRefreshUsesForceFetch(t *testing.T) {
	st := newFakeStore()
	st.addFeed(store.Feed{ID: 1, URL: "u1", Status: "active", UpdateInterval: 30})
	ft := newFakeFetcher()
	ft.feeds["u1"] = &fetcher.ParsedFeed{}

	s := New(st, ft)
	if _, err := s.ForceRefreshFeed(context.Background(), 1); err != nil {
		t.Fatalf("ForceRefreshFeed: %v", err)
	}
	if atomic.LoadInt32(&ft.forceCalls) != 1 {
		t.Errorf("force calls = %d, want 1", ft.forceCalls)
	}
	if atomic.LoadInt32(&ft.condCalls) != 0 {
		t.Errorf("conditional calls = %d, want 0", ft.condCalls)
	}
}

func TestRefreshAllSkipsPaused(t *testing.T) {
	st := newFakeStore()
	st.addFeed(store.Feed{ID: 1, URL: "u1", Status: "active", UpdateInterval: 30})
	st.addFeed(store.Feed{ID: 2, URL: "u2", Status: "paused", UpdateInterval: 30})
	ft := newFakeFetcher()
	ft.feeds["u1"] = &fetcher.ParsedFeed{}
	ft.feeds["u2"] = &fetcher.ParsedFeed{}

	s := New(st, ft)
	results, err := s.RefreshAll(context.Background())
	if err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if len(results) != 1 || results[0].FeedID != 1 {
		t.Errorf("expected only feed 1 refreshed, got %+v", results)
	}
}

func TestPauseResume(t *testing.T) {
	st := newFakeStore()
	st.addFeed(store.Feed{ID: 1, URL: "u1", Status: "active"})
	s := New(st, newFakeFetcher())

	if err := s.PauseFeed(1); err != nil {
		t.Fatalf("PauseFeed: %v", err)
	}
	if st.statusUpdates[1] != "paused" {
		t.Errorf("status = %q, want paused", st.statusUpdates[1])
	}
	if err := s.ResumeFeed(1); err != nil {
		t.Fatalf("ResumeFeed: %v", err)
	}
	if st.statusUpdates[1] != "active" {
		t.Errorf("status = %q, want active", st.statusUpdates[1])
	}
}

func TestConcurrencyLimit(t *testing.T) {
	st := newFakeStore()
	ft := newFakeFetcher()
	ft.delay = 20 * time.Millisecond

	const n = 8
	feeds := make([]store.Feed, n)
	for i := 0; i < n; i++ {
		id := int64(i + 1)
		url := fmt.Sprintf("u%d", id)
		feeds[i] = store.Feed{ID: id, URL: url, Status: "active", UpdateInterval: 30}
		st.addFeed(feeds[i])
		ft.feeds[url] = &fetcher.ParsedFeed{}
	}

	const limit = 3
	s := New(st, ft, WithConfig(Config{Concurrency: limit}))
	if _, err := s.RefreshAll(context.Background()); err != nil {
		t.Fatalf("RefreshAll: %v", err)
	}
	if max := atomic.LoadInt32(&ft.maxConcurr); max > limit {
		t.Errorf("max concurrency = %d, exceeds limit %d", max, limit)
	}
	if atomic.LoadInt32(&ft.maxConcurr) == 0 {
		t.Error("expected concurrency to be observed")
	}
}

func TestStartStopRunsTick(t *testing.T) {
	st := newFakeStore() // forUpdate empty -> ticks do no fetching
	s := New(st, newFakeFetcher(), WithConfig(Config{PollInterval: time.Hour}))

	s.Start(context.Background())
	// 启动会立即触发一次 Tick；等待其执行。
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt32(&st.listCalls) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	s.Stop()

	if atomic.LoadInt32(&st.listCalls) == 0 {
		t.Error("expected at least one tick (GetFeedsForUpdate call)")
	}
	// 重复 Stop 不应 panic。
	s.Stop()
}

func TestToStoreItemMapping(t *testing.T) {
	fallback := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	upd := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	// 无 link 时回退到 guid 作为 URL；无 published 时回退到 fallback。
	item := toStoreItem(7, fetcher.ParsedItem{
		GUID:       "guid-only",
		Title:      "t",
		Updated:    upd,
		Categories: []string{"a", "b"},
	}, fallback)

	if item.FeedID != 7 {
		t.Errorf("feedID = %d", item.FeedID)
	}
	if item.URL != "guid-only" {
		t.Errorf("url = %q, want guid fallback", item.URL)
	}
	if !item.PublishedAt.Equal(fallback) {
		t.Errorf("published = %v, want fallback", item.PublishedAt)
	}
	if item.UpdatedAt == nil || !item.UpdatedAt.Equal(upd) {
		t.Errorf("updatedAt = %v", item.UpdatedAt)
	}
	if item.Categories != `["a","b"]` {
		t.Errorf("categories = %q", item.Categories)
	}

	// 空分类编码为空串。
	if got := encodeCategories(nil); got != "" {
		t.Errorf("empty categories = %q, want empty", got)
	}
}
