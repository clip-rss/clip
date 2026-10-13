package logging_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/clip-rss/clip/internal/logging"
)

// TestSourcePointsAtCallSite 端到端校验来源位：经 logging.Printf 写出的行，来源必须是
// 真正的调用点（本文件这一行），而不是被包装层顶替成 logging 包自己。
//
// 这个用例必须待在外部测试包 logging_test：callerSource 会跳过「函数名属于
// internal/logging 的帧」，而本包的函数名是 .../internal/logging_test.*，前缀对不上，
// 于是能像普通调用方一样被记进来源位。同目录下的包内测试（package logging）则会一路
// 跳到 testing 包那一帧，所以那边只能用 stripSource 抹掉来源再断言。
// tempLogsDir 把 os.UserConfigDir() 重定向到 t.TempDir() 下，并返回本次的日志目录。
//
// 三个平台读的变量不同：Windows 读 %AppData%，darwin 读 $HOME（再拼
// Library/Application Support），linux 读 $XDG_CONFIG_HOME、退到 $HOME/.config。
// 只设 AppData 在 macOS 上是死代码——os.UserConfigDir() 照旧返回真实配置目录，
// 测试便去读一个不存在、也不属于本次运行的路径。
func tempLogsDir(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	t.Setenv("AppData", base)
	t.Setenv("XDG_CONFIG_HOME", base)
	t.Setenv("HOME", base)
	cfg, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	return filepath.Join(cfg, "clip", "logs")
}

func TestSourcePointsAtCallSite(t *testing.T) {
	dir := tempLogsDir(t)

	if err := logging.Init(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := logging.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	_, file, line, _ := runtime.Caller(0)
	logging.Printf("test", "hello from the call site")
	want := fmt.Sprintf("%s:%d", filepath.Base(file), line+1) // 紧贴上面那行

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var logFile string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "clip-") && strings.HasSuffix(e.Name(), ".log") {
			logFile = filepath.Join(dir, e.Name())
		}
	}
	if logFile == "" {
		t.Fatalf("no session log file in %s", dir)
	}
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	got := lines[len(lines)-1]
	if src := sourceFieldOf(got); src != want {
		t.Errorf("source = %q, want %q (line: %q)", src, want, got)
	}
	if !strings.HasSuffix(got, "hello from the call site") {
		t.Errorf("line = %q, want the message from the call site", got)
	}
}

// sourceFieldOf 取出一行里的来源字段（紧跟在 [模块] 之后）。
func sourceFieldOf(line string) string {
	i := strings.Index(line, "] ")
	if i < 0 {
		return ""
	}
	field, _, _ := strings.Cut(line[i+2:], " ")
	return field
}
