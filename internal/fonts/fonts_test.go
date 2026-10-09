package fonts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// serve 起一个按 path 分发固定字节的测试服务器，并统计每个路径被请求的次数。
func serve(t *testing.T, blobs map[string][]byte) (*httptest.Server, map[string]int) {
	t.Helper()
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits[r.URL.Path]++
		body, ok := blobs[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, hits
}

func TestDownloadWritesVerifiedFiles(t *testing.T) {
	regular := []byte("LXGW WenKai Regular")
	bold := []byte("LXGW WenKai Bold")
	srv, _ := serve(t, map[string][]byte{"/LXGWWenKai-Regular.ttf": regular, "/LXGWWenKai-Bold.ttf": bold})
	dir := t.TempDir()

	files := []File{
		{URL: srv.URL + "/LXGWWenKai-Regular.ttf", Name: "LXGWWenKai-Regular.ttf", SHA256: digest(regular), Size: int64(len(regular))},
		{URL: srv.URL + "/LXGWWenKai-Bold.ttf", Name: "LXGWWenKai-Bold.ttf", SHA256: digest(bold), Size: int64(len(bold))},
	}
	if err := Download(context.Background(), srv.Client(), dir, files, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}

	for i, f := range files {
		got, err := os.ReadFile(filepath.Join(dir, f.Name))
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		want := [][]byte{regular, bold}[i]
		if string(got) != string(want) {
			t.Fatalf("%s content mismatch: got %q", f.Name, got)
		}
	}
}

// 已存在且校验通过的文件必须跳过 —— 否则「重试安装」会把 80 MB 重下一遍。
func TestDownloadSkipsVerifiedFile(t *testing.T) {
	body := []byte("already here")
	srv, hits := serve(t, map[string][]byte{"/a.ttf": body})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.ttf"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	files := []File{{URL: srv.URL + "/a.ttf", Name: "a.ttf", SHA256: digest(body), Size: int64(len(body))}}
	if err := Download(context.Background(), srv.Client(), dir, files, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if hits["/a.ttf"] != 0 {
		t.Fatalf("verified file was re-downloaded (%d hits)", hits["/a.ttf"])
	}
}

// 摘要不符必须报错，且目录里不能留下半个文件或临时文件。
func TestDownloadChecksumMismatchLeavesNoPartial(t *testing.T) {
	body := []byte("tampered payload")
	srv, _ := serve(t, map[string][]byte{"/a.ttf": body})
	dir := t.TempDir()

	files := []File{{URL: srv.URL + "/a.ttf", Name: "a.ttf", SHA256: digest([]byte("expected")), Size: int64(len(body))}}
	if err := Download(context.Background(), srv.Client(), dir, files, nil); err == nil {
		t.Fatal("expected checksum mismatch error, got nil")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("dir should be empty after failed download, got %v", names)
	}
}

// 清单里的文件名必须先是裸文件名；挡不住就可能在字体目录外落文件。
func TestDownloadRejectsBadName(t *testing.T) {
	body := []byte("x")
	srv, hits := serve(t, map[string][]byte{"/a.ttf": body})
	dir := t.TempDir()

	for _, name := range []string{"", "..", "sub/a.ttf", `sub\a.ttf`} {
		t.Run(name, func(t *testing.T) {
			files := []File{{URL: srv.URL + "/a.ttf", Name: name, SHA256: digest(body), Size: int64(len(body))}}
			if err := Download(context.Background(), srv.Client(), dir, files, nil); err == nil {
				t.Fatalf("expected error for name %q", name)
			}
		})
	}
	if hits["/a.ttf"] != 0 {
		t.Fatalf("bad name should be rejected before any request, got %d hits", hits["/a.ttf"])
	}
}

// 进度是跨文件累计的，最后一跳必须落在 total 上。
func TestDownloadAggregatesProgress(t *testing.T) {
	first := []byte("12345")
	second := []byte("6789")
	srv, _ := serve(t, map[string][]byte{"/a.ttf": first, "/b.ttf": second})
	dir := t.TempDir()

	files := []File{
		{URL: srv.URL + "/a.ttf", Name: "a.ttf", SHA256: digest(first), Size: int64(len(first))},
		{URL: srv.URL + "/b.ttf", Name: "b.ttf", SHA256: digest(second), Size: int64(len(second))},
	}
	wantTotal := int64(len(first) + len(second))
	var got []int64
	lastDone := int64(-1)
	if err := Download(context.Background(), srv.Client(), dir, files, func(done, total int64) {
		if total != wantTotal {
			t.Errorf("total = %d, want %d", total, wantTotal)
		}
		if done < lastDone {
			t.Errorf("progress went backwards: %d after %d", done, lastDone)
		}
		lastDone = done
		got = append(got, done)
	}); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if len(got) == 0 || got[len(got)-1] != wantTotal {
		t.Fatalf("final progress = %v, want last entry %d", got, wantTotal)
	}
}
