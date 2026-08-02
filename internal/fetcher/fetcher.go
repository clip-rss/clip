// Package fetcher 负责 HTTP 抓取与 RSS/Atom Feed 解析。
//
// 主要职责：
//   - HTTP 抓取（超时、重试、条件 GET）
//   - RSS 2.0 / Atom 解析为统一数据模型
//   - 正文清洗（防 XSS）与摘要生成
//   - 文章去重、Feed/Favicon 自动发现
//   - 并发控制与连续失败的智能退避
package fetcher

import (
	"context"
	"sync"
	"time"
)

// summaryLength 摘要纯文本字符上限。
const summaryLength = 200

// maxConcurrency 默认并发抓取协程上限。
const maxConcurrency = 5

// maxBackoff 连续失败退避的上限（24 小时）。
const maxBackoff = 24 * time.Hour

// Fetcher 抓取编排器：组合 HTTP 客户端、解析、清洗与去重，
// 并维护条件 GET 缓存与并发控制。
type Fetcher struct {
	client      *Client
	concurrency int

	mu   sync.Mutex
	cond map[string]ConditionalHeaders // 按 Feed URL 缓存 ETag/Last-Modified
}

// Option 配置 Fetcher。
type Option func(*Fetcher)

// WithClient 注入自定义 HTTP 客户端。
func WithClient(c *Client) Option {
	return func(f *Fetcher) { f.client = c }
}

// WithConcurrency 设置批量抓取的并发上限。
func WithConcurrency(n int) Option {
	return func(f *Fetcher) {
		if n > 0 {
			f.concurrency = n
		}
	}
}

// New 创建 Fetcher。
func New(opts ...Option) *Fetcher {
	f := &Fetcher{
		client:      NewClient(),
		concurrency: maxConcurrency,
		cond:        make(map[string]ConditionalHeaders),
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// FetchFeed 抓取并解析单个 Feed。
//   - 自动携带上次缓存的条件 GET 头；
//   - 服务器返回 304 时，feed 为 nil 且 result.NotModified 为 true；
//   - 成功时返回经清洗与去重的 ParsedFeed。
func (f *Fetcher) FetchFeed(ctx context.Context, feedURL string) (*ParsedFeed, *FetchResult, error) {
	return f.fetch(ctx, feedURL, f.getCond(feedURL))
}

// FetchFeedForce 强制全量抓取，忽略条件 GET 缓存（不发送 ETag/If-Modified-Since），
// 用于用户主动“强制刷新”。仍会用最新响应更新缓存校验头。
func (f *Fetcher) FetchFeedForce(ctx context.Context, feedURL string) (*ParsedFeed, *FetchResult, error) {
	return f.fetch(ctx, feedURL, ConditionalHeaders{})
}

// fetch 执行抓取、解析、清洗与去重的统一流程。
func (f *Fetcher) fetch(ctx context.Context, feedURL string, cond ConditionalHeaders) (*ParsedFeed, *FetchResult, error) {
	result, err := f.client.Fetch(ctx, feedURL, cond)
	if err != nil {
		return nil, nil, err
	}

	if result.NotModified {
		// 304 沿用已验证内容对应的校验头；响应可能补充新的 validator。
		f.setCond(feedURL, ConditionalHeaders{ETag: result.ETag, LastModified: result.LastModified})
		return nil, result, nil
	}

	feed, err := Parse(result.Body)
	if err != nil {
		return nil, result, err
	}

	CleanFeed(feed)
	feed.Items = Dedup(feed.Items)
	// 仅在内容成功解析后更新 validator，避免损坏响应的 ETag 导致下次 304 被误判为成功。
	f.setCond(feedURL, ConditionalHeaders{ETag: result.ETag, LastModified: result.LastModified})
	return feed, result, nil
}

// BatchResult 单个 Feed 的批量抓取结果。
type BatchResult struct {
	URL    string
	Feed   *ParsedFeed
	Result *FetchResult
	Err    error
}

// FetchMany 并发抓取多个 Feed，并发度受 Fetcher.concurrency 限制。
// 返回与输入等长的结果切片（顺序对应）。
func (f *Fetcher) FetchMany(ctx context.Context, urls []string) []BatchResult {
	results := make([]BatchResult, len(urls))
	sem := make(chan struct{}, f.concurrency)
	var wg sync.WaitGroup

	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			feed, res, err := f.FetchFeed(ctx, u)
			results[i] = BatchResult{URL: u, Feed: feed, Result: res, Err: err}
		}(i, u)
	}

	wg.Wait()
	return results
}

// Discover 抓取网页并提取其中声明的 Feed 链接（<link rel="alternate">）。
// 用于「输入网站首页 URL 自动发现订阅源」：先 GET 页面 HTML，再解析其中的 Feed 声明。
func (f *Fetcher) Discover(ctx context.Context, pageURL string) ([]DiscoveredFeed, error) {
	res, err := f.client.Fetch(ctx, pageURL, ConditionalHeaders{})
	if err != nil {
		return nil, err
	}
	return DiscoverFeeds(res.Body, pageURL), nil
}

// Client 返回底层 HTTP 客户端，供外部配置代理等。
func (f *Fetcher) Client() *Client { return f.client }

// SeedConditional 预置某 Feed 的条件 GET 头（例如从持久化层恢复）。
func (f *Fetcher) SeedConditional(feedURL string, cond ConditionalHeaders) {
	f.setCond(feedURL, cond)
}

func (f *Fetcher) getCond(feedURL string) ConditionalHeaders {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cond[feedURL]
}

func (f *Fetcher) setCond(feedURL string, cond ConditionalHeaders) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cond[feedURL] = cond
}

// CleanFeed 就地清洗 Feed：清洗正文 HTML、生成纯文本摘要、解析相对链接。
func CleanFeed(feed *ParsedFeed) {
	for i := range feed.Items {
		item := &feed.Items[i]

		// 摘要优先取原始 summary，缺失时回退到正文。
		summarySource := item.Summary
		if summarySource == "" {
			summarySource = item.Content
		}
		item.Summary = Summarize(summarySource, summaryLength)
		item.Content = Sanitize(item.Content)

		// 将相对链接解析为绝对地址。
		if feed.Link != "" {
			base := parseBase(feed.Link)
			if item.Link != "" {
				item.Link = resolveURL(base, item.Link)
			}
			if item.Enclosure != "" {
				item.Enclosure = resolveURL(base, item.Enclosure)
			}
		}
	}
}

// Backoff 根据连续失败次数计算下次重试的退避间隔。
// 采用指数退避：base * 2^(errorCount-1)，封顶 maxBackoff（24h）。
// errorCount <= 0 时返回 base。
func Backoff(errorCount int, base time.Duration) time.Duration {
	if base <= 0 {
		base = time.Minute
	}
	if errorCount <= 0 {
		return base
	}
	d := base
	for i := 1; i < errorCount; i++ {
		d *= 2
		if d >= maxBackoff {
			return maxBackoff
		}
	}
	if d > maxBackoff {
		return maxBackoff
	}
	return d
}
