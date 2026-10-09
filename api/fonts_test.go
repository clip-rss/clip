package api

import (
	"net/http"
	"net/http/httptest"
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
