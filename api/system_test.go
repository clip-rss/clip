package api

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/store"
)

// changelogServer 起一个返回固定 Markdown 的测试服务端，并记录被请求的次数。
func changelogServer(t *testing.T, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// newChangelogService 组一个带缓存能力的 SystemService，noUpdate 由调用方通过指针控制。
func newChangelogService(t *testing.T, url string, noUpdate *bool) *SystemService {
	t.Helper()
	return &SystemService{
		AppVersion:   "0.2.0",
		ChangelogURL: url,
		Store:        newTestStore(t),
		NoUpdateFn:   func() bool { return *noUpdate },
		LanguageFn:   func() string { return "en" },
		HTTPClient:   newTestChangelogClient(),
	}
}

// newTestChangelogClient 造一个不重试的抓取客户端。
//
// 生产配置带 2 次重试，用在这些用例里会让「命中服务端几次」的断言全部偏移，
// 而且失败路径要真等退避睡眠。重试本身已由 fetcher 包自己的测试覆盖。
func newTestChangelogClient() *fetcher.Client {
	return fetcher.NewClient(fetcher.WithMaxRetry(0))
}

// TestFetchChangelogServesCacheWhenNoUpdate 确认无新版时第二次调用不再走网络。
// 这是本功能的核心断言。
func TestFetchChangelogServesCacheWhenNoUpdate(t *testing.T) {
	const body = "## 0.2.0\n\n### 新增\n- 甲"
	srv, hits := changelogServer(t, body)
	noUpdate := true
	svc := newChangelogService(t, srv.URL, &noUpdate)

	first, err := svc.FetchChangelog()
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	second, err := svc.FetchChangelog()
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}

	if first != body || second != body {
		t.Errorf("content mismatch: first=%q second=%q want %q", first, second, body)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("server hits = %d, want 1 (second call must come from cache)", got)
	}
}

// TestFetchChangelogAlwaysFetchesWhenUpdateAvailable 有新版（或尚未检查）时必须每次都抓，
// 且不得写缓存——那时抓到的是新版日志，缓存下来会在升级后被当作当前版本的日志复用。
func TestFetchChangelogAlwaysFetchesWhenUpdateAvailable(t *testing.T) {
	srv, hits := changelogServer(t, "## 0.3.0")
	noUpdate := false
	svc := newChangelogService(t, srv.URL, &noUpdate)

	for i := 0; i < 3; i++ {
		if _, err := svc.FetchChangelog(); err != nil {
			t.Fatalf("fetch %d: %v", i, err)
		}
	}
	if got := hits.Load(); got != 3 {
		t.Errorf("server hits = %d, want 3", got)
	}

	if _, found, err := svc.Store.GetChangelogCache(); err != nil {
		t.Fatal(err)
	} else if found {
		t.Error("must not cache while an update is pending")
	}
}

// TestFetchChangelogIgnoresCacheFromOtherVersion 升级后旧版缓存必须失效。
func TestFetchChangelogIgnoresCacheFromOtherVersion(t *testing.T) {
	const body = "## 0.3.0"
	srv, hits := changelogServer(t, body)
	noUpdate := true
	svc := newChangelogService(t, srv.URL, &noUpdate)

	if err := svc.Store.SaveChangelogCache(store.ChangelogCache{
		Version:  "0.1.0", // 上一版留下的缓存
		Markdown: "## 0.1.0",
	}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.FetchChangelog()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got != body {
		t.Errorf("content = %q, want %q (stale cache must be ignored)", got, body)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("server hits = %d, want 1", n)
	}

	// 抓取后应改写成当前版本的缓存
	cache, found, err := svc.Store.GetChangelogCache()
	if err != nil {
		t.Fatal(err)
	}
	if !found || cache.Version != "0.2.0" || cache.Markdown != body {
		t.Errorf("cache after refetch = %+v (found=%v), want version 0.2.0 with fresh body", cache, found)
	}
}

// TestFetchChangelogFallsBackToCacheOnFailure 抓取失败（离线/5xx）时回退展示缓存。
func TestFetchChangelogFallsBackToCacheOnFailure(t *testing.T) {
	const cachedBody = "## 0.2.0\n\ncached"
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)

	noUpdate := false // 未确认无新版 → 不吃缓存快路径，一定会尝试抓取
	svc := newChangelogService(t, failing.URL, &noUpdate)
	if err := svc.Store.SaveChangelogCache(store.ChangelogCache{
		Version:  "0.2.0",
		Markdown: cachedBody,
	}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.FetchChangelog()
	if err != nil {
		t.Fatalf("should fall back to cache, got error: %v", err)
	}
	if got != cachedBody {
		t.Errorf("content = %q, want cached %q", got, cachedBody)
	}
}

// TestFetchChangelogFailureWithoutCacheReturnsError 没有缓存时失败仍要如实报错。
func TestFetchChangelogFailureWithoutCacheReturnsError(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)

	noUpdate := true
	svc := newChangelogService(t, failing.URL, &noUpdate)

	if _, err := svc.FetchChangelog(); err == nil {
		t.Fatal("expected an error when fetch fails and no cache exists")
	}
}

// TestFetchChangelogWithoutStore Store 为 nil 时行为与加缓存前一致：每次都抓，不 panic。
func TestFetchChangelogWithoutStore(t *testing.T) {
	const body = "## 0.2.0"
	srv, hits := changelogServer(t, body)
	svc := &SystemService{
		AppVersion:   "0.2.0",
		ChangelogURL: srv.URL,
		NoUpdateFn:   func() bool { return true },
		LanguageFn:   func() string { return "en" },
		HTTPClient:   newTestChangelogClient(),
	}

	for i := 0; i < 2; i++ {
		got, err := svc.FetchChangelog()
		if err != nil {
			t.Fatalf("fetch %d: %v", i, err)
		}
		if got != body {
			t.Errorf("content = %q, want %q", got, body)
		}
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("server hits = %d, want 2 (no store means no caching)", n)
	}
}

// TestFetchChangelogWithoutNoUpdateFn NoUpdateFn 为 nil（未注入）时按「未确认」处理：
// 照常抓取但不落缓存，避免把未经校验的内容长期钉住。
func TestFetchChangelogWithoutNoUpdateFn(t *testing.T) {
	srv, hits := changelogServer(t, "## 0.2.0")
	svc := &SystemService{
		AppVersion:   "0.2.0",
		ChangelogURL: srv.URL,
		Store:        newTestStore(t),
		LanguageFn:   func() string { return "en" },
		HTTPClient:   newTestChangelogClient(),
	}

	if _, err := svc.FetchChangelog(); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if _, err := svc.FetchChangelog(); err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("server hits = %d, want 2", n)
	}
	if _, found, err := svc.Store.GetChangelogCache(); err != nil {
		t.Fatal(err)
	} else if found {
		t.Error("must not cache when no-update state is unknown")
	}
}

// TestFetchChangelogRecoversFromCorruptCache 缓存损坏时忽略它重新抓取，而不是把错误抛给用户。
func TestFetchChangelogRecoversFromCorruptCache(t *testing.T) {
	const body = "## 0.2.0\n\nfresh"
	srv, hits := changelogServer(t, body)
	noUpdate := true
	svc := newChangelogService(t, srv.URL, &noUpdate)

	// 直接写入无法解析的值，模拟外部改写/损坏。
	if err := svc.Store.SetJSONSetting("changelog_cache", "not-a-cache-object"); err != nil {
		t.Fatal(err)
	}

	got, err := svc.FetchChangelog()
	if err != nil {
		t.Fatalf("corrupt cache should self-heal by refetching, got: %v", err)
	}
	if got != body {
		t.Errorf("content = %q, want %q", got, body)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("server hits = %d, want 1", n)
	}
}

// TestFetchChangelogNotConfigured 未配置 URL 时仍按语言返回本地化错误（回归保护）。
func TestFetchChangelogNotConfigured(t *testing.T) {
	noUpdate := true
	svc := newChangelogService(t, "", &noUpdate)
	if _, err := svc.FetchChangelog(); err == nil || err.Error() != "Changelog URL is not configured" {
		t.Fatalf("error = %v, want the localized not-configured message", err)
	}
}
