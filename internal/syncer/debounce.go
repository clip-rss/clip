package syncer

import (
	"sync"
	"time"
)

// Debouncer 把一串密集的触发合并成末次触发之后延迟一次的调用。
//
// 用途：用户在设置页连着调五六项，每一项都会触发一次推送意图。逐个上传既慢
// 也浪费网盘的请求配额（坚果云对频率有限制），而中间那几个版本没人关心 ——
// 只有最后那份配置需要出现在远端。
//
// 每次 Trigger 重置计时，因此持续操作期间不会推送，停手 delay 之后推一次。
type Debouncer struct {
	delay time.Duration
	fn    func()

	mu      sync.Mutex
	timer   *time.Timer
	stopped bool
}

// NewDebouncer 创建去抖器，fn 在最后一次 Trigger 之后 delay 时执行。
// fn 在独立的 goroutine 中运行（time.AfterFunc 的语义），不阻塞 Trigger 的调用方。
func NewDebouncer(delay time.Duration, fn func()) *Debouncer {
	return &Debouncer{delay: delay, fn: fn}
}

// Trigger 安排一次延迟执行；已有待执行的安排则重新计时。
// Stop 之后调用无效果。
func (d *Debouncer) Trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return
	}
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.delay, d.fn)
}

// Stop 取消尚未执行的安排，并让后续 Trigger 不再生效。应用退出时调用。
//
// ⚠️ 不等待已经开始执行的 fn。调用方若需要「退出前一定推送完」，
// 那是另一回事，得在关闭流程里显式同步调用一次 Sync。
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopped = true
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}
