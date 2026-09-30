package logging

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

const (
	// timeFormat 行内时间戳：毫秒精度 + 本地时区偏移，如 2026-09-28T11:51:32.123+08:00。
	timeFormat = "2006-01-02T15:04:05.000-07:00"

	moduleKey     = "module" // 记录上携带模块名的属性键
	sourceKey     = "source" // 记录上携带调用点（file.go:line）的属性键
	defaultModule = "app"    // 未携带模块时的兜底
)

// formatHandler 把每条 slog 记录格式化成统一的一行：
//
//	2026-09-28T11:51:32.123+08:00 INFO  [app] main.go:412 application started version=0.7.0
//
// 五要素齐备：时间、左对齐补宽到 6 的级别、方括号里的模块（取自记录上的 module 属性，
// 缺省 app）、调用来源 file.go:line、消息。更多属性经 key=value 追加在行尾。它替换 slog
// 默认的 TextHandler——后者的输出不利于直接阅读与 grep。
type formatHandler struct {
	// mu 用指针而非值：With* 会复制 handler，复制语句不得拷贝含锁的值，也让派生
	// handler 与父级共享同一把锁（它们写同一个 out，本就该串行化）。
	mu     *sync.Mutex  // 串行化整行构建，避免并发记录交错写坏行
	level  slog.Leveler // 运行时可调的级别（slog.LevelVar）
	out    io.Writer
	groups []string    // WithGroup 路径，用于点号拼接属性键
	attrs  []slog.Attr // With 携带的预置属性
}

func newFormatHandler(out io.Writer, level slog.Leveler) *formatHandler {
	return &formatHandler{out: out, level: level, mu: new(sync.Mutex)}
}

func (h *formatHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

func (h *formatHandler) clone() *formatHandler {
	c := *h
	c.groups = append([]string(nil), h.groups...)
	c.attrs = append([]slog.Attr(nil), h.attrs...)
	return &c
}

func (h *formatHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := h.clone()
	c.attrs = append(c.attrs, attrs...)
	return c
}

func (h *formatHandler) WithGroup(name string) slog.Handler {
	c := h.clone()
	c.groups = append(c.groups, name)
	return c
}

func (h *formatHandler) Handle(_ context.Context, r slog.Record) error {
	var buf bytes.Buffer

	buf.WriteString(r.Time.Format(timeFormat))
	buf.WriteByte(' ')
	buf.WriteString(fmt.Sprintf("%-5s", levelName(r.Level)))
	buf.WriteByte(' ')
	buf.WriteByte('[')
	buf.WriteString(h.moduleOf(r))
	buf.WriteString("]")
	// 来源紧跟模块：`[updater] main.go:412 checking for updates`。空值表示这行没有
	// Go 调用点（前端桥接），此时连同分隔空格一起省掉。
	if src := h.sourceOf(r); src != "" {
		buf.WriteByte(' ')
		buf.WriteString(src)
	}
	if r.Message != "" {
		buf.WriteByte(' ')
		buf.WriteString(r.Message)
	}

	for _, a := range h.attrs {
		appendAttr(&buf, h.groups, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		appendAttr(&buf, h.groups, a)
		return true
	})
	buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(buf.Bytes()) // 单次 Write：整行原子落盘
	return err
}

// levelName 把 slog 级别收敛为 5 字符大写（DEBUG/INFO/WARN/ERROR），供 %-6s 对齐。
func levelName(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return "DEBUG"
	case l < slog.LevelWarn:
		return "INFO"
	case l < slog.LevelError:
		return "WARN"
	default:
		return "ERROR"
	}
}

// moduleOf 从预置属性或记录属性中取模块名，缺省 app。
func (h *formatHandler) moduleOf(r slog.Record) string {
	for _, a := range h.attrs {
		if a.Key == moduleKey {
			return a.Value.String()
		}
	}
	var mod string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == moduleKey {
			mod = a.Value.String()
			return false
		}
		return true
	})
	if mod == "" {
		return defaultModule
	}
	return mod
}

// sourceOf 返回该行的调用来源（"main.go:412"），用于标识这条日志是在哪儿记的。
//
// 优先取记录上显式的 source 属性：包装函数在真正的调用点算好放进来（slog 记录的
// Record.PC 只指向「直接调用者」，多一层包装就丢了调用点，见 logging.go 的 callerSource）。
// 没有该属性时按 Record.PC 还原，覆盖直接调 logger.* 的调用方（如 Wails 内部）。
// 空串表示没有 Go 调用点，渲染时省略来源位。
func (h *formatHandler) sourceOf(r slog.Record) string {
	if s, ok := findAttr(h.attrs, sourceKey); ok {
		return s
	}
	if s, ok := findRecordAttr(r, sourceKey); ok {
		return s
	}
	return resolveSource(r.PC)
}

// findAttr 在预置属性（With 携带）里找字符串属性。
func findAttr(attrs []slog.Attr, key string) (string, bool) {
	for _, a := range attrs {
		if a.Key == key && a.Value.Kind() == slog.KindString {
			return a.Value.String(), true
		}
	}
	return "", false
}

// findRecordAttr 在记录自身的属性里找字符串属性。
func findRecordAttr(r slog.Record, key string) (string, bool) {
	var (
		val string
		ok  bool
	)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key && a.Value.Kind() == slog.KindString {
			val, ok = a.Value.String(), true
			return false
		}
		return true
	})
	return val, ok
}

// resolveSource 把调用方 PC 还原成 "file.go:line"。只取文件名：完整路径因机器而异，
// 对定位问题没有额外价值，反而让日志行变得很长。
func resolveSource(pc uintptr) string {
	if pc == 0 {
		return ""
	}
	frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	if frame.File == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", filepath.Base(frame.File), frame.Line)
}

// isMeta 判断属性是否为结构性字段（module / source）：两者已经渲染在行首的固定位置，
// 不再作为 key=value 出现在行尾。
func isMeta(a slog.Attr) bool {
	return (a.Key == moduleKey || a.Key == sourceKey) && a.Value.Kind() == slog.KindString
}

// appendAttr 把一个属性（或其组内子属性）追加进缓冲区，样式为 ` key=value`。
// 无键的组被展平；带键的组把键并入属性键前缀。
func appendAttr(buf *bytes.Buffer, groups []string, a slog.Attr) {
	if a.Key == "" && !isGroup(a) {
		return
	}
	if isMeta(a) {
		return
	}
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		if a.Key != "" {
			groups = append(append([]string(nil), groups...), a.Key)
		}
		for _, sub := range v.Group() {
			appendAttr(buf, groups, sub)
		}
		return
	}
	buf.WriteByte(' ')
	buf.WriteString(joinKeys(groups, a.Key))
	buf.WriteByte('=')
	buf.WriteString(quoteValue(v))
}

func isGroup(a slog.Attr) bool {
	return a.Value.Resolve().Kind() == slog.KindGroup
}

func joinKeys(groups []string, key string) string {
	if len(groups) == 0 {
		return key
	}
	return strings.Join(append(append([]string(nil), groups...), key), ".")
}

// quoteValue 把属性值渲染为字符串：常规值原样输出，含空白/引号/等号等特殊字符时转成
// Go 引号字符串，保证整行仍是可读的 key=value 列表。
func quoteValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		s := v.String()
		if s != "" && !needQuote(s) {
			return s
		}
		return strconv.Quote(s)
	case slog.KindBool:
		return strconv.FormatBool(v.Bool())
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().Format(timeFormat)
	case slog.KindInt64:
		return strconv.FormatInt(v.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(v.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(v.Float64(), 'g', -1, 64)
	default:
		return fmt.Sprintf("%v", v.Any())
	}
}

// needQuote 判断字符串是否需要引号包裹。
func needQuote(s string) bool {
	for _, r := range s {
		if r == '"' || r == '=' || unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return true
		}
	}
	return false
}
