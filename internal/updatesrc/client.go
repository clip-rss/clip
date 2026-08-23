// Package updatesrc 提供软件更新的下载源：一个面向弱网优化的 HTTP 客户端，
// 以及包装 Wails GitHub provider、支持断点续传的 Provider。
//
// 存在的理由是 Wails 的默认配置在中国大陆网络下必然失败：
// pkg/updater/providers/github 未注入 HTTPClient 时用 &http.Client{Timeout: 30s}，
// 而 http.Client.Timeout 覆盖「连接 + 重定向 + 读完整个 body」，对 ~8 MB 的更新包
// 等于要求全程跑满 260 KB/s。它的 Download 也是单发 GET，无 Range、无重试，
// 传到 90% 遇到一个 RST 就得从零重来。
package updatesrc

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

// 分段超时：只惩罚「卡住」，不惩罚「慢」。刻意不设 http.Client.Timeout，
// 因为那是覆盖整个请求（含 body 读取）的墙钟上限，与「大文件慢速下载」直接冲突。
const (
	dialTimeout           = 10 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	responseHeaderTimeout = 30 * time.Second
	idleConnTimeout       = 90 * time.Second
	keepAliveInterval     = 30 * time.Second
)

// Proxy 是可在运行时热更新的代理配置，供 http.Transport.Proxy 使用。
//
// 用 atomic.Pointer 而非「改设置时重建 Transport」：Transport 持有连接池，
// 运行中替换它会与正在进行的下载产生数据竞争。这里让 Transport 全程不变，
// 只把它查询的那个值换掉。
type Proxy struct {
	u atomic.Pointer[url.URL]
}

// SetProxy 更新代理地址；host 为空或 port 非正时清除，回落到环境变量。
//
// 签名与 fetcher.Client.SetProxy 一致，两者可被同一个观察者接口统一驱动。
func (p *Proxy) SetProxy(host string, port int) {
	if host == "" || port <= 0 {
		p.u.Store(nil)
		return
	}
	u, err := url.Parse(fmt.Sprintf("http://%s:%d", host, port))
	if err != nil {
		p.u.Store(nil)
		return
	}
	p.u.Store(u)
}

// proxyFunc 返回可直接赋给 http.Transport.Proxy 的函数：用户在设置里配了代理就用它，
// 没配则回落到 http.ProxyFromEnvironment。
//
// 回落这一步是有意义的：GUI 应用由 Finder / 资源管理器启动时继承不到 shell 环境，
// 但用户仍可能通过 launchctl setenv 或系统代理设置注入 HTTPS_PROXY。
func (p *Proxy) proxyFunc() func(*http.Request) (*url.URL, error) {
	return func(r *http.Request) (*url.URL, error) {
		if u := p.u.Load(); u != nil {
			return u, nil
		}
		return http.ProxyFromEnvironment(r)
	}
}

// NewClient 返回更新流程专用的 HTTP 客户端。proxy 为 nil 时只按环境变量取代理。
func NewClient(proxy *Proxy) *http.Client {
	if proxy == nil {
		proxy = &Proxy{}
	}
	return &http.Client{
		// ⚠️ 有意留空 Timeout，理由见包注释与上面的超时常量。
		Transport: &http.Transport{
			Proxy: proxy.proxyFunc(),
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: keepAliveInterval,
			}).DialContext,
			TLSHandshakeTimeout:   tlsHandshakeTimeout,
			ResponseHeaderTimeout: responseHeaderTimeout,
			ExpectContinueTimeout: 1 * time.Second,
			IdleConnTimeout:       idleConnTimeout,
			ForceAttemptHTTP2:     true,
		},
	}
}
