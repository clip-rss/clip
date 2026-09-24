package logging

import (
	"fmt"
	"os"
	"sync"
)

// rotatingWriter 是一个按大小轮转的 io.Writer：单文件超过 maxSize 后，
// 现有文件依次改名为 .1/.2/…（保留 maxBackups 代），再从空文件继续写。
//
// slog 的 TextHandler 与标准库 log 都是每条记录一次 Write，因此轮转以整条记录为
// 边界，不会写出被截断的半条记录。所有方法用同一把锁串行化，可安全并发写
// （调度器会并发抓取，日志来自多个 goroutine）。
type rotatingWriter struct {
	mu         sync.Mutex
	path       string
	maxSize    int64
	maxBackups int

	f      *os.File
	size   int64
	closed bool
}

// newRotatingWriter 打开（或按 append 续写）path，返回可用的写入器。
func newRotatingWriter(path string, maxSize int64, maxBackups int) (*rotatingWriter, error) {
	w := &rotatingWriter{path: path, maxSize: maxSize, maxBackups: maxBackups}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

// open 以追加模式打开当前日志文件并读取其现有大小（续写已有日志）。
func (w *rotatingWriter) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.f = f
	w.size = info.Size()
	return nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, os.ErrClosed
	}
	// 仅在文件已有内容时轮转，避免单条超大记录把空文件也切走（只会徒增空文件）。
	if w.size > 0 && w.size+int64(len(p)) > w.maxSize {
		w.rotate() // best-effort：失败时保持当前文件继续写，不丢日志
	}
	if w.f == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

// rotate 关闭当前文件，把 .(N-1) → .N 依次改名（丢弃最老一代），
// 再把当前文件改名为 .1，最后重开一个空的当前文件。尽力而为：任一步失败都不 panic。
func (w *rotatingWriter) rotate() {
	if w.f != nil {
		w.f.Close()
		w.f = nil
	}
	_ = os.Remove(fmt.Sprintf("%s.%d", w.path, w.maxBackups))
	for i := w.maxBackups - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", w.path, i), fmt.Sprintf("%s.%d", w.path, i+1))
	}
	_ = os.Rename(w.path, w.path+".1")
	w.size = 0
	_ = w.open() // 失败时 w.f 仍为 nil，Write 里会再兜底重开
}

// Close 关闭底层文件。关闭后再写返回 os.ErrClosed（主要供测试释放句柄；
// Windows 下不关闭无法删除文件）。
func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}
