package api

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clip-rss/clip/internal/fetcher"
)

// newFontService 用本地 httptest server 替换远程 catalogURL，隔离网络。
func newFontService(t *testing.T, handler http.HandlerFunc) *FontService {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := fetcher.NewClient(fetcher.WithHTTPClient(srv.Client()))
	return &FontService{fetch: client, catalogURL: srv.URL}
}

func TestFetchFontCatalog(t *testing.T) {
	svc := newFontService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"version": 1,
			"release": "fonts-v1",
			"fonts": [
				{
					"id": "lxgw-wenkai",
					"name": "霞鹜文楷",
					"family": "LXGW WenKai",
					"version": "1.522",
					"license": "SIL OFL 1.1",
					"licenseFile": "lxgw-wenkai/OFL.txt",
					"url": "https://github.com/clip-rss/clip-font/releases/latest/download",
					"files": [
						{"weight": 300, "format": "ttf", "file": "LXGWWenKai-Light.ttf", "size": 28267156, "sha256": "abc"},
						{"weight": 400, "format": "ttf", "file": "LXGWWenKai-Regular.ttf", "size": 25575676, "sha256": "def"}
					]
				},
				{
					"id": "chill-huosong",
					"name": "寒蝉活宋体",
					"family": "ChillHuoSong_F",
					"version": "1.000",
					"license": "SIL OFL 1.1",
					"licenseFile": "chill-huosong/LICENSE.txt",
					"url": "https://github.com/clip-rss/clip-font/releases/latest/download",
					"files": [
						{"style": "Regular", "weight": 400, "format": "otf", "file": "ChillHuoSong_F_Regular.otf", "size": 32135884, "sha256": "ghi"}
					]
				}
			]
		}`))
	})

	cat, err := svc.FetchFontCatalog()
	if err != nil {
		t.Fatalf("FetchFontCatalog: %v", err)
	}
	if cat.Version != 1 || cat.Release != "fonts-v1" || len(cat.Fonts) != 2 {
		t.Fatalf("catalog head mismatch: %+v", cat)
	}

	wenkai := cat.Fonts[0]
	if wenkai.ID != "lxgw-wenkai" || wenkai.Name != "霞鹜文楷" || wenkai.Family != "LXGW WenKai" {
		t.Fatalf("wenkai mismatched: %+v", wenkai)
	}
	if len(wenkai.Files) != 2 || wenkai.Files[1].Weight != 400 || wenkai.Files[1].File != "LXGWWenKai-Regular.ttf" {
		t.Fatalf("wenkai files mismatched: %+v", wenkai.Files)
	}

	huosong := cat.Fonts[1]
	if huosong.Files[0].Style != "Regular" || huosong.Files[0].Format != "otf" {
		t.Fatalf("huosong style/format mismatched: %+v", huosong.Files[0])
	}
}

func TestFetchFontCatalogBadJSON(t *testing.T) {
	svc := newFontService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("this is not json"))
	})

	_, err := svc.FetchFontCatalog()
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "parse catalog") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchFontCatalogServerError(t *testing.T) {
	svc := newFontService(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	if _, err := svc.FetchFontCatalog(); err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

// 未知的 schema 版本必须放行并照常解析。
//
// 这是有意为之：硬失败会让上游一个字段（发布方控制）变成字体库的远程开关 ——
// 上游仅递增 version、形状却没变时，所有未更新的 App 会当场坏掉。见 fontCatalogVersion。
func TestFetchFontCatalogToleratesUnknownVersion(t *testing.T) {
	svc := newFontService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"version": 99,
			"release": "fonts-v99",
			"fonts": [{"id": "x", "name": "x", "family": "X", "version": "1",
				"license": "SIL OFL 1.1", "licenseFile": "x/OFL.txt",
				"url": "https://example.invalid",
				"files": [{"weight": 400, "format": "ttf", "file": "x.ttf", "size": 1, "sha256": "a"}]}]
		}`))
	})

	cat, err := svc.FetchFontCatalog()
	if err != nil {
		t.Fatalf("unknown schema version must not fail: %v", err)
	}
	if cat.Version != 99 || len(cat.Fonts) != 1 {
		t.Fatalf("catalog not parsed: %+v", cat)
	}
}

// release 清单的 files[].file 带目录前缀，而 release 资产是扁平裸名。
func TestAssetNameStripsDirectoryPrefix(t *testing.T) {
	cases := map[string]string{
		"lxgw-wenkai/LXGWWenKai-Regular.ttf":    "LXGWWenKai-Regular.ttf",
		"chill-huosong/ChillHuoSong_F_Bold.otf": "ChillHuoSong_F_Bold.otf",
		"LXGWWenKai-Regular.ttf":                "LXGWWenKai-Regular.ttf",
		"  lxgw-wenkai/LXGWWenKai-Light.ttf  ":  "LXGWWenKai-Light.ttf",
		// 挡不住的形态一律返回空串，交由调用方报错。
		"":        "",
		"..":      "",
		".":       "",
		"a/..":    "",
		`sub\a.t`: "",
	}
	for in, want := range cases {
		if got := assetName(in); got != want {
			t.Errorf("assetName(%q) = %q, want %q", in, got, want)
		}
	}
}

// 端到端：清单里给的是带前缀的仓库路径，下载地址必须落到扁平资产上。
func TestDownloadFontUsesFlatAssetNames(t *testing.T) {
	body := []byte("LXGW WenKai Regular")
	sum := sha256.Sum256(body)

	var manifestHit, assetHit, prefixedHit int
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			manifestHit++
			_, _ = fmt.Fprintf(w, `{"version":1,"release":"fonts-v1","fonts":[{
				"id":"lxgw-wenkai","name":"霞鹜文楷","family":"LXGW WenKai","version":"1.522",
				"license":"SIL OFL 1.1","licenseFile":"lxgw-wenkai/OFL.txt",
				"url":"%s/releases/latest/download",
				"files":[{"weight":400,"format":"ttf","file":"lxgw-wenkai/LXGWWenKai-Regular.ttf",
					"size":%d,"sha256":"%x"}]}]}`, srv.URL, len(body), sum)
		case "/releases/latest/download/LXGWWenKai-Regular.ttf":
			assetHit++
			_, _ = w.Write(body)
		case "/releases/latest/download/lxgw-wenkai/LXGWWenKai-Regular.ttf":
			prefixedHit++
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	client := fetcher.NewClient(fetcher.WithHTTPClient(srv.Client()))
	svc := &FontService{fetch: client, dl: srv.Client(), catalogURL: srv.URL + "/manifest.json", fontDir: dir}

	if err := svc.DownloadFont("lxgw-wenkai"); err != nil {
		t.Fatalf("DownloadFont: %v", err)
	}
	if manifestHit != 1 {
		t.Errorf("manifest fetched %d times, want 1", manifestHit)
	}
	if assetHit != 1 {
		t.Errorf("flat asset fetched %d times, want 1", assetHit)
	}
	if prefixedHit != 0 {
		t.Errorf("prefixed asset path was requested %d times, want 0", prefixedHit)
	}
	got, err := os.ReadFile(filepath.Join(dir, "LXGWWenKai-Regular.ttf"))
	if err != nil {
		t.Fatalf("read downloaded font: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("content mismatch: got %q", got)
	}
}

func TestDownloadFontUnknownID(t *testing.T) {
	svc := newFontService(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":1,"release":"fonts-v1","fonts":[]}`))
	})
	svc.fontDir = t.TempDir()

	if err := svc.DownloadFont("nope"); err == nil {
		t.Fatal("expected error for unknown font id, got nil")
	}
}
