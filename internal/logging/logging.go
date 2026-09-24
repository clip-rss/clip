// Package logging 提供落盘的运行时日志。
//
// 它做三件事，共同修掉「生产环境没有任何可查日志」的黑洞：
//  1. 打开一个按大小轮转的日志文件 <os.UserConfigDir()>/clip/logs/clip.log；
//  2. 把标准库 log 包的全局输出重定向到该文件，让既有十几处 log.Printf/Println
//     零改动即落盘；
//  3. 暴露一个写同一文件的 *slog.Logger，注入 Wails 的 Options.Logger 后，
//     生产构建下原本被 io.Discard 丢弃的 app.Logger.* 也能留下痕迹。
//
// 开发构建（无 production 构建标签）同时把日志写到 stderr，保持终端可见的开发体验；
// 生产构建只写文件（GUI 应用里 stderr 不可见）。见 console_dev.go / console_prod.go。
package logging

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	dirName    = "logs"     // 位于 <configDir>/clip/ 下
	fileName   = "clip.log" // 当前日志文件名
	maxSize    = 5 << 20    // 单文件 5 MiB 后轮转
	maxBackups = 3          // 保留 clip.log.1 .. clip.log.3
)

var (
	// level 是运行时可调的日志级别，默认 Info；留作后续「排障时临时开 debug」的接口。
	level = new(slog.LevelVar)

	// logger 是写日志文件的 slog.Logger。Init 之前是一个丢弃日志的安全占位，
	// 使得任何早于 Init 的调用不至于 panic（尽管 Init 应当最先执行）。
	logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	mu     sync.Mutex
	writer *rotatingWriter
)

// Init 初始化文件日志并接管全局 log 输出。必须在 main() 最早处调用——
// 任何 log.Print* 可能触发之前——因为 log.SetOutput 是进程级的全局副作用。
//
// 失败时降级为写 stderr 并返回错误，绝不静默归零：静默归零会重演生产环境
// io.Discard 的黑洞，让排障者以为「有日志」却什么都拿不到。
func Init() error {
	mu.Lock()
	defer mu.Unlock()

	dir, err := Dir()
	if err != nil {
		fallback()
		return fmt.Errorf("logging: resolve log dir: %w", err)
	}

	rw, err := newRotatingWriter(filepath.Join(dir, fileName), maxSize, maxBackups)
	if err != nil {
		fallback()
		return fmt.Errorf("logging: open log file: %w", err)
	}
	writer = rw

	// dev 下用 MultiWriter 兼顾 stderr；prod 下 consoleWriter 为 nil，只写文件。
	var out io.Writer = rw
	if consoleWriter != nil {
		out = io.MultiWriter(rw, consoleWriter)
	}

	level.Set(slog.LevelInfo)
	logger = slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: level}))

	// 关键一步：既有十几处 log.Printf/Println 由此零改动落盘。
	log.SetOutput(out)
	return nil
}

// fallback 在文件日志初始化失败时把两条输出都退回 stderr。
func fallback() {
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	log.SetOutput(os.Stderr)
}

// Dir 返回日志目录 <configDir>/clip/logs，并确保其存在。
// 路径的唯一定义在此（同 store 对 clip.db 的处理），前端与 UI 不重复拼接。
func Dir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(configDir, "clip", dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Logger 返回写日志文件的 slog.Logger，用于注入 Wails Options.Logger。
func Logger() *slog.Logger { return logger }

// Close 关闭底层日志文件，供退出时优雅收尾。关闭后的写入被安全丢弃
// （rotatingWriter 内的 closed 守卫），不会 panic。
func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if writer == nil {
		return nil
	}
	return writer.Close()
}

// SetLevel 运行时调整日志级别（预留给后续的排障开关）。
func SetLevel(l slog.Level) { level.Set(l) }

// SessionHeader 记录一行会话头：版本 / 平台 / 配置目录 / 代理是否启用。
// 在设置加载完成后由 main 调用，为每份日志钉上排障所需的环境快照。
func SessionHeader(version, platform, configDir string, proxyEnabled bool) {
	logger.Info("session started",
		"version", version,
		"platform", platform,
		"configDir", configDir,
		"proxy", proxyEnabled,
	)
}

// FromFrontend 记录一条来自前端的运行时日志。
//
// level 取 debug/info/warn/error，其它值按 info 处理；scope 是来源（组件或 store），
// message 是内容。前端经 SystemService.Log 调到这里（见 Utils/Log.ts）。
func FromFrontend(level, scope, message string) {
	logger.Log(context.Background(), parseLevel(level), message,
		"src", "frontend",
		"scope", scope,
	)
}

// parseLevel 把前端传来的级别字符串解析为 slog.Level，未知值按 Info。
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
