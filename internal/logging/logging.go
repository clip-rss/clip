// Package logging 提供落盘的运行时日志。
//
// 每次启动创建一个独立会话日志文件 <os.UserConfigDir()>/clip/logs/clip-YYYY-MM-DD-HH-mm-ss.log。
// 目录里最多保留 maxLogFiles 个会话文件：启动时创建新文件后，超出的最旧文件会被删除。
// 每行日志统一为
//
//	2026-09-28T11:51:32.123+08:00 INFO  [app] main.go:620 application started
//
// 即「时间 + 级别 + 模块 + 来源 + 消息」：模块名标注来源（app/updater/scheduler/...），
// 来源是记这条日志的调用点 file.go:line。
//
// 这一包还顺手修掉生产环境的两个黑洞：
//  1. 把标准库 log 包的全局输出重定向到文件（经 lineTagger 补齐格式），既有
//     log.Printf/Println 零改动也能落盘；
//  2. 暴露一个写同一文件的 *slog.Logger，注入 Wails 的 Options.Logger 后，
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
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	dirName       = "logs"                // 位于 <configDir>/clip/ 下
	filePrefix    = "clip-"               // 会话日志文件名前缀
	logExt        = ".log"                // 会话日志文件扩展名
	sessionLayout = "2006-01-02-15-04-05" // 文件名里的时间戳：clip-YYYY-MM-DD-HH-mm-ss.log
	maxLogFiles   = 3                     // 目录里最多保留的会话日志数，超出的删最旧
)

var (
	// level 是运行时可调的日志级别，默认 Info；留作后续「排障时临时开 debug」的接口。
	level = new(slog.LevelVar)

	// logger 是写日志文件的 slog.Logger。Init 之前是一个丢弃日志的安全占位，
	// 使得任何早于 Init 的调用不至于 panic（尽管 Init 应当最先执行）。
	logger = slog.New(newFormatHandler(io.Discard, level))

	mu     sync.Mutex
	file   *os.File    // 当前会话日志文件，Close 时释放
	tagger *lineTagger // 标准库 log 兜底（仅 Init 成功后存在）
	stream io.Writer   // 最终写入流：文件，dev 下含 stderr
)

// Init 初始化会话日志并接管全局 log 输出。必须在 main() 最早处调用——
// 任何 log.Print* 可能触发之前——因为 log.SetOutput 是进程级的全局副作用。
//
// 每次启动创建新的 clip-<时间戳>.log，并把目录收敛回 maxLogFiles 个会话文件。
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

	path := newSessionPath(dir)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fallback()
		return fmt.Errorf("logging: open log file %s: %w", path, err)
	}
	file = f

	// 先建文件、再收敛到上限：新文件计入名额（它最新，永远不会被删），保证目录里
	// 包括本会话文件在内不超过 maxLogFiles 个。收敛失败不致命（只影响老日志清理），
	// 记到 stderr 后继续启动。
	if err := cleanupLogs(dir); err != nil {
		fmt.Fprintf(os.Stderr, "logging: cleanup logs: %v\n", err)
	}

	var out io.Writer = f
	if consoleWriter != nil {
		out = io.MultiWriter(f, consoleWriter)
	}
	stream = out

	tagger = newLineTagger(stream)
	level.Set(slog.LevelInfo)
	logger = slog.New(newFormatHandler(stream, level))

	// 标准库 log 接管：去掉它自带的日期时间（由 lineTagger 统一补前缀），但保留
	// Lshortfile——裸行本身拿不到调用点，靠它在行首写上 "main.go:559: "，lineTagger
	// 再把这截挪进来源位。log.Fatalf 的 osExit 分支同样会先经过这里把行写全。
	log.SetFlags(log.Lshortfile)
	log.SetOutput(tagger)
	return nil
}

// fallback 在文件日志初始化失败时把两条输出都退回 stderr。
func fallback() {
	stream = os.Stderr
	logger = slog.New(newFormatHandler(os.Stderr, level))
	log.SetFlags(log.Lshortfile)
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

// Close 冲刷残留半行并关闭当前会话日志文件，供退出时优雅收尾。关闭后的写入被安全
// 丢弃，不会 panic。
func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if file == nil {
		return nil
	}
	var err error
	if tagger != nil {
		err = tagger.Close()
	}
	if ferr := file.Close(); err == nil {
		err = ferr
	}
	file = nil
	return err
}

// SetLevel 运行时调整日志级别（预留给后续的排障开关）。
func SetLevel(l slog.Level) { level.Set(l) }

// SessionHeader 记录一行会话头：版本 / 平台 / 配置目录 / 代理是否启用。
// 在设置加载完成后由 main 调用，成为新会话日志文件的第一行。
func SessionHeader(version, platform, configDir string, proxyEnabled bool) {
	logFrom(slog.LevelInfo, "app", "application started",
		slog.String("version", version),
		slog.String("platform", platform),
		slog.String("configDir", configDir),
		slog.Bool("proxy", proxyEnabled),
	)
}

// FromFrontend 记录一条来自前端的运行时日志。
//
// level 取 debug/info/warn/error，其它值按 info 处理；scope 是来源（组件或 store），
// 直接作为行内的模块名（如 [settings/data]），message 是内容。前端经
// SystemService.Log 调到这里（见 Utils/Log.ts）。
//
// 来源位留空（source 传空串）：这行没有 Go 调用点，scope 已经指明了前端是哪个
// 组件或 store，写个 FromFrontend 只是噪音。
func FromFrontend(level, scope, message string) {
	if strings.TrimSpace(scope) == "" {
		scope = "frontend"
	}
	logAt(parseLevel(level), scope, "", message)
}

// 以下便捷函数带模块名：模块是行内方括号里的逻辑来源（app/updater/scheduler/...），
// 调用点由 logFrom 从调用栈还原。凡是能归属的调用点都应带模块，避免全部落在兜底的
// [app] 上。

// Printf 以 Info 级别记录一条格式化消息。
func Printf(module, format string, a ...any) {
	logFrom(slog.LevelInfo, module, fmt.Sprintf(format, a...))
}

// Println 以 Info 级别记录一条拼接消息（对齐 log.Println 的调用习惯）。
func Println(module string, a ...any) {
	logFrom(slog.LevelInfo, module, fmt.Sprint(a...))
}

// Warnf 以 Warn 级别记录一条格式化消息。
func Warnf(module, format string, a ...any) {
	logFrom(slog.LevelWarn, module, fmt.Sprintf(format, a...))
}

// Errorf 以 Error 级别记录一条格式化消息。
func Errorf(module, format string, a ...any) {
	logFrom(slog.LevelError, module, fmt.Sprintf(format, a...))
}

// Debugf 以 Debug 级别记录一条格式化消息（默认级别下不输出，排障时用 SetLevel 打开）。
func Debugf(module, format string, a ...any) {
	logFrom(slog.LevelDebug, module, fmt.Sprintf(format, a...))
}

// logFrom 记录一条来自当前调用点的日志：来源由调用栈还原，其余交给 logAt。
//
// 先挡一次级别再回溯栈：Debugf 这类被禁用的级别不该白付一次栈回溯的代价。
func logFrom(l slog.Level, module, msg string, attrs ...slog.Attr) {
	if !logger.Enabled(context.Background(), l) {
		return
	}
	logAt(l, module, callerSource(), msg, attrs...)
}

// logAt 记录一条日志。source 是调用点 "file.go:line"，空串表示没有 Go 调用点
// （前端桥接），由 formatHandler 决定省略来源位。
func logAt(l slog.Level, module, source, msg string, attrs ...slog.Attr) {
	logger.LogAttrs(context.Background(), l, msg, append([]slog.Attr{
		slog.String(moduleKey, module),
		slog.String(sourceKey, source),
	}, attrs...)...)
}

// pkgPrefix 是本包函数名的前缀（形如 .../internal/logging.），取自本包一个类型的
// PkgPath，免得在这里再抄一遍模块路径。跳帧只认这个前缀，模块改名不影响。
//
// 取类型而不是取 Printf 的函数名：后者会让初始化依赖成环
// （pkgPrefix → Printf → logFrom → callerSource → pkgPrefix）。
var pkgPrefix = reflect.TypeOf(formatHandler{}).PkgPath() + "."

// callerSource 返回首个本包之外的栈帧位置 "file.go:line"，即真正的调用点。
//
// 包装函数（Printf/Errorf → logFrom）会引入多级本包帧，取第一个调用者只会让每行都
// 标成 logging 包自己；因此这里逐帧跳过「函数名属于本包」的帧。按包名跳比数固定层数
// 稳：加一层包装、或者包装函数被内联，结果都不变。
func callerSource() string {
	var pcs [32]uintptr
	// skip [runtime.Callers, callerSource]
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		f, more := frames.Next()
		if f.File != "" && !strings.HasPrefix(f.Function, pkgPrefix) {
			return fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line)
		}
		if !more {
			return ""
		}
	}
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

// cleanupLogs 把日志目录收敛回上限：clip-*.log 只保留最新的 maxLogFiles 个，多余按
// 最旧删除，目录里的其它文件互不干预。
//
// 由 Init 在创建本会话文件之后调用，单实例应用下不存在正在写入的文件被误删。
func cleanupLogs(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	type sess struct {
		path string
		mod  time.Time
	}
	var sessions []sess

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		full := filepath.Join(dir, name)

		if !strings.HasPrefix(name, filePrefix) || !strings.HasSuffix(name, logExt) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, sess{path: full, mod: info.ModTime()})
	}

	if len(sessions) <= maxLogFiles {
		return nil
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].mod.After(sessions[j].mod) })
	for _, s := range sessions[maxLogFiles:] {
		if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// newSessionPath 返回本会话的日志路径 clip-YYYY-MM-DD-HH-mm-ss.log。
// 同一秒内多次启动（或目录里已有同名文件）时追加序号，避免两个会话写进同一个文件。
func newSessionPath(dir string) string {
	base := filePrefix + time.Now().Format(sessionLayout) + logExt
	p := filepath.Join(dir, base)
	for i := 2; ; i++ {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
		p = filepath.Join(dir, fmt.Sprintf("%s-%d%s",
			filePrefix+time.Now().Format(sessionLayout), i, logExt))
	}
}
