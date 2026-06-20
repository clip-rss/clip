package store

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *Store {
	// 使用临时数据库
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	// 临时覆盖 dbPathFunc
	originalFunc := dbPathFunc
	dbPathFunc = func() (string, error) {
		return dbPath, nil
	}
	t.Cleanup(func() {
		dbPathFunc = originalFunc
	})

	store, err := New()
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
		os.Remove(dbPath)
	})

	return store
}

// TestCategoryOperations 测试分类 CRUD
func TestCategoryOperations(t *testing.T) {
	store := setupTestDB(t)

	// 创建根分类
	cat1 := &Category{
		Name:      "技术",
		SortOrder: 1,
	}
	if err := store.CreateCategory(cat1); err != nil {
		t.Fatalf("failed to create category: %v", err)
	}
	if cat1.ID == 0 {
		t.Fatal("category ID should not be 0")
	}

	// 创建子分类
	cat2 := &Category{
		Name:      "前端",
		ParentID:  &cat1.ID,
		SortOrder: 1,
	}
	if err := store.CreateCategory(cat2); err != nil {
		t.Fatalf("failed to create child category: %v", err)
	}

	// 获取分类
	retrieved, err := store.GetCategory(cat1.ID)
	if err != nil {
		t.Fatalf("failed to get category: %v", err)
	}
	if retrieved.Name != "技术" {
		t.Errorf("expected name '技术', got '%s'", retrieved.Name)
	}

	// 列出所有分类
	categories, err := store.ListCategories()
	if err != nil {
		t.Fatalf("failed to list categories: %v", err)
	}
	if len(categories) != 2 {
		t.Errorf("expected 2 categories, got %d", len(categories))
	}

	// 列出根分类
	rootCats, err := store.ListRootCategories()
	if err != nil {
		t.Fatalf("failed to list root categories: %v", err)
	}
	if len(rootCats) != 1 {
		t.Errorf("expected 1 root category, got %d", len(rootCats))
	}

	// 列出子分类
	childCats, err := store.ListChildCategories(cat1.ID)
	if err != nil {
		t.Fatalf("failed to list child categories: %v", err)
	}
	if len(childCats) != 1 {
		t.Errorf("expected 1 child category, got %d", len(childCats))
	}

	// 更新分类
	cat1.Name = "技术类"
	if err := store.UpdateCategory(cat1); err != nil {
		t.Fatalf("failed to update category: %v", err)
	}
	updated, _ := store.GetCategory(cat1.ID)
	if updated.Name != "技术类" {
		t.Errorf("expected updated name '技术类', got '%s'", updated.Name)
	}

	// 删除分类（级联删除子分类）
	if err := store.DeleteCategory(cat1.ID); err != nil {
		t.Fatalf("failed to delete category: %v", err)
	}

	// 验证删除
	categories, _ = store.ListCategories()
	if len(categories) != 0 {
		t.Errorf("expected 0 categories after deletion, got %d", len(categories))
	}
}

// TestFeedOperations 测试订阅源 CRUD
func TestFeedOperations(t *testing.T) {
	store := setupTestDB(t)

	// 创建分类
	cat := &Category{Name: "新闻", SortOrder: 1}
	store.CreateCategory(cat)

	// 创建订阅源
	feed := &Feed{
		URL:            "https://example.com/feed.xml",
		Title:          "Example Feed",
		Description:    "An example RSS feed",
		Link:           "https://example.com",
		Icon:           "https://example.com/favicon.ico",
		CategoryID:     &cat.ID,
		UpdateInterval: 30,
		MaxItems:       100,
		Status:         "active",
	}
	if err := store.CreateFeed(feed); err != nil {
		t.Fatalf("failed to create feed: %v", err)
	}
	if feed.ID == 0 {
		t.Fatal("feed ID should not be 0")
	}

	// 根据 ID 获取
	retrieved, err := store.GetFeed(feed.ID)
	if err != nil {
		t.Fatalf("failed to get feed: %v", err)
	}
	if retrieved.Title != "Example Feed" {
		t.Errorf("expected title 'Example Feed', got '%s'", retrieved.Title)
	}

	// 根据 URL 获取
	byURL, err := store.GetFeedByURL(feed.URL)
	if err != nil {
		t.Fatalf("failed to get feed by URL: %v", err)
	}
	if byURL.ID != feed.ID {
		t.Errorf("expected ID %d, got %d", feed.ID, byURL.ID)
	}

	// 列出所有订阅源
	feeds, err := store.ListFeeds()
	if err != nil {
		t.Fatalf("failed to list feeds: %v", err)
	}
	if len(feeds) != 1 {
		t.Errorf("expected 1 feed, got %d", len(feeds))
	}

	// 列出带未读计数的订阅源
	feedsWithUnread, err := store.ListFeedsWithUnread()
	if err != nil {
		t.Fatalf("failed to list feeds with unread: %v", err)
	}
	if len(feedsWithUnread) != 1 {
		t.Errorf("expected 1 feed, got %d", len(feedsWithUnread))
	}
	if feedsWithUnread[0].UnreadCount != 0 {
		t.Errorf("expected 0 unread, got %d", feedsWithUnread[0].UnreadCount)
	}

	// 更新订阅源
	feed.Title = "Updated Feed"
	if err := store.UpdateFeed(feed); err != nil {
		t.Fatalf("failed to update feed: %v", err)
	}
	updated, _ := store.GetFeed(feed.ID)
	if updated.Title != "Updated Feed" {
		t.Errorf("expected updated title, got '%s'", updated.Title)
	}

	// 更新状态
	if err := store.UpdateFeedStatus(feed.ID, "paused"); err != nil {
		t.Fatalf("failed to update feed status: %v", err)
	}
	updated, _ = store.GetFeed(feed.ID)
	if updated.Status != "paused" {
		t.Errorf("expected status 'paused', got '%s'", updated.Status)
	}

	// 更新错误
	if err := store.UpdateFeedError(feed.ID, "connection timeout"); err != nil {
		t.Fatalf("failed to update feed error: %v", err)
	}
	updated, _ = store.GetFeed(feed.ID)
	if updated.ErrorCount != 1 {
		t.Errorf("expected error count 1, got %d", updated.ErrorCount)
	}

	// 重置错误
	if err := store.ResetFeedError(feed.ID); err != nil {
		t.Fatalf("failed to reset feed error: %v", err)
	}
	updated, _ = store.GetFeed(feed.ID)
	if updated.ErrorCount != 0 {
		t.Errorf("expected error count 0, got %d", updated.ErrorCount)
	}

	// 删除订阅源
	if err := store.DeleteFeed(feed.ID); err != nil {
		t.Fatalf("failed to delete feed: %v", err)
	}

	// 验证删除
	_, err = store.GetFeed(feed.ID)
	if err == nil {
		t.Error("expected error when getting deleted feed")
	}
}

// TestItemOperations 测试文章 CRUD
func TestItemOperations(t *testing.T) {
	store := setupTestDB(t)

	// 创建订阅源
	feed := &Feed{
		URL:            "https://example.com/feed.xml",
		Title:          "Example Feed",
		UpdateInterval: 30,
		MaxItems:       100,
		Status:         "active",
	}
	store.CreateFeed(feed)

	// 创建文章
	now := time.Now()
	item := &Item{
		FeedID:      feed.ID,
		Title:       "Test Article",
		Author:      "John Doe",
		PublishedAt: now,
		URL:         "https://example.com/article1",
		Content:     "This is the content",
		Summary:     "This is the summary",
		IsRead:      false,
		IsStarred:   false,
	}
	if err := store.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}
	if item.ID == 0 {
		t.Fatal("item ID should not be 0")
	}

	// 测试 CreateItemIfNotExists
	item2 := &Item{
		FeedID:      feed.ID,
		Title:       "Test Article 2",
		URL:         "https://example.com/article1", // 相同 URL
		PublishedAt: now,
	}
	created, err := store.CreateItemIfNotExists(item2)
	if err != nil {
		t.Fatalf("failed to create item if not exists: %v", err)
	}
	if created {
		t.Error("expected item not to be created (duplicate URL)")
	}

	// 获取文章
	retrieved, err := store.GetItem(item.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}
	if retrieved.Title != "Test Article" {
		t.Errorf("expected title 'Test Article', got '%s'", retrieved.Title)
	}

	// 列出订阅源的文章
	items, err := store.ListItemsByFeed(feed.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to list items: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}

	// 标记为已读
	if err := store.MarkItemAsRead(item.ID); err != nil {
		t.Fatalf("failed to mark item as read: %v", err)
	}
	updated, _ := store.GetItem(item.ID)
	if !updated.IsRead {
		t.Error("expected item to be marked as read")
	}

	// 标记为未读
	if err := store.MarkItemAsUnread(item.ID); err != nil {
		t.Fatalf("failed to mark item as unread: %v", err)
	}
	updated, _ = store.GetItem(item.ID)
	if updated.IsRead {
		t.Error("expected item to be marked as unread")
	}

	// 切换星标
	if err := store.ToggleItemStar(item.ID); err != nil {
		t.Fatalf("failed to toggle star: %v", err)
	}
	updated, _ = store.GetItem(item.ID)
	if !updated.IsStarred {
		t.Error("expected item to be starred")
	}

	// 更新笔记
	if err := store.UpdateItemNote(item.ID, "My note"); err != nil {
		t.Fatalf("failed to update note: %v", err)
	}
	updated, _ = store.GetItem(item.ID)
	if updated.Note != "My note" {
		t.Errorf("expected note 'My note', got '%s'", updated.Note)
	}

	// 获取未读计数
	count, err := store.GetUnreadCount()
	if err != nil {
		t.Fatalf("failed to get unread count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 unread, got %d", count)
	}

	// 批量标记已读
	if err := store.MarkItemsAsRead([]int64{item.ID}); err != nil {
		t.Fatalf("failed to mark items as read: %v", err)
	}
	count, _ = store.GetUnreadCount()
	if count != 0 {
		t.Errorf("expected 0 unread after batch mark, got %d", count)
	}

	// 删除文章
	if err := store.DeleteItem(item.ID); err != nil {
		t.Fatalf("failed to delete item: %v", err)
	}

	// 验证删除
	_, err = store.GetItem(item.ID)
	if err == nil {
		t.Error("expected error when getting deleted item")
	}
}

// TestSearchItems 测试全文搜索
func TestSearchItems(t *testing.T) {
	store := setupTestDB(t)

	// 创建订阅源
	feed := &Feed{
		URL:            "https://example.com/feed.xml",
		Title:          "Example Feed",
		UpdateInterval: 30,
		MaxItems:       100,
		Status:         "active",
	}
	store.CreateFeed(feed)

	// 创建多篇文章
	items := []*Item{
		{
			FeedID:      feed.ID,
			Title:       "Go Programming Tutorial",
			Summary:     "Learn Go language basics",
			URL:         "https://example.com/go-tutorial",
			PublishedAt: time.Now(),
		},
		{
			FeedID:      feed.ID,
			Title:       "React Hooks Guide",
			Summary:     "Modern React development",
			URL:         "https://example.com/react-hooks",
			PublishedAt: time.Now(),
		},
		{
			FeedID:      feed.ID,
			Title:       "Database Optimization",
			Summary:     "How to optimize database queries",
			URL:         "https://example.com/db-optimization",
			PublishedAt: time.Now(),
		},
	}

	for _, item := range items {
		store.CreateItem(item)
	}

	// 搜索包含 "Go" 的文章
	results, err := store.SearchItems("Go", 10, 0)
	if err != nil {
		t.Fatalf("failed to search items: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'Go', got %d", len(results))
	}

	// 搜索包含 "React" 的文章
	results, err = store.SearchItems("React", 10, 0)
	if err != nil {
		t.Fatalf("failed to search items: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'React', got %d", len(results))
	}
}

// TestBatchOperations 测试批量操作
func TestBatchOperations(t *testing.T) {
	store := setupTestDB(t)

	// 创建订阅源
	feed := &Feed{
		URL:            "https://example.com/feed.xml",
		Title:          "Example Feed",
		UpdateInterval: 30,
		MaxItems:       100,
		Status:         "active",
	}
	store.CreateFeed(feed)

	// 创建多篇文章
	var itemIDs []int64
	for i := 0; i < 5; i++ {
		item := &Item{
			FeedID:      feed.ID,
			Title:       "Article " + string(rune('A'+i)),
			URL:         "https://example.com/article" + string(rune('0'+i)),
			PublishedAt: time.Now(),
			IsRead:      false,
		}
		store.CreateItem(item)
		itemIDs = append(itemIDs, item.ID)
	}

	// 批量标记已读
	if err := store.MarkItemsAsRead(itemIDs[:3]); err != nil {
		t.Fatalf("failed to batch mark as read: %v", err)
	}

	// 验证未读数
	count, _ := store.GetUnreadCount()
	if count != 2 {
		t.Errorf("expected 2 unread after batch mark, got %d", count)
	}

	// 标记整个订阅源为已读
	if err := store.MarkAllAsReadByFeed(feed.ID); err != nil {
		t.Fatalf("failed to mark all as read by feed: %v", err)
	}

	count, _ = store.GetUnreadCount()
	if count != 0 {
		t.Errorf("expected 0 unread after mark all, got %d", count)
	}

	// 批量标记未读
	if err := store.MarkItemsAsUnread(itemIDs); err != nil {
		t.Fatalf("failed to batch mark as unread: %v", err)
	}

	count, _ = store.GetUnreadCount()
	if count != 5 {
		t.Errorf("expected 5 unread after batch unmark, got %d", count)
	}
}

// seedSearchItems 建一个源并插入若干用于搜索测试的文章。
func seedSearchItems(t *testing.T, store *Store, titles []string) int64 {
	t.Helper()
	feed := &Feed{URL: "https://s.example/feed", Title: "S", UpdateInterval: 30, MaxItems: 100, Status: "active"}
	if err := store.CreateFeed(feed); err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}
	for i, title := range titles {
		it := &Item{
			FeedID:      feed.ID,
			Title:       title,
			URL:         fmt.Sprintf("https://s.example/%d", i),
			PublishedAt: time.Now(),
		}
		if err := store.CreateItem(it); err != nil {
			t.Fatalf("CreateItem: %v", err)
		}
	}
	return feed.ID
}

// TestSearchChineseSubstring 中文子串：2 字走 LIKE、≥3 字走 FTS，均应命中。
func TestSearchChineseSubstring(t *testing.T) {
	store := setupTestDB(t)
	seedSearchItems(t, store, []string{
		"科技爱好者周刊第 400 期",
		"前端每周清单",
		"数据库优化实践",
	})

	cases := []struct {
		kw   string
		want int
	}{
		{"周刊", 1},     // 2 字 → LIKE
		{"爱好者", 1},    // 3 字 → FTS
		{"科技爱好", 1},   // 4 字 → FTS
		{"清单", 1},     // 2 字 → LIKE
		{"不存在的词", 0},
	}
	for _, c := range cases {
		got, err := store.SearchItems(c.kw, 10, 0)
		if err != nil {
			t.Fatalf("SearchItems(%q): %v", c.kw, err)
		}
		if len(got) != c.want {
			t.Errorf("SearchItems(%q) = %d, want %d", c.kw, len(got), c.want)
		}
	}
}

// TestSearchEnglishCaseInsensitive 英文大小写不敏感。
func TestSearchEnglishCaseInsensitive(t *testing.T) {
	store := setupTestDB(t)
	seedSearchItems(t, store, []string{"Swift Concurrency Guide", "Go Modules"})

	for _, kw := range []string{"Swift", "swift", "SWIFT", "concurrency"} {
		got, err := store.SearchItems(kw, 10, 0)
		if err != nil {
			t.Fatalf("SearchItems(%q): %v", kw, err)
		}
		if len(got) != 1 {
			t.Errorf("SearchItems(%q) = %d, want 1", kw, len(got))
		}
	}
}

// TestSearchSpecialCharsNoError 含 FTS 语法字符的输入不应报错。
func TestSearchSpecialCharsNoError(t *testing.T) {
	store := setupTestDB(t)
	seedSearchItems(t, store, []string{`Title with "quotes" and dash-here`})

	for _, kw := range []string{`"`, `a"b`, `c-d`, `*`, `foo:bar`, `(x)`} {
		if _, err := store.SearchItems(kw, 10, 0); err != nil {
			t.Errorf("SearchItems(%q) unexpected error: %v", kw, err)
		}
	}
}

// TestSearchMultiTokenAND 多 token 之间为 AND。
func TestSearchMultiTokenAND(t *testing.T) {
	store := setupTestDB(t)
	seedSearchItems(t, store, []string{
		"科技爱好者周刊 Swift 专题",
		"科技爱好者周刊 React 专题",
	})

	// 两个都 ≥3 字 → FTS AND
	got, err := store.SearchItems("爱好者 Swift", 10, 0)
	if err != nil {
		t.Fatalf("SearchItems: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("AND search = %d, want 1", len(got))
	}

	// 含 2 字短词 → LIKE AND
	got, err = store.SearchItems("周刊 React", 10, 0)
	if err != nil {
		t.Fatalf("SearchItems: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("AND search (like) = %d, want 1", len(got))
	}
}

// TestFTSMigrationRebuildsIndex 迁移后 user_version 应为 1，且已有文章可被搜索到。
func TestFTSMigrationRebuildsIndex(t *testing.T) {
	store := setupTestDB(t)
	seedSearchItems(t, store, []string{"科技爱好者周刊"})

	var version int
	if err := store.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 1 {
		t.Errorf("user_version = %d, want 1", version)
	}

	// 再次运行迁移应是 no-op（不报错、版本不变）。
	if err := store.migrateFTSTokenizer(); err != nil {
		t.Fatalf("re-run migrateFTSTokenizer: %v", err)
	}
}
