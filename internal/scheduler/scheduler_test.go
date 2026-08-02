package scheduler

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/store"
)

// ---- 测试替身：FeedStore ----

type fakeStore struct {
	mu sync.Mutex

	feeds     map[int64]*store.Feed
	forUpdate []store.Feed

	items    []store.Item
	existing map[string]bool

	errorUpdates   map[int64]int
	resets         map[int64]int
	lastUpdated    map[int64]time.Time
	cleanups       map[int64]int
	statusUpdates  map[int64]string
	listCalls      int32
	listFeedsCalls int32
	applyErr       error
	markErr        error
	recordErr      error
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

func (f *fakeStore) ListActiveFeeds() ([]store.Feed, error) {
	atomic.AddInt32(&f.listCalls, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.Feed, 0, len(f.forUpdate))
	for _, candidate := range f.forUpdate {
		if current := f.feeds[candidate.ID]; current != nil {
			out = append(out, *current)
		} else {
			out = append(out, candidate)
		}
	}
	return out, nil
}

func (f *fakeStore) ListFeeds() ([]store.Feed, error) {
	atomic.AddInt32(&f.listFeedsCalls, 1)
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

func (f *fakeStore) RecordFeedFailure(id int64, attemptedAt time.Time, msg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recordErr != nil {
		return f.recordErr
	}
	f.errorUpdates[id]++
	if fd := f.feeds[id]; fd != nil {
		fd.ErrorCount++
		fd.LastAttempted = &attemptedAt
	}
	return nil
}

func (f *fakeStore) MarkFeedNotModified(id int64, checkedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markErr != nil {
		return f.markErr
	}
	f.resets[id]++
	f.lastUpdated[id] = checkedAt
	if fd := f.feeds[id]; fd != nil {
		fd.ErrorCount = 0
		fd.LastUpdated = &checkedAt
		fd.LastAttempted = &checkedAt
	}
	return nil
}

func (f *fakeStore) ApplyFeedRefresh(
	id int64,
	checkedAt time.Time,
	items []store.RefreshItem,
	maxItems int,
) ([]store.Item, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.applyErr != nil {
		return nil, f.applyErr
	}
	created := make([]store.Item, 0, len(items))
	for _, candidate := range items {
		if candidate.Item == nil {
			continue
		}
		item := candidate.Item
		key := keyOf(item.FeedID, item.URL)
		if f.existing[key] {
			continue
		}
		f.existing[key] = true
		item.ID = int64(len(f.items) + 1)
		f.items = append(f.items, *item)
		created = append(created, *item)
	}
	if maxItems > 0 {
		f.cleanups[id]++
	}
	f.lastUpdated[id] = checkedAt
	f.resets[id]++
	if fd := f.feeds[id]; fd != nil {
		fd.ErrorCount = 0
		fd.LastUpdated = &checkedAt
		fd.LastAttempted = &checkedAt
	}
	return created, nil
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

type fakeNotifier struct {
	mu    sync.Mutex
	calls [][]NewItem
}

func (n *fakeNotifier) Notify(_ context.Context, _ store.Feed, items []NewItem) {
	n.mu.Lock()
	defer n.mu.Unlock()
	cp := append([]NewItem(nil), items...)
	n.calls = append(n.calls, cp)
}

func (n *fakeNotifier) notifications() [][]NewItem {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([][]NewItem(nil), n.calls...)
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

	em := &fakeEmitter{}
	s := New(st, ft, WithEmitter(em))
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
	if em.count() != 1 || em.events[0].name != FeedErrorEvent {
		t.Fatalf("expected one %q event, got %+v", FeedErrorEvent, em.events)
	}
	if data, ok := em.events[0].data.(map[string]any); !ok || data["feedId"] != int64(1) {
		t.Errorf("feed:error payload wrong: %+v", em.events[0].data)
	}
}

func TestTickFiltersBackoffAndFallsBackToDefault(t *testing.T) {
	fixed := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	recent := fixed.Add(-1 * time.Minute)
	old := fixed.Add(-200 * time.Hour)

	st := newFakeStore()
	st.forUpdate = []store.Feed{
		{ID: 1, URL: "u1", UpdateInterval: 30, ErrorCount: 0},                         // 到期
		{ID: 2, URL: "u2", UpdateInterval: 0},                                         // 回退到 DefaultInterval(30m) -> 到期
		{ID: 3, URL: "u3", UpdateInterval: 30, ErrorCount: 5, LastAttempted: &recent}, // 退避中 -> 跳过
		{ID: 4, URL: "u4", UpdateInterval: 30, ErrorCount: 2, LastAttempted: &old},    // 退避已过 -> 到期
	}
	for _, fd := range st.forUpdate {
		st.addFeed(fd)
	}

	ft := newFakeFetcher()
	ft.feeds["u1"] = &fetcher.ParsedFeed{}
	ft.feeds["u2"] = &fetcher.ParsedFeed{}
	ft.feeds["u4"] = &fetcher.ParsedFeed{}

	s := New(st, ft,
		withClock(func() time.Time { return fixed }),
		WithConfig(Config{DefaultInterval: 30 * time.Minute}),
	)
	results := s.Tick(context.Background())

	if len(results) != 3 {
		t.Fatalf("due feeds = %d, want 3 (%+v)", len(results), results)
	}
	got := map[int64]bool{}
	for _, r := range results {
		got[r.FeedID] = true
	}
	if !got[1] || !got[2] || !got[4] {
		t.Errorf("expected feeds 1, 2, 4 to be refreshed, got %+v", got)
	}
	if got[3] {
		t.Errorf("backoff feed 3 should be skipped, got %+v", got)
	}
}

func TestTickManualWhenDefaultIsZero(t *testing.T) {
	fixed := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	st := newFakeStore()
	st.forUpdate = []store.Feed{
		{ID: 1, URL: "u1", UpdateInterval: 0}, // DefaultInterval=0 -> 手动，跳过
	}
	st.addFeed(st.forUpdate[0])

	ft := newFakeFetcher()
	ft.feeds["u1"] = &fetcher.ParsedFeed{}

	s := New(st, ft,
		withClock(func() time.Time { return fixed }),
		WithConfig(Config{DefaultInterval: 0}),
	)
	results := s.Tick(context.Background())

	if len(results) != 0 {
		t.Fatalf("due feeds = %d, want 0 (manual) (%+v)", len(results), results)
	}
}

func TestIsDueUsesLastAttemptedForBackoffAndDefaultInterval(t *testing.T) {
	fixed := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	recent := fixed.Add(-time.Minute)
	s := New(newFakeStore(), newFakeFetcher(),
		withClock(func() time.Time { return fixed }),
		WithConfig(Config{DefaultInterval: 30 * time.Minute}),
	)

	if s.isDue(store.Feed{UpdateInterval: 30, ErrorCount: 1, LastAttempted: &recent}) {
		t.Fatal("a never-successful feed must back off from its latest attempt")
	}
	if s.isDue(store.Feed{UpdateInterval: 0, LastAttempted: &recent}) {
		t.Fatal("a feed using the global interval must not refresh every poll")
	}
	s.SetDefaultInterval(0)
	if s.isDue(store.Feed{UpdateInterval: 0}) {
		t.Fatal("setting the global interval to zero must enable manual mode immediately")
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

func TestConcurrencyLimitIsSharedAcrossRefreshBatches(t *testing.T) {
	st := newFakeStore()
	ft := newFakeFetcher()
	ft.delay = 20 * time.Millisecond
	for i := 1; i <= 8; i++ {
		id := int64(i)
		url := fmt.Sprintf("u%d", i)
		st.addFeed(store.Feed{ID: id, URL: url, Status: "active", UpdateInterval: 30})
		ft.feeds[url] = &fetcher.ParsedFeed{}
	}

	s := New(st, ft, WithConfig(Config{Concurrency: 3}))
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.RefreshAll(context.Background())
		}()
	}
	wg.Wait()
	if max := atomic.LoadInt32(&ft.maxConcurr); max > 3 {
		t.Fatalf("shared max concurrency = %d, want <= 3", max)
	}
}

func TestSameFeedRefreshesAreSerialized(t *testing.T) {
	st := newFakeStore()
	st.addFeed(store.Feed{ID: 1, URL: "u1", Status: "active", UpdateInterval: 30})
	ft := newFakeFetcher()
	ft.delay = 20 * time.Millisecond
	ft.feeds["u1"] = &fetcher.ParsedFeed{}
	s := New(st, ft, WithConfig(Config{Concurrency: 5}))

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.RefreshFeed(context.Background(), 1)
		}()
	}
	wg.Wait()
	if max := atomic.LoadInt32(&ft.maxConcurr); max != 1 {
		t.Fatalf("same-feed max concurrency = %d, want 1", max)
	}
}

func TestPersistenceFailureIsReported(t *testing.T) {
	st := newFakeStore()
	st.applyErr = errors.New("disk full")
	st.addFeed(store.Feed{ID: 1, URL: "u1", Status: "active", UpdateInterval: 30})
	ft := newFakeFetcher()
	ft.feeds["u1"] = &fetcher.ParsedFeed{Items: []fetcher.ParsedItem{{GUID: "g1", Link: "https://x/1"}}}
	em := &fakeEmitter{}
	s := New(st, ft, WithEmitter(em))

	result, _ := s.RefreshFeed(context.Background(), 1)
	if result.Err == nil || !errors.Is(result.Err, st.applyErr) {
		t.Fatalf("persistence error was hidden: %+v", result)
	}
	if st.errorUpdates[1] != 1 {
		t.Fatalf("persistence failure attempts = %d, want 1", st.errorUpdates[1])
	}
	if em.count() != 1 || em.events[0].name != FeedErrorEvent {
		t.Fatalf("expected persistence failure event, got %+v", em.events)
	}
}

func TestNotModifiedStateFailureIsReported(t *testing.T) {
	st := newFakeStore()
	st.markErr = errors.New("database unavailable")
	st.addFeed(store.Feed{ID: 1, URL: "u1", Status: "active", UpdateInterval: 30})
	ft := newFakeFetcher()
	ft.notModified["u1"] = true
	s := New(st, ft)

	result, _ := s.RefreshFeed(context.Background(), 1)
	if result.Err == nil || !errors.Is(result.Err, st.markErr) {
		t.Fatalf("304 state error was hidden: %+v", result)
	}
	if st.errorUpdates[1] != 1 {
		t.Fatalf("304 state failure attempts = %d, want 1", st.errorUpdates[1])
	}
}

func TestFailureRecordingErrorIsJoined(t *testing.T) {
	st := newFakeStore()
	st.recordErr = errors.New("cannot record attempt")
	st.addFeed(store.Feed{ID: 1, URL: "u1", Status: "active", UpdateInterval: 30})
	fetchErr := errors.New("network down")
	ft := newFakeFetcher()
	ft.errs["u1"] = fetchErr
	s := New(st, ft)

	result, _ := s.RefreshFeed(context.Background(), 1)
	if !errors.Is(result.Err, fetchErr) || !errors.Is(result.Err, st.recordErr) {
		t.Fatalf("joined refresh error = %v, want fetch and persistence causes", result.Err)
	}
}

func TestOfflineModeBlocksNetworkRequests(t *testing.T) {
	st := newFakeStore()
	feed := store.Feed{ID: 1, URL: "u1", Status: "active", UpdateInterval: 30}
	st.addFeed(feed)
	st.forUpdate = []store.Feed{feed}
	ft := newFakeFetcher()
	ft.feeds["u1"] = &fetcher.ParsedFeed{}
	s := New(st, ft)
	s.SetOfflineMode(true)

	if results := s.Tick(context.Background()); len(results) != 0 {
		t.Fatalf("offline tick returned %+v", results)
	}
	result, _ := s.RefreshFeed(context.Background(), 1)
	if !errors.Is(result.Err, ErrOffline) {
		t.Fatalf("offline refresh error = %v, want %v", result.Err, ErrOffline)
	}
	if calls := atomic.LoadInt32(&ft.condCalls); calls != 0 {
		t.Fatalf("offline fetch calls = %d, want 0", calls)
	}
}

func TestReturningOnlineTriggersImmediateDueScan(t *testing.T) {
	st := newFakeStore()
	feed := store.Feed{ID: 1, URL: "u1", Status: "active", UpdateInterval: 30}
	st.addFeed(feed)
	st.forUpdate = []store.Feed{feed}
	ft := newFakeFetcher()
	ft.feeds["u1"] = &fetcher.ParsedFeed{}
	s := New(st, ft, WithConfig(Config{PollInterval: time.Hour}))
	s.SetOfflineMode(true)
	s.Start(context.Background())
	t.Cleanup(s.Stop)

	s.SetOfflineMode(false)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt32(&ft.condCalls) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if calls := atomic.LoadInt32(&ft.condCalls); calls != 1 {
		t.Fatalf("online resume fetch calls = %d, want 1", calls)
	}
}

func TestGoingOfflineStopsQueuedFetches(t *testing.T) {
	st := newFakeStore()
	ft := newFakeFetcher()
	ft.delay = 100 * time.Millisecond
	for i := 1; i <= 2; i++ {
		id := int64(i)
		url := fmt.Sprintf("u%d", i)
		st.addFeed(store.Feed{ID: id, URL: url, Status: "active", UpdateInterval: 30})
		ft.feeds[url] = &fetcher.ParsedFeed{}
	}
	s := New(st, ft, WithConfig(Config{Concurrency: 1}))
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = s.RefreshAll(context.Background())
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt32(&ft.current) == 0 {
		time.Sleep(time.Millisecond)
	}
	s.SetOfflineMode(true)
	<-done
	if calls := atomic.LoadInt32(&ft.condCalls); calls != 1 {
		t.Fatalf("fetches started after going offline = %d, want only the in-flight request", calls)
	}
}

func TestStartStopRunsStartupRefresh(t *testing.T) {
	st := newFakeStore() // no feeds → startup due scan does no fetching
	s := New(st, newFakeFetcher(), WithConfig(Config{PollInterval: time.Hour}))

	s.Start(context.Background())
	// 启动会立即触发到期扫描；等待其执行。
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt32(&st.listCalls) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	s.Stop()

	if atomic.LoadInt32(&st.listCalls) == 0 {
		t.Error("expected at least one startup due scan")
	}
	// 重复 Stop 不应 panic。
	s.Stop()
}

func TestStartupScanRespectsManualMode(t *testing.T) {
	st := newFakeStore()
	feed := store.Feed{ID: 1, URL: "u1", Status: "active", UpdateInterval: 0}
	st.addFeed(feed)
	st.forUpdate = []store.Feed{feed}
	ft := newFakeFetcher()
	ft.feeds["u1"] = &fetcher.ParsedFeed{}
	s := New(st, ft, WithConfig(Config{PollInterval: time.Hour, DefaultInterval: 0}))

	s.Start(context.Background())
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt32(&st.listCalls) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	s.Stop()
	if calls := atomic.LoadInt32(&ft.condCalls); calls != 0 {
		t.Fatalf("manual-mode startup fetch calls = %d, want 0", calls)
	}
}

func TestPrunedItemsAreNotReportedAsNewAgain(t *testing.T) {
	st, err := store.NewWithPath(filepath.Join(t.TempDir(), "clip.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	feed := &store.Feed{URL: "u1", Title: "feed", Status: "active", UpdateInterval: 30, MaxItems: 2}
	if err := st.CreateFeed(feed); err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ft := newFakeFetcher()
	ft.feeds["u1"] = &fetcher.ParsedFeed{Items: []fetcher.ParsedItem{
		{GUID: "g3", Title: "three", Link: "https://x/3", Published: base.Add(3 * time.Hour)},
		{GUID: "g2", Title: "two", Link: "https://x/2", Published: base.Add(2 * time.Hour)},
		{GUID: "g1", Title: "one", Link: "https://x/1", Published: base.Add(time.Hour)},
	}}
	notifier := &fakeNotifier{}
	s := New(st, ft, WithNotifier(notifier))
	first, _ := s.RefreshFeed(context.Background(), feed.ID)
	if first.Err != nil || first.NewItems != 2 {
		t.Fatalf("first refresh = %+v, want two retained new items", first)
	}

	second, _ := s.RefreshFeed(context.Background(), feed.ID)
	if second.Err != nil || second.NewItems != 0 {
		t.Fatalf("second refresh = %+v, pruned history must not reappear", second)
	}

	ft.feeds["u1"] = &fetcher.ParsedFeed{Items: append(
		[]fetcher.ParsedItem{{GUID: "g4", Title: "four", Link: "https://x/4", Published: base.Add(4 * time.Hour)}},
		ft.feeds["u1"].Items...,
	)}
	third, _ := s.RefreshFeed(context.Background(), feed.ID)
	if third.Err != nil || third.NewItems != 1 {
		t.Fatalf("third refresh = %+v, want only the genuinely new item", third)
	}
	notifications := notifier.notifications()
	if len(notifications) != 1 || len(notifications[0]) != 1 || notifications[0][0].Title != "four" {
		t.Fatalf("notifications = %+v, want only the genuinely new item", notifications)
	}
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
