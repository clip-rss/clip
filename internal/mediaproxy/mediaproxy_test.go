package mediaproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/clip-rss/clip/internal/fetcher"
)

// articleURL 是正文图片请求要伪装成的来源：代理必须用它的**源站**作 Referer。
const articleURL = "https://sspai.com/post/114823"

// newProxyWithCDN 起一个假的图片 CDN，行为对齐实测到的少数派 CDN：
// **没有 Referer 就 403，有任意非空 Referer 才给图**。
// 返回的计数用于断言缓存是否真的挡住了第二次请求。
func newProxyWithCDN(t *testing.T) (*Proxy, string, *int32) {
	t.Helper()
	var hits int32
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("Referer") == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("PNGDATA"))
	}))
	t.Cleanup(cdn.Close)

	client := fetcher.NewClient(fetcher.WithHTTPClient(&http.Client{Timeout: 5 * time.Second}))
	return New(client), cdn.URL, &hits
}

// get 走一遍中间件。落在 next 上视为失败：命中代理路径的请求不该被放过。
func get(t *testing.T, p *Proxy, target, referer string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		PathPrefix+"?u="+url.QueryEscape(target)+"&r="+url.QueryEscape(referer), nil)
	p.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("请求不该落到 next（应被代理接管）")
	})).ServeHTTP(rec, req)
	return rec
}

func TestProxySendsArticleOriginAsReferer(t *testing.T) {
	p, cdnURL, hits := newProxyWithCDN(t)

	rec := get(t, p, cdnURL+"/a.png", articleURL)

	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200（说明 Referer 没带上）", rec.Code)
	}
	if got := rec.Body.String(); got != "PNGDATA" {
		t.Errorf("响应体 = %q, 期望 %q", got, "PNGDATA")
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, 期望 image/png", got)
	}
	if n := atomic.LoadInt32(hits); n != 1 {
		t.Errorf("上游被请求 %d 次, 期望 1 次", n)
	}
}

// 核心回归：没有这层代理时，图片请求不带 Referer，CDN 一律 403。
func TestProxyRefererIsRequiredUpstream(t *testing.T) {
	p, cdnURL, _ := newProxyWithCDN(t)

	// r 为空 → 伪造不出 Referer → 上游 403 应如实透传（而不是被吞成成功）。
	rec := get(t, p, cdnURL+"/a.png", "")

	if rec.Code != http.StatusForbidden {
		t.Errorf("状态码 = %d, 期望 403（上游无 Referer 时的真实响应应透传）", rec.Code)
	}
}

func TestProxyCachesImages(t *testing.T) {
	p, cdnURL, hits := newProxyWithCDN(t)

	for i := 0; i < 2; i++ {
		if rec := get(t, p, cdnURL+"/a.png", articleURL); rec.Code != http.StatusOK {
			t.Fatalf("第 %d 次请求状态码 = %d, 期望 200", i+1, rec.Code)
		}
	}

	if n := atomic.LoadInt32(hits); n != 1 {
		t.Errorf("上游被请求 %d 次, 期望 1 次（第二次应命中内存缓存）", n)
	}
}

func TestProxyIncludesRefererInCacheKey(t *testing.T) {
	p, cdnURL, hits := newProxyWithCDN(t)

	get(t, p, cdnURL+"/a.png", articleURL)
	get(t, p, cdnURL+"/a.png", "https://other.example.com/post/1")

	// 同一张图、不同源站 Referer 是两条不同的缓存条目：防盗链结果可能不同，
	// 合并会串味。
	if n := atomic.LoadInt32(hits); n != 2 {
		t.Errorf("上游被请求 %d 次, 期望 2 次（Referer 不同不应复用缓存）", n)
	}
}

func TestProxyForwardsRangeAndPartialStatus(t *testing.T) {
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=0-3" {
			t.Errorf("上游收到的 Range = %q, 期望 bytes=0-3", r.Header.Get("Range"))
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Range", "bytes 0-3/10")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("ABCD"))
	}))
	defer cdn.Close()

	client := fetcher.NewClient(fetcher.WithHTTPClient(&http.Client{Timeout: 5 * time.Second}))
	p := New(client)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		PathPrefix+"?u="+url.QueryEscape(cdn.URL+"/v.mp4")+"&r="+url.QueryEscape(articleURL), nil)
	req.Header.Set("Range", "bytes=0-3")
	p.Middleware(http.NotFoundHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("状态码 = %d, 期望 206", rec.Code)
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 0-3/10" {
		t.Errorf("Content-Range = %q, 期望 bytes 0-3/10", got)
	}
	if got := rec.Body.String(); got != "ABCD" {
		t.Errorf("响应体 = %q, 期望 ABCD", got)
	}
}

func TestProxyStreamsUncachedLargeBody(t *testing.T) {
	// 长度未知（chunked）时不进缓存，但仍应完整转发。
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, strings.Repeat("x", 1024))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer cdn.Close()

	client := fetcher.NewClient(fetcher.WithHTTPClient(&http.Client{Timeout: 5 * time.Second}))
	p := New(client)

	rec := get(t, p, cdn.URL+"/big.png", articleURL)
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", rec.Code)
	}
	if n := rec.Body.Len(); n != 1024 {
		t.Errorf("响应体长度 = %d, 期望 1024", n)
	}
}

func TestProxyRejectsNonHTTPScheme(t *testing.T) {
	p := New(fetcher.NewClient())

	for _, target := range []string{"file:///etc/passwd", "data:image/png;base64,AAAA", "javascript:alert(1)"} {
		rec := get(t, p, target, articleURL)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("target %q 状态码 = %d, 期望 400", target, rec.Code)
		}
	}
}

func TestProxyRejectsNonGet(t *testing.T) {
	p := New(fetcher.NewClient())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, PathPrefix+"?u=https://example.com/a.png", nil)
	p.Middleware(http.NotFoundHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("状态码 = %d, 期望 405", rec.Code)
	}
}

func TestMiddlewarePassesThroughOtherPaths(t *testing.T) {
	p := New(fetcher.NewClient())

	called := false
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	p.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	})).ServeHTTP(rec, req)

	if !called {
		t.Fatal("非代理路径没有被交给 next")
	}
	if rec.Code != http.StatusTeapot {
		t.Errorf("状态码 = %d, 期望 418（应原样来自 next）", rec.Code)
	}
}

func TestProxyDoesNotCacheFailures(t *testing.T) {
	p, cdnURL, _ := newProxyWithCDN(t)

	// 上游 403：要如实透传，但**不能被缓存** —— 否则一次防盗链拦截或抖动会被
	// WebView 记上一整天，重开文章也恢复不了。
	forbidden := get(t, p, cdnURL+"/a.png", "")
	if cc := forbidden.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("上游失败的 Cache-Control = %q, 期望 no-store", cc)
	}

	// 参数非法同样是失败，不能带长缓存。
	bad := get(t, p, "file:///etc/passwd", articleURL)
	if cc := bad.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("非法参数的 Cache-Control = %q, 期望 no-store", cc)
	}
}

func TestProxyCachesSuccessfulImageWithLongMaxAge(t *testing.T) {
	p, cdnURL, _ := newProxyWithCDN(t)

	ok := get(t, p, cdnURL+"/a.png", articleURL)

	if cc := ok.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=") {
		t.Errorf("成功响应的 Cache-Control = %q, 期望含 max-age", cc)
	}
}

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"http", "http://example.com/a.png", false},
		{"https", "https://example.com/a.png", false},
		{"空", "", true},
		{"只有路径", "/a.png", true},
		{"file 协议", "file:///etc/passwd", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseTarget(tc.in); (err != nil) != tc.wantErr {
				t.Errorf("parseTarget(%q) err = %v, wantErr = %v", tc.in, err, tc.wantErr)
			}
		})
	}
}

func TestOriginOf(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://sspai.com/post/114823", "https://sspai.com/"},
		{"http://example.com:8080/a/b", "http://example.com:8080/"},
		{"", ""},
		{"不是 URL", ""},
		{"javascript:alert(1)", ""},
	}
	for _, tc := range tests {
		if got := originOf(tc.in); got != tc.want {
			t.Errorf("originOf(%q) = %q, 期望 %q", tc.in, got, tc.want)
		}
	}
}

func TestLRUCacheEvictsByBytes(t *testing.T) {
	c := newLRUCache(10)
	c.add("a", []byte("123456"), "image/png")
	c.add("b", []byte("123456"), "image/png")

	// 上限 10 字节：写入第二条后第一条应被淘汰。
	if _, _, ok := c.get("a"); ok {
		t.Error("超限后最早的条目应被淘汰")
	}
	if _, _, ok := c.get("b"); !ok {
		t.Error("最新写入的条目应仍在缓存里")
	}
}

func TestLRUCacheSkipsOversizedEntry(t *testing.T) {
	c := newLRUCache(4)
	c.add("a", []byte("1234"), "image/png")
	c.add("big", []byte("12345"), "image/png")

	// 单条超过总上限时不入缓存，也不能把已有条目挤掉。
	if _, _, ok := c.get("a"); !ok {
		t.Error("单条超限不该挤掉其他条目")
	}
	if _, _, ok := c.get("big"); ok {
		t.Error("单条超限的条目不该入缓存")
	}
}
