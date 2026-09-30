package logging

import (
	"bytes"
	"io"
	"regexp"
	"sync"
	"time"
)

// lineTagger 兜住那些没走 slog 的日志：经 log.SetOutput 接进流后，任何裸行
// （第三方库、遗漏的 log.Printf、log.Fatalf）都会被补上统一前缀
//
//	2026-09-28T11:51:39.000+08:00 INFO  [app] main.go:559 failed to init store: …
//
// 保证日志目录里每一行都符合「时间 + 级别 + 模块 + 来源 + 消息」的不变式。这类行拿不到
// 级别与模块信息，一律按 INFO / app 处理——自己的调用点都已迁移到带模块的
// logging.*f，这里只兜住迁移遗漏与第三方输出。
//
// 来源靠标准库 log 的 Lshortfile 提供（见 Init 里的 log.SetFlags）：它写在行首形如
// "main.go:559: "，这里负责把这截挪到来源位，而不是让它留在消息里。
type lineTagger struct {
	mu  sync.Mutex // 保护半行缓冲，标准库 log 可能被多 goroutine 调
	out io.Writer
	buf []byte // 跨 Write 的未完结半行
}

// shortFilePrefix 匹配标准库 log 在 Lshortfile 下写在行首的 "file.go:123: "。
// 只可能出现在行首，且只包住第一行（多行消息的后续行没有前缀）。
var shortFilePrefix = regexp.MustCompile(`^([^\s:]+\.go:\d+): `)

func newLineTagger(out io.Writer) *lineTagger {
	return &lineTagger{out: out}
}

func (t *lineTagger) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.buf = append(t.buf, p...)
	for {
		n := bytes.IndexByte(t.buf, '\n')
		if n < 0 {
			break
		}
		line := t.buf[:n+1]
		t.buf = t.buf[n+1:]
		if err := t.writePrefixed(line); err != nil {
			return len(p), err
		}
	}
	return len(p), nil
}

func (t *lineTagger) writePrefixed(line []byte) error {
	src, rest := "", line
	if m := shortFilePrefix.FindSubmatchIndex(line); m != nil {
		src = string(line[m[2]:m[3]])
		rest = line[m[1]:]
	}

	out := make([]byte, 0, len(line)+48)
	out = append(out, time.Now().Format(timeFormat)...)
	out = append(out, " INFO  [app] "...)
	if src != "" {
		out = append(out, src...)
		out = append(out, ' ')
	}
	out = append(out, rest...)
	_, err := t.out.Write(out)
	return err
}

// Close 冲刷残留的半行（进程正常退出时把最后一行没换行的日志也写全）。
func (t *lineTagger) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.buf) == 0 {
		return nil
	}
	line := append([]byte(nil), t.buf...)
	line = append(line, '\n')
	t.buf = t.buf[:0]
	return t.writePrefixed(line)
}
