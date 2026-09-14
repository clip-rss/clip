package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const samplePage = `<!DOCTYPE html><html><head>
<title>Blog</title>
<link rel="alternate" type="application/rss+xml" title="RSS Feed" href="/feed.xml">
<link rel="alternate" type="application/atom+xml" title="Atom Feed" href="https://cdn.example.com/atom.xml">
<link rel="icon" href="/static/favicon.png">
<link rel="stylesheet" href="/style.css">
</head><body>hi</body></html>`

func TestDiscoverFeeds(t *testing.T) {
	feeds := DiscoverFeeds([]byte(samplePage), "https://example.com/blog/")
	if len(feeds) != 2 {
		t.Fatalf("feeds = %d, want 2 (%+v)", len(feeds), feeds)
	}
	if feeds[0].URL != "https://example.com/feed.xml" {
		t.Errorf("relative feed not resolved: %q", feeds[0].URL)
	}
	if feeds[0].Type != "application/rss+xml" || feeds[0].Title != "RSS Feed" {
		t.Errorf("feed meta wrong: %+v", feeds[0])
	}
	if feeds[1].URL != "https://cdn.example.com/atom.xml" {
		t.Errorf("absolute feed url wrong: %q", feeds[1].URL)
	}
}

func TestDiscoverFeedsNone(t *testing.T) {
	if feeds := DiscoverFeeds([]byte("<html><body>no feeds</body></html>"), "https://x.com"); len(feeds) != 0 {
		t.Errorf("expected no feeds, got %+v", feeds)
	}
}

func TestDiscover(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(samplePage))
	}))
	defer srv.Close()

	feeds, err := New().Discover(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(feeds) != 2 {
		t.Fatalf("feeds = %d, want 2 (%+v)", len(feeds), feeds)
	}
	// 相对地址应基于页面 URL 解析为绝对地址。
	if feeds[0].URL != srv.URL+"/feed.xml" {
		t.Errorf("relative feed not resolved: %q, want %q", feeds[0].URL, srv.URL+"/feed.xml")
	}
}

func TestDiscoverFavicon(t *testing.T) {
	got := DiscoverFavicon([]byte(samplePage), "https://example.com/blog/")
	if got != "https://example.com/static/favicon.png" {
		t.Errorf("favicon = %q", got)
	}
}

func TestDiscoverFaviconFallback(t *testing.T) {
	got := DiscoverFavicon([]byte("<html><head></head><body></body></html>"), "https://example.com/blog/")
	if got != "https://example.com/favicon.ico" {
		t.Errorf("fallback favicon = %q, want https://example.com/favicon.ico", got)
	}
}

// 403 页面照样返回：站点用 403 拦住爬虫/未登录请求是常态，
// 而那个页面本身就带着站点的图标声明，当失败丢掉就白瞎了。
func TestFetchPageKeepsBodyOnClientError(t *testing.T) {
	page := []byte(`<html><head><link rel="icon" href="/a.png"></head><body>403</body></html>`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write(page)
	}))
	defer srv.Close()

	f := New(WithClient(NewClient(WithMaxRetry(0))))
	body, err := f.FetchPage(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchPage = %v, want 403 页面原样返回", err)
	}
	if string(body) != string(page) {
		t.Errorf("body = %.60q, want 403 页面", body)
	}
}

// ResolveFavicon 必须用上 403 页面里的图标声明。
// 网页请求 403（而 /favicon.ico 返回 204 空响应），图标只存在于那个页面里。
func TestResolveFaviconFromForbiddenPage(t *testing.T) {
	const icon = "data:image/svg+xml;base64,PHN2Zy8+"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<html><head><link rel="icon" href="` + icon + `"></head></html>`))
	}))
	defer srv.Close()

	f := New(WithClient(NewClient(WithMaxRetry(0))))
	if got := f.ResolveFavicon(context.Background(), "", srv.URL); got != icon {
		t.Errorf("ResolveFavicon = %q, want %q（应取自 403 页面，而非 /favicon.ico 兜底）", got, icon)
	}
}

// 抓 Feed 那条路不受上面改动影响：4xx 仍算失败，不得把错误页当 Feed 解析。
func TestFetchStillFailsOnClientError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<html><body>nope</body></html>"))
	}))
	defer srv.Close()

	f := New(WithClient(NewClient(WithMaxRetry(0))))
	res, err := f.Client().Fetch(context.Background(), srv.URL, ConditionalHeaders{})
	if err == nil || res != nil {
		t.Fatalf("Fetch = (%+v, %v), want (nil, error)", res, err)
	}
}
