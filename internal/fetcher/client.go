package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

// DefaultUserAgent 默认 User-Agent。
const DefaultUserAgent = "Clip/1.0 (+https://github.com/clip-rss/clip)"

// DefaultTimeout 单次请求默认超时时间。
const DefaultTimeout = 20 * time.Second

// maxResponseBytes 限制响应体大小，防止超大 Feed 占用内存（10 MiB）。
const maxResponseBytes = 10 << 20

// maxDownloadBytes 限制下载资源（如图片）的响应体大小（50 MiB）。
const maxDownloadBytes = 50 << 20

// Client 封装 HTTP 抓取：超时、重试、自定义 UA、条件 GET。
type Client struct {
	http      *http.Client
	userAgent string
	maxRetry  int
}

// ClientOption 配置 Client 的可选项。
type ClientOption func(*Client)

// WithUserAgent 设置 User-Agent。
func WithUserAgent(ua string) ClientOption {
	return func(c *Client) { c.userAgent = ua }
}

// WithTimeout 设置请求超时时间。
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.http.Timeout = d }
}

// WithMaxRetry 设置失败重试次数（针对网络错误与 5xx）。
func WithMaxRetry(n int) ClientOption {
	return func(c *Client) { c.maxRetry = n }
}

// WithProxy 设置 HTTP 代理地址。
func WithProxy(host string, port int) ClientOption {
	return func(c *Client) { applyProxy(c.http, host, port) }
}

// SetProxy 运行时更新 HTTP 代理。host 为空时清除代理。
func (c *Client) SetProxy(host string, port int) {
	applyProxy(c.http, host, port)
}

func applyProxy(client *http.Client, host string, port int) {
	if host == "" || port <= 0 {
		client.Transport = nil
		return
	}
	proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%d", host, port))
	if err != nil {
		return
	}
	client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
}

// WithHTTPClient 注入自定义 *http.Client（主要用于测试）。
func WithHTTPClient(h *http.Client) ClientOption {
	return func(c *Client) { c.http = h }
}

// NewClient 创建 HTTP 客户端。
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		http:      &http.Client{Timeout: DefaultTimeout},
		userAgent: DefaultUserAgent,
		maxRetry:  2,
	}
	for _, opt := range opts {
		opt(c)
	}
	// Cookie jar 供反爬挑战的凭据复用（见 challenge.go）。在构造期装好而非按需创建：
	// FetchMany 会并发调用，运行时改 c.http.Jar 会产生数据竞争。
	// 仅存内存、不落盘；jar 自身是并发安全的。
	if c.http.Jar == nil {
		if jar, err := cookiejar.New(nil); err == nil {
			c.http.Jar = jar
		}
	}
	return c
}

// solveChallenge 尝试求解响应体中的反爬 JS 挑战。成功则把凭据写入 cookie jar
// （后续请求由 http.Client 自动携带，凭据可复用，不必每次重解），返回 true。
//
// 非挑战页、载荷结构不符、或无 jar 时返回 false，调用方沿用原始解析错误。
func (c *Client) solveChallenge(rawURL string, body []byte) bool {
	if c.http.Jar == nil || !looksLikeWAFChallenge(body) {
		return false
	}
	value, ok := solveWAFChallenge(body)
	if !ok {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	c.http.Jar.SetCookies(u, []*http.Cookie{{
		Name:  wafCookieName,
		Value: value,
		Path:  "/",
	}})
	return true
}

// ConditionalHeaders 条件 GET 所需的缓存校验头。
type ConditionalHeaders struct {
	ETag         string
	LastModified string
}

// FetchResult HTTP 抓取结果。
type FetchResult struct {
	Body         []byte
	StatusCode   int
	ETag         string // 响应中的 ETag，供下次条件 GET 使用
	LastModified string // 响应中的 Last-Modified
	NotModified  bool   // 服务器返回 304
}

// Fetch 抓取 URL，携带条件 GET 头；返回结果或错误。
// 对网络错误和 5xx 状态码进行指数退避重试。
func (c *Client) Fetch(ctx context.Context, rawURL string, cond ConditionalHeaders) (*FetchResult, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetry; attempt++ {
		if attempt > 0 {
			if err := sleepCtx(ctx, retryBackoff(attempt)); err != nil {
				return nil, err
			}
		}

		res, retryable, err := c.do(ctx, rawURL, cond)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
	}
	return nil, fmt.Errorf("fetcher: fetch %q failed after retries: %w", rawURL, lastErr)
}

// do 执行单次请求。retryable 指示该错误是否值得重试。
func (c *Client) do(ctx context.Context, rawURL string, cond ConditionalHeaders) (res *FetchResult, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("fetcher: build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml;q=0.9, */*;q=0.8")
	if cond.ETag != "" {
		req.Header.Set("If-None-Match", cond.ETag)
	}
	if cond.LastModified != "" {
		req.Header.Set("If-Modified-Since", cond.LastModified)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("fetcher: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return &FetchResult{
			StatusCode:   resp.StatusCode,
			NotModified:  true,
			ETag:         firstNonEmpty(resp.Header.Get("ETag"), cond.ETag),
			LastModified: firstNonEmpty(resp.Header.Get("Last-Modified"), cond.LastModified),
		}, false, nil
	}

	if resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("fetcher: server error: %s", resp.Status)
	}
	if resp.StatusCode >= 400 {
		return nil, false, fmt.Errorf("fetcher: client error: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, true, fmt.Errorf("fetcher: read body: %w", err)
	}

	return &FetchResult{
		Body:         body,
		StatusCode:   resp.StatusCode,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}, false, nil
}

// Get 下载一个二进制资源（如图片），返回响应体字节与 Content-Type。
//
// 与 Fetch 不同：不携带条件 GET 头、Accept 头面向任意资源，且响应体上限更大
// （maxDownloadBytes）。复用同一 http.Client，因此代理、超时、重试策略一致。
func (c *Client) Get(ctx context.Context, rawURL string) (body []byte, contentType string, err error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetry; attempt++ {
		if attempt > 0 {
			if err := sleepCtx(ctx, retryBackoff(attempt)); err != nil {
				return nil, "", err
			}
		}

		body, contentType, retryable, err := c.get(ctx, rawURL)
		if err == nil {
			return body, contentType, nil
		}
		lastErr = err
		if !retryable {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("fetcher: download %q failed after retries: %w", rawURL, lastErr)
}

// get 执行单次下载请求。retryable 指示该错误是否值得重试。
func (c *Client) get(ctx context.Context, rawURL string) (body []byte, contentType string, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", false, fmt.Errorf("fetcher: build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "image/*, */*;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", true, fmt.Errorf("fetcher: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, "", true, fmt.Errorf("fetcher: server error: %s", resp.Status)
	}
	if resp.StatusCode >= 400 {
		return nil, "", false, fmt.Errorf("fetcher: client error: %s", resp.Status)
	}

	body, err = io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes))
	if err != nil {
		return nil, "", true, fmt.Errorf("fetcher: read body: %w", err)
	}
	return body, resp.Header.Get("Content-Type"), false, nil
}

// retryBackoff 计算第 attempt 次重试的等待时间（指数，封顶 8s）。
func retryBackoff(attempt int) time.Duration {
	d := time.Duration(1<<uint(attempt-1)) * time.Second
	if d > 8*time.Second {
		d = 8 * time.Second
	}
	return d
}

// sleepCtx 在 ctx 取消时提前返回的可中断睡眠。
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
