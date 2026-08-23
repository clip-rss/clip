package store

import (
	"strings"
	"testing"
)

func TestChangelogCacheRoundTrip(t *testing.T) {
	st := setupTestDB(t)

	got, found, err := st.GetChangelogCache()
	if err != nil {
		t.Fatalf("GetChangelogCache (missing): %v", err)
	}
	if found {
		t.Fatal("missing cache reported as found")
	}
	if got != (ChangelogCache{}) {
		t.Errorf("missing cache should return zero value, got %+v", got)
	}

	want := ChangelogCache{Version: "0.2.0", Markdown: "## 0.2.0\n\n### 新增\n- 甲"}
	if err := st.SaveChangelogCache(want); err != nil {
		t.Fatalf("SaveChangelogCache: %v", err)
	}

	got, found, err = st.GetChangelogCache()
	if err != nil {
		t.Fatalf("GetChangelogCache: %v", err)
	}
	if !found {
		t.Fatal("stored cache reported as missing")
	}
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

// TestChangelogCacheOverwrite 再次保存必须整体覆盖，不能残留上一版内容。
func TestChangelogCacheOverwrite(t *testing.T) {
	st := setupTestDB(t)

	if err := st.SaveChangelogCache(ChangelogCache{Version: "0.1.0", Markdown: "old"}); err != nil {
		t.Fatal(err)
	}
	want := ChangelogCache{Version: "0.2.0", Markdown: "new"}
	if err := st.SaveChangelogCache(want); err != nil {
		t.Fatal(err)
	}

	got, found, err := st.GetChangelogCache()
	if err != nil {
		t.Fatal(err)
	}
	if !found || got != want {
		t.Errorf("after overwrite = %+v (found=%v), want %+v", got, found, want)
	}
}

// TestChangelogCacheDecodeFailureIsError 值损坏时报错而非静默当成「无缓存」，
// 与 GetJSONSetting 的约定一致；调用方再决定忽略后重新抓取。
func TestChangelogCacheDecodeFailureIsError(t *testing.T) {
	st := setupTestDB(t)

	if _, err := st.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)`, changelogCacheKey, `{not json`,
	); err != nil {
		t.Fatal(err)
	}

	got, found, err := st.GetChangelogCache()
	if err == nil {
		t.Fatal("corrupt cache should return an error")
	}
	if found {
		t.Error("found should be false when decoding fails")
	}
	if got != (ChangelogCache{}) {
		t.Errorf("failed decode should return zero value, got %+v", got)
	}
	if !strings.Contains(err.Error(), changelogCacheKey) {
		t.Errorf("error should name the key, got %v", err)
	}
}

// TestChangelogCacheDoesNotTouchSettings 缓存占独立键，不能影响 key='app' 的全局设置。
func TestChangelogCacheDoesNotTouchSettings(t *testing.T) {
	st := setupTestDB(t)

	before, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveChangelogCache(ChangelogCache{Version: "0.2.0", Markdown: "x"}); err != nil {
		t.Fatal(err)
	}
	after, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Errorf("settings changed after caching changelog: %+v → %+v", before, after)
	}
}
