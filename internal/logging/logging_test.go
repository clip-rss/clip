package logging

import (
	"bytes"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// stripTs 剥掉一行的固定长度时间前缀（29 字节时间 + 1 空格）。
func stripTs(line string) string {
	if len(line) < 30 {
		return line
	}
	return line[30:]
}

// sourceFieldRe 匹配行内的来源字段（file.go:line）。
var sourceFieldRe = regexp.MustCompile(`^\S+\.go:\d+$`)

// stripSource 抹掉行内的来源字段，供只关心「级别 / 模块 / 消息 / 属性」布局的断言使用。
//
// 本包测试里经 logging.Printf 这类包装函数写出的来源，必然是「testing 包」的那一帧：
// 包装层与本包的测试函数都在 internal/logging 内，被 callerSource 按包名跳过，于是落在
// testing.go:NNN 上——行号随 Go 版本浮动，断言时只能抹掉。来源本身由 source_test.go
// （外部测试包，不在被跳过的包内）端到端校验。
func stripSource(line string) string {
	i := strings.Index(line, "] ")
	if i < 0 {
		return line
	}
	head, rest := line[:i+2], line[i+2:]
	field, tail, ok := strings.Cut(rest, " ")
	if ok && sourceFieldRe.MatchString(field) {
		return head + tail
	}
	return line
}

// TestFormatHandler 逐级验证行格式：时间 + 级别（补宽 6）+ [模块] + 消息 + 属性。
func TestFormatHandler(t *testing.T) {
	cases := []struct {
		name string
		log  func(l *slog.Logger)
		want string // 去掉行首时间戳后的期望剩余部分
	}{
		{
			"info with module and attrs",
			func(l *slog.Logger) { l.Info("application started", moduleKey, "app", "version", "0.7.0") },
			"INFO  [app] application started version=0.7.0\n",
		},
		{
			"warn pads to 6",
			func(l *slog.Logger) { l.Warn("request timeout, retrying", moduleKey, "updater") },
			"WARN  [updater] request timeout, retrying\n",
		},
		{
			"error width 5",
			func(l *slog.Logger) { l.Error("failed", moduleKey, "updater", "err", "context deadline exceeded") },
			`ERROR [updater] failed err="context deadline exceeded"` + "\n",
		},
		{
			"module defaults to app",
			func(l *slog.Logger) { l.Info("plain line") },
			"INFO  [app] plain line\n",
		},
		{
			"empty source (frontend) omits the field",
			func(l *slog.Logger) {
				l.Info("opml import failed", moduleKey, "settings/data", sourceKey, "")
			},
			"INFO  [settings/data] opml import failed\n",
		},
		{
			"messages keep inner spaces unquoted",
			func(l *slog.Logger) { l.Warn("feed 12 refresh failed: boom", moduleKey, "scheduler") },
			"WARN  [scheduler] feed 12 refresh failed: boom\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			tc.log(slog.New(newFormatHandler(&buf, level)))

			got := stripTs(stripSource(buf.String()))
			if got != tc.want {
				t.Errorf("line = %q, want %q", got, tc.want)
			}
			// 时间戳必须是毫秒 + 本地偏移的 ISO 形如 2026-09-28T11:51:32.123+08:00。
			prefix := buf.String()[:30]
			re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}[+-]\d{2}:\d{2} $`)
			if !re.MatchString(prefix) {
				t.Errorf("timestamp prefix %q doesn't match schema", prefix)
			}
		})
	}
}

// TestSourceAttrRendersBetweenModuleAndMessage 钉住来源位的契约：source 属性渲染在
// [模块] 与消息之间，且不会作为 key=value 掉到行尾。这里不抹来源——要断言的就是它。
func TestSourceAttrRendersBetweenModuleAndMessage(t *testing.T) {
	var buf bytes.Buffer
	slog.New(newFormatHandler(&buf, level)).Info(
		"checking for updates", moduleKey, "updater", sourceKey, "main.go:412")

	if got := stripTs(buf.String()); got != "INFO  [updater] main.go:412 checking for updates\n" {
		t.Errorf("line = %q", got)
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

func TestFromFrontendUsesScopeAsModule(t *testing.T) {
	saved := logger
	t.Cleanup(func() { logger = saved })

	var buf bytes.Buffer
	logger = slog.New(newFormatHandler(&buf, level))

	FromFrontend("warn", "settings/data", "opml import failed")

	// 精确匹配同时钉住了「前端行不带来源位」：logAt 收到空 source，行内就没有这一段。
	if got := stripTs(buf.String()); got != "WARN  [settings/data] opml import failed\n" {
		t.Errorf("frontend line = %q", got)
	}
}

func TestModuleHelpers(t *testing.T) {
	saved := logger
	t.Cleanup(func() { logger = saved })

	var buf bytes.Buffer
	logger = slog.New(newFormatHandler(&buf, level))

	Printf("scheduler", "feed %d refresh failed", 7)
	Errorf("updater", "check failed: %v", "timeout")

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines: %q", len(lines), buf.String())
	}
	if got := stripTs(stripSource(lines[0])); got != "INFO  [scheduler] feed 7 refresh failed" {
		t.Errorf("Printf line = %q", got)
	}
	if got := stripTs(stripSource(lines[1])); got != "ERROR [updater] check failed: timeout" {
		t.Errorf("Errorf line = %q", got)
	}
}

func TestCleanupLogs(t *testing.T) {
	dir := t.TempDir()

	// 伪造 5 个会话文件，mtime 从旧到新。
	sessions := []string{
		"clip-2026-09-20-10-00-00.log",
		"clip-2026-09-22-10-00-00.log",
		"clip-2026-09-24-10-00-00.log",
		"clip-2026-09-26-10-00-00.log",
		"clip-2026-09-28-10-00-00.log",
	}
	for i, n := range sessions {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("line\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		mt := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC).Add(time.Duration(i*2) * 24 * time.Hour)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	// 与日志无关的文件。
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cleanupLogs(dir); err != nil {
		t.Fatal(err)
	}

	// 会话文件只保留最新 3 个。
	for _, n := range []string{"clip-2026-09-20-10-00-00.log", "clip-2026-09-22-10-00-00.log"} {
		if _, err := os.Stat(filepath.Join(dir, n)); !os.IsNotExist(err) {
			t.Errorf("old session %s should be removed", n)
		}
	}
	for _, n := range []string{
		"clip-2026-09-24-10-00-00.log",
		"clip-2026-09-26-10-00-00.log",
		"clip-2026-09-28-10-00-00.log",
	} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("session %s should be kept: %v", n, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Errorf("unrelated file should survive: %v", err)
	}
}

func TestCleanupLogsKeepsAllWhenUnderLimit(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"clip-2026-09-27-10-00-00.log", "clip-2026-09-28-10-00-00.log"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := cleanupLogs(dir); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"clip-2026-09-27-10-00-00.log", "clip-2026-09-28-10-00-00.log"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("session %s should be kept when under limit: %v", n, err)
		}
	}
}

func TestNewSessionPathAvoidsCollision(t *testing.T) {
	dir := t.TempDir()
	first := newSessionPath(dir)
	if !strings.HasPrefix(filepath.Base(first), filePrefix) || !strings.HasSuffix(filepath.Base(first), logExt) {
		t.Fatalf("session path %q doesn't match clip-?.log", first)
	}
	// 已存在同名文件时，下一次应返回带序号的另一条路径。
	if err := os.WriteFile(first, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := newSessionPath(dir)
	if second == first {
		t.Fatalf("collision not avoided: %q reused", first)
	}
}

func TestLineTaggerPrefixesRawLines(t *testing.T) {
	var buf bytes.Buffer
	tg := newLineTagger(&buf)

	// 裸行跨多次 Write 也要逐行补前缀；未完结的半行由 Close 冲刷。
	if _, err := tg.Write([]byte("check failed\nstill pending")); err != nil {
		t.Fatal(err)
	}
	if _, err := tg.Write([]byte(" data\n")); err != nil {
		t.Fatal(err)
	}
	if err := tg.Close(); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines: %q", len(lines), buf.String())
	}
	re := regexp.MustCompile(`^[0-9T:.\-+]+ INFO  \[app\] `)
	for i, want := range []string{"check failed", "still pending data"} {
		if !re.MatchString(lines[i]) || !strings.HasSuffix(stripTs(lines[i]), want) {
			t.Errorf("line %d = %q, want prefixed %q", i, lines[i], want)
		}
	}
}

// TestLineTaggerMovesShortFilePrefix 验证标准库 log 在 Lshortfile 下写的行首
// "main.go:559: " 被挪进来源位，而不是留在消息里。
func TestLineTaggerMovesShortFilePrefix(t *testing.T) {
	var buf bytes.Buffer
	tg := newLineTagger(&buf)

	// 第一行带前缀、第二行不带（多行消息的后续行没有前缀），第三行压根不是 Go 文件。
	if _, err := tg.Write([]byte("main.go:559: failed to init store: boom\nplain line\nC:\\a:b weird\n")); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	want := []string{
		"INFO  [app] main.go:559 failed to init store: boom",
		"INFO  [app] plain line",
		"INFO  [app] C:\\a:b weird",
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines: %q", len(lines), buf.String())
	}
	for i, w := range want {
		if got := stripTs(lines[i]); got != w {
			t.Errorf("line %d = %q, want %q", i, got, w)
		}
	}
}

// TestInitEndToEnd 用临时 AppData 跑真实 Init 流程：连续多次「启动 + 关闭」后目录始终
// 不超过 maxLogFiles 个会话文件，且每份文件都写入符合行格式的会话头。这里刻意不做
// 文件名的顺序断言——同秒内的快速重启会复用带序号的命名，保留逻辑看的是 mtime。
func TestInitEndToEnd(t *testing.T) {
	config := t.TempDir()
	t.Setenv("AppData", config)
	dir := filepath.Join(config, "clip", dirName)

	sessionFiles := func() []string {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasPrefix(e.Name(), filePrefix) && strings.HasSuffix(e.Name(), logExt) {
				names = append(names, e.Name())
			}
		}
		return names
	}

	for i := 0; i < 5; i++ {
		if err := Init(); err != nil {
			t.Fatalf("Init #%d: %v", i+1, err)
		}
		SessionHeader("0.7.0", "windows", `C:\cfg`, false)
		log.Print("raw std log line")
		if err := Close(); err != nil {
			t.Fatalf("Close #%d: %v", i+1, err)
		}
		if n := len(sessionFiles()); n > maxLogFiles {
			t.Fatalf("start #%d: %d session files, want <= %d", i+1, n, maxLogFiles)
		}
	}
	if n := len(sessionFiles()); n != maxLogFiles {
		t.Fatalf("after 5 starts: %d session files, want %d", n, maxLogFiles)
	}

	// 最新一份文件的第一行是会话头，四要素齐备。
	files := sessionFiles()
	var newest string
	var newestMod time.Time
	for _, name := range files {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.ModTime().After(newestMod) {
			newestMod = info.ModTime()
			newest = filepath.Join(dir, name)
		}
	}
	data, err := os.ReadFile(newest)
	if err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)[0]
	if got := stripTs(stripSource(first)); got != "INFO  [app] application started version=0.7.0 platform=windows configDir=C:\\cfg proxy=false" {
		t.Errorf("session header = %q", got)
	}

	// 第二行是 log.Print 的裸行：来源必须落在本文件上，即 log.Lshortfile 写在行首的
	// "logging_test.go:NN: " 真被 lineTagger 挪进了来源位（这条不经本包包装函数，来源
	// 由标准库自己算出，所以能精确到本文件的调用行）。
	raw := strings.SplitN(strings.TrimSpace(string(data)), "\n", 3)[1]
	rawRe := regexp.MustCompile(`^INFO  \[app\] logging_test\.go:\d+ raw std log line$`)
	if !rawRe.MatchString(stripTs(raw)) {
		t.Errorf("raw std log line = %q", stripTs(raw))
	}
}

func TestLineTaggerFlushesTailOnClose(t *testing.T) {
	var buf bytes.Buffer
	tg := newLineTagger(&buf)
	if _, err := tg.Write([]byte("dangling")); err != nil {
		t.Fatal(err)
	}
	if err := tg.Close(); err != nil {
		t.Fatal(err)
	}
	if got := stripTs(buf.String()); got != "INFO  [app] dangling\n" {
		t.Errorf("tail line = %q", got)
	}
}
