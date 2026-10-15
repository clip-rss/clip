package fontproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newServer 建一个 root 指向临时目录、next 打标记的中间件测试服务器。
func newServer(t *testing.T, root string) *httptest.Server {
	t.Helper()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("next-handler"))
	})
	srv := httptest.NewServer(New(root).Middleware(next))
	t.Cleanup(srv.Close)
	return srv
}

// newFontFile 在 root/<id>/ 下写一个内容固定的字体文件，返回文件名。
func newFontFile(t *testing.T, root, id, name string) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("font-bytes-"+name), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestServesFontFile(t *testing.T) {
	root := t.TempDir()
	newFontFile(t, root, "lxgw-wenkai", "LXGWWenKai-Regular.ttf")
	srv := newServer(t, root)

	resp, err := http.Get(srv.URL + "/__clip/font/lxgw-wenkai/LXGWWenKai-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "font/ttf" {
		t.Fatalf("content-type = %q, want font/ttf", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "max-age=86400") {
		t.Errorf("cache-control = %q, want long max-age", cc)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "font-bytes-LXGWWenKai-Regular.ttf" {
		t.Fatalf("body = %q", body)
	}
}

func TestHeadReturnsHeadersOnly(t *testing.T) {
	root := t.TempDir()
	newFontFile(t, root, "huosong", "ChillHuoSong_F_Regular.otf")
	srv := newServer(t, root)

	req, _ := http.NewRequest(http.MethodHead, srv.URL+"/__clip/font/huosong/ChillHuoSong_F_Regular.otf", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "font/otf" {
		t.Fatalf("content-type = %q, want font/otf", ct)
	}
	if n, _ := io.ReadAll(resp.Body); len(n) != 0 {
		t.Fatalf("HEAD body = %q, want empty", n)
	}
}

func TestUnknownFontIs404(t *testing.T) {
	srv := newServer(t, t.TempDir())
	resp, err := http.Get(srv.URL + "/__clip/font/nope/LXGWWenKai-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// 路径穿越与畸形路径一律拒之门外，不碰文件系统。
func TestRejectsUnsafeAndMalformedPaths(t *testing.T) {
	srv := newServer(t, t.TempDir())

	for _, p := range []string{
		"/__clip/font/../secret/x",
		"/__clip/font/a/../../x",
		"/__clip/font/a/b/c",
		"/__clip/font/a",
		"/__clip/font/a/",
		"/__clip/font//b",
		"/__clip/font/a%2Fb/x", // %2F 不会被解码成段，仍应 400
	} {
		resp, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s => status %d, want 400", p, resp.StatusCode)
		}
	}
}

func TestPassesThroughOtherPaths(t *testing.T) {
	srv := newServer(t, t.TempDir())
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "next-handler" {
		t.Fatalf("non-font path went to handler, want next-handler, got %q", body)
	}
}
