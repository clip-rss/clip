package webdav

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

/* ---------- 测试脚手架 ---------- */

// recordedRequest 记录一次请求的关键信息，供断言协议细节。
type recordedRequest struct {
	Method        string
	Path          string
	Depth         string
	IfMatch       string
	IfNoneMatch   string
	ContentType   string
	ContentLength int64
	Body          string
	User          string
	Pass          string
	HadAuth       bool
}

// fakeDAV 一个极简 WebDAV 服务器替身。
//
// 用 httptest.NewTLSServer 而非 NewServer：客户端强制 https，明文服务器根本
// 连不上。srv.Client() 返回信任其自签证书的客户端，经 WithHTTPClient 注入，
// 这样测试走的是真实 TLS 路径，不需要在生产代码里留测试专用旁路。
type fakeDAV struct {
	t        *testing.T
	srv      *httptest.Server
	requests []recordedRequest
	handler  func(w http.ResponseWriter, r *http.Request, body string)
}

func newFakeDAV(t *testing.T) *fakeDAV {
	t.Helper()
	f := &fakeDAV{t: t}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body := string(raw)
		user, pass, hadAuth := r.BasicAuth()
		f.requests = append(f.requests, recordedRequest{
			Method:        r.Method,
			Path:          r.URL.Path,
			Depth:         r.Header.Get("Depth"),
			IfMatch:       r.Header.Get("If-Match"),
			IfNoneMatch:   r.Header.Get("If-None-Match"),
			ContentType:   r.Header.Get("Content-Type"),
			ContentLength: r.ContentLength,
			Body:          body,
			User:          user,
			Pass:          pass,
			HadAuth:       hadAuth,
		})
		if f.handler != nil {
			f.handler(w, r, body)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// client 基于替身服务器构造客户端。
func (f *fakeDAV) client(opts ...Option) *Client {
	f.t.Helper()
	all := append([]Option{WithHTTPClient(f.srv.Client())}, opts...)
	c, err := New(Config{
		URL:      f.srv.URL + "/dav/",
		Username: "alice",
		Password: "app-token",
	}, all...)
	if err != nil {
		f.t.Fatalf("New: %v", err)
	}
	return c
}

// methods 返回已记录请求的方法序列，便于断言调用顺序。
func (f *fakeDAV) methods() []string {
	out := make([]string, 0, len(f.requests))
	for _, r := range f.requests {
		out = append(out, r.Method)
	}
	return out
}

// last 返回最后一次请求。
func (f *fakeDAV) last() recordedRequest {
	f.t.Helper()
	if len(f.requests) == 0 {
		f.t.Fatal("no requests recorded")
	}
	return f.requests[len(f.requests)-1]
}

// multistatusXML 拼一份 207 响应体。prefix 用于模拟各家不同的命名空间前缀。
func multistatusXML(prefix, href, etag string, size int64, collection bool) string {
	p := ""
	if prefix != "" {
		p = prefix + ":"
	}
	ns := `xmlns:` + strings.TrimSuffix(prefix, ":") + `="DAV:"`
	if prefix == "" {
		ns = `xmlns="DAV:"`
	}
	resourceType := ""
	if collection {
		resourceType = fmt.Sprintf("<%sresourcetype><%scollection/></%sresourcetype>", p, p, p)
	} else {
		resourceType = fmt.Sprintf("<%sresourcetype/>", p)
	}
	return fmt.Sprintf(`<?xml version="1.0"?>
<%[1]smultistatus %[2]s>
  <%[1]sresponse>
    <%[1]shref>%[3]s</%[1]shref>
    <%[1]spropstat>
      <%[1]sprop>
        <%[1]sgetetag>%[4]s</%[1]sgetetag>
        <%[1]sgetcontentlength>%[5]d</%[1]sgetcontentlength>
        <%[1]sgetlastmodified>Mon, 02 Jan 2006 15:04:05 GMT</%[1]sgetlastmodified>
        %[6]s
      </%[1]sprop>
      <%[1]sstatus>HTTP/1.1 200 OK</%[1]sstatus>
    </%[1]spropstat>
  </%[1]sresponse>
</%[1]smultistatus>`, p, ns, href, etag, size, resourceType)
}

/* ---------- 配置校验 ---------- */

func TestNewRejectsPlainHTTP(t *testing.T) {
	_, err := New(Config{URL: "http://dav.example.com/dav/"})
	if err == nil {
		t.Fatal("plain http should be rejected")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("err = %v, want ErrInvalidConfig", err)
	}
	// 明文会泄露凭据，报错必须说清原因而非只说「地址无效」。
	if !strings.Contains(err.Error(), "https") {
		t.Errorf("error should explain the https requirement, got %q", err)
	}
}

func TestNewRejectsBadConfig(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "empty", url: ""},
		{name: "whitespace only", url: "   "},
		{name: "no scheme", url: "dav.example.com/dav/"},
		{name: "no host", url: "https:///dav/"},
		{name: "ftp scheme", url: "ftp://dav.example.com/dav/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(Config{URL: tt.url}); !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("err = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

// 基地址少写结尾斜杠是最常见的手输错误，不能让最后一段被当成文件名替换掉。
func TestBaseURLWithoutTrailingSlash(t *testing.T) {
	f := newFakeDAV(t)
	c, err := New(Config{URL: f.srv.URL + "/dav/clip"}, WithHTTPClient(f.srv.Client()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusOK)
	}
	if _, _, err := c.Get(context.Background(), "settings.json"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := f.last().Path; got != "/dav/clip/settings.json" {
		t.Errorf("path = %q, want /dav/clip/settings.json", got)
	}
}

func TestResolveRejectsEscapingPath(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	if _, _, err := c.Get(context.Background(), "../../etc/passwd"); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("err = %v, want ErrInvalidConfig for escaping path", err)
	}
	if len(f.requests) != 0 {
		t.Errorf("escaping path should not reach the server, got %d requests", len(f.requests))
	}
}

func TestWithTimeoutDoesNotMutateInjectedClient(t *testing.T) {
	f := newFakeDAV(t)
	injected := f.srv.Client()
	injected.Timeout = 2 * time.Second
	c, err := New(
		Config{URL: f.srv.URL + "/dav/"},
		WithTimeout(7*time.Minute),
		WithHTTPClient(injected),
	)
	if err != nil {
		t.Fatal(err)
	}
	if c.http.Timeout != 7*time.Minute {
		t.Errorf("timeout = %v, want 7m", c.http.Timeout)
	}
	if injected.Timeout != 2*time.Second {
		t.Errorf("injected client mutated: %v", injected.Timeout)
	}
}

/* ---------- 认证 ---------- */

func TestRequestsCarryBasicAuth(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusOK)
	}
	if _, _, err := c.Get(context.Background(), "settings.json"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	got := f.last()
	if !got.HadAuth {
		t.Fatal("request carried no Basic auth")
	}
	if got.User != "alice" || got.Pass != "app-token" {
		t.Errorf("credentials = %q/%q, want alice/app-token", got.User, got.Pass)
	}
}

func TestUnauthorizedMapsToSentinel(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			f := newFakeDAV(t)
			c := f.client()
			f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
				w.WriteHeader(status)
			}
			_, _, err := c.Get(context.Background(), "settings.json")
			if !errors.Is(err, ErrUnauthorized) {
				t.Errorf("err = %v, want ErrUnauthorized", err)
			}
		})
	}
}

/* ---------- Stat / PROPFIND ---------- */

// Depth 不是可选的：缺失时部分服务器直接 400，另一些返回整个子树。
func TestStatSendsDepthZero(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		if r.Header.Get("Depth") == "" {
			// 模拟真实服务器对缺失 Depth 的反应。
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(multistatusXML("d", "/dav/settings.json", `"abc123"`, 42, false)))
	}

	st, err := c.Stat(context.Background(), "settings.json")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := f.last().Depth; got != "0" {
		t.Errorf("Depth = %q, want 0", got)
	}
	if st.ETag != "abc123" {
		t.Errorf("etag = %q, want abc123", st.ETag)
	}
	if st.Size != 42 {
		t.Errorf("size = %d, want 42", st.Size)
	}
	if st.LastModified.IsZero() {
		t.Error("last modified should be parsed")
	}
	if st.IsDir {
		t.Error("file should not be reported as a directory")
	}
}

func TestStatRequestsOnlyNeededProps(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(multistatusXML("d", "/dav/settings.json", `"e"`, 1, false)))
	}
	if _, err := c.Stat(context.Background(), "settings.json"); err != nil {
		t.Fatalf("Stat: %v", err)
	}
	body := f.last().Body
	// <allprop> 在大目录上会让服务器吐出大量无用数据。
	if strings.Contains(body, "allprop") {
		t.Error("PROPFIND should request specific props, not allprop")
	}
	for _, want := range []string{"getetag", "getcontentlength", "getlastmodified"} {
		if !strings.Contains(body, want) {
			t.Errorf("PROPFIND body missing %s: %s", want, body)
		}
	}
}

// 各家服务器命名空间前缀不一（d: / D: / lp1: / 无前缀），按 URI 解析才能通吃。
func TestStatParsesAnyNamespacePrefix(t *testing.T) {
	for _, prefix := range []string{"d", "D", "lp1", ""} {
		name := prefix
		if name == "" {
			name = "(none)"
		}
		t.Run(name, func(t *testing.T) {
			f := newFakeDAV(t)
			c := f.client()
			f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
				w.WriteHeader(http.StatusMultiStatus)
				_, _ = w.Write([]byte(multistatusXML(prefix, "/dav/settings.json", `"tag9"`, 7, false)))
			}
			st, err := c.Stat(context.Background(), "settings.json")
			if err != nil {
				t.Fatalf("Stat: %v", err)
			}
			if st.ETag != "tag9" {
				t.Errorf("etag = %q, want tag9", st.ETag)
			}
		})
	}
}

func TestStatDetectsCollection(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(multistatusXML("d", "/dav/", "", 0, true)))
	}
	st, err := c.Stat(context.Background(), "")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !st.IsDir {
		t.Error("collection should be reported as a directory")
	}
}

// 属性可分散在多个 propstat 中，未取到的那组状态为 404。
// 若不按状态过滤，404 组的空值会覆盖掉 200 组的真实值。
func TestStatIgnoresNotFoundPropstat(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:href>/dav/settings.json</d:href>
    <d:propstat>
      <d:prop><d:getetag>"real"</d:getetag></d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
    <d:propstat>
      <d:prop><d:getcontentlength/><d:getlastmodified/></d:prop>
      <d:status>HTTP/1.1 404 Not Found</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`))
	}
	st, err := c.Stat(context.Background(), "settings.json")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if st.ETag != "real" {
		t.Errorf("etag = %q, want real (404 propstat must not clobber it)", st.ETag)
	}
}

func TestStatNotFound(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusNotFound)
	}
	if _, err := c.Stat(context.Background(), "settings.json"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestStatMalformedXML(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<html><body>Login page</body></html>`))
	}
	_, err := c.Stat(context.Background(), "settings.json")
	if !errors.Is(err, ErrBadResponse) {
		t.Errorf("err = %v, want ErrBadResponse", err)
	}
}

func TestStatEmptyMultistatus(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><d:multistatus xmlns:d="DAV:"></d:multistatus>`))
	}
	if _, err := c.Stat(context.Background(), "settings.json"); !errors.Is(err, ErrBadResponse) {
		t.Errorf("err = %v, want ErrBadResponse", err)
	}
}

/* ---------- ETag 归一化 ---------- */

// 同一 ETag 在不同响应里可能写作 "abc" / W/"abc" / abc。不归一化会把
// 「没变」误判成「变了」，触发无谓的冲突提示。
func TestNormalizeETag(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: `"abc123"`, want: "abc123"},
		{raw: `W/"abc123"`, want: "abc123"},
		{raw: `w/"abc123"`, want: "abc123"},
		{raw: `abc123`, want: "abc123"},
		{raw: `  "abc123"  `, want: "abc123"},
		{raw: ``, want: ""},
		{raw: `"66f-5d8a1b2c"`, want: "66f-5d8a1b2c"},
	}
	for _, tt := range tests {
		if got := normalizeETag(tt.raw); got != tt.want {
			t.Errorf("normalizeETag(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestGetNormalizesWeakETag(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.Header().Set("ETag", `W/"weak-tag"`)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
	body, etag, err := c.Get(context.Background(), "settings.json")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if etag != "weak-tag" {
		t.Errorf("etag = %q, want weak-tag", etag)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q", body)
	}
}

/* ---------- Get ---------- */

// 首次同步时远端配置必然不存在，这是正常状态而非错误。
func TestGetNotFoundIsSentinel(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusNotFound)
	}
	_, _, err := c.Get(context.Background(), "settings.json")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// 上限存在的目的是防止异常或恶意服务器持续灌数据把内存吃满。
func TestGetTruncatesOversizedResponse(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		chunk := strings.Repeat("x", 64<<10)
		for i := 0; i < 40; i++ { // 2.5 MiB > 1 MiB 上限
			_, _ = w.Write([]byte(chunk))
		}
	}
	body, _, err := c.Get(context.Background(), "settings.json")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(body) > maxResponseBytes {
		t.Errorf("body = %d bytes, want <= %d", len(body), maxResponseBytes)
	}
}

func TestGetToStreamsLargeResponse(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	want := bytes.Repeat([]byte("database-block"), 120000)
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.Header().Set("ETag", `"db-v1"`)
		_, _ = w.Write(want)
	}

	var got bytes.Buffer
	etag, written, err := c.GetTo(context.Background(), "backup.db", &got, int64(len(want)+1))
	if err != nil {
		t.Fatalf("GetTo: %v", err)
	}
	if etag != "db-v1" || written != int64(len(want)) {
		t.Errorf("etag/written = %q/%d, want db-v1/%d", etag, written, len(want))
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Error("streamed body differs")
	}
}

func TestGetToRejectsResponseOverLimit(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), 2048))
	}

	var got bytes.Buffer
	_, _, err := c.GetTo(context.Background(), "backup.db", &got, 1024)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("err = %v, want ErrResponseTooLarge", err)
	}
}

/* ---------- Put ---------- */

func TestPutSendsBodyAndReturnsETag(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.Header().Set("ETag", `"new-tag"`)
		w.WriteHeader(http.StatusCreated)
	}
	etag, err := c.Put(context.Background(), "settings.json", []byte(`{"a":1}`), "")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if etag != "new-tag" {
		t.Errorf("etag = %q, want new-tag", etag)
	}
	got := f.last()
	if got.Method != http.MethodPut {
		t.Errorf("method = %s, want PUT", got.Method)
	}
	if got.Body != `{"a":1}` {
		t.Errorf("body = %q", got.Body)
	}
	if got.IfMatch != "" {
		t.Errorf("If-Match should be absent when not requested, got %q", got.IfMatch)
	}
}

func TestPutStreamSendsFileMetadataAndCreatePrecondition(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.Header().Set("ETag", `"db-v1"`)
		w.WriteHeader(http.StatusCreated)
	}
	body := "sqlite-database"
	etag, err := c.PutStream(
		context.Background(),
		"backup.db",
		strings.NewReader(body),
		int64(len(body)),
		PutOptions{ContentType: "application/vnd.sqlite3", IfNoneMatch: true},
	)
	if err != nil {
		t.Fatalf("PutStream: %v", err)
	}
	if etag != "db-v1" {
		t.Errorf("etag = %q, want db-v1", etag)
	}
	got := f.last()
	if got.Body != body || got.ContentLength != int64(len(body)) {
		t.Errorf("body/length = %q/%d", got.Body, got.ContentLength)
	}
	if got.ContentType != "application/vnd.sqlite3" {
		t.Errorf("content type = %q", got.ContentType)
	}
	if got.IfNoneMatch != "*" || got.IfMatch != "" {
		t.Errorf("conditions = If-None-Match %q, If-Match %q", got.IfNoneMatch, got.IfMatch)
	}
}

func TestPutSendsIfMatchQuoted(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.Header().Set("ETag", `"v2"`)
		w.WriteHeader(http.StatusNoContent)
	}
	// 内部存裸值，发送时须补引号（RFC 要求 entity-tag 形式）。
	if _, err := c.Put(context.Background(), "settings.json", []byte("{}"), "v1"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if got := f.last().IfMatch; got != `"v1"` {
		t.Errorf("If-Match = %q, want \"v1\"", got)
	}
}

func TestPutConflictMapsToSentinel(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusPreconditionFailed)
	}
	_, err := c.Put(context.Background(), "settings.json", []byte("{}"), "stale")
	if !errors.Is(err, ErrConflict) {
		t.Errorf("err = %v, want ErrConflict", err)
	}
}

// 多数服务器的 PUT 响应不带 ETag，需补一次 PROPFIND 取新值。
func TestPutFallsBackToStatForETag(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusCreated) // 不带 ETag
			return
		}
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(multistatusXML("d", "/dav/settings.json", `"from-stat"`, 2, false)))
	}
	etag, err := c.Put(context.Background(), "settings.json", []byte("{}"), "")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if etag != "from-stat" {
		t.Errorf("etag = %q, want from-stat", etag)
	}
	if got := f.methods(); len(got) != 2 || got[0] != "PUT" || got[1] != "PROPFIND" {
		t.Errorf("methods = %v, want [PUT PROPFIND]", got)
	}
}

// 内容已写成功，只是拿不到新 ETag。不该把整次同步判为失败。
func TestPutSucceedsWhenETagUnobtainable(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusInternalServerError) // 补取 ETag 失败
	}
	etag, err := c.Put(context.Background(), "settings.json", []byte("{}"), "")
	if err != nil {
		t.Fatalf("Put should tolerate ETag lookup failure, got %v", err)
	}
	if etag != "" {
		t.Errorf("etag = %q, want empty", etag)
	}
}

func TestPutStorageAndLockErrors(t *testing.T) {
	tests := []struct {
		status int
		want   error
	}{
		{status: http.StatusInsufficientStorage, want: ErrInsufficientStorage},
		{status: http.StatusLocked, want: ErrLocked},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.status), func(t *testing.T) {
			f := newFakeDAV(t)
			c := f.client()
			f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
				w.WriteHeader(tt.status)
			}
			_, err := c.Put(context.Background(), "settings.json", []byte("{}"), "")
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

/* ---------- Mkcol / MkcolAll ---------- */

// 405 = 该地址已存在集合，是重复建目录的正常回应。
func TestMkcolTreatsExistingAsSuccess(t *testing.T) {
	for _, status := range []int{
		http.StatusMethodNotAllowed,
		http.StatusMovedPermanently,
		http.StatusFound,
	} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := newFakeDAV(t)
			c := f.client()
			f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
				w.WriteHeader(status)
			}
			if err := c.Mkcol(context.Background(), "clip/"); err != nil {
				t.Errorf("Mkcol on existing dir should succeed, got %v", err)
			}
		})
	}
}

// MKCOL 在父目录不存在时返回 409，多数服务器不会自动补建，故必须逐级建。
func TestMkcolAllCreatesParentsInOrder(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusCreated)
	}
	if err := c.MkcolAll(context.Background(), "clip/config/v1/"); err != nil {
		t.Fatalf("MkcolAll: %v", err)
	}

	var paths []string
	for _, r := range f.requests {
		if r.Method != "MKCOL" {
			t.Fatalf("unexpected method %s", r.Method)
		}
		paths = append(paths, r.Path)
	}
	want := []string{"/dav/clip/", "/dav/clip/config/", "/dav/clip/config/v1/"}
	if len(paths) != len(want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("paths[%d] = %q, want %q", i, paths[i], want[i])
		}
	}
}

func TestMkcolAllStopsOnRealError(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusUnauthorized)
	}
	err := c.MkcolAll(context.Background(), "clip/config/")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	// 认证失败时不该继续尝试后续层级。
	if len(f.requests) != 1 {
		t.Errorf("requests = %d, want 1 (should abort on first failure)", len(f.requests))
	}
}

func TestMkcolAllEmptyPathIsNoop(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	for _, p := range []string{"", "/", "///"} {
		if err := c.MkcolAll(context.Background(), p); err != nil {
			t.Errorf("MkcolAll(%q) = %v, want nil", p, err)
		}
	}
	if len(f.requests) != 0 {
		t.Errorf("empty path should issue no requests, got %d", len(f.requests))
	}
}

func TestMkcolParentMissingMapsToSentinel(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusConflict)
	}
	if err := c.Mkcol(context.Background(), "a/b/"); !errors.Is(err, ErrNotCollection) {
		t.Errorf("err = %v, want ErrNotCollection", err)
	}
}

/* ---------- Delete ---------- */

func TestDelete(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusNoContent)
	}
	if err := c.Delete(context.Background(), "probe.tmp"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := f.last().Method; got != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", got)
	}
}

/* ---------- 网络与上下文 ---------- */

func TestNetworkFailureMapsToSentinel(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.srv.Close() // 关掉服务器模拟连不上

	_, _, err := c.Get(context.Background(), "settings.json")
	if !errors.Is(err, ErrNetwork) {
		t.Errorf("err = %v, want ErrNetwork", err)
	}
}

func TestContextCancellation(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := c.Get(ctx, "settings.json")
	if err == nil {
		t.Fatal("cancelled context should fail")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want to wrap context.Canceled", err)
	}
}

/* ---------- 错误信息 ---------- */

// 错误体常是整页 HTML；全量塞进错误信息会污染日志且无助定位。
func TestErrorTruncatesLongBody(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat("verbose html error ", 200)))
	}
	_, _, err := c.Get(context.Background(), "settings.json")
	if err == nil {
		t.Fatal("expected error")
	}
	if len(err.Error()) > 400 {
		t.Errorf("error message too long (%d chars): %q", len(err.Error()), err.Error())
	}
}

// 错误信息里绝不能出现密码 —— 它会进日志与用户提交的问题报告。
func TestErrorNeverLeaksPassword(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	_, _, err := c.Get(context.Background(), "settings.json")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "app-token") {
		t.Errorf("error leaked the password: %q", err)
	}
}

func TestStatusErrorCarriesDiagnostics(t *testing.T) {
	f := newFakeDAV(t)
	c := f.client()
	f.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusInsufficientStorage)
	}
	_, err := c.Put(context.Background(), "settings.json", []byte("{}"), "")

	var davErr *Error
	if !errors.As(err, &davErr) {
		t.Fatalf("err = %v, want *webdav.Error", err)
	}
	if davErr.Status != http.StatusInsufficientStorage {
		t.Errorf("status = %d, want 507", davErr.Status)
	}
	if davErr.Op != "put" {
		t.Errorf("op = %q, want put", davErr.Op)
	}
	if davErr.Path != "settings.json" {
		t.Errorf("path = %q, want settings.json", davErr.Path)
	}
}

/* ---------- 辅助函数 ---------- */

func TestSplitPath(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{in: "a/b/c", want: []string{"a", "b", "c"}},
		{in: "/a/b/", want: []string{"a", "b"}},
		{in: "a//b", want: []string{"a", "b"}},
		{in: "", want: nil},
		{in: "/", want: nil},
		{in: "single", want: []string{"single"}},
	}
	for _, tt := range tests {
		got := splitPath(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("splitPath(%q) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range tt.want {
			if got[i] != tt.want[i] {
				t.Errorf("splitPath(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
			}
		}
	}
}

func TestParseHTTPTimeFormats(t *testing.T) {
	// 各家服务器的日期格式不统一，解析失败也不该让同步失败。
	valid := []string{
		"Mon, 02 Jan 2006 15:04:05 GMT",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Monday, 02-Jan-06 15:04:05 GMT",
	}
	for _, raw := range valid {
		if parseHTTPTime(raw).IsZero() {
			t.Errorf("parseHTTPTime(%q) returned zero", raw)
		}
	}
	if !parseHTTPTime("not a date").IsZero() {
		t.Error("unparseable date should yield zero time, not an error")
	}
}

/* ---------- 代理 ---------- */

// WithProxy 的可观测契约是「Transport.Proxy 对给定请求返回哪个代理地址」。
// 真起一个支持 CONNECT 隧道的 HTTPS 代理成本过高，这里直接验配置结果。
func TestWithProxyConfiguresTransport(t *testing.T) {
	c, err := New(Config{URL: "https://dav.example.com/dav/"}, WithProxy("127.0.0.1", 8080))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tr, ok := c.http.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport = %T, want *http.Transport", c.http.Transport)
	}
	if tr.Proxy == nil {
		t.Fatal("Transport.Proxy is nil, proxy not configured")
	}

	req, _ := http.NewRequest(http.MethodGet, "https://dav.example.com/dav/x.json", nil)
	proxyURL, err := tr.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy(): %v", err)
	}
	if proxyURL == nil {
		t.Fatal("Proxy() returned nil URL")
	}
	if got, want := proxyURL.String(), "http://127.0.0.1:8080"; got != want {
		t.Errorf("proxy = %q, want %q", got, want)
	}
}

func TestWithProxyIgnoresInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
	}{
		{name: "empty host", host: "", port: 8080},
		{name: "zero port", host: "127.0.0.1", port: 0},
		{name: "negative port", host: "127.0.0.1", port: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(Config{URL: "https://dav.example.com/dav/"}, WithProxy(tt.host, tt.port))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			// 配置无效时不应装上 Transport —— 否则会把请求发到一个不存在的代理，
			// 表现为「同步突然全部超时」，比直连失败更难排查。
			if c.http.Transport != nil {
				t.Errorf("Transport = %v, want nil for invalid proxy config", c.http.Transport)
			}
		})
	}
}

// 选项顺序不影响结果。WithProxy 只记地址，真正应用发生在 New 里所有 Option
// 跑完之后，故 WithHTTPClient 写在前或后都一样。
func TestProxyIsOrderIndependent(t *testing.T) {
	custom := func() *http.Client { return &http.Client{} }

	orders := []struct {
		name string
		opts func(h *http.Client) []Option
	}{
		{
			name: "client then proxy",
			opts: func(h *http.Client) []Option {
				return []Option{WithHTTPClient(h), WithProxy("127.0.0.1", 8080)}
			},
		},
		{
			name: "proxy then client",
			opts: func(h *http.Client) []Option {
				return []Option{WithProxy("127.0.0.1", 8080), WithHTTPClient(h)}
			},
		},
	}
	for _, o := range orders {
		t.Run(o.name, func(t *testing.T) {
			h := custom()
			c, err := New(Config{URL: "https://dav.example.com/dav/"}, o.opts(h)...)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			tr, ok := c.http.Transport.(*http.Transport)
			if !ok || tr.Proxy == nil {
				t.Fatalf("proxy not applied (Transport = %#v)", c.http.Transport)
			}
			req, _ := http.NewRequest(http.MethodGet, "https://dav.example.com/dav/x", nil)
			got, err := tr.Proxy(req)
			if err != nil || got == nil {
				t.Fatalf("Proxy() = %v, %v", got, err)
			}
			if got.String() != "http://127.0.0.1:8080" {
				t.Errorf("proxy = %q, want http://127.0.0.1:8080", got)
			}
		})
	}
}

// 不得改动调用方持有的 *http.Client。
//
// 这不是洁癖：*http.Client 在 Go 里通常是全应用共用的单例，就地给它装上代理
// Transport 会把无关请求（RSS 抓取、更新检查）一起送进用户的代理，且极难排查。
func TestProxyDoesNotMutateCallerClient(t *testing.T) {
	shared := &http.Client{Timeout: 7 * time.Second}

	c, err := New(Config{URL: "https://dav.example.com/dav/"},
		WithHTTPClient(shared), WithProxy("127.0.0.1", 8080))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if shared.Transport != nil {
		t.Error("caller's http.Client was mutated: Transport got set on it")
	}
	if c.http == shared {
		t.Error("client should hold a copy, not the caller's instance")
	}
	// 副本要保留原有设置，只换 Transport。
	if c.http.Timeout != 7*time.Second {
		t.Errorf("copy lost Timeout: got %v, want 7s", c.http.Timeout)
	}
}

// 无代理时不复制，直接用注入的实例（省一次拷贝，也便于测试比对身份）。
func TestNoProxyKeepsInjectedClient(t *testing.T) {
	shared := &http.Client{}
	c, err := New(Config{URL: "https://dav.example.com/dav/"}, WithHTTPClient(shared))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.http != shared {
		t.Error("without proxy the injected client should be used as-is")
	}
}

/* ---------- 状态码分类 ---------- */

func TestClassifyStatus(t *testing.T) {
	tests := []struct {
		status   int
		sentinel error // nil 表示该状态码无对应哨兵
	}{
		{status: 401, sentinel: ErrUnauthorized},
		{status: 403, sentinel: ErrUnauthorized},
		{status: 404, sentinel: ErrNotFound},
		{status: 405, sentinel: nil},
		{status: 409, sentinel: ErrNotCollection},
		{status: 412, sentinel: ErrConflict},
		{status: 413, sentinel: nil},
		{status: 423, sentinel: ErrLocked},
		{status: 429, sentinel: nil},
		{status: 507, sentinel: ErrInsufficientStorage},
		{status: 500, sentinel: nil},
		{status: 502, sentinel: nil},
		{status: 418, sentinel: nil},
		{status: 302, sentinel: nil},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.status), func(t *testing.T) {
			sentinel, msg := classifyStatus(tt.status)
			if sentinel != tt.sentinel {
				t.Errorf("sentinel = %v, want %v", sentinel, tt.sentinel)
			}
			// 每个分支都要有面向用户的说明，否则界面上会出现空错误。
			if msg == "" {
				t.Error("message is empty; every status must carry user-facing text")
			}
		})
	}
}

// 2xx 不是错误。
func TestStatusErrorNilOnSuccess(t *testing.T) {
	for _, code := range []int{200, 201, 204, 207} {
		if err := statusError("put", "x.json", code, nil); err != nil {
			t.Errorf("statusError(%d) = %v, want nil", code, err)
		}
	}
}

// 错误信息里要带上状态码与操作，便于用户反馈时定位；同时不能把整页 HTML 全塞进去。
func TestErrorMessageIncludesContextAndTruncatesBody(t *testing.T) {
	longBody := strings.Repeat("<html>error page</html>", 100)
	err := statusError("put", "clip/settings.json", 500, []byte(longBody))
	msg := err.Error()

	for _, want := range []string{"put", "clip/settings.json", "500"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message %q missing %q", msg, want)
		}
	}
	if len(msg) > 500 {
		t.Errorf("error message too long (%d chars); body should be truncated", len(msg))
	}
	if !strings.Contains(msg, "…") {
		t.Error("truncated body should be marked with an ellipsis")
	}
}
