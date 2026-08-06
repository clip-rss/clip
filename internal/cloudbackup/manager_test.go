package cloudbackup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/clip-rss/clip/internal/store"
	"github.com/clip-rss/clip/internal/webdav"
)

type memoryRemote struct {
	mu       sync.Mutex
	files    map[string][]byte
	versions map[string]int
}

func newMemoryRemote() *memoryRemote {
	return &memoryRemote{files: map[string][]byte{}, versions: map[string]int{}}
}

func (r *memoryRemote) Get(_ context.Context, path string) ([]byte, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, ok := r.files[path]
	if !ok {
		return nil, "", webdav.ErrNotFound
	}
	return append([]byte(nil), data...), fmt.Sprint(r.versions[path]), nil
}

func (r *memoryRemote) PutStream(
	_ context.Context,
	path string,
	src io.Reader,
	_ int64,
	opts webdav.PutOptions,
) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, exists := r.files[path]
	if opts.IfNoneMatch && exists {
		return "", webdav.ErrConflict
	}
	if opts.IfMatch != "" && (!exists || opts.IfMatch != fmt.Sprint(r.versions[path])) {
		return "", webdav.ErrConflict
	}
	data, err := io.ReadAll(src)
	if err != nil {
		return "", err
	}
	r.versions[path]++
	r.files[path] = data
	return fmt.Sprint(r.versions[path]), nil
}

func (r *memoryRemote) GetTo(
	_ context.Context,
	path string,
	dst io.Writer,
	maxBytes int64,
) (string, int64, error) {
	r.mu.Lock()
	data, ok := r.files[path]
	version := r.versions[path]
	r.mu.Unlock()
	if !ok {
		return "", 0, webdav.ErrNotFound
	}
	if int64(len(data)) > maxBytes {
		return "", 0, webdav.ErrResponseTooLarge
	}
	n, err := io.Copy(dst, bytes.NewReader(data))
	return fmt.Sprint(version), n, err
}

func (r *memoryRemote) MkcolAll(context.Context, string) error { return nil }

func (r *memoryRemote) Stat(_ context.Context, path string) (webdav.Stat, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, ok := r.files[path]
	if !ok {
		return webdav.Stat{}, webdav.ErrNotFound
	}
	return webdav.Stat{ETag: fmt.Sprint(r.versions[path]), Size: int64(len(data))}, nil
}

func (r *memoryRemote) Delete(_ context.Context, path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.files[path]; !ok {
		return webdav.ErrNotFound
	}
	delete(r.files, path)
	delete(r.versions, path)
	return nil
}

func newStore(t *testing.T, name string) *store.Store {
	t.Helper()
	st, err := store.NewWithPath(filepath.Join(t.TempDir(), name+".db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func addFeed(t *testing.T, st *store.Store, title string) {
	t.Helper()
	f := &store.Feed{
		URL:            "https://example.com/" + title,
		Title:          title,
		Status:         "active",
		UpdateInterval: 30,
		MaxItems:       100,
	}
	if err := st.CreateFeed(f); err != nil {
		t.Fatal(err)
	}
}

func TestBackupPublishesVersionedManifestAndAppliesRetention(t *testing.T) {
	st := newStore(t, "source")
	remote := newMemoryRemote()
	m := New(st)
	if err := m.SaveConfig(Config{Enabled: true, Interval: IntervalDaily, Retention: 3}); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		addFeed(t, st, fmt.Sprintf("feed-%d", i))
		m.now = func() time.Time { return base.Add(time.Duration(i) * time.Hour) }
		if _, err := m.Backup(context.Background(), remote); err != nil {
			t.Fatalf("backup %d: %v", i, err)
		}
	}

	list, err := m.List(context.Background(), remote)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("backups = %d, want 3", len(list))
	}
	if !list[0].CreatedAt.After(list[1].CreatedAt) {
		t.Error("backups not sorted newest first")
	}
	remote.mu.Lock()
	defer remote.mu.Unlock()
	if len(remote.files) != 4 { // 3 database files + manifest
		t.Errorf("remote files = %d, want 4", len(remote.files))
	}
}

func TestRestoreUsesHashAndStagesDatabase(t *testing.T) {
	source := newStore(t, "source")
	addFeed(t, source, "from-cloud")
	remote := newMemoryRemote()
	sourceManager := New(source)
	info, err := sourceManager.Backup(context.Background(), remote)
	if err != nil {
		t.Fatal(err)
	}

	targetPath := filepath.Join(t.TempDir(), "target.db")
	target, err := store.NewWithPath(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	addFeed(t, target, "local-old")
	cfg, _ := target.GetSettings()
	cfg.Theme = "dark"
	if err := target.UpdateSettings(cfg); err != nil {
		t.Fatal(err)
	}
	targetManager := New(target)
	result, err := targetManager.Restore(context.Background(), remote, info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !result.RestartRequired || result.RollbackPath == "" {
		t.Errorf("result = %+v", result)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(targetPath); err != nil {
		t.Fatalf("remove old database: %v", err)
	}
	if err := os.Rename(targetPath+".pending", targetPath); err != nil {
		t.Fatalf("apply staged restore: %v", err)
	}
	_ = os.Remove(targetPath + "-wal")
	_ = os.Remove(targetPath + "-shm")

	reopened, err := store.NewWithPath(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	feeds, err := reopened.ListFeeds()
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 1 || feeds[0].Title != "from-cloud" {
		t.Errorf("feeds = %+v", feeds)
	}
	settings, err := reopened.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Theme != "dark" {
		t.Errorf("theme = %q, want dark", settings.Theme)
	}
}

func TestRestoreRejectsTamperedSnapshot(t *testing.T) {
	st := newStore(t, "source")
	addFeed(t, st, "feed")
	remote := newMemoryRemote()
	m := New(st)
	info, err := m.Backup(context.Background(), remote)
	if err != nil {
		t.Fatal(err)
	}
	remote.mu.Lock()
	remote.files[info.File][0] ^= 0xff
	remote.mu.Unlock()

	_, err = m.Restore(context.Background(), remote, info.ID)
	if err == nil || !strings.Contains(err.Error(), "校验和") {
		t.Fatalf("err = %v, want checksum failure", err)
	}
}
