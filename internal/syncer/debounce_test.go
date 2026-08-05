package syncer

import (
	"sync/atomic"
	"testing"
	"time"
)

// 测试用的短延时。真实取值是 PushDebounce（8s），测试里不等那么久。
const testDelay = 30 * time.Millisecond

// TestDebouncerCoalescesBurst 连续触发只执行一次 —— 用户在设置页连调五六项时，
// 中间那几个版本没人关心，只有最后那份配置需要出现在远端。
func TestDebouncerCoalescesBurst(t *testing.T) {
	var calls atomic.Int32
	done := make(chan struct{}, 1)
	d := NewDebouncer(testDelay, func() {
		calls.Add(1)
		select {
		case done <- struct{}{}:
		default:
		}
	})
	defer d.Stop()

	for range 5 {
		d.Trigger()
		time.Sleep(testDelay / 5) // 间隔短于延时，应持续重新计时
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("去抖函数始终未执行")
	}
	// 再等一段，确认没有第二次执行。
	time.Sleep(3 * testDelay)
	if got := calls.Load(); got != 1 {
		t.Errorf("执行 %d 次, want 1", got)
	}
}

// TestDebouncerRunsAgainAfterFiring 一次执行后仍可再次安排，
// 否则应用只会推送第一次改动。
func TestDebouncerRunsAgainAfterFiring(t *testing.T) {
	var calls atomic.Int32
	fired := make(chan struct{}, 4)
	d := NewDebouncer(testDelay, func() {
		calls.Add(1)
		fired <- struct{}{}
	})
	defer d.Stop()

	for i := range 2 {
		d.Trigger()
		select {
		case <-fired:
		case <-time.After(2 * time.Second):
			t.Fatalf("第 %d 轮未执行", i+1)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("执行 %d 次, want 2", got)
	}
}

// TestDebouncerStopCancelsPending Stop 必须取消尚未执行的安排。
// 应用退出后仍触发一次网络推送，会在关闭流程里访问已释放的资源。
func TestDebouncerStopCancelsPending(t *testing.T) {
	var calls atomic.Int32
	d := NewDebouncer(testDelay, func() { calls.Add(1) })

	d.Trigger()
	d.Stop()
	time.Sleep(4 * testDelay)

	if got := calls.Load(); got != 0 {
		t.Errorf("Stop 后仍执行了 %d 次", got)
	}
}

// TestDebouncerTriggerAfterStopIsNoop Stop 之后的触发不得复活定时器。
// 关闭流程里保存设置是很常见的，那会顺手触发一次推送。
func TestDebouncerTriggerAfterStopIsNoop(t *testing.T) {
	var calls atomic.Int32
	d := NewDebouncer(testDelay, func() { calls.Add(1) })

	d.Stop()
	d.Trigger()
	time.Sleep(4 * testDelay)

	if got := calls.Load(); got != 0 {
		t.Errorf("Stop 后的 Trigger 执行了 %d 次", got)
	}
}

func TestDebouncerStopIsIdempotent(t *testing.T) {
	d := NewDebouncer(testDelay, func() {})
	d.Trigger()
	d.Stop()
	d.Stop() // 不应 panic
}

// TestDebouncerConcurrentTriggers 并发触发不得竞争。用 -race 跑。
func TestDebouncerConcurrentTriggers(t *testing.T) {
	var calls atomic.Int32
	d := NewDebouncer(testDelay, func() { calls.Add(1) })
	defer d.Stop()

	done := make(chan struct{})
	for range 16 {
		go func() {
			for range 20 {
				d.Trigger()
			}
			done <- struct{}{}
		}()
	}
	for range 16 {
		<-done
	}

	// 全部触发都在延时窗口内完成，最终只应执行一次。
	time.Sleep(6 * testDelay)
	if got := calls.Load(); got != 1 {
		t.Errorf("执行 %d 次, want 1", got)
	}
}

// TestPushDebounceIsWithinPlannedRange 计划要求 5–10 秒：太短会撞上坚果云的
// 频率限制，太长则用户切完设置迟迟看不到同步。
func TestPushDebounceIsWithinPlannedRange(t *testing.T) {
	if PushDebounce < 5*time.Second || PushDebounce > 10*time.Second {
		t.Errorf("PushDebounce = %v，应在 5–10s 之间", PushDebounce)
	}
	if StartupDelay <= 0 || StartupDelay > 30*time.Second {
		t.Errorf("StartupDelay = %v，应为一个不阻塞启动的小正值", StartupDelay)
	}
}
