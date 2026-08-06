package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openCloudTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := NewWithPath(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestBackupForCloudStripsSettingsAndKeepsData(t *testing.T) {
	st := openCloudTestStore(t)
	feed := &Feed{URL: "https://cloud.example/feed", Title: "Cloud", Status: "active", UpdateInterval: 30, MaxItems: 100}
	if err := st.CreateFeed(feed); err != nil {
		t.Fatal(err)
	}
	secretMarker := "must-not-appear-in-cloud-snapshot"
	if err := st.SetJSONSetting("webdav", map[string]string{"passwordCipher": secretMarker}); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "cloud.db")
	version, err := st.BackupForCloud(dest)
	if err != nil {
		t.Fatalf("BackupForCloud: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Errorf("version = %d, want %d", version, CurrentSchemaVersion)
	}

	db, err := sql.Open("sqlite", dest)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var settings, feeds int
	if err := db.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&settings); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM feeds`).Scan(&feeds); err != nil {
		t.Fatal(err)
	}
	if settings != 0 || feeds != 1 {
		t.Errorf("settings/feeds = %d/%d, want 0/1", settings, feeds)
	}
	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secretMarker) {
		t.Error("deleted settings still remain in snapshot free pages")
	}
}

func TestStageCloudRestorePreservesLocalSettingsAndCreatesRollback(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "target.db")
	target, err := NewWithPath(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	old := &Feed{URL: "https://old.example/feed", Title: "Old", Status: "active", UpdateInterval: 30, MaxItems: 100}
	if err := target.CreateFeed(old); err != nil {
		t.Fatal(err)
	}
	cfg, _ := target.GetSettings()
	cfg.Theme = "sepia"
	cfg.ProxyHost = "127.0.0.1"
	cfg.ProxyPort = 7890
	if err := target.UpdateSettings(cfg); err != nil {
		t.Fatal(err)
	}
	if err := target.SetJSONSetting("webdav", map[string]string{"url": "local"}); err != nil {
		t.Fatal(err)
	}

	source, err := NewWithPath(filepath.Join(t.TempDir(), "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	fresh := &Feed{URL: "https://new.example/feed", Title: "New", Status: "active", UpdateInterval: 30, MaxItems: 100}
	if err := source.CreateFeed(fresh); err != nil {
		t.Fatal(err)
	}
	cloud := filepath.Join(t.TempDir(), "cloud.db")
	if _, err := source.BackupForCloud(cloud); err != nil {
		t.Fatal(err)
	}

	rollback, err := target.StageCloudRestore(cloud)
	if err != nil {
		t.Fatalf("StageCloudRestore: %v", err)
	}
	if _, err := os.Stat(rollback); err != nil {
		t.Fatalf("rollback missing: %v", err)
	}
	if err := target.SetPendingJSONSetting(
		"cloud_backup_state",
		map[string]string{"lastError": ""},
	); err != nil {
		t.Fatalf("SetPendingJSONSetting: %v", err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	if err := applyPendingRestore(targetPath); err != nil {
		t.Fatalf("applyPendingRestore: %v", err)
	}

	restored, err := NewWithPath(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	feeds, err := restored.ListFeeds()
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 1 || feeds[0].Title != "New" {
		t.Errorf("restored feeds = %+v", feeds)
	}
	got, err := restored.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "sepia" || got.ProxyHost != "127.0.0.1" || got.ProxyPort != 7890 {
		t.Errorf("local settings not preserved: %+v", got)
	}
	var state map[string]any
	foundState, err := restored.GetJSONSetting("cloud_backup_state", &state)
	if err != nil || !foundState {
		t.Errorf("cloud backup state missing after restore: found=%v err=%v", foundState, err)
	}
	var webdav map[string]string
	found, err := restored.GetJSONSetting("webdav", &webdav)
	if err != nil || !found || webdav["url"] != "local" {
		t.Errorf("webdav setting = %+v, found=%v, err=%v", webdav, found, err)
	}

	rollbackStore, err := NewWithPath(rollback)
	if err != nil {
		t.Fatal(err)
	}
	defer rollbackStore.Close()
	rollbackFeeds, err := rollbackStore.ListFeeds()
	if err != nil {
		t.Fatal(err)
	}
	if len(rollbackFeeds) != 1 || rollbackFeeds[0].Title != "Old" {
		t.Errorf("rollback feeds = %+v", rollbackFeeds)
	}
}

func TestValidateClipDBRejectsNewerSchema(t *testing.T) {
	st := openCloudTestStore(t)
	dest := filepath.Join(t.TempDir(), "future.db")
	if err := st.BackupTo(dest); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 99`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if err := validateClipDB(dest); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("err = %v, want newer schema rejection", err)
	}
}
