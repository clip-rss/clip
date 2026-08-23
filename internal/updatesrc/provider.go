package updatesrc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

// 重试与停滞检测参数。
const (
	// maxAttempts 是单次 Download 的总尝试次数（含首次）。
	maxAttempts = 6
	// copyBufSize 与 Wails 内置 provider 保持一致。
	copyBufSize = 64 << 10
)

// 停滞检测的两个时长。写成 var 而非 const 只为让测试调小它们 —— 否则覆盖停滞路径
// 的用例每跑一次就得真等 20 秒。生产代码不修改它们。
var (
	// stallTimeoutVar 是「连接还在、但一个字节都不来」的容忍时长。GFW 的常见形态是
	// 限速到 0 而不断开连接，此时 TCP 层无异常、ResponseHeaderTimeout 也已过去，
	// 只有按数据到达间隔判定才能发现。
	stallTimeoutVar = 20 * time.Second
	// stallCheckIntervalVar 是停滞看门狗的轮询间隔。
	stallCheckIntervalVar = 2 * time.Second
)

// 本包的错误文本一律用英文。它们最终作为 updater.ErrorInfo.Message 原样送到更新窗口，
// 而窗口是靠**匹配英文关键词**把原始错误映射成本地化文案的（build/updater/window.html
// 的 renderError）。这里写中文会有两个后果：英文 / 繁中界面里漏出简体中文，且匹配不中
// 任何分支、直接把内部诊断信息糊到用户脸上。关键词有意选得能命中对应分支。
var (
	// errStalled 表示连接未断但长时间无数据到达，由看门狗主动掐断。
	errStalled = errors.New("updatesrc: download stalled, no data received before timeout")
	// errIncomplete 表示读到 EOF 但字节数不足 —— 服务端或中间设备提前截断了响应体。
	errIncomplete = errors.New("updatesrc: response body truncated, connection closed early")
)

// statusError 是非 2xx 响应。单独成类型以便按状态码判定是否值得重试。
type statusError struct {
	code int
}

func (e *statusError) Error() string {
	return fmt.Sprintf("updatesrc: HTTP %d %s", e.code, http.StatusText(e.code))
}

// writeError 标记「写入 dst 失败」——磁盘满、权限不足等本地故障。必须与传输层错误
// 区分开：retryable 对未知错误默认放行重试，而本地写盘失败重试一万次也不会好。
type writeError struct {
	err error
}

func (e *writeError) Error() string { return "updatesrc: write failed: " + e.err.Error() }
func (e *writeError) Unwrap() error { return e.err }

// Config 配置 ResumeProvider。
type Config struct {
	// Repository 是 "owner/repo"，必填。
	Repository string
	// ChecksumAsset 是校验和 sidecar 的资产名，透传给内层 provider。
	ChecksumAsset string
	// Token 是可选的 GitHub PAT。仅在跨主机重定向时会被剥离。
	Token string
	// Proxy 是运行时可热更新的代理配置，可为 nil。
	Proxy *Proxy

	// sleep 注入退避等待，供测试免于真实睡眠。nil 时用 time.After。
	sleep func(context.Context, time.Duration) error
}

// ResumeProvider 包装 Wails 的 GitHub provider：Check 完全复用内层实现，
// 只重写 Download 以支持断点续传、指数退避重试与停滞检测。
//
// 为什么续传可以放在 Provider 层：Wails 的 updater.download 把 dst 包成
// io.MultiWriter(临时文件, hasher) 并**流式**计算摘要（pkg/updater/download.go），
// 摘要只取决于写入 dst 的字节序列。因此只要我们始终顺序追加、不回退、不重复写，
// 中途换几条 TCP 连接对校验完全透明。
//
// Name() 继承内层的 "github" —— 必须如此：updater.DownloadAndInstall 用
// findProvider(cfg.Providers, pending.Provider) 按名字把下载路由回产出该 release
// 的 provider，改名会导致下载阶段找不到 provider。
type ResumeProvider struct {
	*github.Provider

	client *http.Client
	token  string
	sleep  func(context.Context, time.Duration) error
}

// 编译期确认包装后仍满足 Provider 契约：Name/Check 由嵌入的 *github.Provider 提供，
// Download 由本类型重写。
var _ updater.Provider = (*ResumeProvider)(nil)

// New 构造 ResumeProvider。
func New(cfg Config) (*ResumeProvider, error) {
	client := NewClient(cfg.Proxy)
	inner, err := github.New(github.Config{
		Repository:    cfg.Repository,
		ChecksumAsset: cfg.ChecksumAsset,
		Token:         cfg.Token,
		// 关键：把同一个客户端交给内层，Check 与校验和 sidecar 的抓取也一并受益于
		// 分段超时与代理配置。
		HTTPClient: client,
	})
	if err != nil {
		return nil, err
	}
	sleep := cfg.sleep
	if sleep == nil {
		sleep = sleepCtx
	}
	return &ResumeProvider{Provider: inner, client: client, token: cfg.Token, sleep: sleep}, nil
}

// Download 流式下载 rel 的产物到 dst，失败时带 Range 头从断点续传。
//
// 契约与 updater.Provider 一致：onProgress 报告的是**累计**写入字节数，跨重试单调递增。
func (p *ResumeProvider) Download(ctx context.Context, rel *updater.Release, dst io.Writer, onProgress func(written, total int64)) error {
	urlStr, err := assetURL(rel)
	if err != nil {
		return err
	}

	total := int64(0)
	if rel.Artifact.Size > 0 {
		total = rel.Artifact.Size
	}

	var written int64
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			if err := p.sleep(ctx, backoff(attempt)); err != nil {
				return err
			}
		}

		n, discovered, err := p.stream(ctx, urlStr, written, total, dst, onProgress)
		written += n
		if discovered > 0 {
			total = discovered
		}

		if err == nil {
			// 读到 EOF 不代表拿全了：中间设备可能在 Content-Length 之前就关掉连接。
			// 放过去的话会在 verify 阶段变成一句莫名的「校验失败」，反而更难排查。
			if total > 0 && written < total {
				lastErr = fmt.Errorf("%w: 已写入 %d / %d 字节", errIncomplete, written, total)
			} else {
				return nil
			}
		} else {
			lastErr = err
		}

		// 调用方取消（用户关窗、应用退出）优先于一切重试。
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if !retryable(lastErr) {
			return lastErr
		}
	}

	return fmt.Errorf("updatesrc: download failed after %d attempts (%d bytes written): %w", maxAttempts, written, lastErr)
}

// stream 执行一次下载尝试，从 from 字节处开始。返回本次写入的字节数、探测到的资源总长
// （未知时为 0）与错误。
func (p *ResumeProvider) stream(
	ctx context.Context,
	urlStr string,
	from, knownTotal int64,
	dst io.Writer,
	onProgress func(written, total int64),
) (int64, int64, error) {
	// 停滞看门狗通过取消这个派生 ctx 掐断卡死的 Read。
	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, urlStr, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	if from > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(from, 10)+"-")
	}

	resp, err := p.do(req)
	if err != nil {
		return 0, 0, classifyCancel(ctx, reqCtx, err)
	}
	defer resp.Body.Close()

	// 416：请求的范围越界。若我们要的正是「已全部拿到」之后的位置，那就是已经下完了。
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && knownTotal > 0 && from >= knownTotal {
		return 0, knownTotal, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, 0, &statusError{code: resp.StatusCode}
	}

	total := totalFrom(resp, from)

	// 要了 Range 却收到 200：服务端（或中间缓存）不支持 Range，返回的是完整对象。
	// 此时不能直接写 —— 前 from 个字节已经进过 dst 和 hasher，重复写会毁掉摘要。
	// 丢弃这部分再接着写：浪费流量，但结果正确，好过整轮失败。
	if from > 0 && resp.StatusCode == http.StatusOK {
		if _, err := io.CopyN(io.Discard, resp.Body, from); err != nil {
			return 0, total, classifyCancel(ctx, reqCtx, err)
		}
	}

	// —— 停滞看门狗 ——
	var lastByteAt atomic.Int64
	lastByteAt.Store(time.Now().UnixNano())
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		ticker := time.NewTicker(stallCheckIntervalVar)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if time.Since(time.Unix(0, lastByteAt.Load())) > stallTimeoutVar {
					cancel() // 让阻塞中的 Read 立刻返回 context.Canceled
					return
				}
			}
		}
	}()

	written := int64(0)
	buf := make([]byte, copyBufSize)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			lastByteAt.Store(time.Now().UnixNano())
			if _, werr := dst.Write(buf[:n]); werr != nil {
				// 写盘失败（磁盘满等）与网络无关，不该重试。
				return written, total, &writeError{err: werr}
			}
			written += int64(n)
			if onProgress != nil {
				onProgress(from+written, total)
			}
		}
		if rerr == io.EOF {
			return written, total, nil
		}
		if rerr != nil {
			return written, total, classifyCancel(ctx, reqCtx, rerr)
		}
	}
}

// do 发请求并在跨主机重定向时剥掉 Authorization。
//
// GitHub 的 browser_download_url 会 302 到预签名的对象存储地址；把 PAT 一路带过去
// 会被对端拒绝。内层 provider 有等价逻辑，但那是未导出方法，无法复用。
func (p *ResumeProvider) do(req *http.Request) (*http.Response, error) {
	client := *p.client // 浅拷贝：只为改 CheckRedirect，不动共享的 Transport
	client.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) > 0 && !strings.EqualFold(via[len(via)-1].URL.Host, r.URL.Host) {
			r.Header.Del("Authorization")
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return client.Do(req)
}

// assetURL 取出 Check 阶段存进 Metadata 的资产下载地址。
func assetURL(rel *updater.Release) (string, error) {
	if rel == nil || rel.Metadata == nil {
		return "", errors.New("updatesrc: invalid release: missing metadata")
	}
	urlStr, ok := rel.Metadata["github.asset.url"].(string)
	if !ok || urlStr == "" {
		return "", errors.New("updatesrc: invalid release: metadata has no asset URL")
	}
	return urlStr, nil
}

// totalFrom 推断资源总长：206 优先解析 Content-Range 的分母，否则用
// ContentLength 加上已有的偏移。无法确定时返回 0。
func totalFrom(resp *http.Response, from int64) int64 {
	if resp.StatusCode == http.StatusPartialContent {
		// 形如 "bytes 200-1023/1024"
		if cr := resp.Header.Get("Content-Range"); cr != "" {
			if i := strings.LastIndex(cr, "/"); i >= 0 {
				if size, err := strconv.ParseInt(strings.TrimSpace(cr[i+1:]), 10, 64); err == nil && size > 0 {
					return size
				}
			}
		}
		if resp.ContentLength > 0 {
			return from + resp.ContentLength
		}
		return 0
	}
	if resp.ContentLength > 0 {
		return resp.ContentLength
	}
	return 0
}

// classifyCancel 把「看门狗掐断」与「调用方取消」区分开：两者在 Read 处都表现为
// context.Canceled，但前者应当重试、后者必须立即放弃。
func classifyCancel(outer, inner context.Context, err error) error {
	if outer.Err() == nil && inner.Err() != nil {
		return errStalled
	}
	return err
}

// retryable 判断错误是否值得再试一次。
//
// 默认倾向于重试：弱网下的传输层错误形态繁多（RST、TLS 握手失败、意外 EOF……），
// 逐一枚举必然漏项。只把「重试一定也不会好」的情况判为致命：明确的 4xx 语义错误
// （资产不存在、无权限）和本地写盘失败。
func retryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errStalled) || errors.Is(err, errIncomplete) {
		return true
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// 走到这里说明不是看门狗掐的（那种已转成 errStalled），是调用方要停。
		return false
	}
	var we *writeError
	if errors.As(err, &we) {
		return false
	}
	var se *statusError
	if errors.As(err, &se) {
		switch {
		case se.code == http.StatusRequestTimeout, se.code == http.StatusTooManyRequests:
			return true
		case se.code >= 500:
			return true
		default:
			return false
		}
	}
	// 传输层错误：重试。
	return true
}

// backoff 返回第 attempt 次重试前的等待时长：1s、2s、4s、8s，上限 15s。
func backoff(attempt int) time.Duration {
	if attempt < 1 {
		return 0
	}
	d := time.Second << (attempt - 1)
	if d > 15*time.Second {
		d = 15 * time.Second
	}
	return d
}

// sleepCtx 等待 d，期间 ctx 被取消则立刻返回其错误。
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
