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
		{name: "traditional chinese", locale: "zh_Hant_TW", want: "zh-TW"},
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
		DefaultUpdateInterval: 30,
		DefaultMaxItems:       50,
		NotificationMode:      NotifyOff,
		AutoMarkReadDelay:     2000,
		WindowWidth:           1366,
		WindowHeight:          768,
		ReaderFontFamily:      "serif",
		ReaderFontSize:        18,
		ReaderLineHeight:      2.0,
		ReaderWidth:           "full",
		ReaderBackground:      "sepia",
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

func TestGlobalIntervalUpdateIsAtomic(t *testing.T) {
	st := setupTestDB(t)
	before, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	feed := &Feed{URL: "https://atomic.example/feed", Title: "A", UpdateInterval: 30, MaxItems: 100, Status: "active"}
	if err := st.CreateFeed(feed); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`
		CREATE TRIGGER reject_global_interval BEFORE UPDATE OF update_interval ON feeds BEGIN
			SELECT RAISE(ABORT, 'simulated interval failure');
		END
	`); err != nil {
		t.Fatal(err)
	}

	after := before
	after.DefaultUpdateInterval = 15
	if err := st.UpdateSettingsAndFeedIntervals(after); err == nil {
		t.Fatal("global interval update should fail")
	}
	persisted, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if persisted.DefaultUpdateInterval != before.DefaultUpdateInterval {
		t.Fatalf("settings interval = %d after rollback, want %d", persisted.DefaultUpdateInterval, before.DefaultUpdateInterval)
	}
	persistedFeed, err := st.GetFeed(feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persistedFeed.UpdateInterval != 30 {
		t.Fatalf("feed interval = %d after rollback, want 30", persistedFeed.UpdateInterval)
	}
}

func TestUnsupportedGlobalIntervalMigratesToDefault(t *testing.T) {
	st := setupTestDB(t)
	feed := &Feed{URL: "https://legacy.example/feed", Title: "Legacy", UpdateInterval: 5, MaxItems: 100, Status: "active"}
	if err := st.CreateFeed(feed); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)`,
		settingsKey, `{"defaultUpdateInterval":5}`,
	); err != nil {
		t.Fatal(err)
	}

	got, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultUpdateInterval != DefaultSettings().DefaultUpdateInterval {
		t.Fatalf("migrated interval = %d, want default %d", got.DefaultUpdateInterval, DefaultSettings().DefaultUpdateInterval)
	}
	persistedFeed, err := st.GetFeed(feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persistedFeed.UpdateInterval != DefaultSettings().DefaultUpdateInterval {
		t.Fatalf("feed interval = %d after migration, want default %d", persistedFeed.UpdateInterval, DefaultSettings().DefaultUpdateInterval)
	}
}

func TestUpdateSettingsRejectsUnsupportedGlobalInterval(t *testing.T) {
	st := setupTestDB(t)
	settings := DefaultSettings()
	settings.DefaultUpdateInterval = 5
	if err := st.UpdateSettings(settings); err == nil {
		t.Fatal("unsupported global interval should be rejected")
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

// TestReaderPrefsDefaults 阅读排版默认值需与收归前的前端 ReaderStore 初值一致，
// 否则老用户升级后排版会被静默改掉。
func TestReaderPrefsDefaults(t *testing.T) {
	d := DefaultSettings()
	if d.ReaderFontFamily != "sans" {
		t.Errorf("fontFamily = %q, want sans", d.ReaderFontFamily)
	}
	if d.ReaderFontSize != 16 {
		t.Errorf("fontSize = %d, want 16", d.ReaderFontSize)
	}
	if d.ReaderLineHeight != 1.8 {
		t.Errorf("lineHeight = %v, want 1.8", d.ReaderLineHeight)
	}
	if d.ReaderWidth != "640" {
		t.Errorf("width = %q, want 640", d.ReaderWidth)
	}
	if d.ReaderBackground != "default" {
		t.Errorf("background = %q, want default", d.ReaderBackground)
	}
}

// TestReaderPrefsFallbackOnLegacyDB 老库的 settings JSON 里没有 reader* 字段，
// 读出来必须是默认值而不是零值（零值会让字号变 0、行高变 0）。
func TestReaderPrefsFallbackOnLegacyDB(t *testing.T) {
	st := setupTestDB(t)

	// 模拟阶段 A 之前的历史数据：完全没有 reader* 字段。
	if _, err := st.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)`,
		settingsKey, `{"theme":"dark","language":"zh","defaultUpdateInterval":30}`,
	); err != nil {
		t.Fatalf("seed legacy settings: %v", err)
	}

	got, err := st.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	d := DefaultSettings()
	if got.ReaderFontSize != d.ReaderFontSize || got.ReaderLineHeight != d.ReaderLineHeight {
		t.Errorf("legacy db reader prefs = size %d / line %v, want size %d / line %v",
			got.ReaderFontSize, got.ReaderLineHeight, d.ReaderFontSize, d.ReaderLineHeight)
	}
	if got.ReaderFontFamily != d.ReaderFontFamily ||
		got.ReaderWidth != d.ReaderWidth ||
		got.ReaderBackground != d.ReaderBackground {
		t.Errorf("legacy db reader prefs fell back wrong: %+v", got)
	}
}

// TestReaderLineHeightJSONRoundTrip 行高是唯一的浮点字段，
// 断言 1.5/1.8/2.0 经 JSON 往返后仍能用 == 精确比较（结构体可比较性依赖此）。
func TestReaderLineHeightJSONRoundTrip(t *testing.T) {
	st := setupTestDB(t)

	for _, lh := range []float64{1.5, 1.8, 2.0} {
		s := DefaultSettings()
		s.ReaderLineHeight = lh
		if err := st.UpdateSettings(s); err != nil {
			t.Fatalf("UpdateSettings(%v): %v", lh, err)
		}
		got, err := st.GetSettings()
		if err != nil {
			t.Fatalf("GetSettings(%v): %v", lh, err)
		}
		if got.ReaderLineHeight != lh {
			t.Errorf("lineHeight round trip = %v, want %v", got.ReaderLineHeight, lh)
		}
		if got != s {
			t.Errorf("full struct round trip mismatch at lineHeight %v", lh)
		}
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
