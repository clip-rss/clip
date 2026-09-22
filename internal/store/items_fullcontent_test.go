package store

import (
	"database/sql"
	"testing"
	"time"
)

// createTestFeed 建一个启用中的源（字段取值与 store_test.go 里的内联写法一致）。
func createTestFeed(t *testing.T, st *Store, feedURL string) *Feed {
	t.Helper()
	feed := &Feed{URL: feedURL, Title: "测试源", UpdateInterval: 30, MaxItems: 100, Status: "active"}
	if err := st.CreateFeed(feed); err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}
	return feed
}

// createTestItem 建一篇最小可用的文章，返回其 ID。
func createTestItem(t *testing.T, st *Store, feedID int64, url string) int64 {
	t.Helper()
	item := &Item{
		FeedID:      feedID,
		Title:       "标题",
		URL:         url,
		PublishedAt: time.Now(),
		Content:     "<p>RSS 给的摘要</p>",
		Summary:     "摘要",
	}
	if err := st.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}
	return item.ID
}

// TestSaveFullContent 验证全文落库，且不覆盖 RSS 原文。
func TestSaveFullContent(t *testing.T) {
	st := setupTestDB(t)
	feed := createTestFeed(t, st, "https://example.com/feed")
	id := createTestItem(t, st, feed.ID, "https://example.com/post/1")

	const extracted = "<p>提取出来的完整正文</p>"
	if err := st.SaveFullContent(id, extracted); err != nil {
		t.Fatalf("SaveFullContent: %v", err)
	}

	got, err := st.GetItem(id)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if got.FullContent != extracted {
		t.Errorf("full_content = %q, 期望 %q", got.FullContent, extracted)
	}
	// 关键不变量：RSS 原文必须原样保留，否则提取错了无法回退。
	if got.Content != "<p>RSS 给的摘要</p>" {
		t.Errorf("content 被改动了：%q", got.Content)
	}
}

// TestSaveFullContentUnknownItem 对不存在的文章应报错而不是静默成功。
func TestSaveFullContentUnknownItem(t *testing.T) {
	st := setupTestDB(t)
	if err := st.SaveFullContent(99999, "<p>x</p>"); err == nil {
		t.Fatal("对不存在的文章写入 full_content 应报错")
	}
}

// TestFullContentDefaultsEmpty 新建文章时 full_content 应为空串而非 NULL——
// 列声明是 NOT NULL DEFAULT ''，扫描目标也是 string。
func TestFullContentDefaultsEmpty(t *testing.T) {
	st := setupTestDB(t)
	feed := createTestFeed(t, st, "https://example.com/feed")
	id := createTestItem(t, st, feed.ID, "https://example.com/post/1")

	got, err := st.GetItem(id)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if got.FullContent != "" {
		t.Errorf("新文章 full_content 应为空串，得到 %q", got.FullContent)
	}
}

// TestListItemsIncludesFullContent 列表查询同样要带上 full_content，
// 否则从列表点进阅读界面时提取结果会丢。
func TestListItemsIncludesFullContent(t *testing.T) {
	st := setupTestDB(t)
	feed := createTestFeed(t, st, "https://example.com/feed")
	id := createTestItem(t, st, feed.ID, "https://example.com/post/1")

	const extracted = "<p>提取出来的完整正文</p>"
	if err := st.SaveFullContent(id, extracted); err != nil {
		t.Fatalf("SaveFullContent: %v", err)
	}

	items, err := st.ListAllItems(10, 0)
	if err != nil {
		t.Fatalf("ListAllItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("期望 1 篇文章，得到 %d", len(items))
	}
	if items[0].FullContent != extracted {
		t.Errorf("列表里的 full_content = %q，期望 %q", items[0].FullContent, extracted)
	}
}

// TestSaveFullContentKeepsFTSSearchable 写入 full_content 会触发 items_fts_update
// 触发器（索引列不含 full_content，触发器只是原值删了再插回去）。
// 这里守住「搜索行为与本阶段之前完全一致」这条验收标准。
func TestSaveFullContentKeepsFTSSearchable(t *testing.T) {
	st := setupTestDB(t)
	feed := createTestFeed(t, st, "https://example.com/feed")

	item := &Item{
		FeedID:      feed.ID,
		Title:       "量子计算入门",
		URL:         "https://example.com/post/1",
		PublishedAt: time.Now(),
		Summary:     "一篇关于量子计算的科普",
	}
	if err := st.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	if err := st.SaveFullContent(item.ID, "<p>正文里出现了量子纠缠这个词</p>"); err != nil {
		t.Fatalf("SaveFullContent: %v", err)
	}

	// 标题仍可搜到。
	byTitle, err := st.SearchItems("量子计算", 10, 0)
	if err != nil {
		t.Fatalf("SearchItems(标题): %v", err)
	}
	if len(byTitle) != 1 {
		t.Errorf("写入 full_content 后按标题搜索应命中 1 篇，得到 %d", len(byTitle))
	}

	// full_content 不进索引：只出现在正文里的词搜不到（本阶段的明确取舍）。
	byBody, err := st.SearchItems("量子纠缠", 10, 0)
	if err != nil {
		t.Fatalf("SearchItems(正文): %v", err)
	}
	if len(byBody) != 0 {
		t.Errorf("full_content 未进 FTS 索引，不该被搜到，却命中 %d 篇", len(byBody))
	}

	// 索引未被写坏（触发器用错形式会报 database disk image is malformed）。
	if err := st.integrityCheck(); err != nil {
		t.Errorf("FTS 索引在写入 full_content 后损坏：%v", err)
	}
}

// integrityCheck 跑一次 SQLite 完整性检查，用于捕捉 FTS 外部内容表被写坏的情况。
func (s *Store) integrityCheck() error {
	var result string
	if err := s.db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return sql.ErrConnDone
	}
	return nil
}

// TestMigrateItemFullContent 验证旧库（无 full_content 列）能补上该列，
// 且迁移可重复执行。列嗅探式迁移没有 user_version 门控，幂等性必须自证。
func TestMigrateItemFullContent(t *testing.T) {
	st := setupTestDB(t)

	for i := 0; i < 2; i++ {
		if err := st.migrateItemFullContent(); err != nil {
			t.Fatalf("第 %d 次 migrateItemFullContent: %v", i+1, err)
		}
	}

	has, err := st.hasColumn("items", "full_content")
	if err != nil {
		t.Fatalf("hasColumn: %v", err)
	}
	if !has {
		t.Fatal("迁移后 items 表仍没有 full_content 列")
	}
}

// TestHasColumn 覆盖列嗅探的正反两例——它现在是两个迁移共用的基础件。
func TestHasColumn(t *testing.T) {
	st := setupTestDB(t)

	has, err := st.hasColumn("items", "full_content")
	if err != nil {
		t.Fatalf("hasColumn: %v", err)
	}
	if !has {
		t.Error("items.full_content 应存在")
	}

	has, err = st.hasColumn("items", "no_such_column")
	if err != nil {
		t.Fatalf("hasColumn: %v", err)
	}
	if has {
		t.Error("items.no_such_column 不应存在")
	}
}

// TestMigrateItemFullContentOnLegacyDB 模拟真正的旧库：建一张不含 full_content
// 的 items 表，跑迁移后应能正常读写出该列。
func TestMigrateItemFullContentOnLegacyDB(t *testing.T) {
	st := setupTestDB(t)

	// 丢掉新列，回到阶段 28 之前的表结构。
	if _, err := st.db.Exec(`ALTER TABLE items DROP COLUMN full_content`); err != nil {
		t.Fatalf("drop column: %v", err)
	}

	has, err := st.hasColumn("items", "full_content")
	if err != nil {
		t.Fatalf("hasColumn: %v", err)
	}
	if has {
		t.Fatal("drop 后不该还有 full_content 列")
	}

	if err := st.migrateItemFullContent(); err != nil {
		t.Fatalf("migrateItemFullContent: %v", err)
	}

	// 补列后，老文章与新建文章都要能正常读写。
	feed := createTestFeed(t, st, "https://example.com/feed")
	id := createTestItem(t, st, feed.ID, "https://example.com/post/1")
	if err := st.SaveFullContent(id, "<p>正文</p>"); err != nil {
		t.Fatalf("SaveFullContent: %v", err)
	}
	got, err := st.GetItem(id)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if got.FullContent != "<p>正文</p>" {
		t.Errorf("full_content = %q", got.FullContent)
	}
}
