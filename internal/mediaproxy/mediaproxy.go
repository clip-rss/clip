// Package mediaproxy 为正文里的远程图片/视频提供本地代理。
//
// 解决的问题是**防盗链**：正文图片由 app 代抓（而不是从文章源站发出），国内 CDN
// 普遍会校验 Referer，某类站点对空 Referer 直接 403、对任意非空 Referer 放行。
// 前端给图片加 referrerpolicy=no-referrer 只能救「拦截跨域 Referer」那一类站点，
// 反而打死这一类 —— 两类规则互斥，不存在能同时满足的 referrerpolicy。
//
// 代理同时满足两者：由服务端带着**文章源站自己的** Referer 去抓图 —— 既非空、
// 又是同站来源，两类规则都能过。
//
// 接入方式是把 Middleware 挂到 Wails 资产服务器中间件链的最前面
// （application.AssetOptions.Middleware）。前端把正文媒体的地址重写成
// /__clip/media?u=<原始地址>&r=<文章源站>。
//
// 为什么用相对路径而不是写死 wails:// —— Windows 上页面源是 http://wails.localhost，
// macOS/Linux 上才是 wails://localhost；相对路径在两端都解析到资产服务器，一套通吃。
// dev 模式下资产服务器的兜底 handler 本身就是指向 Vite 的反向代理，而本中间件排在它
// 之前，所以 dev 与生产走的是同一条路径，无需 Vite 侧配置。
package mediaproxy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/clip-rss/clip/internal/fetcher"
)

// PathPrefix 是代理路由。⚠️ 与 frontend/src/Utils/Media.ts 的常量是一份契约，两侧必须同时改。
const PathPrefix = "/__clip/media"

const (
	// fetchTimeout 单次上游抓取的上限。图片远用不到这么久，兜底防止请求悬挂。
	fetchTimeout = 30 * time.Second
	// maxCacheableImage 单张图片进内存缓存的上限。超过则直接流式转发、不缓存。
	maxCacheableImage = 8 << 20
	// maxStreamedBytes 流式转发时的读取上限（不缓存的图片、以及将来的视频）。
	maxStreamedBytes = 512 << 20
	// defaultCacheBytes 图片缓存的字节上限。
	defaultCacheBytes = 64 << 20
	// cacheMaxAge 回给 WebView 的缓存时长（秒）。WebView 对本地主机响应不一定落盘
	// 缓存，这一层主要给浏览器内的重复请求兜底，真正的复用靠 Proxy 的内存缓存。
	cacheMaxAge = 86400
)

// Proxy 是媒体代理。零值不可用，须经 New 构造。
type Proxy struct {
	client *fetcher.Client
	cache  *lruCache
}

// New 创建代理。client 复用抓取用的客户端，因此代理设置、超时、UA 与 cookie jar
// 都与订阅抓取保持一致。
func New(client *fetcher.Client) *Proxy {
	return &Proxy{client: client, cache: newLRUCache(defaultCacheBytes)}
}

// Middleware 返回 Wails 资产服务器中间件：只接管 PathPrefix，其余原样交给 next。
func (p *Proxy) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != PathPrefix {
			next.ServeHTTP(w, r)
			return
		}
		p.serve(w, r)
	})
}

// serve 处理一次代理请求。
func (p *Proxy) serve(w http.ResponseWriter, r *http.Request) {
	// 失败一律不缓存：400/502 万一是网络抖动或上游抽风，被 WebView 记上一整天
	// 会让裂图一直挂着、重开文章也恢复不了。
	fail := func(msg string, code int) {
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, msg, code)
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		fail("method not allowed", http.StatusMethodNotAllowed)
		return
	}

	target, err := parseTarget(r.URL.Query().Get("u"))
	if err != nil {
		fail("invalid media url", http.StatusBadRequest)
		return
	}
	referer := originOf(r.URL.Query().Get("r"))

	// 命中缓存：图片已经在上游抓过，直接回内存里的字节。
	if body, contentType, ok := p.cache.get(cacheKey(target, referer)); ok {
		writeCached(w, r, body, contentType)
		return
	}

	// 跟随前端断开取消上游请求：用户划走文章时不必把图抓完。
	ctx, cancel := context.WithTimeout(r.Context(), fetchTimeout)
	defer cancel()

	resp, err := p.client.FetchMedia(ctx, target, referer, r.Header)
	if err != nil {
		fail("media fetch failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyUpstreamHeaders(w.Header(), resp.Header)
	// 只有成功响应允许长缓存。403/404 要被如实透给 WebView 渲染成裂图，但不能被
	// 缓存 —— 防盗链拦截或一次抖动都可能是暂时的。
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
		w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(cacheMaxAge))
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}

	// 只有「200 + 图片 + 长度已知且不超上限」才读进内存缓存；其余（206 分段响应、
	// 未知长度、视频）一律流式转发，避免把大文件整个吞进内存。
	if resp.StatusCode == http.StatusOK &&
		isImage(resp.Header.Get("Content-Type")) &&
		resp.ContentLength >= 0 && resp.ContentLength <= maxCacheableImage {

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxCacheableImage+1))
		if err != nil {
			fail("media read failed", http.StatusBadGateway)
			return
		}
		if int64(len(body)) > maxCacheableImage {
			// 上游声明的长度不可信（说好 ≤ 8 MB 却发了更多）。此时流已被读掉一截，
			// 再转发剩下的只会得到一段残缺响应，如实报错更好。
			fail("media too large", http.StatusBadGateway)
			return
		}
		p.cache.add(cacheKey(target, referer), body, resp.Header.Get("Content-Type"))
		writeCached(w, r, body, resp.Header.Get("Content-Type"))
		return
	}

	w.WriteHeader(resp.StatusCode)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(w, io.LimitReader(resp.Body, maxStreamedBytes))
}

// writeCached 写出一份已在内存里的响应体。
func writeCached(w http.ResponseWriter, r *http.Request, body []byte, contentType string) {
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(cacheMaxAge))
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

// copyUpstreamHeaders 透传与响应语义有关的头。
//
// 刻意不逐条拷贝：Transfer-Encoding / Connection 这类由 Go 的 http 栈自己管，
// Content-Encoding 会被 transport 透明解压后自动去掉。Content-Length 只在流式
// 转发时跟着上游走（缓存路径自己算）。
func copyUpstreamHeaders(dst, src http.Header) {
	for _, k := range []string{
		"Content-Type", "Content-Length", "Content-Range",
		"Accept-Ranges", "ETag", "Last-Modified",
	} {
		if v := src.Get(k); v != "" {
			dst.Set(k, v)
		}
	}
}

// parseTarget 校验要代理的地址：只放行 http(s) 绝对地址。
func parseTarget(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("mediaproxy: empty url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("mediaproxy: unsupported scheme")
	}
	if u.Host == "" {
		return "", errors.New("mediaproxy: missing host")
	}
	return u.String(), nil
}

// originOf 把文章地址归约为源站 origin，作为要伪造的 Referer。
//
// 归约而非原样透传有两个原因：一是防盗链白名单普遍按域名前缀匹配，origin 足够；
// 二是**必须重新序列化**——直接拿用户可控的字符串写请求头，等于把头注入的口子
// 交给正文内容。
func originOf(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + "/"
}

func isImage(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "image/")
}

func cacheKey(target, referer string) string {
	return target + "\x00" + referer
}
