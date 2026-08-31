package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/scheduler"
	"github.com/clip-rss/clip/internal/store"
)

// newTestStore 在临时文件创建一个真实 Store（迁移后的空库）。
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := t.TempDir() + "/api_test.db"
	st, err := store.NewWithPath(dbPath)
	if err != nil {
		t.Fatalf("store.NewWithPath: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// seedFeed 直接建一个源，返回其 ID。
func seedFeed(t *testing.T, st *store.Store, url, title string) int64 {
	t.Helper()
	f := &store.Feed{URL: url, Title: title, Status: "active", UpdateInterval: 30, MaxItems: 100}
	if err := st.CreateFeed(f); err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}
	return f.ID
}

const sampleRSS = `<?xml version="1.0"?>
<rss version="2.0"><channel>
  <title>Example Feed</title>
  <link>https://example.com</link>
  <description>An example feed</description>
  <item><title>First</title><link>https://example.com/1</link><guid>g1</guid></item>
  <item><title>Second</title><link>https://example.com/2</link><guid>g2</guid></item>
</channel></rss>`

// --- FeedService (集成：httptest + 真实 fetcher/scheduler/store) ---

func TestAddFeedFetchesAndPersists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	st := newTestStore(t)
	ft := fetcher.New()
	sch := scheduler.New(st, ft)
	svc := NewFeedService(st, ft, sch)

	feed, err := svc.AddFeed(srv.URL, 0)
	if err != nil {
		t.Fatalf("AddFeed: %v", err)
	}
	if feed.Title != "Example Feed" {
		t.Errorf("title = %q, want Example Feed", feed.Title)
	}
	if feed.ID == 0 {
		t.Fatal("feed ID should be set")
	}

	// 初次抓取应已写入两篇文章。
	items, err := st.ListItemsByFeed(feed.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListItemsByFeed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("persisted items = %d, want 2", len(items))
	}

	// 重复添加同 URL 应失败。
	if _, err := svc.AddFeed(srv.URL, 0); err == nil {
		t.Error("expected duplicate AddFeed to fail")
	}
}

func TestAddFeedIntoCategory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	st := newTestStore(t)
	ft := fetcher.New()
	sch := scheduler.New(st, ft)
	cat := &store.Category{Name: "技术"}
	if err := st.CreateCategory(cat); err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	svc := NewFeedService(st, ft, sch)

	feed, err := svc.AddFeed(srv.URL, cat.ID)
	if err != nil {
		t.Fatalf("AddFeed: %v", err)
	}
	if feed.CategoryID == nil || *feed.CategoryID != cat.ID {
		t.Errorf("feed category = %v, want %d", feed.CategoryID, cat.ID)
	}
}

func TestPreviewFeed(t *testing.T) {
	st := newTestStore(t)
	ft := fetcher.New()
	svc := NewFeedService(st, ft, scheduler.New(st, ft))

	// 1) 直接是 Feed 地址。
	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer feedSrv.Close()

	preview, err := svc.PreviewFeed(feedSrv.URL)
	if err != nil {
		t.Fatalf("PreviewFeed(feed): %v", err)
	}
	if preview.Title != "Example Feed" || preview.ItemCount != 2 {
		t.Errorf("preview = %+v, want title=Example Feed itemCount=2", preview)
	}
	if preview.URL != feedSrv.URL {
		t.Errorf("preview url = %q, want %q", preview.URL, feedSrv.URL)
	}
	if preview.AlreadyAdded {
		t.Error("preview.AlreadyAdded should be false before adding")
	}

	// 2) 普通网页地址 → 自动发现页面中声明的 Feed。
	var pageURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><link rel="alternate" type="application/rss+xml" href="` + pageURL + `/feed.xml"></head><body>hi</body></html>`))
	})
	pageSrv := httptest.NewServer(mux)
	defer pageSrv.Close()
	pageURL = pageSrv.URL

	preview2, err := svc.PreviewFeed(pageSrv.URL)
	if err != nil {
		t.Fatalf("PreviewFeed(page): %v", err)
	}
	if preview2.URL != pageSrv.URL+"/feed.xml" {
		t.Errorf("discovered url = %q, want %q", preview2.URL, pageSrv.URL+"/feed.xml")
	}
	if preview2.Title != "Example Feed" {
		t.Errorf("discovered title = %q, want Example Feed", preview2.Title)
	}

	// 3) 空地址应报错。
	if _, err := svc.PreviewFeed("  "); err == nil {
		t.Error("expected empty url error")
	}
}

// 复现并验证：支持条件 GET 的源（返回 ETag，对 If-None-Match 回 304）此前被抓过、
// fetcher 已缓存 ETag 后，PreviewFeed/AddFeed 仍应强制全量抓取，不因 304 误判为「找不到源」。
func TestPreviewAndAddForceFullFetchIgnoring304(t *testing.T) {
	const etag = `"v1"`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	st := newTestStore(t)
	ft := fetcher.New()
	svc := NewFeedService(st, ft, scheduler.New(st, ft))

	// 预热：让 fetcher 缓存该源的 ETag（模拟之前已抓取/订阅过）。
	if _, _, err := ft.FetchFeed(context.Background(), srv.URL); err != nil {
		t.Fatalf("warm fetch: %v", err)
	}

	// PreviewFeed 不应因条件 GET 命中 304 而失败。
	preview, err := svc.PreviewFeed(srv.URL)
	if err != nil {
		t.Fatalf("PreviewFeed after cached ETag: %v", err)
	}
	if preview.Title != "Example Feed" {
		t.Errorf("preview title = %q, want Example Feed", preview.Title)
	}

	// AddFeed 同样应强制全量抓取，不因 304 报 empty feed。
	feed, err := svc.AddFeed(srv.URL, 0)
	if err != nil {
		t.Fatalf("AddFeed after cached ETag: %v", err)
	}
	if feed.Title != "Example Feed" {
		t.Errorf("feed title = %q, want Example Feed", feed.Title)
	}
}

func TestAddFeedRejectsEmptyAndBadURL(t *testing.T) {
	st := newTestStore(t)
	ft := fetcher.New()
	svc := NewFeedService(st, ft, scheduler.New(st, ft))

	if _, err := svc.AddFeed("   ", 0); err == nil {
		t.Error("expected empty url error")
	}
	if _, err := svc.AddFeed("http://127.0.0.1:0/nope", 0); err == nil {
		t.Error("expected fetch error for unreachable url")
	}
}

func TestFeedCRUDAndPauseResume(t *testing.T) {
	st := newTestStore(t)
	ft := fetcher.New()
	svc := NewFeedService(st, ft, scheduler.New(st, ft))

	id := seedFeed(t, st, "https://a.example/feed", "A")

	got, err := svc.GetFeed(id)
	if err != nil {
		t.Fatalf("GetFeed: %v", err)
	}
	got.Title = "A renamed"
	if err := svc.UpdateFeed(*got); err != nil {
		t.Fatalf("UpdateFeed: %v", err)
	}
	if after, _ := svc.GetFeed(id); after.Title != "A renamed" {
		t.Errorf("title not updated: %q", after.Title)
	}

	if err := svc.PauseFeed(id); err != nil {
		t.Fatalf("PauseFeed: %v", err)
	}
	if after, _ := svc.GetFeed(id); after.Status != "paused" {
		t.Errorf("status = %q, want paused", after.Status)
	}
	if err := svc.ResumeFeed(id); err != nil {
		t.Fatalf("ResumeFeed: %v", err)
	}
	if after, _ := svc.GetFeed(id); after.Status != "active" {
		t.Errorf("status = %q, want active", after.Status)
	}

	if err := svc.DeleteFeed(id); err != nil {
		t.Fatalf("DeleteFeed: %v", err)
	}
	if _, err := svc.GetFeed(id); err == nil {
		t.Error("expected GetFeed to fail after delete")
	}
}

// --- ItemService ---

func TestItemServiceOps(t *testing.T) {
	st := newTestStore(t)
	feedID := seedFeed(t, st, "https://i.example/feed", "I")
	svc := NewItemService(st)

	a := &store.Item{FeedID: feedID, Title: "Alpha", URL: "https://i.example/a", Summary: "alpha summary"}
	b := &store.Item{FeedID: feedID, Title: "Beta", URL: "https://i.example/b", Summary: "beta summary"}
	for _, it := range []*store.Item{a, b} {
		if _, err := st.CreateItemIfNotExists(it); err != nil {
			t.Fatalf("seed item: %v", err)
		}
	}

	items, err := svc.ListItems(feedID, 10, 0)
	if err != nil || len(items) != 2 {
		t.Fatalf("ListItems = %d items, err %v", len(items), err)
	}
	if all, _ := svc.ListItems(0, 10, 0); len(all) != 2 {
		t.Errorf("ListItems(all) = %d, want 2", len(all))
	}

	// 已读/未读。
	if err := svc.MarkRead(a.ID); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if unread, _ := svc.ListUnreadItems(10, 0); len(unread) != 1 {
		t.Errorf("unread = %d, want 1", len(unread))
	}
	if err := svc.MarkUnread(a.ID); err != nil {
		t.Fatalf("MarkUnread: %v", err)
	}
	if cnt, _ := svc.GetUnreadCount(); cnt != 2 {
		t.Errorf("unread count = %d, want 2", cnt)
	}

	// 批量已读 + 全源已读。
	if err := svc.BatchMarkRead([]int64{a.ID, b.ID}); err != nil {
		t.Fatalf("BatchMarkRead: %v", err)
	}
	if cnt, _ := svc.GetUnreadCount(); cnt != 0 {
		t.Errorf("after batch read, unread = %d, want 0", cnt)
	}

	// 星标。
	if err := svc.ToggleStar(a.ID); err != nil {
		t.Fatalf("ToggleStar: %v", err)
	}
	if starred, _ := svc.ListStarredItems(10, 0); len(starred) != 1 {
		t.Errorf("starred = %d, want 1", len(starred))
	}

	// 笔记 + 搜索。
	if err := svc.AddNote(a.ID, "my private note"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	if got, _ := svc.GetItem(a.ID); got.Note != "my private note" {
		t.Errorf("note = %q", got.Note)
	}
	if found, _ := svc.SearchItems("Alpha", 10, 0); len(found) == 0 {
		t.Error("SearchItems(Alpha) returned no results")
	}
	if empty, _ := svc.SearchItems("   ", 10, 0); len(empty) != 0 {
		t.Errorf("blank search should return empty, got %d", len(empty))
	}
}

// TestSearchItemsServiceChinese 验证绑定层经由 store 的中文子串搜索可用。
func TestSearchItemsServiceChinese(t *testing.T) {
	st := newTestStore(t)
	feedID := seedFeed(t, st, "https://s.example/feed", "S")
	svc := NewItemService(st)

	it := &store.Item{FeedID: feedID, Title: "科技爱好者周刊", URL: "https://s.example/1"}
	if _, err := st.CreateItemIfNotExists(it); err != nil {
		t.Fatalf("seed item: %v", err)
	}

	if found, _ := svc.SearchItems("周刊", 10, 0); len(found) != 1 {
		t.Errorf("SearchItems(周刊) = %d, want 1", len(found))
	}
	if empty, _ := svc.SearchItems("  ", 10, 0); len(empty) != 0 {
		t.Errorf("blank search should be empty, got %d", len(empty))
	}
}

// --- CategoryService ---

func TestCategoryServiceCRUDAndMove(t *testing.T) {
	st := newTestStore(t)
	csvc := NewCategoryService(st)
	feedID := seedFeed(t, st, "https://c.example/feed", "C")

	root, err := csvc.AddCategory("技术", 0)
	if err != nil {
		t.Fatalf("AddCategory: %v", err)
	}
	if root.ParentID != nil {
		t.Error("root category should have nil parent")
	}
	child, err := csvc.AddCategory("前端", root.ID)
	if err != nil {
		t.Fatalf("AddCategory child: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Errorf("child parent = %v, want %d", child.ParentID, root.ID)
	}

	if _, err := csvc.AddCategory("  ", 0); err == nil {
		t.Error("expected empty name error")
	}

	cats, _ := csvc.ListCategories()
	if len(cats) != 2 {
		t.Errorf("categories = %d, want 2", len(cats))
	}

	// 移动源到分类。
	if err := csvc.MoveToCategory(feedID, child.ID); err != nil {
		t.Fatalf("MoveToCategory: %v", err)
	}
	cwf, err := csvc.GetCategoryWithFeeds(child.ID)
	if err != nil {
		t.Fatalf("GetCategoryWithFeeds: %v", err)
	}
	if len(cwf.Feeds) != 1 || cwf.Feeds[0].ID != feedID {
		t.Errorf("category feeds wrong: %+v", cwf.Feeds)
	}

	// 移出分类（0 = 未分类）。
	if err := csvc.MoveToCategory(feedID, 0); err != nil {
		t.Fatalf("MoveToCategory(0): %v", err)
	}
	if unc, _ := csvc.GetUncategorizedFeeds(); len(unc) != 1 {
		t.Errorf("uncategorized = %d, want 1", len(unc))
	}

	// 删除。
	if err := csvc.DeleteCategory(child.ID); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}
	if cats, _ := csvc.ListCategories(); len(cats) != 1 {
		t.Errorf("after delete categories = %d, want 1", len(cats))
	}
}

// --- SettingsService ---

func TestSettingsService(t *testing.T) {
	st := newTestStore(t)
	svc := NewSettingsService(st, nil, nil)
	feedA := &store.Feed{URL: "https://a.example/feed", Title: "A", Status: "active", UpdateInterval: 30, MaxItems: 100}
	feedB := &store.Feed{URL: "https://b.example/feed", Title: "B", Status: "active", UpdateInterval: 60, MaxItems: 100}
	for _, feed := range []*store.Feed{feedA, feedB} {
		if err := st.CreateFeed(feed); err != nil {
			t.Fatalf("CreateFeed: %v", err)
		}
	}

	got, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got != store.DefaultSettings() {
		t.Errorf("defaults = %+v", got)
	}

	got.Theme = "dark"
	got.DefaultUpdateInterval = 120
	if err := svc.UpdateSettings(got); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	after, _ := svc.GetSettings()
	if after.Theme != "dark" || after.DefaultUpdateInterval != 120 {
		t.Errorf("settings not persisted: %+v", after)
	}
	feeds, err := st.ListFeeds()
	if err != nil {
		t.Fatal(err)
	}
	for _, feed := range feeds {
		if feed.UpdateInterval != 120 {
			t.Errorf("feed %d interval = %d, want global interval 120", feed.ID, feed.UpdateInterval)
		}
	}

	// 单源可在全局应用后覆盖；修改其它设置时不应重置该覆盖值。
	feedA, _ = st.GetFeed(feedA.ID)
	feedA.UpdateInterval = 60
	if err := st.UpdateFeed(feedA); err != nil {
		t.Fatal(err)
	}
	after.Language = "en"
	if err := svc.UpdateSettings(after); err != nil {
		t.Fatal(err)
	}
	feedA, _ = st.GetFeed(feedA.ID)
	if feedA.UpdateInterval != 60 {
		t.Errorf("unchanged global interval reset per-feed override to %d", feedA.UpdateInterval)
	}

	// 再次修改全局值时，所有现有源（包括单源覆盖）都应统一更新。
	after.DefaultUpdateInterval = 0
	if err := svc.UpdateSettings(after); err != nil {
		t.Fatal(err)
	}
	feeds, _ = st.ListFeeds()
	for _, feed := range feeds {
		if feed.UpdateInterval != 0 {
			t.Errorf("feed %d interval = %d, want global manual mode 0", feed.ID, feed.UpdateInterval)
		}
	}
}

// --- OPMLService ---

const importOPML = `<?xml version="1.0"?>
<opml version="2.0"><head><title>t</title></head><body>
  <outline text="科技">
    <outline text="HN" type="rss" xmlUrl="https://hn.example/rss" htmlUrl="https://hn.example"/>
  </outline>
  <outline text="独立" type="rss" xmlUrl="https://solo.example/feed"/>
</body></opml>`

func TestOPMLImportExportRoundTrip(t *testing.T) {
	st := newTestStore(t)
	svc := NewOPMLService(st, nil, nil)

	res, err := svc.ImportOPML(importOPML)
	if err != nil {
		t.Fatalf("ImportOPML: %v", err)
	}
	if res.Categories != 1 || res.Feeds != 2 || res.Skipped != 0 {
		t.Errorf("import result = %+v, want 1 cat / 2 feeds / 0 skipped", res)
	}

	// 再次导入应全部跳过（URL 已存在）。
	res2, _ := svc.ImportOPML(importOPML)
	if res2.Feeds != 0 || res2.Skipped != 2 {
		t.Errorf("re-import = %+v, want 0 feeds / 2 skipped", res2)
	}

	// 导出后应能被重新解析，且包含两个源。
	out, err := svc.buildOPML()
	if err != nil {
		t.Fatalf("buildOPML: %v", err)
	}
	reimport := NewOPMLService(newTestStore(t), nil, nil)
	back, err := reimport.ImportOPML(out)
	if err != nil {
		t.Fatalf("re-parse exported OPML: %v", err)
	}
	if back.Feeds != 2 {
		t.Errorf("exported feeds = %d, want 2", back.Feeds)
	}

	if _, err := svc.ImportOPML("   "); err == nil {
		t.Error("expected empty content error")
	}
}

// TestImportOPMLFromURL 覆盖远程导入：正常直链复用本地导入的统计语义，
// 非法地址在发请求前就被挡住，失败路径都返回错误而不是 panic。
func TestImportOPMLFromURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/feeds.opml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(importOPML))
	})
	mux.HandleFunc("/empty.opml", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Run("valid url imports", func(t *testing.T) {
		svc := NewOPMLService(newTestStore(t), fetcher.NewClient(), nil)
		res, err := svc.ImportOPMLFromURL(srv.URL + "/feeds.opml")
		if err != nil {
			t.Fatalf("ImportOPMLFromURL: %v", err)
		}
		if res.Categories != 1 || res.Feeds != 2 || res.Skipped != 0 {
			t.Errorf("import result = %+v, want 1 cat / 2 feeds / 0 skipped", res)
		}

		// 再次导入同一地址应全部跳过，与本地导入语义一致。
		again, err := svc.ImportOPMLFromURL(srv.URL + "/feeds.opml")
		if err != nil {
			t.Fatalf("re-import: %v", err)
		}
		if again.Feeds != 0 || again.Skipped != 2 {
			t.Errorf("re-import = %+v, want 0 feeds / 2 skipped", again)
		}
	})

	t.Run("rejects bad urls", func(t *testing.T) {
		svc := NewOPMLService(newTestStore(t), fetcher.NewClient(), nil)
		for _, raw := range []string{
			"", "   ",
			"file:///etc/passwd",
			"ftp://example.com/feeds.opml",
			"not-a-url",
			"https://", // 缺主机名
		} {
			if _, err := svc.ImportOPMLFromURL(raw); err == nil {
				t.Errorf("ImportOPMLFromURL(%q) should fail", raw)
			}
		}
	})

	t.Run("propagates fetch and parse failures", func(t *testing.T) {
		svc := NewOPMLService(newTestStore(t), fetcher.NewClient(), nil)
		if _, err := svc.ImportOPMLFromURL(srv.URL + "/missing.opml"); err == nil {
			t.Error("expected error for 404 response")
		}
		if _, err := svc.ImportOPMLFromURL(srv.URL + "/empty.opml"); err == nil {
			t.Error("expected error for empty body")
		}
	})

	t.Run("nil client reports unavailable", func(t *testing.T) {
		svc := NewOPMLService(newTestStore(t), nil, nil)
		if _, err := svc.ImportOPMLFromURL(srv.URL + "/feeds.opml"); err == nil {
			t.Error("expected error when HTTP client is missing")
		}
	})
}

func TestSystemServiceSetOnline(t *testing.T) {
	var got bool
	called := false
	svc := &SystemService{OnlineChangedFn: func(online bool) {
		called = true
		got = online
	}}
	svc.SetOnline(false)
	if !called || got {
		t.Fatalf("online callback called=%v value=%v, want called with false", called, got)
	}
}

func TestImageFilename(t *testing.T) {
	tests := []struct {
		name        string
		rawURL      string
		contentType string
		want        string
	}{
		{"path with extension", "https://example.com/a/b/photo.png", "", "photo.png"},
		{"query string ignored", "https://example.com/img.jpg?v=2&s=100", "", "img.jpg"},
		{"no extension, png", "https://example.com/raw/abc123", "image/png", "abc123.png"},
		{"no extension, jpeg", "https://example.com/photo", "image/jpeg", "photo.jpg"},
		{"no extension, svg", "https://example.com/icon", "image/svg+xml", "icon.svg"},
		{"unknown content type", "https://example.com/data", "application/octet-stream", "data"},
		{"root path fallback", "https://example.com", "", "image"},
		{"empty path fallback", "https://example.com/", "image/webp", "image.webp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := imageFilename(tt.rawURL, tt.contentType); got != tt.want {
				t.Errorf("imageFilename(%q, %q) = %q, want %q", tt.rawURL, tt.contentType, got, tt.want)
			}
		})
	}
}

func TestExtFromContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want string
	}{
		{"image/png", ".png"},
		{"image/jpeg", ".jpg"},
		{"IMAGE/JPEG", ".jpg"},
		{"image/gif", ".gif"},
		{"image/webp", ".webp"},
		{"image/svg+xml", ".svg"},
		{"image/avif", ".avif"},
		{"image/bmp", ".bmp"},
		{"text/html", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := extFromContentType(tt.ct); got != tt.want {
			t.Errorf("extFromContentType(%q) = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestDownloadImageRejectsEmptyURL(t *testing.T) {
	svc := &SystemService{HTTPClient: fetcher.NewClient()}
	if _, err := svc.DownloadImage("   "); err == nil {
		t.Fatal("expected error for empty URL")
	}
	if _, err := svc.DownloadImage(""); err == nil {
		t.Fatal("expected error for empty URL")
	}
}
