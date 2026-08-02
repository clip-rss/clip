package fetcher

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchFeedEndToEnd(t *testing.T) {
	const etag = `"feed-1"`
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	f := New()
	ctx := context.Background()

	feed, res, err := f.FetchFeed(ctx, srv.URL)
	if err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}
	if res.NotModified {
		t.Fatal("first fetch should not be 304")
	}
	if feed == nil || len(feed.Items) != 2 {
		t.Fatalf("feed items = %v", feed)
	}
	// 正文应被清洗（CDATA 中的 HTML 标签保留为安全标签）。
	if feed.Items[0].Content == "" {
		t.Error("content should be present after clean")
	}
	if feed.Items[0].Summary == "" {
		t.Error("summary should be generated")
	}

	// 第二次抓取：Fetcher 应复用缓存的 ETag 并得到 304。
	feed2, res2, err := f.FetchFeed(ctx, srv.URL)
	if err != nil {
		t.Fatalf("second FetchFeed: %v", err)
	}
	if !res2.NotModified {
		t.Fatalf("second fetch should be 304, got %+v", res2)
	}
	if feed2 != nil {
		t.Error("feed should be nil on 304")
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("server calls = %d, want 2", got)
	}
}

func TestFetchManyConcurrencyLimit(t *testing.T) {
	var current, max int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&current, 1)
		for {
			old := atomic.LoadInt32(&max)
			if cur <= old || atomic.CompareAndSwapInt32(&max, old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&current, -1)
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	const limit = 2
	f := New(WithConcurrency(limit))
	urls := make([]string, 6)
	for i := range urls {
		// 不同 query 以避免共享条件 GET 缓存。
		urls[i] = fmt.Sprintf("%s/?n=%d", srv.URL, i)
	}

	results := f.FetchMany(context.Background(), urls)
	if len(results) != len(urls) {
		t.Fatalf("results = %d, want %d", len(results), len(urls))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("result %d error: %v", i, r.Err)
		}
		if r.Feed == nil || len(r.Feed.Items) != 2 {
			t.Errorf("result %d feed wrong: %+v", i, r.Feed)
		}
	}
	if got := atomic.LoadInt32(&max); got > limit {
		t.Errorf("max concurrency observed = %d, exceeds limit %d", got, limit)
	}
	if atomic.LoadInt32(&max) == 0 {
		t.Error("expected some concurrency to be observed")
	}
}

func TestBackoff(t *testing.T) {
	base := time.Minute
	if got := Backoff(0, base); got != base {
		t.Errorf("Backoff(0) = %v, want %v", got, base)
	}
	if got := Backoff(1, base); got != base {
		t.Errorf("Backoff(1) = %v, want %v", got, base)
	}
	if got := Backoff(2, base); got != 2*base {
		t.Errorf("Backoff(2) = %v, want %v", got, 2*base)
	}
	if got := Backoff(3, base); got != 4*base {
		t.Errorf("Backoff(3) = %v, want %v", got, 4*base)
	}
	if got := Backoff(100, base); got != maxBackoff {
		t.Errorf("Backoff(100) = %v, want cap %v", got, maxBackoff)
	}
	// base 非法时回退到 1 分钟。
	if got := Backoff(1, 0); got != time.Minute {
		t.Errorf("Backoff with zero base = %v, want 1m", got)
	}
}

func TestCleanFeedResolvesRelativeLinks(t *testing.T) {
	feed := &ParsedFeed{
		Link: "https://example.com/blog/",
		Items: []ParsedItem{
			{Title: "a", Link: "post/1", Content: "<p>x</p>", Enclosure: "/files/a.mp3"},
		},
	}
	CleanFeed(feed)
	if feed.Items[0].Link != "https://example.com/blog/post/1" {
		t.Errorf("relative link not resolved: %q", feed.Items[0].Link)
	}
	if feed.Items[0].Enclosure != "https://example.com/files/a.mp3" {
		t.Errorf("relative enclosure not resolved: %q", feed.Items[0].Enclosure)
	}
}

func TestParseFailureDoesNotCacheConditionalHeaders(t *testing.T) {
	const etag = `"broken-v1"`
	requests := make([]string, 0, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Header.Get("If-None-Match"))
		w.Header().Set("ETag", etag)
		_, _ = w.Write([]byte("this is not a feed"))
	}))
	defer srv.Close()

	f := New()
	for range 2 {
		if _, _, err := f.FetchFeed(context.Background(), srv.URL); err == nil {
			t.Fatal("malformed feed should fail parsing")
		}
	}
	if len(requests) != 2 || requests[0] != "" || requests[1] != "" {
		t.Fatalf("conditional headers after parse failure = %#v, want two empty values", requests)
	}
}
