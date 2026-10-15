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
	const family = "lxgw-wenkai"
	if err := Download(context.Background(), srv.Client(), dir, family, files, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}

	for i, f := range files {
		got, err := os.ReadFile(filepath.Join(dir, family, f.Name))
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		want := [][]byte{regular, bold}[i]
		if string(got) != string(want) {
			t.Fatalf("%s content mismatch: got %q", f.Name, got)
		}
	}
}

// 落盘必须落在 <dir>/<family>/ 子目录，而不是 <dir> 根——按字体族分目录的契约。
func TestDownloadWritesUnderFamilyDir(t *testing.T) {
	body := []byte("one file")
	srv, _ := serve(t, map[string][]byte{"/a.ttf": body})
	dir := t.TempDir()

	files := []File{{URL: srv.URL + "/a.ttf", Name: "a.ttf", SHA256: digest(body), Size: int64(len(body))}}
	if err := Download(context.Background(), srv.Client(), dir, "lxgw-wenkai", files, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "lxgw-wenkai", "a.ttf")); err != nil {
		t.Fatalf("font not under family dir: %v", err)
	}
	rootEntries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rootEntries) != 1 || rootEntries[0].Name() != "lxgw-wenkai" {
		t.Fatalf("root should hold exactly one family dir, got %v", rootEntries)
	}
}

// 已存在且校验通过的文件必须跳过 —— 否则「重试安装」会把 80 MB 重下一遍。
func TestDownloadSkipsVerifiedFile(t *testing.T) {
	body := []byte("already here")
	srv, hits := serve(t, map[string][]byte{"/a.ttf": body})
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "lxgw-wenkai"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lxgw-wenkai", "a.ttf"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	files := []File{{URL: srv.URL + "/a.ttf", Name: "a.ttf", SHA256: digest(body), Size: int64(len(body))}}
	if err := Download(context.Background(), srv.Client(), dir, "lxgw-wenkai", files, nil); err != nil {
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
	if err := Download(context.Background(), srv.Client(), dir, "lxgw-wenkai", files, nil); err == nil {
		t.Fatal("expected checksum mismatch error, got nil")
	}

	// 族目录会被预创建，但失败后里面不能留下半个文件或临时文件。
	entries, err := os.ReadDir(filepath.Join(dir, "lxgw-wenkai"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("family dir should be empty after failed download, got %v", names)
	}
}

// 族目录名必须合法——它来自远端清单，挡不住就可能把文件写到字体目录之外。
func TestDownloadRejectsBadFamily(t *testing.T) {
	srv, hits := serve(t, map[string][]byte{"/a.ttf": []byte("x")})
	dir := t.TempDir()
	files := []File{{URL: srv.URL + "/a.ttf", Name: "a.ttf", SHA256: digest([]byte("x")), Size: 1}}

	for _, family := range []string{"", "..", ".", "sub/a", `sub\a`, "/abs"} {
		if err := Download(context.Background(), srv.Client(), dir, family, files, nil); err == nil {
			t.Errorf("expected error for family %q", family)
		}
	}
	if hits["/a.ttf"] != 0 {
		t.Fatalf("bad family should be rejected before any request, got %d hits", hits["/a.ttf"])
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
			if err := Download(context.Background(), srv.Client(), dir, "lxgw-wenkai", files, nil); err == nil {
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
	if err := Download(context.Background(), srv.Client(), dir, "lxgw-wenkai", files, func(done, total int64) {
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

// 手工搭两个族目录，一个内部带子目录垃圾、根目录放一级散文件（旧扁平布局残留），
// List 只认合法族目录及其内部合法裸文件，并按 ID 排序。
func TestListScansFamilyDirs(t *testing.T) {
	dir := t.TempDir()
	mk := func(p string) {
		t.Helper()
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("chill-huosong/ChillHuoSong_Regular.otf")
	mk("lxgw-wenkai/LXGWWenKai-Regular.ttf")
	mk("lxgw-wenkai/LXGWWenKai-Bold.ttf")
	mk("lxgw-wenkai/nested-junk/ignored.txt") // 族内的子目录不算字重文件
	mk("loose-file.ttf")                      // 根目录散文件（旧扁平布局残留），忽略

	got, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List returned %d families, want 2: %+v", len(got), got)
	}
	// 排序稳定：chill-huosong 在 lxgw-wenkai 之前。
	if got[0].ID != "chill-huosong" || got[1].ID != "lxgw-wenkai" {
		t.Fatalf("family order or ids wrong: %+v", got)
	}
	wenkai := got[1]
	if len(wenkai.Files) != 2 { // nested-junk 子目录不算文件
		t.Fatalf("wenkai files = %+v, want exactly the two ttf files", wenkai.Files)
	}
	if wenkai.TotalSize != 2 { // 两个 1 字节文件
		t.Fatalf("wenkai total size = %d, want 2", wenkai.TotalSize)
	}
}

// List 对不存在的目录返回空列表，等价于「什么都没装」。
func TestListEmptyWhenDirMissing(t *testing.T) {
	got, err := List(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty list, got %+v", got)
	}
}

func TestRemoveDeletesWholeFamily(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "lxgw-wenkai"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"a.ttf", "b.ttf"} {
		if err := os.WriteFile(filepath.Join(dir, "lxgw-wenkai", f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Remove(dir, "lxgw-wenkai"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "lxgw-wenkai")); !os.IsNotExist(err) {
		t.Fatalf("family dir should be gone, stat err = %v", err)
	}
	got, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("list after remove = %+v, want empty", got)
	}
}

// 删除不存在的族是幂等的；目标不是目录要报错而不是误删文件。
func TestRemoveMissingOrNotDir(t *testing.T) {
	dir := t.TempDir()
	if err := Remove(dir, "nope"); err != nil {
		t.Fatalf("removing missing family should be nil, got %v", err)
	}
	loose := filepath.Join(dir, "loose.ttf")
	if err := os.WriteFile(loose, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir, "loose.ttf"); err == nil {
		t.Fatal("removing a file (not a dir) should error")
	}
	if _, err := os.Stat(loose); err != nil {
		t.Fatalf("loose file must survive a rejected Remove: %v", err)
	}
}

// id 来自远端清单，Remove 必须挡下一切可能逃出字体根目录的形态。
// 注：Windows 盘相对路径（C:evil）在 filepath.Join 里会丢弃前缀，由 Remove 里的
// Rel 双重检查兜底，但它在 POSIX 上不构成逃逸，故不放入这个跨平台集合。
func TestRemoveRejectsUnsafeID(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"", "..", ".", "a/b", `a\b`, "/abs"} {
		if err := Remove(dir, id); err == nil {
			t.Errorf("expected error for id %q", id)
		}
	}
	// 安全 id 不该被误伤
	if err := Remove(dir, "lxgw-wenkai"); err != nil {
		t.Errorf("safe id should be removable: %v", err)
	}
}
