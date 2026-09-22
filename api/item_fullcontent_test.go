package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/reader"
	"github.com/clip-rss/clip/internal/store"
)

// articleHTML 是一份可被 readability 识别的最小文章页。
func articleHTML(body string) string {
	return `<!doctype html><html><head><title>原文标题</title></head><body>
<nav><a href="/">首页</a></nav>
<article><h1>原文标题</h1>` + body + `</article>
<footer>版权所有</footer></body></html>`
}

// newFullTextService 建一个注入了真实 fetcher 的 ItemService。
func newFullTextService(t *testing.T, st *store.Store) *ItemService {
	t.Helper()
	return NewItemService(st, fetcher.New())
}

// seedArticle 建一个源 + 一条指向 articleURL 的文章，返回文章 ID 与源 ID。
func seedArticle(t *testing.T, st *store.Store, articleURL string) (itemID, feedID int64) {
	t.Helper()
	feedID = seedFeed(t, st, "https://i.example/feed", "I")
	it := &store.Item{
		FeedID:  feedID,
		Title:   "Alpha",
		URL:     articleURL,
		Content: "<p>RSS 给的摘要</p>",
		Summary: "摘要",
	}
	if err := st.CreateItem(it); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	return it.ID, feedID
}

// TestFetchFullContentExtractsAndPersists 正常路径：提取成功、正文落库、
// 第二次调用不再请求原文站。
func TestFetchFullContentExtractsAndPersists(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(articleHTML(`
			<p>这是原文正文的第一段，足够长以便提取器识别为文章主体。</p>
			<p>这是原文正文的第二段，同样需要一定的长度。</p>
			<p>第三段收尾。</p>`)))
	}))
	defer srv.Close()

	st := newTestStore(t)
	svc := newFullTextService(t, st)
	itemID, _ := seedArticle(t, st, srv.URL)

	got, err := svc.FetchFullContent(itemID)
	if err != nil {
		t.Fatalf("FetchFullContent: %v", err)
	}
	if !strings.Contains(got, "这是原文正文的第一段") {
		t.Errorf("提取结果缺少正文：%s", got)
	}
	if hits != 1 {
		t.Errorf("首次调用应请求原文 1 次，实际 %d", hits)
	}

	// RSS 原文必须保留——提取结果不许覆盖它。
	item, err := st.GetItem(itemID)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if item.Content != "<p>RSS 给的摘要</p>" {
		t.Errorf("content 被覆盖了：%q", item.Content)
	}
	if item.FullContent == "" {
		t.Error("full_content 未落库")
	}

	// 第二次调用读库即可，不该再联网。
	again, err := svc.FetchFullContent(itemID)
	if err != nil {
		t.Fatalf("第二次 FetchFullContent: %v", err)
	}
	if again != got {
		t.Errorf("第二次返回的内容与首次不一致")
	}
	if hits != 1 {
		t.Errorf("第二次调用不应再请求原文，请求次数 = %d", hits)
	}
}

// TestFetchFullContentNotFound 原文页 404：返回错误，且不写库。
func TestFetchFullContentNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	st := newTestStore(t)
	svc := newFullTextService(t, st)
	itemID, feedID := seedArticle(t, st, srv.URL)

	if _, err := svc.FetchFullContent(itemID); err == nil {
		t.Fatal("404 应返回错误")
	}

	item, _ := st.GetItem(itemID)
	if item.FullContent != "" {
		t.Errorf("失败时不该写库，full_content = %q", item.FullContent)
	}
	assertFeedErrorCountUnchanged(t, st, feedID)
}

// TestFetchFullContentEmptyResponse 200 但响应体为空：提不出正文，返回错误。
func TestFetchFullContentEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	st := newTestStore(t)
	svc := newFullTextService(t, st)
	itemID, feedID := seedArticle(t, st, srv.URL)

	_, err := svc.FetchFullContent(itemID)
	if err == nil {
		t.Fatal("空响应应返回错误")
	}
	if !errors.Is(err, reader.ErrNoContent) {
		t.Errorf("期望错误链上带 reader.ErrNoContent，得到 %v", err)
	}
	assertFeedErrorCountUnchanged(t, st, feedID)
}

// TestFetchFullContentNonHTMLResponse 非 HTML 响应（JSON）：
// 要么报「提不出正文」，要么返回一份可用的内容——但绝不能是「成功且什么都没存」。
func TestFetchFullContentNonHTMLResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"not found","code":404}`))
	}))
	defer srv.Close()

	st := newTestStore(t)
	svc := newFullTextService(t, st)
	itemID, _ := seedArticle(t, st, srv.URL)

	got, err := svc.FetchFullContent(itemID)
	item, _ := st.GetItem(itemID)
	if err != nil {
		if item.FullContent != "" {
			t.Errorf("报错时不该写库，full_content = %q", item.FullContent)
		}
		return
	}
	// 若提取器认为这页有正文，那这份内容必须与落库的一致。
	if item.FullContent != got {
		t.Errorf("返回值与落库内容不一致")
	}
}

// TestFetchFullContentEmptyURL 没有原文地址：明确的错误，不发起任何请求。
func TestFetchFullContentEmptyURL(t *testing.T) {
	st := newTestStore(t)
	svc := newFullTextService(t, st)

	feedID := seedFeed(t, st, "https://i.example/feed", "I")
	it := &store.Item{FeedID: feedID, Title: "无链接", URL: "", Content: "<p>x</p>"}
	if err := st.CreateItem(it); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	_, err := svc.FetchFullContent(it.ID)
	if err == nil {
		t.Fatal("URL 为空应返回错误")
	}
	// 确认用的是 i18n 目录里的键，而不是键名本身（拼错键会原样返回键名）。
	if strings.Contains(err.Error(), "fulltext.") {
		t.Errorf("错误文案未命中 i18n 目录：%v", err)
	}
}

// TestFetchFullContentWithoutFetcher 未注入 fetcher：返回「不可用」而不是崩在 nil。
func TestFetchFullContentWithoutFetcher(t *testing.T) {
	st := newTestStore(t)
	svc := NewItemService(st, nil) // 不注入 fetcher
	itemID, _ := seedArticle(t, st, "https://example.com/post")

	_, err := svc.FetchFullContent(itemID)
	if err == nil {
		t.Fatal("未注入 fetcher 应返回错误")
	}
}

// TestFetchFullContentUnknownItem 不存在的文章：透传 store 的错误。
func TestFetchFullContentUnknownItem(t *testing.T) {
	st := newTestStore(t)
	svc := newFullTextService(t, st)
	if _, err := svc.FetchFullContent(99999); err == nil {
		t.Fatal("不存在的文章应返回错误")
	}
}

// TestFetchFullContentMessageFollowsLanguage 错误文案跟随设置语言。
func TestFetchFullContentMessageFollowsLanguage(t *testing.T) {
	st := newTestStore(t)
	svc := newFullTextService(t, st)
	feedID := seedFeed(t, st, "https://i.example/feed", "I")
	it := &store.Item{FeedID: feedID, Title: "无链接", URL: ""}
	if err := st.CreateItem(it); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	settings.Language = "en"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	_, err = svc.FetchFullContent(it.ID)
	if err == nil || !strings.Contains(err.Error(), "no original address") {
		t.Errorf("英文文案 = %v", err)
	}

	settings.Language = "zh"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	_, err = svc.FetchFullContent(it.ID)
	if err == nil || !strings.Contains(err.Error(), "没有可抓取的原文地址") {
		t.Errorf("中文文案 = %v", err)
	}
}

// assertFeedErrorCountUnchanged 全文提取失败**不得**影响订阅源的失败计数——
// 那是「已失效」分组的判定依据，混用会让健康的源被误判。
func assertFeedErrorCountUnchanged(t *testing.T, st *store.Store, feedID int64) {
	t.Helper()
	feed, err := st.GetFeed(feedID)
	if err != nil {
		t.Fatalf("GetFeed: %v", err)
	}
	if feed.ErrorCount != 0 {
		t.Errorf("提取失败不该改动 feeds.error_count，得到 %d", feed.ErrorCount)
	}
	if feed.Status != "active" {
		t.Errorf("提取失败不该改动 feeds.status，得到 %q", feed.Status)
	}
}
