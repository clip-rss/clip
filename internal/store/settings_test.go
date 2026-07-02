package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSettingsDefaultsWhenMissing(t *testing.T) {
	restore := stubSystemLocale(t, "zh-CN")
	defer restore()

	st := setupTestDB(t)

	got, err := st.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	want := DefaultSettings()
	if got != want {
		t.Errorf("defaults = %+v, want %+v", got, want)
	}

	// 首次读取会把默认设置写入数据库，之后环境变化也不覆盖用户设置。
	restore()
	stubSystemLocale(t, "en-US")
	got, err = st.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings after persisted default: %v", err)
	}
	if got.Language != "zh" {
		t.Errorf("persisted language = %q, want zh", got.Language)
	}
}

func TestDefaultSettingsLanguageFromSystemLocale(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		want   string
	}{
		{name: "simplified chinese", locale: "zh-CN", want: "zh"},
		{name: "traditional chinese", locale: "zh_Hant_TW", want: "zh"},
		{name: "english", locale: "en-US", want: "en"},
		{name: "japanese", locale: "ja-JP", want: "en"},
		{name: "empty", locale: "", want: "en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := stubSystemLocale(t, tt.locale)
			defer restore()

			if got := DefaultSettings().Language; got != tt.want {
				t.Errorf("language = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	st := setupTestDB(t)

	want := Settings{
		Theme:                 "dark",
		Language:              "en",
		DefaultUpdateInterval: 15,
		DefaultMaxItems:       50,
		NotificationMode:      NotifyOff,
		AutoMarkReadDelay:     2000,
		LaunchMinimized:       true,
		WindowWidth:           1366,
		WindowHeight:          768,
	}
	if err := st.UpdateSettings(want); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	got, err := st.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}

	// 覆盖写入：再次更新应替换而非追加。
	want.Theme = "light"
	if err := st.UpdateSettings(want); err != nil {
		t.Fatalf("UpdateSettings (overwrite): %v", err)
	}
	got, _ = st.GetSettings()
	if got.Theme != "light" {
		t.Errorf("theme = %q, want light", got.Theme)
	}
}

// TestSettingsPartialMergeKeepsDefaults 验证旧数据缺失字段时回退默认值。
func TestSettingsPartialMergeKeepsDefaults(t *testing.T) {
	restore := stubSystemLocale(t, "en-US")
	defer restore()

	st := setupTestDB(t)

	// 模拟仅持久化了部分字段的历史数据。
	if _, err := st.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)`,
		settingsKey, `{"theme":"dark"}`,
	); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	got, err := st.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.Theme != "dark" {
		t.Errorf("theme = %q, want dark", got.Theme)
	}
	if got.DefaultUpdateInterval != 30 || got.Language != "en" {
		t.Errorf("missing fields should fall back to defaults, got %+v", got)
	}
}

func stubSystemLocale(t *testing.T, locale string) func() {
	t.Helper()
	previous := systemLocale
	systemLocale = func() string { return locale }
	return func() {
		systemLocale = previous
	}
}

// makeItem 在指定源下插入一篇带读/星标态的文章。
func makeItem(t *testing.T, st *Store, feedID int64, url string, read, starred bool) {
	t.Helper()
	it := &Item{
		FeedID:      feedID,
		Title:       url,
		URL:         url,
		PublishedAt: time.Now(),
		IsRead:      read,
		IsStarred:   starred,
	}
	if err := st.CreateItem(it); err != nil {
		t.Fatalf("CreateItem(%s): %v", url, err)
	}
}

// TestPruneReadItems 只删除「已读且未收藏」，保留未读与收藏。
func TestPruneReadItems(t *testing.T) {
	st := setupTestDB(t)
	feed := &Feed{URL: "https://p.example/feed", Title: "P", UpdateInterval: 30, MaxItems: 100, Status: "active"}
	if err := st.CreateFeed(feed); err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}

	makeItem(t, st, feed.ID, "https://p.example/1", true, false)  // 删
	makeItem(t, st, feed.ID, "https://p.example/2", true, false)  // 删
	makeItem(t, st, feed.ID, "https://p.example/3", true, true)   // 保留：已读但收藏
	makeItem(t, st, feed.ID, "https://p.example/4", false, false) // 保留：未读

	removed, err := st.PruneReadItems()
	if err != nil {
		t.Fatalf("PruneReadItems: %v", err)
	}
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}

	items, err := st.ListItemsByFeed(feed.ID, 100, 0)
	if err != nil {
		t.Fatalf("ListItemsByFeed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("remaining = %d, want 2", len(items))
	}
}

// TestBackupTo VACUUM INTO 生成的副本可独立打开且包含数据。
func TestBackupTo(t *testing.T) {
	st := setupTestDB(t)
	feed := &Feed{URL: "https://b.example/feed", Title: "Backup Me", UpdateInterval: 30, MaxItems: 100, Status: "active"}
	if err := st.CreateFeed(feed); err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}

	dest := filepath.Join(t.TempDir(), "backup.db")
	if err := st.BackupTo(dest); err != nil {
		t.Fatalf("BackupTo: %v", err)
	}

	// 独立打开副本，断言数据在。
	copyStore, err := NewWithPath(dest)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer copyStore.Close()

	feeds, err := copyStore.ListFeeds()
	if err != nil {
		t.Fatalf("ListFeeds on backup: %v", err)
	}
	if len(feeds) != 1 || feeds[0].Title != "Backup Me" {
		t.Errorf("backup feeds = %+v, want one 'Backup Me'", feeds)
	}
}
