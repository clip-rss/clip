package syncer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/clip-rss/clip/internal/store"
	"github.com/clip-rss/clip/internal/webdav"
)

// 真实客户端必须满足 Remote 接口。这行断言是接口与 webdav 包之间的唯一约束点：
// 少了它，webdav.Client 改签名后本包依然编译通过，直到 api 层组装时才炸。
var _ Remote = (*webdav.Client)(nil)

/* ---------- 假远端 ---------- */

// fakeRemote 内存版远端。冲突判定是本阶段最易出错的部分，用假远端把每种
// 组合钉死，比连真服务器可靠也快得多（真机验证是阶段 G 的事）。
type fakeRemote struct {
	mu sync.Mutex

	exists bool
	data   []byte
	etag   string
	dirs   map[string]bool

	// 注入的故障
	getErr      error // 非 nil 时 Get 直接返回它
	putErr      error // 非 nil 时 Put 直接返回它
	mkcolErr    error // 非 nil 时 MkcolAll 直接返回它
	needsDir    bool  // 目录未建时 Put 报 missingDirErr
	strictMatch bool  // 严格校验 If-Match，不符回 412
	noPutETag   bool  // 模拟 PUT 响应不带 ETag 的服务器

	// missingDirErr 目录不存在时 Put 返回的错误。各家服务器不一致：
	// 有的回 409（父集合不存在），有的直接回 404。零值按 409 处理。
	missingDirErr error

	// beforePut 在 Put 真正执行前触发一次（触发后自动清空）。
	//
	// 用来制造「GET 之后、PUT 之前远端被另一台机器改写」这个竞态窗口 ——
	// If-Match 要挡的正是它。从外部改 ETag 是复现不出来的：推送带的
	// ifMatch 取自同一次同步刚做的 GET，提前改只会让 GET 读到新值。
	beforePut func()

	// 调用记录
	gets, puts, mkcols int
	lastIfMatch        string
	etagSeq            int

	// 并发观测：记录同时在途的请求数峰值。
	// 用它检查同步是否真的串行 —— 交错本身不产生内存竞争（共享状态都在
	// SQLite 里过），-race 看不见，只能直接观测在途请求。
	inFlight    atomic.Int32
	maxInFlight atomic.Int32
	traceDelay  time.Duration // 非零时在请求中停留一会儿，放大交错窗口
}

// enter / leave 记录在途请求峰值。
func (f *fakeRemote) enter() {
	n := f.inFlight.Add(1)
	for {
		max := f.maxInFlight.Load()
		if n <= max || f.maxInFlight.CompareAndSwap(max, n) {
			break
		}
	}
	f.mu.Lock()
	delay := f.traceDelay
	f.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
}

func (f *fakeRemote) leave() { f.inFlight.Add(-1) }

func newFakeRemote() *fakeRemote {
	return &fakeRemote{dirs: map[string]bool{}}
}

func (f *fakeRemote) Get(_ context.Context, path string) ([]byte, string, error) {
	f.enter()
	defer f.leave()

	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets++
	if f.getErr != nil {
		return nil, "", f.getErr
	}
	if !f.exists {
		return nil, "", webdav.ErrNotFound
	}
	return f.data, f.etag, nil
}

func (f *fakeRemote) Put(_ context.Context, path string, data []byte, ifMatch string) (string, error) {
	f.enter()
	defer f.leave()

	// 在取锁之前触发，好让钩子能调用 writeAsOtherDevice 等自带加锁的方法。
	f.mu.Lock()
	hook := f.beforePut
	f.beforePut = nil // 只触发一次，不影响建目录后的重试
	f.mu.Unlock()
	if hook != nil {
		hook()
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.puts++
	f.lastIfMatch = ifMatch
	if f.putErr != nil {
		return "", f.putErr
	}
	if f.needsDir && !f.dirs[remoteDir] {
		if f.missingDirErr != nil {
			return "", f.missingDirErr
		}
		return "", webdav.ErrNotCollection
	}
	if f.strictMatch && ifMatch != "" && ifMatch != f.etag {
		return "", webdav.ErrConflict
	}

	f.data = append([]byte(nil), data...)
	f.exists = true
	f.etagSeq++
	f.etag = fmt.Sprintf("etag-%d", f.etagSeq)
	if f.noPutETag {
		f.etag = ""
	}
	return f.etag, nil
}

func (f *fakeRemote) MkcolAll(_ context.Context, path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mkcols++
	if f.mkcolErr != nil {
		return f.mkcolErr
	}
	f.dirs[path] = true
	return nil
}

// writeAsOtherDevice 模拟另一台机器写入远端。
func (f *fakeRemote) writeAsOtherDevice(t *testing.T, s SyncableSettings, device string, revision int) {
	t.Helper()
	data, err := Encode(Payload{
		SchemaVersion: SchemaVersion,
		Revision:      revision,
		UpdatedAt:     time.Now().UTC(),
		DeviceName:    device,
		Settings:      s,
	})
	if err != nil {
		t.Fatalf("encode remote payload: %v", err)
	}
	f.writeRaw(data)
}

// writeRaw 直接放一份原始内容到远端（用于损坏 / 高版本 / 非本应用的文件）。
func (f *fakeRemote) writeRaw(data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data = append([]byte(nil), data...)
	f.exists = true
	f.etagSeq++
	f.etag = fmt.Sprintf("etag-%d", f.etagSeq)
	f.dirs[remoteDir] = true
}

func (f *fakeRemote) putCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.puts
}

/* ---------- 测试环境 ---------- */

type env struct {
	t      *testing.T
	sy     *Syncer
	st     *store.Store
	remote *fakeRemote
}

// newEnv 组一套「真库 + 假远端」的同步环境。
// 用真 store 而非假的：状态持久化与设置校验都是同步正确性的一部分。
func newEnv(t *testing.T) *env {
	t.Helper()
	st := newTestStore(t)
	remote := newFakeRemote()
	return &env{t: t, sy: New(st, st, remote), st: st, remote: remote}
}

// sync 执行一次同步，出错即失败。
func (e *env) sync() Result {
	e.t.Helper()
	res, err := e.sy.Sync(context.Background())
	if err != nil {
		e.t.Fatalf("Sync: %v", err)
	}
	return res
}

// setLocal 改本机设置。
func (e *env) setLocal(mutate func(*store.Settings)) {
	e.t.Helper()
	s, err := e.st.GetSettings()
	if err != nil {
		e.t.Fatalf("GetSettings: %v", err)
	}
	mutate(&s)
	if err := e.st.UpdateSettings(s); err != nil {
		e.t.Fatalf("UpdateSettings: %v", err)
	}
}

func (e *env) local() store.Settings {
	e.t.Helper()
	s, err := e.st.GetSettings()
	if err != nil {
		e.t.Fatalf("GetSettings: %v", err)
	}
	return s
}

func (e *env) state() State {
	e.t.Helper()
	st, err := loadState(e.st)
	if err != nil {
		e.t.Fatalf("loadState: %v", err)
	}
	return st
}

// remotePayload 解析远端当前内容。
func (e *env) remotePayload() Payload {
	e.t.Helper()
	e.remote.mu.Lock()
	data := append([]byte(nil), e.remote.data...)
	e.remote.mu.Unlock()

	p, err := Decode(data, store.DefaultSettings())
	if err != nil {
		e.t.Fatalf("decode remote: %v", err)
	}
	return p
}

/* ---------- 判定表 ---------- */

// TestSyncDecisionTable 五种「远端 × 本地」组合各走对应分支。
//
// 每种情形都由真实的同步操作铺垫（而非手工伪造 State）：这样不仅验证了分支
// 选得对，也顺带验证了上一次同步把基线记对了 —— 基线记错的话，
// 后续判定全会跑偏，而那种错误在单看一次同步时是看不出来的。
func TestSyncDecisionTable(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *env)
		want  Action
	}{
		{
			name:  "远端不存在 → 推送",
			setup: func(e *env) {},
			want:  ActionPushed,
		},
		{
			name: "远端未变 & 本地未变 → 无操作",
			setup: func(e *env) {
				e.setLocal(func(s *store.Settings) { s.Theme = "dark" })
				e.sync() // 先推一次，两侧对齐
			},
			want: ActionNoop,
		},
		{
			name: "远端未变 & 本地变了 → 推送",
			setup: func(e *env) {
				e.sync()
				e.setLocal(func(s *store.Settings) { s.ReaderFontSize = 18 })
			},
			want: ActionPushed,
		},
		{
			name: "远端变了 & 本地未变 → 拉取",
			setup: func(e *env) {
				e.sync()
				remote := From(e.local())
				remote.Theme = "sepia"
				e.remote.writeAsOtherDevice(t, remote, "other-mac", 2)
			},
			want: ActionPulled,
		},
		{
			name: "远端变了 & 本地也变了 → 冲突",
			setup: func(e *env) {
				// 本机先改出一份非默认配置并推上去，确立基线。
				e.setLocal(func(s *store.Settings) { s.Theme = "dark" })
				e.sync()

				remote := From(e.local())
				remote.Theme = "sepia"
				e.remote.writeAsOtherDevice(t, remote, "other-mac", 2)
				e.setLocal(func(s *store.Settings) { s.ReaderFontSize = 18 })
			},
			want: ActionConflict,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			c.setup(e)

			got := e.sync()
			if got.Action != c.want {
				t.Fatalf("动作 = %q, want %q", got.Action, c.want)
			}

			switch c.want {
			case ActionConflict:
				if got.Conflict == nil {
					t.Fatal("冲突结果缺少 Conflict 信息")
				}
				if got.Conflict.RemoteDeviceName != "other-mac" {
					t.Errorf("远端机器名 = %q, want other-mac", got.Conflict.RemoteDeviceName)
				}
				if e.state().Conflict == nil {
					t.Error("冲突未持久化，重启后将无声无息")
				}
			case ActionPulled:
				if got.Settings == nil {
					t.Error("拉取结果应带上应用后的设置，省前端一次往返")
				}
			default:
				if e.state().Conflict != nil {
					t.Errorf("%s 后仍残留冲突标记", c.want)
				}
			}
		})
	}
}

// TestConflictPersistsUntilResolved 冲突未裁决前，反复同步必须一直报冲突。
//
// 冲突分支不动基线，正是为了这个：一旦把基线前移，下一次同步就会看到
// 「两侧都没变」而返回 noop —— 冲突凭空消失，本地的改动没推上去、远端的改动
// 也没应用下来，两侧各自沉默地丢掉了一半。这是验收标准里明确禁止的静默丢数据。
func TestConflictPersistsUntilResolved(t *testing.T) {
	e := newEnv(t)
	setupConflict(t, e)

	localBefore := e.local()
	remoteBefore := e.remotePayload().Settings

	for i := range 3 {
		res := e.sync()
		if res.Action != ActionConflict {
			t.Fatalf("第 %d 次重复同步动作 = %q, want conflict（冲突被静默吞掉）", i+2, res.Action)
		}
	}

	// 期间两侧内容都不能被动过。
	if e.local() != localBefore {
		t.Error("未裁决的冲突期间本机设置被改动")
	}
	if got := e.remotePayload().Settings; got != remoteBefore {
		t.Error("未裁决的冲突期间远端被改写")
	}
	if e.remote.putCount() != 1 {
		t.Errorf("冲突期间发生了 %d 次写入（应只有铺垫时的那一次）", e.remote.putCount())
	}

	// 裁决之后才允许收敛。
	if _, err := e.sy.Resolve(context.Background(), true); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res := e.sync(); res.Action != ActionNoop {
		t.Errorf("裁决后动作 = %q, want noop", res.Action)
	}
}

// TestFirstSyncOnFreshMachinePulls 新机器接入已有同步时直接拉取，不弹冲突。
//
// 从状态看这是「两侧都变了」（本机没有任何基线），但本机配置仍是出厂默认 ——
// 用一份没人动过的默认配置去打扰用户做二选一没有意义。这是第二台机器接入的
// 主路径，弹窗会让人以为坏了。
func TestFirstSyncOnFreshMachinePulls(t *testing.T) {
	e := newEnv(t)

	remote := From(store.DefaultSettings())
	remote.Theme = "dark"
	remote.ReaderFontSize = 18
	e.remote.writeAsOtherDevice(t, remote, "first-mac", 5)

	res := e.sync()
	if res.Action != ActionPulled {
		t.Fatalf("动作 = %q, want pulled", res.Action)
	}
	if got := e.local(); got.Theme != "dark" || got.ReaderFontSize != 18 {
		t.Errorf("远端配置未应用：theme=%q fontSize=%d", got.Theme, got.ReaderFontSize)
	}
	if e.remote.putCount() != 0 {
		t.Error("拉取分支不应写远端")
	}
}

// TestPullAppliesThroughSettingsStore 拉取必须经 SettingsStore 写入，
// 使 api 层的副作用（更新间隔应用到订阅源、调度器、代理）得以触发。
func TestPullAppliesThroughSettingsStore(t *testing.T) {
	st := newTestStore(t)
	spy := &spySettings{inner: st}
	remote := newFakeRemote()
	sy := New(spy, st, remote)

	incoming := From(store.DefaultSettings())
	incoming.DefaultUpdateInterval = 60
	remote.writeAsOtherDevice(t, incoming, "other", 1)

	res, err := sy.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Action != ActionPulled {
		t.Fatalf("动作 = %q, want pulled", res.Action)
	}
	if spy.updates != 1 {
		t.Errorf("UpdateSettings 调用 %d 次, want 1", spy.updates)
	}
	if spy.lastWritten.DefaultUpdateInterval != 60 {
		t.Errorf("写入的更新间隔 = %d, want 60", spy.lastWritten.DefaultUpdateInterval)
	}
}

// spySettings 记录写入调用，验证拉取确实走了可带副作用的那条路径。
type spySettings struct {
	inner       *store.Store
	updates     int
	lastWritten store.Settings
}

func (s *spySettings) GetSettings() (store.Settings, error) { return s.inner.GetSettings() }

func (s *spySettings) UpdateSettings(v store.Settings) error {
	s.updates++
	s.lastWritten = v
	return s.inner.UpdateSettings(v)
}

/* ---------- 两个基线哈希的必要性 ---------- */

// TestPullWithUnknownValuesConverges 远端含本端不认识的取值时，拉取后必须收敛：
// 下一次同步应为 noop，不能反复拉取同一份配置。
//
// 这是 State 存两个哈希（RemoteHash / LocalHash）的直接理由。Apply 会把越界字段
// 回落成本机原值，因此「应用后的本机配置」与「远端载荷」并不相等。若只存一个
// 哈希，第二次同步会把这个天然差异误判成「远端又变了」，于是每次同步都拉一遍，
// 在两台版本不同的机器之间来回打转。
func TestPullWithUnknownValuesConverges(t *testing.T) {
	e := newEnv(t)

	incoming := From(store.DefaultSettings())
	incoming.Theme = "hologram" // 高版本客户端的新主题，本端不认识
	incoming.ReaderFontSize = 18
	e.remote.writeAsOtherDevice(t, incoming, "future-pc", 3)

	if res := e.sync(); res.Action != ActionPulled {
		t.Fatalf("首次动作 = %q, want pulled", res.Action)
	}
	local := e.local()
	if local.Theme != store.DefaultSettings().Theme {
		t.Errorf("未知主题被写入本机: %q", local.Theme)
	}
	if local.ReaderFontSize != 18 {
		t.Errorf("同一载荷里的合法字段未生效: %d", local.ReaderFontSize)
	}

	st := e.state()
	if st.RemoteHash == st.LocalHash {
		t.Error("两个基线哈希相同，说明测试前提不成立（应因回落而不同）")
	}

	// 关键断言：再同步一次必须收敛为 noop。
	if res := e.sync(); res.Action != ActionNoop {
		t.Fatalf("第二次动作 = %q, want noop（拉取未收敛，会反复拉同一份配置）", res.Action)
	}
	if res := e.sync(); res.Action != ActionNoop {
		t.Fatalf("第三次动作 = %q, want noop", res.Action)
	}
}

// TestSyncWorksWithoutServerETag 服务器不返回 ETag 时同步照常收敛。
//
// 变化判定用的是内容哈希而非 ETag，正是为了不依赖各家服务器是否吐 ETag。
func TestSyncWorksWithoutServerETag(t *testing.T) {
	e := newEnv(t)
	e.remote.noPutETag = true

	if res := e.sync(); res.Action != ActionPushed {
		t.Fatalf("动作 = %q, want pushed", res.Action)
	}
	if got := e.state().RemoteETag; got != "" {
		t.Fatalf("测试前提不成立：ETag = %q，应为空", got)
	}
	if res := e.sync(); res.Action != ActionNoop {
		t.Errorf("无 ETag 时第二次动作 = %q, want noop", res.Action)
	}
}

/* ---------- 推送路径 ---------- */

// TestPushCreatesDirectoryOnDemand 首次推送时 clip/ 必然不存在：
// 撞上 409 / 404 应建目录并重试一次。
func TestPushCreatesDirectoryOnDemand(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"服务器回 409", webdav.ErrNotCollection},
		{"服务器回 404", webdav.ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.remote.needsDir = true
			e.remote.missingDirErr = tc.err

			if res := e.sync(); res.Action != ActionPushed {
				t.Fatalf("动作 = %q, want pushed", res.Action)
			}
			if e.remote.mkcols == 0 {
				t.Error("未建目录")
			}
			if !e.remote.dirs[remoteDir] {
				t.Errorf("建的目录不是 %q", remoteDir)
			}
		})
	}
}

// TestPushSendsIfMatch 已知远端 ETag 时推送必须带 If-Match，
// 收窄「读到写」之间的竞态窗口。
func TestPushSendsIfMatch(t *testing.T) {
	e := newEnv(t)
	e.sync() // 首次推送，拿到 ETag

	e.setLocal(func(s *store.Settings) { s.Theme = "dark" })
	if res := e.sync(); res.Action != ActionPushed {
		t.Fatalf("动作 = %q, want pushed", res.Action)
	}
	if e.remote.lastIfMatch == "" {
		t.Error("第二次推送未带 If-Match")
	}
}

// TestPushOn412BecomesConflict PUT 撞上 412 说明 GET 与 PUT 之间远端被改写。
// 不能重试推送（会覆盖对方刚写的内容），必须转成冲突。
func TestPushOn412BecomesConflict(t *testing.T) {
	e := newEnv(t)
	e.sync()
	e.remote.strictMatch = true

	e.setLocal(func(s *store.Settings) { s.Theme = "dark" })
	localBefore := e.local()

	// 另一台机器恰好在本次同步的 GET 之后、PUT 之前写入。
	// 必须在这个窗口里动手：推送带的 If-Match 取自同一次同步刚做的 GET，
	// 提前改 ETag 只会让那次 GET 读到新值，触发不了 412。
	raced := From(store.DefaultSettings())
	raced.Theme = "sepia"
	e.remote.beforePut = func() {
		e.remote.writeAsOtherDevice(t, raced, "racing-mac", 7)
	}

	res := e.sync()
	if res.Action != ActionConflict {
		t.Fatalf("动作 = %q, want conflict", res.Action)
	}
	if res.Conflict == nil {
		t.Fatal("缺少冲突信息")
	}
	// 412 后重读远端，冲突信息应指向真正抢先的那台机器。
	if res.Conflict.RemoteDeviceName != "racing-mac" {
		t.Errorf("远端机器名 = %q, want racing-mac", res.Conflict.RemoteDeviceName)
	}
	if e.state().Conflict == nil {
		t.Error("冲突未持久化")
	}
	// 关键：不得重试推送把对方刚写的内容盖掉。
	if got := e.remotePayload().Settings; got != raced {
		t.Errorf("远端被覆盖：%+v，应保持抢先方写入的 %+v", got, raced)
	}
	if e.local() != localBefore {
		t.Error("冲突分支不应改动本机设置")
	}
}

/* ---------- 拒绝应用的两种远端内容 ---------- */

// TestSyncRejectsNewerSchemaWithoutPushing 高版本载荷既不应用也不推送。
//
// 推上去会把新版客户端的配置覆盖成旧格式 —— 比不同步严重得多。
func TestSyncRejectsNewerSchemaWithoutPushing(t *testing.T) {
	e := newEnv(t)
	e.remote.writeRaw([]byte(`{"schemaVersion":99,"revision":9,"settings":{"theme":"dark"}}`))

	before := e.local()
	_, err := e.sy.Sync(context.Background())
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("err = %v, want ErrSchemaTooNew", err)
	}
	if e.remote.putCount() != 0 {
		t.Error("拒绝应用的同时不得推送，否则会覆盖高版本客户端的配置")
	}
	if e.local() != before {
		t.Error("本机设置被高版本载荷改动")
	}
	if e.state().LastError == "" {
		t.Error("失败原因未记录，设置页无从展示")
	}
}

// TestSyncRejectsForeignFile 远端同名文件不是 Clip 写的时候不能覆盖它 ——
// 用户可能把同步目录指到了别的东西上。
func TestSyncRejectsForeignFile(t *testing.T) {
	e := newEnv(t)
	e.remote.writeRaw([]byte(`{"note":"我的记事本"}`))

	_, err := e.sy.Sync(context.Background())
	if !errors.Is(err, ErrForeignPayload) {
		t.Fatalf("err = %v, want ErrForeignPayload", err)
	}
	if e.remote.putCount() != 0 {
		t.Error("覆盖了用户的其他文件")
	}
}

/* ---------- 失败路径 ---------- */

// TestNetworkFailureKeepsBaselines 网络失败只记错误，不动基线。
//
// 基线被前移会让下次同步误认为两侧已对齐，从而静默丢掉一侧的改动。
func TestNetworkFailureKeepsBaselines(t *testing.T) {
	e := newEnv(t)
	e.sync() // 建立基线
	e.setLocal(func(s *store.Settings) { s.Theme = "dark" })
	before := e.state()

	e.remote.getErr = webdav.ErrNetwork
	if _, err := e.sy.Sync(context.Background()); !errors.Is(err, webdav.ErrNetwork) {
		t.Fatalf("err = %v, want ErrNetwork", err)
	}

	after := e.state()
	if after.RemoteHash != before.RemoteHash || after.LocalHash != before.LocalHash {
		t.Error("失败后基线被改动")
	}
	if after.LastError == "" {
		t.Error("未记录失败原因")
	}
	if !after.LastSyncAt.Equal(before.LastSyncAt) {
		t.Error("失败不应更新上次同步成功时间")
	}

	// 恢复网络后仍应把本地改动推上去。
	e.remote.getErr = nil
	if res := e.sync(); res.Action != ActionPushed {
		t.Errorf("恢复后动作 = %q, want pushed", res.Action)
	}
}

// TestFailureKeepsPendingConflict 冲突待裁决期间的同步失败不得清掉冲突。
func TestFailureKeepsPendingConflict(t *testing.T) {
	e := newEnv(t)
	e.setLocal(func(s *store.Settings) { s.Theme = "dark" })
	e.sync()
	remote := From(e.local())
	remote.Theme = "sepia"
	e.remote.writeAsOtherDevice(t, remote, "other", 2)
	e.setLocal(func(s *store.Settings) { s.ReaderFontSize = 18 })
	if res := e.sync(); res.Action != ActionConflict {
		t.Fatalf("动作 = %q, want conflict", res.Action)
	}

	e.remote.getErr = webdav.ErrNetwork
	if _, err := e.sy.Sync(context.Background()); err == nil {
		t.Fatal("应返回网络错误")
	}
	if e.state().Conflict == nil {
		t.Error("同步失败把待裁决的冲突清掉了")
	}
}

// TestSuccessClearsLastError 一次成功同步应清掉上次的错误，避免旧错误常驻界面。
func TestSuccessClearsLastError(t *testing.T) {
	e := newEnv(t)
	e.remote.getErr = webdav.ErrNetwork
	if _, err := e.sy.Sync(context.Background()); err == nil {
		t.Fatal("应返回网络错误")
	}
	if e.state().LastError == "" {
		t.Fatal("测试前提不成立：未记录错误")
	}

	e.remote.getErr = nil
	e.sync()
	if got := e.state().LastError; got != "" {
		t.Errorf("成功后 LastError = %q, want 空", got)
	}
}

func TestSyncWithoutRemoteIsNotConfigured(t *testing.T) {
	st := newTestStore(t)
	sy := New(st, st, nil)

	if _, err := sy.Sync(context.Background()); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("Sync err = %v, want ErrNotConfigured", err)
	}
	if _, err := sy.Resolve(context.Background(), true); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("Resolve err = %v, want ErrNotConfigured", err)
	}
}

// TestSetRemoteEnablesSync 用户保存配置后无需重启即可同步。
func TestSetRemoteEnablesSync(t *testing.T) {
	st := newTestStore(t)
	sy := New(st, st, nil)
	if _, err := sy.Sync(context.Background()); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}

	sy.SetRemote(newFakeRemote())
	res, err := sy.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync after SetRemote: %v", err)
	}
	if res.Action != ActionPushed {
		t.Errorf("动作 = %q, want pushed", res.Action)
	}
}

// TestCorruptLocalStateIsNotIgnored 本地同步状态损坏时必须报错。
//
// 静默当成「从未同步」会让引擎拿着空基线做判定，把远端配置当成新的推送目标 ——
// 可能覆盖另一台机器刚写的内容。
func TestCorruptLocalStateIsNotIgnored(t *testing.T) {
	e := newEnv(t)
	if err := e.st.SetJSONSetting(stateKey, "这不是一个 State 对象"); err != nil {
		t.Fatal(err)
	}

	if _, err := e.sy.Sync(context.Background()); err == nil {
		t.Error("状态损坏时 Sync 应报错，而非当成从未同步")
	}
	if _, err := e.sy.Status(); err == nil {
		t.Error("状态损坏时 Status 应报错")
	}
}

// TestPullFailureKeepsBaselines 写库失败时基线不得前移。
//
// 若前移，引擎会以为这次拉取成功了，下次同步看到「两侧都没变」而 noop ——
// 远端那份改动再也不会被应用，而用户看不到任何异常。
func TestPullFailureKeepsBaselines(t *testing.T) {
	st := newTestStore(t)
	failing := &failingSettings{inner: st}
	remote := newFakeRemote()
	sy := New(failing, st, remote)

	incoming := From(store.DefaultSettings())
	incoming.Theme = "sepia"
	remote.writeAsOtherDevice(t, incoming, "other", 1)

	failing.failWrite = true
	if _, err := sy.Sync(context.Background()); err == nil {
		t.Fatal("写库失败时 Sync 应报错")
	}

	state, err := loadState(st)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if state.RemoteHash != "" || state.LocalHash != "" {
		t.Error("拉取失败却前移了基线，远端的改动将永远不再被应用")
	}
	if state.LastError == "" {
		t.Error("未记录失败原因")
	}

	// 恢复后应重新拉取。
	failing.failWrite = false
	res, err := sy.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync after recovery: %v", err)
	}
	if res.Action != ActionPulled {
		t.Errorf("恢复后动作 = %q, want pulled", res.Action)
	}
}

// failingSettings 可注入写入失败的设置存储。
type failingSettings struct {
	inner     *store.Store
	failWrite bool
}

func (s *failingSettings) GetSettings() (store.Settings, error) { return s.inner.GetSettings() }

func (s *failingSettings) UpdateSettings(v store.Settings) error {
	if s.failWrite {
		return errors.New("模拟写库失败")
	}
	return s.inner.UpdateSettings(v)
}

// TestMkcolFailureSurfaces 建目录失败时报建目录的错误 —— 它更接近根因
// （通常是用户填的地址不存在，或该目录没有写权限），比 Put 的 409 更可诊断。
func TestMkcolFailureSurfaces(t *testing.T) {
	e := newEnv(t)
	e.remote.needsDir = true
	e.remote.mkcolErr = webdav.ErrUnauthorized

	_, err := e.sy.Sync(context.Background())
	if !errors.Is(err, webdav.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized（建目录的错误）", err)
	}
	if e.state().LastError == "" {
		t.Error("未记录失败原因")
	}
}

// TestConflictAfterRaceWhenRereadFails 412 之后重读远端也失败时，
// 仍必须记下冲突 —— 冲突本身是确定的，展示信息差一点不影响结论。
// 这里若退化成普通错误，用户就没有裁决入口了。
func TestConflictAfterRaceWhenRereadFails(t *testing.T) {
	e := newEnv(t)
	e.sync()
	e.remote.strictMatch = true
	e.setLocal(func(s *store.Settings) { s.Theme = "dark" })

	e.remote.beforePut = func() {
		raced := From(store.DefaultSettings())
		raced.Theme = "sepia"
		e.remote.writeAsOtherDevice(t, raced, "racing-mac", 7)
		// 紧接着网络也断了，412 后的重读拿不到内容。
		e.remote.mu.Lock()
		e.remote.getErr = webdav.ErrNetwork
		e.remote.mu.Unlock()
	}

	res := e.sync()
	if res.Action != ActionConflict {
		t.Fatalf("动作 = %q, want conflict", res.Action)
	}
	if e.state().Conflict == nil {
		t.Error("冲突未持久化，用户将没有裁决入口")
	}
	// 回退用的是本次同步开头读到的那份远端信息。
	if res.Conflict.RemoteDeviceName == "" {
		t.Error("冲突信息为空，弹窗无从展示远端来源")
	}
}

/* ---------- 冲突裁决 ---------- */

func TestResolveKeepLocal(t *testing.T) {
	e := newEnv(t)
	setupConflict(t, e)
	localBefore := e.local()

	res, err := e.sy.Resolve(context.Background(), true)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Action != ActionPushed {
		t.Fatalf("动作 = %q, want pushed", res.Action)
	}
	if e.local() != localBefore {
		t.Error("选择保留本地却改动了本机设置")
	}
	if got := e.remotePayload().Settings; got != From(localBefore) {
		t.Errorf("远端 = %+v，应为本地 %+v", got, From(localBefore))
	}
	if e.state().Conflict != nil {
		t.Error("裁决后冲突未清除")
	}

	// 裁决完两侧应已对齐。
	if res := e.sync(); res.Action != ActionNoop {
		t.Errorf("裁决后同步动作 = %q, want noop", res.Action)
	}
}

func TestResolveKeepRemote(t *testing.T) {
	e := newEnv(t)
	setupConflict(t, e)
	remoteWanted := e.remotePayload().Settings

	res, err := e.sy.Resolve(context.Background(), false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Action != ActionPulled {
		t.Fatalf("动作 = %q, want pulled", res.Action)
	}
	if got := From(e.local()); got != remoteWanted {
		t.Errorf("本机 = %+v，应为远端 %+v", got, remoteWanted)
	}
	if e.state().Conflict != nil {
		t.Error("裁决后冲突未清除")
	}
	if res := e.sync(); res.Action != ActionNoop {
		t.Errorf("裁决后同步动作 = %q, want noop", res.Action)
	}
}

// TestResolveRevisionAdvances 保留本地时推上去的版本号必须高于远端当前值，
// 否则另一台机器看到的版本号会倒退。
func TestResolveRevisionAdvances(t *testing.T) {
	e := newEnv(t)
	setupConflict(t, e)
	before := e.remotePayload().Revision

	if _, err := e.sy.Resolve(context.Background(), true); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := e.remotePayload().Revision; got <= before {
		t.Errorf("版本号 = %d，应大于裁决前的 %d", got, before)
	}
}

func TestResolveWithoutConflict(t *testing.T) {
	e := newEnv(t)
	e.sync()

	if _, err := e.sy.Resolve(context.Background(), true); !errors.Is(err, ErrNoConflict) {
		t.Errorf("err = %v, want ErrNoConflict", err)
	}
}

// TestResolveWhenRemoteDeleted 弹窗期间远端被删掉：无论用户选哪边，
// 能做的只有把本地推上去重建。
func TestResolveWhenRemoteDeleted(t *testing.T) {
	for _, keepLocal := range []bool{true, false} {
		t.Run(fmt.Sprintf("keepLocal=%v", keepLocal), func(t *testing.T) {
			e := newEnv(t)
			setupConflict(t, e)

			e.remote.mu.Lock()
			e.remote.exists = false
			e.remote.mu.Unlock()

			res, err := e.sy.Resolve(context.Background(), keepLocal)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if res.Action != ActionPushed {
				t.Errorf("动作 = %q, want pushed", res.Action)
			}
			if e.state().Conflict != nil {
				t.Error("冲突未清除")
			}
		})
	}
}

// TestResolveRereadsRemote 弹窗可能开了很久，裁决时必须以最新的远端为准。
func TestResolveRereadsRemote(t *testing.T) {
	e := newEnv(t)
	setupConflict(t, e)

	// 弹窗期间远端又被改了一次。
	latest := From(store.DefaultSettings())
	latest.Theme = "light"
	latest.ReaderWidth = "full"
	e.remote.writeAsOtherDevice(t, latest, "third-mac", 9)

	res, err := e.sy.Resolve(context.Background(), false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Action != ActionPulled {
		t.Fatalf("动作 = %q, want pulled", res.Action)
	}
	if got := e.local(); got.ReaderWidth != "full" {
		t.Errorf("应用的是旧快照而非最新远端：readerWidth = %q", got.ReaderWidth)
	}
}

// setupConflict 制造一个待裁决的冲突。
func setupConflict(t *testing.T, e *env) {
	t.Helper()
	e.setLocal(func(s *store.Settings) { s.Theme = "dark" })
	e.sync()

	remote := From(e.local())
	remote.Theme = "sepia"
	e.remote.writeAsOtherDevice(t, remote, "other-mac", 2)
	e.setLocal(func(s *store.Settings) { s.ReaderFontSize = 18 })

	if res := e.sync(); res.Action != ActionConflict {
		t.Fatalf("铺垫失败：动作 = %q, want conflict", res.Action)
	}
}

/* ---------- 状态 ---------- */

func TestStatus(t *testing.T) {
	e := newEnv(t)

	st, err := e.sy.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.LastSyncAt != nil {
		t.Error("从未同步时 LastSyncAt 应为 null，免得前端判别魔法时间值")
	}
	if !st.HasPending {
		t.Error("从未推送过，应报告有待推送改动")
	}
	if st.DeviceName == "" {
		t.Error("缺本机名，冲突弹窗无法对照展示")
	}

	e.sync()
	st, err = e.sy.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.LastSyncAt == nil {
		t.Error("同步成功后 LastSyncAt 仍为 null")
	}
	if st.HasPending {
		t.Error("刚推送完却报告有待推送改动")
	}

	e.setLocal(func(s *store.Settings) { s.Theme = "dark" })
	st, _ = e.sy.Status()
	if !st.HasPending {
		t.Error("本地改动后未报告待推送")
	}
}

// TestStatusIgnoresNonSyncedChanges 只有同步范围内的改动才算「待推送」。
// 窗口尺寸每次关窗都会变，若算进去，界面会永远显示「有待同步的改动」。
func TestStatusIgnoresNonSyncedChanges(t *testing.T) {
	e := newEnv(t)
	e.sync()

	e.setLocal(func(s *store.Settings) {
		s.WindowWidth = 999
		s.WindowHeight = 555
		s.ProxyHost = "10.0.0.1"
		s.ProxyPort = 1080
	})

	st, err := e.sy.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.HasPending {
		t.Error("窗口尺寸 / 代理变化被算成了待推送改动")
	}
	if res := e.sync(); res.Action != ActionNoop {
		t.Errorf("动作 = %q, want noop", res.Action)
	}
}

// TestNonSyncedFieldsSurvivePull 拉取不得覆盖本机的窗口尺寸与代理设置。
// 代理被覆盖尤其糟：家里的代理地址同步到公司机器会让全部抓取失败。
func TestNonSyncedFieldsSurvivePull(t *testing.T) {
	e := newEnv(t)
	e.setLocal(func(s *store.Settings) {
		s.WindowWidth = 1440
		s.WindowHeight = 900
		s.ProxyHost = "127.0.0.1"
		s.ProxyPort = 7890
	})
	e.sync()

	incoming := From(e.local())
	incoming.Theme = "sepia"
	e.remote.writeAsOtherDevice(t, incoming, "other", 2)
	if res := e.sync(); res.Action != ActionPulled {
		t.Fatalf("动作 = %q, want pulled", res.Action)
	}

	got := e.local()
	if got.WindowWidth != 1440 || got.WindowHeight != 900 {
		t.Errorf("窗口尺寸被覆盖：%dx%d", got.WindowWidth, got.WindowHeight)
	}
	if got.ProxyHost != "127.0.0.1" || got.ProxyPort != 7890 {
		t.Errorf("代理设置被覆盖：%s:%d", got.ProxyHost, got.ProxyPort)
	}
	if got.Theme != "sepia" {
		t.Errorf("同步范围内的主题未生效：%q", got.Theme)
	}
}

// TestPayloadCarriesNoNonSyncedFields 载荷里不该出现窗口尺寸与代理字段。
// 光靠 Apply 不写它们不够 —— 一旦进了载荷，就等于把用户的内网代理地址
// 上传到了网盘。
func TestPayloadCarriesNoNonSyncedFields(t *testing.T) {
	e := newEnv(t)
	e.setLocal(func(s *store.Settings) {
		s.WindowWidth = 1440
		s.ProxyHost = "10.1.2.3"
		s.ProxyPort = 8080
	})
	e.sync()

	e.remote.mu.Lock()
	raw := string(e.remote.data)
	e.remote.mu.Unlock()

	for _, banned := range []string{"windowWidth", "windowHeight", "proxyHost", "proxyPort", "10.1.2.3"} {
		if strings.Contains(raw, banned) {
			t.Errorf("载荷含不应同步的内容 %q:\n%s", banned, raw)
		}
	}
}

/* ---------- 并发 ---------- */

// TestConcurrentSyncsAreSerialized 手动按钮与 debounce 推送可能同时触发。
// 两者交错会各自基于同一份旧状态做判定，后写的覆盖前一次的基线。
// 用 -race 跑这个测试。
func TestConcurrentSyncsAreSerialized(t *testing.T) {
	e := newEnv(t)
	// 在每个远端请求里停留一会儿：没有互斥的话，多个同步会在这个窗口里叠起来。
	e.remote.traceDelay = 5 * time.Millisecond

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				if _, err := e.sy.Sync(context.Background()); err != nil {
					t.Errorf("Sync: %v", err)
				}
				return
			}
			if _, err := e.sy.Status(); err != nil {
				t.Errorf("Status: %v", err)
			}
		}(i)
	}
	wg.Wait()

	// 核心断言：任一时刻只应有一个同步在跟远端打交道。
	// 交错本身不会产生内存竞争（共享状态都经 SQLite），-race 报不出来 ——
	// 它的危害是逻辑上的：两次同步各自读到同一份旧状态做判定，
	// 后写的那次覆盖前一次的基线，于是一侧的改动被静默丢掉。
	if got := e.remote.maxInFlight.Load(); got > 1 {
		t.Errorf("同时在途的远端请求峰值 = %d，同步未串行", got)
	}

	// 并发结束后状态应自洽：再同步一次为 noop。
	e.remote.traceDelay = 0
	if res := e.sync(); res.Action != ActionNoop {
		t.Errorf("并发后动作 = %q, want noop", res.Action)
	}
}
