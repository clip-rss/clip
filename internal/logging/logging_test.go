package logging

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotatingWriterRotatesAndKeepsBackups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.log")
	w, err := newRotatingWriter(path, 100, 2)
	if err != nil {
		t.Fatalf("newRotatingWriter: %v", err)
	}
	line := []byte(strings.Repeat("x", 40) + "\n") // 41 字节
	for i := 0; i < 12; i++ {
		if _, err := w.Write(line); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// 保留 clip.log + clip.log.1 + clip.log.2，且不产生 .3（maxBackups=2）。
	for _, name := range []string{"clip.log", "clip.log.1", "clip.log.2"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to exist: %v", name, err)
		}
	}
	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Errorf("expected no clip.log.3, stat err=%v", err)
	}

	// 当前文件不应超过 maxSize（轮转后从空文件续写）。
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > 100 {
		t.Errorf("current log size %d exceeds maxSize 100", info.Size())
	}
}

func TestRotatingWriterAppendsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.log")
	if err := os.WriteFile(path, []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := newRotatingWriter(path, 1<<20, 3)
	if err != nil {
		t.Fatalf("newRotatingWriter: %v", err)
	}
	if _, err := w.Write([]byte("more\n")); err != nil {
		t.Fatal(err)
	}
	w.Close()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "existing\nmore\n"; string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestWriteAfterCloseFails(t *testing.T) {
	dir := t.TempDir()
	w, err := newRotatingWriter(filepath.Join(dir, "clip.log"), 1<<20, 3)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	if _, err := w.Write([]byte("x")); err == nil {
		t.Error("expected error writing after close")
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":    slog.LevelDebug,
		"INFO":     slog.LevelInfo,
		"warn":     slog.LevelWarn,
		"warning":  slog.LevelWarn,
		"error":    slog.LevelError,
		"":         slog.LevelInfo,
		"nonsense": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestFromFrontendWritesAttrs(t *testing.T) {
	// 把包级 logger 临时换成写内存缓冲的 handler，断言前端日志带上来源标记与内容。
	saved := logger
	t.Cleanup(func() { logger = saved })

	var buf bytes.Buffer
	logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: level}))

	FromFrontend("warn", "settings/data", "opml import failed")

	out := buf.String()
	for _, want := range []string{
		"level=WARN",
		"src=frontend",
		"scope=settings/data",
		`msg="opml import failed"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("frontend log %q missing %q", out, want)
		}
	}
}

