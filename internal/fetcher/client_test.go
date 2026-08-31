package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientConditionalGET(t *testing.T) {
	const etag = `"v1"`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
		_, _ = w.Write([]byte("<rss></rss>"))
	}))
	defer srv.Close()

	c := NewClient()
	ctx := context.Background()

	// 首次请求：无条件头，应返回 200 与 ETag。
	res, err := c.Fetch(ctx, srv.URL, ConditionalHeaders{})
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if res.NotModified || res.StatusCode != 200 {
		t.Fatalf("first fetch unexpected: %+v", res)
	}
	if res.ETag != etag {
		t.Fatalf("etag = %q, want %q", res.ETag, etag)
	}

	// 第二次：携带 ETag，服务器应返回 304。
	res2, err := c.Fetch(ctx, srv.URL, ConditionalHeaders{ETag: res.ETag})
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if !res2.NotModified {
		t.Fatalf("expected 304 NotModified, got %+v", res2)
	}
	if len(res2.Body) != 0 {
		t.Errorf("304 body should be empty, got %d bytes", len(res2.Body))
	}
}

func TestClientRetryOn5xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient(WithMaxRetry(2))
	res, err := c.Fetch(context.Background(), srv.URL, ConditionalHeaders{})
	if err != nil {
		t.Fatalf("fetch with retry: %v", err)
	}
	if string(res.Body) != "ok" {
		t.Errorf("body = %q", res.Body)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("server calls = %d, want 2", got)
	}
}

func TestClientNoRetryOn4xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(WithMaxRetry(3))
	if _, err := c.Fetch(context.Background(), srv.URL, ConditionalHeaders{}); err == nil {
		t.Fatal("expected error on 404")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("4xx should not retry: calls = %d, want 1", got)
	}
}

func TestClientUserAgent(t *testing.T) {
	const ua = "TestAgent/9.9"
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.UserAgent()
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient(WithUserAgent(ua), WithTimeout(5*time.Second))
	if _, err := c.Fetch(context.Background(), srv.URL, ConditionalHeaders{}); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if v := <-got; v != ua {
		t.Errorf("User-Agent = %q, want %q", v, ua)
	}
}

func TestDefaultUserAgentIsBrowserLike(t *testing.T) {
	if !strings.Contains(DefaultUserAgent, "Mozilla/5.0") {
		t.Errorf("DefaultUserAgent should contain Mozilla/5.0, got %q", DefaultUserAgent)
	}
	if !strings.Contains(DefaultUserAgent, "Chrome/") {
		t.Errorf("DefaultUserAgent should contain Chrome/, got %q", DefaultUserAgent)
	}
	if strings.Contains(DefaultUserAgent, "Clip/") {
		t.Errorf("DefaultUserAgent should not contain Clip/, got %q", DefaultUserAgent)
	}
}

func TestClientSendsBrowserHeaders(t *testing.T) {
	var gotUA, gotAccept, gotAcceptLang, gotCacheControl string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		gotAcceptLang = r.Header.Get("Accept-Language")
		gotCacheControl = r.Header.Get("Cache-Control")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient()
	if _, err := c.Fetch(context.Background(), srv.URL, ConditionalHeaders{}); err != nil {
		t.Fatalf("fetch: %v", err)
	}

	if gotUA == "" {
		t.Error("User-Agent should not be empty")
	}
	if strings.Contains(gotUA, "Clip/") {
		t.Errorf("User-Agent should not be the old Clip UA, got %q", gotUA)
	}

	if !strings.Contains(gotAccept, "text/html") {
		t.Errorf("Accept should contain text/html, got %q", gotAccept)
	}

	if gotAcceptLang == "" {
		t.Error("Accept-Language should not be empty")
	}

	if gotCacheControl == "" {
		t.Error("Cache-Control should not be empty")
	}
}

func TestClientGetDownloadsBinary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 'P', 'N', 'G'})
	}))
	defer srv.Close()

	c := NewClient()
	body, ct, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ct != "image/png" {
		t.Errorf("content-type = %q, want image/png", ct)
	}
	if string(body) != string([]byte{0x89, 'P', 'N', 'G'}) {
		t.Errorf("body = %v", body)
	}
}

func TestClientGetRetriesOn5xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("img"))
	}))
	defer srv.Close()

	c := NewClient(WithMaxRetry(2))
	body, _, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("get with retry: %v", err)
	}
	if string(body) != "img" {
		t.Errorf("body = %q", body)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("server calls = %d, want 2", got)
	}
}

func TestClientGetNoRetryOn4xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewClient(WithMaxRetry(3))
	if _, _, err := c.Get(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error on 403")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("4xx should not retry: calls = %d, want 1", got)
	}
}
