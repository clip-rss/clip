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
