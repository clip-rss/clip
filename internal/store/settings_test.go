package store

import "testing"

func TestSettingsDefaultsWhenMissing(t *testing.T) {
	st := setupTestDB(t)

	got, err := st.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	want := DefaultSettings()
	if got != want {
		t.Errorf("defaults = %+v, want %+v", got, want)
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
	if got.DefaultUpdateInterval != 30 || got.Language != "zh" {
		t.Errorf("missing fields should fall back to defaults, got %+v", got)
	}
}
