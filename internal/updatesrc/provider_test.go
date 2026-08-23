package updatesrc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

// noSleep 替换退避等待，让重试路径的测试不真的睡。
func noSleep(ctx context.Context, _ time.Duration) error { return ctx.Err() }

// newTestProvider 构造一个指向 srv 的 ResumeProvider。
func newTestProvider(t *testing.T) *ResumeProvider {
	t.Helper()
	p, err := New(Config{Repository: "owner/repo", sleep: noSleep})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

// releaseFor 造一个指向 url、声明 size 字节的 release。
func releaseFor(rawURL string, size int64) *updater.Release {
	return &updater.Release{
		Artifact: updater.Artifact{Filename: "Clip.zip", Size: size},
		Metadata: map[string]any{"github.asset.url": rawURL},
	}
}

// payload 生成可辨识的测试数据（内容与位置相关，便于发现错位/重复写）。
func payload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + i%26)
	}
	return b
}

// serveRange 按请求的 Range 头返回 206 部分内容；无 Range 头则返回 200 全量。
// cut > 0 时只写 cut 个字节就中断连接，模拟传输中途被掐。
func serveRange(w http.ResponseWriter, r *http.Request, data []byte, cut int) {
	from := int64(0)
	if rh := r.Header.Get("Range"); rh != "" {
		spec := strings.TrimPrefix(rh, "bytes=")
		spec = strings.TrimSuffix(spec, "-")
		from, _ = strconv.ParseInt(spec, 10, 64)
	}
	if from >= int64(len(data)) {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	rest := data[from:]
	if from > 0 {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, len(data)-1, len(data)))
		w.Header().Set("Content-Length", strconv.Itoa(len(rest)))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.Header().Set("Content-Length", strconv.Itoa(len(rest)))
		w.WriteHeader(http.StatusOK)
	}
	if cut > 0 && cut < len(rest) {
		_, _ = w.Write(rest[:cut])
		w.(http.Flusher).Flush()
		// 劫持底层连接并直接关闭，制造「Content-Length 未满就断开」。
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
		return
	}
	_, _ = w.Write(rest)
}

// TestDownloadResumesAfterInterruption 是本包的核心断言：连接在中途被掐断后，
// 续传必须让 dst 收到与一次性下载**完全一致**的字节序列 —— 不重复、不丢失、不错位。
// 这正是 Wails 流式摘要正确性的前提。
func TestDownloadResumesAfterInterruption(t *testing.T) {
	data := payload(200 << 10) // 200 KiB，够触发多次 64 KiB 读
	var hits atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		// 前两次各传一部分就断开，第三次放行。
		switch n {
		case 1:
			serveRange(w, r, data, 50<<10)
		case 2:
			serveRange(w, r, data, 30<<10)
		default:
			serveRange(w, r, data, 0)
		}
	}))
	defer srv.Close()

	p := newTestProvider(t)
	var got bytes.Buffer
	var maxReported int64
	var lastTotal int64
	onProgress := func(written, total int64) {
		if written < maxReported {
			t.Errorf("进度回退：%d 之后报告了 %d", maxReported, written)
		}
		maxReported = written
		lastTotal = total
	}

	if err := p.Download(context.Background(), releaseFor(srv.URL, int64(len(data))), &got, onProgress); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if hits.Load() < 3 {
		t.Errorf("期望至少 3 次请求（2 次中断 + 1 次成功），实际 %d", hits.Load())
	}
	if !bytes.Equal(got.Bytes(), data) {
		t.Errorf("续传后字节流不一致：得到 %d 字节，期望 %d 字节", got.Len(), len(data))
	}
	if maxReported != int64(len(data)) {
		t.Errorf("最终进度 = %d，期望 %d", maxReported, len(data))
	}
	if lastTotal != int64(len(data)) {
		t.Errorf("最终 total = %d，期望 %d", lastTotal, len(data))
	}
}

// TestDownloadSendsRangeHeader 验证续传请求确实带上了正确的 Range 偏移。
func TestDownloadSendsRangeHeader(t *testing.T) {
	data := payload(100 << 10)
	var ranges []string
	var hits atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ranges = append(ranges, r.Header.Get("Range"))
		if hits.Add(1) == 1 {
			serveRange(w, r, data, 40<<10)
			return
		}
		serveRange(w, r, data, 0)
	}))
	defer srv.Close()

	p := newTestProvider(t)
	var got bytes.Buffer
	if err := p.Download(context.Background(), releaseFor(srv.URL, int64(len(data))), &got, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if len(ranges) != 2 {
		t.Fatalf("期望 2 次请求，实际 %d（%v）", len(ranges), ranges)
	}
	if ranges[0] != "" {
		t.Errorf("首次请求不该带 Range，实际 %q", ranges[0])
	}
	want := "bytes=" + strconv.Itoa(40<<10) + "-"
	if ranges[1] != want {
		t.Errorf("续传 Range = %q，期望 %q", ranges[1], want)
	}
	if !bytes.Equal(got.Bytes(), data) {
		t.Error("续传后字节流不一致")
	}
}

// TestDownloadServerIgnoresRange 覆盖服务端无视 Range、返回 200 全量的情形：
// 已写入的前缀必须被丢弃，不能重复写进 dst（否则摘要必然错）。
func TestDownloadServerIgnoresRange(t *testing.T) {
	data := payload(120 << 10)
	var hits atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			// 首次：传一半就断。
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data[:60<<10])
			w.(http.Flusher).Flush()
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		// 续传请求带了 Range，但这里故意无视它，返回 200 + 全量。
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	p := newTestProvider(t)
	var got bytes.Buffer
	if err := p.Download(context.Background(), releaseFor(srv.URL, int64(len(data))), &got, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if !bytes.Equal(got.Bytes(), data) {
		t.Errorf("忽略 Range 时字节流不一致：得到 %d 字节，期望 %d", got.Len(), len(data))
	}
}

// TestDownloadTruncatedBodyRetries 验证「干净地读到 EOF 但字节数不足」也会重试，
// 而不是放过去让它在 verify 阶段变成一句莫名的校验失败。
func TestDownloadTruncatedBodyRetries(t *testing.T) {
	data := payload(50 << 10)
	var hits atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			// 不声明 Content-Length，写一半就正常返回 —— 客户端只会看到 EOF。
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data[:20<<10])
			return
		}
		serveRange(w, r, data, 0)
	}))
	defer srv.Close()

	p := newTestProvider(t)
	var got bytes.Buffer
	if err := p.Download(context.Background(), releaseFor(srv.URL, int64(len(data))), &got, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if hits.Load() < 2 {
		t.Errorf("截断响应应触发重试，实际只请求了 %d 次", hits.Load())
	}
	if !bytes.Equal(got.Bytes(), data) {
		t.Errorf("重试后字节流不一致：得到 %d 字节，期望 %d", got.Len(), len(data))
	}
}

// TestDownloadFatalStatusNoRetry 验证 4xx 语义错误立即失败，不做无意义的重试。
func TestDownloadFatalStatusNoRetry(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p := newTestProvider(t)
	err := p.Download(context.Background(), releaseFor(srv.URL, 1024), io.Discard, nil)
	if err == nil {
		t.Fatal("期望失败，实际成功")
	}
	if hits.Load() != 1 {
		t.Errorf("404 不该重试，实际请求了 %d 次", hits.Load())
	}
}

// TestDownloadRetriesServerError 验证 5xx 会重试并最终成功。
func TestDownloadRetriesServerError(t *testing.T) {
	data := payload(8 << 10)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		serveRange(w, r, data, 0)
	}))
	defer srv.Close()

	p := newTestProvider(t)
	var got bytes.Buffer
	if err := p.Download(context.Background(), releaseFor(srv.URL, int64(len(data))), &got, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if !bytes.Equal(got.Bytes(), data) {
		t.Error("重试后字节流不一致")
	}
}

// TestDownloadGivesUpAfterMaxAttempts 验证重试有上限，不会无限打下去。
func TestDownloadGivesUpAfterMaxAttempts(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p := newTestProvider(t)
	err := p.Download(context.Background(), releaseFor(srv.URL, 1024), io.Discard, nil)
	if err == nil {
		t.Fatal("期望失败，实际成功")
	}
	if int(hits.Load()) != maxAttempts {
		t.Errorf("请求了 %d 次，期望恰好 %d 次", hits.Load(), maxAttempts)
	}
}

// TestDownloadHonoursContextCancel 验证调用方取消时立即返回，不进重试循环。
func TestDownloadHonoursContextCancel(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	p := newTestProvider(t)
	// 首次请求发出后立刻取消。
	p.sleep = func(c context.Context, _ time.Duration) error { return c.Err() }
	cancel()

	err := p.Download(ctx, releaseFor(srv.URL, 1024), io.Discard, nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v，期望 context.Canceled", err)
	}
}

// TestDownloadWriteErrorNotRetried 验证写盘失败（磁盘满等）不重试 —— 重试也不会好。
func TestDownloadWriteErrorNotRetried(t *testing.T) {
	data := payload(16 << 10)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		serveRange(w, r, data, 0)
	}))
	defer srv.Close()

	p := newTestProvider(t)
	wantErr := errors.New("no space left on device")
	err := p.Download(context.Background(), releaseFor(srv.URL, int64(len(data))),
		failingWriter{err: wantErr}, nil)
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v，期望包含 %v", err, wantErr)
	}
	if hits.Load() != 1 {
		t.Errorf("写盘失败不该重试，实际请求了 %d 次", hits.Load())
	}
}

type failingWriter struct{ err error }

func (f failingWriter) Write(p []byte) (int, error) { return 0, f.err }

// TestAssetURLMissing 验证 metadata 缺失时给出明确错误而不是 panic。
func TestAssetURLMissing(t *testing.T) {
	for _, tc := range []struct {
		name string
		rel  *updater.Release
	}{
		{"nil release", nil},
		{"nil metadata", &updater.Release{}},
		{"empty url", &updater.Release{Metadata: map[string]any{"github.asset.url": ""}}},
		{"wrong type", &updater.Release{Metadata: map[string]any{"github.asset.url": 42}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := assetURL(tc.rel); err == nil {
				t.Error("期望报错，实际为 nil")
			}
		})
	}
}

// TestErrorMessagesAreASCII 锁定本包的错误文本必须是纯 ASCII 英文。
//
// 这些文本会作为 updater.ErrorInfo.Message 原样送进更新窗口，而窗口是靠匹配英文关键词
// 才能映射出本地化文案（build/updater/window.html 的 renderError）。写中文会同时坏两件事：
// 英文 / 繁中界面漏出简体中文，且匹配不中任何分支、把内部诊断信息直接糊给用户。
func TestErrorMessagesAreASCII(t *testing.T) {
	msgs := map[string]string{
		"errStalled":    errStalled.Error(),
		"errIncomplete": errIncomplete.Error(),
		"statusError":   (&statusError{code: http.StatusBadGateway}).Error(),
		"writeError":    (&writeError{err: errors.New("no space left on device")}).Error(),
	}
	// 两个 assetURL 分支的文案也一并检查。
	if _, err := assetURL(nil); err != nil {
		msgs["assetURL/nil"] = err.Error()
	}
	if _, err := assetURL(&updater.Release{Metadata: map[string]any{}}); err != nil {
		msgs["assetURL/noURL"] = err.Error()
	}
	// 重试耗尽时的包装文案。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	p := newTestProvider(t)
	if err := p.Download(context.Background(), releaseFor(srv.URL, 1024), io.Discard, nil); err != nil {
		msgs["exhausted"] = err.Error()
	}

	for name, msg := range msgs {
		if msg == "" {
			t.Errorf("%s: 文案为空", name)
			continue
		}
		for _, r := range msg {
			if r > 127 {
				t.Errorf("%s: 含非 ASCII 字符 %q，完整文案：%s", name, r, msg)
				break
			}
		}
	}
}

// TestNewClientHasNoOverallTimeout 锁定本次修复的核心：http.Client.Timeout 必须为 0。
// 那个字段覆盖「连接 + 读完整个 body」，一旦被设上，8 MB 的更新包在慢速链路上必然
// 被拦腰砍断 —— 这正是国内用户看到 download 阶段失败的根因。
func TestNewClientHasNoOverallTimeout(t *testing.T) {
	c := NewClient(nil)
	if c.Timeout != 0 {
		t.Errorf("Client.Timeout = %v，必须为 0（改用 Transport 分段超时）", c.Timeout)
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport 类型 = %T，期望 *http.Transport", c.Transport)
	}
	if tr.ResponseHeaderTimeout == 0 {
		t.Error("ResponseHeaderTimeout 为 0：去掉整体超时后必须有分段超时兜底")
	}
	if tr.TLSHandshakeTimeout == 0 {
		t.Error("TLSHandshakeTimeout 为 0")
	}
	if tr.Proxy == nil {
		t.Error("Transport.Proxy 为 nil：代理设置将完全失效")
	}
}

// TestProxyHotSwap 验证代理可热更新，且清除后回落到环境变量。
func TestProxyHotSwap(t *testing.T) {
	p := &Proxy{}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/x", nil)
	fn := p.proxyFunc()

	p.SetProxy("127.0.0.1", 7890)
	u, err := fn(req)
	if err != nil {
		t.Fatalf("proxyFunc: %v", err)
	}
	if u == nil || u.Host != "127.0.0.1:7890" {
		t.Fatalf("代理 = %v，期望 127.0.0.1:7890", u)
	}

	// 同一个函数必须看到后续变更（热更新，而非构造时快照）。
	p.SetProxy("10.0.0.1", 1080)
	if u, _ = fn(req); u == nil || u.Host != "10.0.0.1:1080" {
		t.Errorf("热更新后代理 = %v，期望 10.0.0.1:1080", u)
	}

	// 清除后回落到 http.ProxyFromEnvironment。
	//
	// 这里断言的是「委托关系」而非某个具体环境变量的效果：net/http 用 sync.Once 把
	// 环境变量快照缓存在进程级（envProxyFunc），本测试进程里它早已被前面的调用固化，
	// t.Setenv 再改也不会生效。所以直接与 http.ProxyFromEnvironment 的返回值比对。
	p.SetProxy("", 0)
	got, gotErr := fn(req)
	want, wantErr := http.ProxyFromEnvironment(req)
	if fmt.Sprint(got) != fmt.Sprint(want) || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
		t.Errorf("清除后 = (%v, %v)，期望与 http.ProxyFromEnvironment 一致 (%v, %v)",
			got, gotErr, want, wantErr)
	}
}

// TestProxyInvalidInputs 验证非法输入被安全地当作「无代理」，不会 panic 也不会
// 留下上一次的值。
func TestProxyInvalidInputs(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/x", nil)
	envProxy, _ := http.ProxyFromEnvironment(req)

	for _, tc := range []struct {
		name string
		host string
		port int
	}{
		{"空 host", "", 8080},
		{"零端口", "127.0.0.1", 0},
		{"负端口", "127.0.0.1", -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &Proxy{}
			p.SetProxy("10.0.0.1", 1080) // 先设一个有效值
			p.SetProxy(tc.host, tc.port) // 再用非法值覆盖
			// 非法值必须清掉旧值、退回环境变量，而不是留着 10.0.0.1。
			if u, _ := p.proxyFunc()(req); fmt.Sprint(u) != fmt.Sprint(envProxy) {
				t.Errorf("代理 = %v，期望回落到环境变量取值 %v", u, envProxy)
			}
		})
	}
}

// TestTotalFrom 覆盖资源总长的推断：206 优先取 Content-Range 的分母。
func TestTotalFrom(t *testing.T) {
	tests := []struct {
		name string
		code int
		hdr  string
		clen int64
		from int64
		want int64
	}{
		{"206 带 Content-Range", http.StatusPartialContent, "bytes 200-1023/1024", 824, 200, 1024},
		{"206 无 Content-Range", http.StatusPartialContent, "", 824, 200, 1024},
		{"206 都没有", http.StatusPartialContent, "", -1, 200, 0},
		{"200 用 ContentLength", http.StatusOK, "", 1024, 0, 1024},
		{"200 长度未知", http.StatusOK, "", -1, 0, 0},
		{"Content-Range 畸形", http.StatusPartialContent, "bytes 200-1023/abc", 824, 200, 1024},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode:    tc.code,
				Header:        http.Header{},
				ContentLength: tc.clen,
			}
			if tc.hdr != "" {
				resp.Header.Set("Content-Range", tc.hdr)
			}
			if got := totalFrom(resp, tc.from); got != tc.want {
				t.Errorf("totalFrom = %d，期望 %d", got, tc.want)
			}
		})
	}
}

// TestRetryable 覆盖重试判定表。
func TestRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"停滞", errStalled, true},
		{"截断", errIncomplete, true},
		{"包装后的停滞", fmt.Errorf("wrapped: %w", errStalled), true},
		{"调用方取消", context.Canceled, false},
		{"超出 deadline", context.DeadlineExceeded, false},
		{"404", &statusError{code: http.StatusNotFound}, false},
		{"403", &statusError{code: http.StatusForbidden}, false},
		{"408", &statusError{code: http.StatusRequestTimeout}, true},
		{"429", &statusError{code: http.StatusTooManyRequests}, true},
		{"500", &statusError{code: http.StatusInternalServerError}, true},
		{"503", &statusError{code: http.StatusServiceUnavailable}, true},
		{"传输层错误", errors.New("connection reset by peer"), true},
		{"意外 EOF", io.ErrUnexpectedEOF, true},
		{"url.Error 包装", &url.Error{Op: "Get", Err: errors.New("EOF")}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := retryable(tc.err); got != tc.want {
				t.Errorf("retryable(%v) = %v，期望 %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestBackoff 验证退避序列递增且有上限。
func TestBackoff(t *testing.T) {
	if got := backoff(0); got != 0 {
		t.Errorf("backoff(0) = %v，期望 0", got)
	}
	prev := time.Duration(0)
	for i := 1; i <= maxAttempts; i++ {
		d := backoff(i)
		if d < prev {
			t.Errorf("backoff(%d) = %v，比前一次 %v 更小", i, d, prev)
		}
		if d > 15*time.Second {
			t.Errorf("backoff(%d) = %v，超过 15s 上限", i, d)
		}
		prev = d
	}
}

// TestProviderNameIsGithub 锁定 Name() 继承自内层 provider。
//
// updater.DownloadAndInstall 用 findProvider(cfg.Providers, pending.Provider) 按名字
// 把下载路由回产出该 release 的 provider（pkg/updater/updater.go）。若包装层改了名字，
// Check 能成功而 Download 会因找不到 provider 直接失败。
func TestProviderNameIsGithub(t *testing.T) {
	p := newTestProvider(t)
	if got := p.Name(); got != "github" {
		t.Errorf("Name() = %q，必须为 \"github\"（否则下载阶段路由不回来）", got)
	}
}

// TestNewRequiresRepository 验证必填项校验被内层 provider 执行。
func TestNewRequiresRepository(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Error("缺少 Repository 时期望报错")
	}
	if _, err := New(Config{Repository: "no-slash"}); err == nil {
		t.Error("Repository 格式错误时期望报错")
	}
}

// TestDownloadStallDetection 验证「连接不断但没有数据」会被看门狗掐断并重试。
// 这是 GFW 最常见的形态：限速到 0 而不断开，TCP 层看不出任何异常。
//
// 为免真实等待 stallTimeout，这里临时把它调小。
func TestDownloadStallDetection(t *testing.T) {
	origTimeout, origInterval := stallTimeoutVar, stallCheckIntervalVar
	stallTimeoutVar, stallCheckIntervalVar = 150*time.Millisecond, 20*time.Millisecond
	defer func() { stallTimeoutVar, stallCheckIntervalVar = origTimeout, origInterval }()

	data := payload(32 << 10)
	var hits atomic.Int32
	release := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			// 发头 + 少量数据，然后彻底不动，直到测试结束。
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data[:1024])
			w.(http.Flusher).Flush()
			<-release
			return
		}
		serveRange(w, r, data, 0)
	}))
	defer srv.Close()
	defer close(release)

	p := newTestProvider(t)
	var got bytes.Buffer
	if err := p.Download(context.Background(), releaseFor(srv.URL, int64(len(data))), &got, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if hits.Load() < 2 {
		t.Errorf("停滞应被掐断并重试，实际只请求了 %d 次", hits.Load())
	}
	if !bytes.Equal(got.Bytes(), data) {
		t.Errorf("停滞重试后字节流不一致：得到 %d 字节，期望 %d", got.Len(), len(data))
	}
}
