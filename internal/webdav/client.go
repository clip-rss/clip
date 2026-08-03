// Package webdav 提供一个最小 WebDAV 客户端，够用于把单个小配置文件读写到
// 用户自己的网盘（Nextcloud / 坚果云 / 群晖等）。
//
// 只实现同步所需的动词：PROPFIND / GET / PUT / MKCOL / DELETE，标准库实现，
// 不引入第三方依赖。不做 LOCK —— 各家支持参差，且配置同步靠 ETag 乐观并发已足够。
package webdav

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultTimeout 单次请求默认超时。取值高于 RSS 抓取（20s）：
// 自建网盘常跑在家用宽带上，首次建目录链路更慢。
const DefaultTimeout = 30 * time.Second

// maxResponseBytes 响应体上限（1 MiB）。同步载荷实际约 1 KB，PROPFIND 用
// Depth:0 也只有一条记录，留三个数量级余量足够；上限本身是为了防止异常或
// 恶意服务器持续灌数据把内存吃满。
const maxResponseBytes = 1 << 20

// userAgent 请求标识。部分网盘会按 UA 做兼容处理或限流统计。
const userAgent = "Clip/1.0 (+https://github.com/clip-rss/clip)"

// Config 连接配置。
type Config struct {
	// URL 目标目录的 WebDAV 地址，必须是 https。
	// 例：https://dav.jianguoyun.com/dav/clip/
	//     https://cloud.example.com/remote.php/dav/files/alice/clip/
	URL      string
	Username string
	Password string
}

// Client WebDAV 客户端。零值不可用，须经 New 构造。
type Client struct {
	base *url.URL
	cfg  Config
	http *http.Client

	// proxy 待应用的代理地址，由 WithProxy 记下，在 New 里所有 Option 跑完后
	// 才真正套到 http.Client 上。延后应用的原因见 WithProxy 的说明。
	proxy string
}

// Option 可选配置。
type Option func(*Client)

// WithHTTPClient 注入自定义 *http.Client。
// 测试用它接入 httptest.NewTLSServer 的自签证书客户端。
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// WithProxy 设置 HTTP 代理；host 为空或 port 非正时不启用。
//
// 与 fetcher 各自实现而不共用：让 webdav 不依赖 fetcher，两个包互不牵连。
// 代价是几行重复，换来的是可独立测试与替换。
//
// ⚠️ 只记下地址，不在此处改 http.Client。两个原因：
//  1. *http.Client 常被整个应用共用，直接改它的 Transport 会把无关请求
//     一起送进代理。New 里改的是一份副本。
//  2. 就地修改会让 Option 的先后顺序影响结果（WithHTTPClient 在后会覆盖掉
//     已设的代理）。延后到全部 Option 跑完再应用，顺序就无关了。
func WithProxy(host string, port int) Option {
	return func(c *Client) {
		if host == "" || port <= 0 {
			return
		}
		c.proxy = fmt.Sprintf("http://%s:%d", host, port)
	}
}

// applyProxy 把代理套到 http.Client 的副本上，避免改动调用方持有的实例。
// 地址非法时静默跳过：代理配错不该让同步功能整体不可用，请求会直连并在
// 网络层给出可诊断的错误。
func (c *Client) applyProxy() {
	if c.proxy == "" {
		return
	}
	proxyURL, err := url.Parse(c.proxy)
	if err != nil {
		return
	}

	// 浅拷贝，只替换 Transport；超时等其他设置保持注入时的值。
	dup := *c.http
	base, _ := dup.Transport.(*http.Transport)
	if base == nil {
		base, _ = http.DefaultTransport.(*http.Transport)
	}
	var t *http.Transport
	if base != nil {
		t = base.Clone() // 保留 TLS 配置（测试注入的自签证书信任链靠它）
	} else {
		t = &http.Transport{}
	}
	t.Proxy = http.ProxyURL(proxyURL)
	dup.Transport = t
	c.http = &dup
}

// New 校验配置并创建客户端。
//
// 强制 https：WebDAV 用 Basic Auth，凭据基本等同明文随每个请求发送，
// 明文 http 等于把网盘密码交给链路上任何一跳。
func New(cfg Config, opts ...Option) (*Client, error) {
	raw := strings.TrimSpace(cfg.URL)
	if raw == "" {
		return nil, configError("服务器地址不能为空")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, configError("服务器地址格式错误")
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		// ok
	case "http":
		return nil, configError("必须使用 https：WebDAV 的账号密码随每个请求发送，明文 http 会在链路上暴露")
	default:
		return nil, configError("服务器地址必须以 https:// 开头")
	}
	if u.Host == "" {
		return nil, configError("服务器地址缺少主机名")
	}
	// 基地址按目录处理：结尾补 /，否则 url.JoinPath 会把最后一段当文件名替换掉。
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}

	c := &Client{
		base: u,
		cfg:  cfg,
		http: &http.Client{Timeout: DefaultTimeout},
	}
	for _, opt := range opts {
		opt(c)
	}
	// 全部 Option 跑完后再套代理：此时 http.Client 已定，且不受 Option 顺序影响。
	c.applyProxy()
	return c, nil
}

// resolve 把相对路径拼到基地址上。用 JoinPath 而非字符串拼接，
// 由标准库处理转义与多余斜杠（路径里可能有空格或中文）。
func (c *Client) resolve(path string) (string, error) {
	// JoinPath 会按 path.Join 语义清理 ".."，可能爬出基地址之外。
	// 本包的路径都是内部生成的常量，正常不会含 ".."；仍显式拒绝，
	// 避免日后有人把用户输入接进来时静默越界。
	for _, seg := range splitPath(path) {
		if seg == ".." {
			return "", &Error{
				Op:       "resolve",
				Path:     path,
				Msg:      "路径不能包含 ..",
				sentinel: ErrInvalidConfig,
			}
		}
	}
	return c.base.JoinPath(path).String(), nil
}

// newRequest 构造带认证与 UA 的请求。
func (c *Client) newRequest(ctx context.Context, method, rawURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, fmt.Errorf("webdav: build %s request: %w", method, err)
	}
	if c.cfg.Username != "" || c.cfg.Password != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}

// do 发请求并读取（受限的）响应体。
func (c *Client) do(req *http.Request) (int, []byte, http.Header, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, nil, &Error{
			Op:       strings.ToLower(req.Method),
			Path:     req.URL.Path,
			Msg:      "无法连接服务器",
			Err:      err,
			sentinel: ErrNetwork,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return resp.StatusCode, nil, resp.Header, &Error{
			Op:       strings.ToLower(req.Method),
			Path:     req.URL.Path,
			Status:   resp.StatusCode,
			Msg:      "读取响应失败",
			Err:      err,
			sentinel: ErrNetwork,
		}
	}
	return resp.StatusCode, body, resp.Header, nil
}

/* ---------- 动词 ---------- */

// Stat 读取资源元信息（PROPFIND, Depth: 0）。
// 资源不存在时返回的错误满足 errors.Is(err, ErrNotFound)。
func (c *Client) Stat(ctx context.Context, path string) (Stat, error) {
	target, err := c.resolve(path)
	if err != nil {
		return Stat{}, err
	}
	req, err := c.newRequest(ctx, "PROPFIND", target, strings.NewReader(propfindBody))
	if err != nil {
		return Stat{}, err
	}
	// Depth 不是可选的：缺失时部分服务器直接 400，另一些会返回整个子树。
	req.Header.Set("Depth", "0")
	req.Header.Set("Content-Type", `application/xml; charset="utf-8"`)

	status, body, _, err := c.do(req)
	if err != nil {
		return Stat{}, err
	}
	if err := statusError("propfind", path, status, body); err != nil {
		return Stat{}, err
	}
	return parsePropfind(path, body)
}

// Get 下载文件内容，并返回其（已归一化的）ETag。
//
// 文件不存在时返回 ErrNotFound —— 对首次同步而言这是正常状态，调用方应据此
// 走「首次上传」分支，而非报错给用户。
func (c *Client) Get(ctx context.Context, path string) ([]byte, string, error) {
	target, err := c.resolve(path)
	if err != nil {
		return nil, "", err
	}
	req, err := c.newRequest(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", err
	}

	status, body, header, err := c.do(req)
	if err != nil {
		return nil, "", err
	}
	if err := statusError("get", path, status, body); err != nil {
		return nil, "", err
	}
	return body, normalizeETag(header.Get("ETag")), nil
}

// Put 上传文件内容，返回新 ETag。
//
// ifMatch 非空时带 If-Match 头做乐观并发：远端在此期间被改过则服务器回 412，
// 对应错误满足 errors.Is(err, ErrConflict)。
//
// ⚠️ If-Match 并非所有服务器都实现，不能只靠它防冲突 —— 调用方仍须在 Put 前
// Stat 比对 ETag。它是额外一层保险，用于收窄「Stat 与 Put 之间」的竞态窗口。
func (c *Client) Put(ctx context.Context, path string, data []byte, ifMatch string) (string, error) {
	target, err := c.resolve(path)
	if err != nil {
		return "", err
	}
	req, err := c.newRequest(ctx, http.MethodPut, target, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.ContentLength = int64(len(data))
	if ifMatch != "" {
		// 带引号发送：RFC 要求 entity-tag 形式。内部存的是裸值，此处补回。
		req.Header.Set("If-Match", `"`+ifMatch+`"`)
	}

	status, body, header, err := c.do(req)
	if err != nil {
		return "", err
	}
	if err := statusError("put", path, status, body); err != nil {
		return "", err
	}

	// 多数服务器的 PUT 响应不带 ETag，此时补一次 PROPFIND 取新值。
	if etag := normalizeETag(header.Get("ETag")); etag != "" {
		return etag, nil
	}
	st, err := c.Stat(ctx, path)
	if err != nil {
		// 内容已经写成功了，只是拿不到新 ETag。不当失败上报，
		// 返回空值让调用方走「下次同步重新取」的路径。
		return "", nil
	}
	return st.ETag, nil
}

// Delete 删除资源。资源本就不存在时返回 ErrNotFound。
func (c *Client) Delete(ctx context.Context, path string) error {
	target, err := c.resolve(path)
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, http.MethodDelete, target, nil)
	if err != nil {
		return err
	}
	status, body, _, err := c.do(req)
	if err != nil {
		return err
	}
	return statusError("delete", path, status, body)
}

// Mkcol 创建单级目录。目录已存在时返回 nil（幂等）。
func (c *Client) Mkcol(ctx context.Context, path string) error {
	target, err := c.resolve(path)
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, "MKCOL", target, nil)
	if err != nil {
		return err
	}
	status, body, _, err := c.do(req)
	if err != nil {
		return err
	}
	// 405 = 该地址已存在集合。这是重复建目录的正常回应，视作成功。
	// 301/302 也当已存在：部分服务器对已存在的目录回跳转到规范地址。
	if status == http.StatusMethodNotAllowed ||
		status == http.StatusMovedPermanently ||
		status == http.StatusFound {
		return nil
	}
	return statusError("mkcol", path, status, body)
}

// MkcolAll 逐级创建 path 的各级目录（含 path 本身）。
//
// 逐级建是必要的：MKCOL 在父目录不存在时返回 409，多数服务器不会自动补建。
func (c *Client) MkcolAll(ctx context.Context, path string) error {
	segments := splitPath(path)
	if len(segments) == 0 {
		return nil
	}
	for i := range segments {
		partial := strings.Join(segments[:i+1], "/") + "/"
		if err := c.Mkcol(ctx, partial); err != nil {
			return err
		}
	}
	return nil
}

// splitPath 拆出非空路径段。
func splitPath(path string) []string {
	var out []string
	for _, seg := range strings.Split(strings.Trim(path, "/"), "/") {
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}
