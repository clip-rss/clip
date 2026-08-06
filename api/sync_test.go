package api

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/clip-rss/clip/internal/secret"
	"github.com/clip-rss/clip/internal/store"
	"github.com/clip-rss/clip/internal/syncer"
	"github.com/clip-rss/clip/internal/webdav"
)

// newSyncService 组一套真库 + 真加密器的同步服务（远端未配置）。
func newSyncService(t *testing.T) (*SyncService, *SettingsService, *store.Store) {
	t.Helper()
	st := newTestStore(t)
	cipher, err := secret.NewCipher(filepath.Join(filepath.Dir(st.Path()), secret.KeyFileName))
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	settings := NewSettingsService(st, nil, nil)
	svc := NewSyncService(st, settings, cipher)
	ObserveSettings(settings, svc)
	return svc, settings, st
}

/* ---------- 前端契约 ---------- */

// TestFrontendMethodsNeverExposePassword 前端可调用的方法，其入出参里
// 只允许「写方向」出现密码字段。
//
// 阶段 D 把这条约束建立在 syncer 的类型上，这里守的是 api 层没把它绕开 ——
// 比如新增一个返回 webdavConfig 的便捷方法。用反射扫返回类型，
// 比靠 code review 记得住可靠。
func TestFrontendMethodsNeverExposePassword(t *testing.T) {
	svcType := reflect.TypeOf(&SyncService{})

	for i := range svcType.NumMethod() {
		m := svcType.Method(i)
		// 逐个检查返回值类型（跳过 error）。
		for j := range m.Type.NumOut() {
			out := m.Type.Out(j)
			assertNoPasswordField(t, m.Name, out, 0)
		}
	}
}

// assertNoPasswordField 递归检查类型里没有密码字段。
func assertNoPasswordField(t *testing.T, method string, typ reflect.Type, depth int) {
	t.Helper()
	if depth > 4 {
		return // 防御性：避免自引用类型无限递归
	}
	for typ.Kind() == reflect.Ptr || typ.Kind() == reflect.Slice {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return
	}

	for i := range typ.NumField() {
		f := typ.Field(i)
		name := strings.ToLower(f.Name)
		if f.Name == "HasPassword" {
			continue // 唯一允许提及密码的字段
		}
		for _, banned := range []string{"password", "cipher", "secret"} {
			if strings.Contains(name, banned) {
				t.Errorf("%s 的返回类型 %s 含疑似凭据字段 %s —— 密码不得回给前端",
					method, typ.Name(), f.Name)
			}
		}
		assertNoPasswordField(t, method, f.Type, depth+1)
	}
}

// TestLifecycleMethodsAreNotBindable 生命周期方法必须不导出。
//
// wails3 会把 Service 上每个导出方法都生成成前端绑定。前端能调 stop
// 就能静默关掉本次会话的自动推送；反复调 start 会泄漏 Debouncer
// 并叠出多次启动拉取。注释拦不住生成器，只有「不导出」拦得住。
func TestLifecycleMethodsAreNotBindable(t *testing.T) {
	svcType := reflect.TypeOf(&SyncService{})
	banned := []string{"Start", "Stop", "NotifySettingsChanged"}

	for _, name := range banned {
		if _, found := svcType.MethodByName(name); found {
			t.Errorf("%s 是导出方法，会被 wails3 生成成前端绑定；"+
				"应改为未导出方法 + 包级函数供 main 调用", name)
		}
	}
}

/* ---------- 未配置 / 凭据不可用 ---------- */

func TestSyncWithoutConfig(t *testing.T) {
	svc, _, _ := newSyncService(t)

	if _, err := svc.SyncNow(); !errors.Is(err, syncer.ErrNotConfigured) {
		t.Errorf("SyncNow err = %v, want ErrNotConfigured", err)
	}

	view, err := svc.GetWebDAVConfig()
	if err != nil {
		t.Fatalf("GetWebDAVConfig: %v", err)
	}
	if view != (syncer.WebDAVView{}) {
		t.Errorf("未配置时 view = %+v, want 零值", view)
	}
}

// TestNilCipherDegradesGracefully 加密器不可用时，涉及凭据的方法给出明确错误，
// 而不是 nil 解引用崩溃 —— 同步坏了不该拖垮整个应用。
func TestNilCipherDegradesGracefully(t *testing.T) {
	st := newTestStore(t)
	settings := NewSettingsService(st, nil, nil)
	svc := NewSyncService(st, settings, nil)

	if _, err := svc.GetWebDAVConfig(); err == nil {
		t.Error("GetWebDAVConfig 应报错")
	}
	if err := svc.SaveWebDAVConfig(syncer.WebDAVInput{}); err == nil {
		t.Error("SaveWebDAVConfig 应报错")
	}
	if _, err := svc.TestWebDAVConnection(syncer.WebDAVInput{}); err == nil {
		t.Error("TestWebDAVConnection 应报错")
	}
	if _, err := svc.SyncNow(); err == nil {
		t.Error("SyncNow 应报错")
	}
	// 状态查询不涉及凭据，仍应可用（设置页要能展示「同步不可用」）。
	if _, err := svc.GetSyncStatus(); err != nil {
		t.Errorf("GetSyncStatus 不应受影响: %v", err)
	}
	// 生命周期方法不能崩。
	StartSync(svc)
	StopSync(svc)
}

/* ---------- 保存与校验 ---------- */

func TestSaveAndReadBackConfig(t *testing.T) {
	svc, _, _ := newSyncService(t)

	err := svc.SaveWebDAVConfig(syncer.WebDAVInput{
		Enabled:  true,
		URL:      "https://dav.example.com/dav/",
		Username: "alice",
		Password: "hunter2",
	})
	if err != nil {
		t.Fatalf("SaveWebDAVConfig: %v", err)
	}

	view, err := svc.GetWebDAVConfig()
	if err != nil {
		t.Fatalf("GetWebDAVConfig: %v", err)
	}
	if !view.Enabled || view.URL != "https://dav.example.com/dav/" || view.Username != "alice" {
		t.Errorf("view = %+v", view)
	}
	if !view.HasPassword {
		t.Error("hasPassword = false, want true")
	}
}

// TestSaveRejectsPlainHTTP 明文 http 必须在保存阶段就被拒 ——
// 而不是等到同步时才失败。WebDAV 用 Basic Auth，密码随每个请求发送。
func TestSaveRejectsPlainHTTP(t *testing.T) {
	svc, _, _ := newSyncService(t)

	err := svc.SaveWebDAVConfig(syncer.WebDAVInput{
		Enabled:  true,
		URL:      "http://dav.example.com/dav/",
		Username: "alice",
		Password: "hunter2",
	})
	if err == nil {
		t.Fatal("明文 http 应被拒绝")
	}
	if !errors.Is(err, webdav.ErrInvalidConfig) {
		t.Errorf("err = %v, want ErrInvalidConfig", err)
	}

	// ⚠️ 光断言「报错了」是不够的：最初的实现先落库、后校验，
	// 这个测试照样通过 —— 而实际上坏配置已经存进去且 enabled=true，
	// 下次启动每次同步都失败，当初那条错误早已消失。
	view, err := svc.GetWebDAVConfig()
	if err != nil {
		t.Fatalf("GetWebDAVConfig: %v", err)
	}
	if view != (syncer.WebDAVView{}) {
		t.Errorf("被拒的配置仍落了库: %+v", view)
	}
}

// TestSaveKeepsPreviousConfigWhenRejected 拒绝一份新配置时，
// 不能把上一份能用的配置破坏掉。
func TestSaveKeepsPreviousConfigWhenRejected(t *testing.T) {
	svc, _, _ := newSyncService(t)

	if err := svc.SaveWebDAVConfig(syncer.WebDAVInput{
		Enabled: true, URL: "https://good.example.com/dav/", Username: "alice", Password: "pw",
	}); err != nil {
		t.Fatalf("首次保存: %v", err)
	}

	// 用户手滑把地址改成了 http。
	if err := svc.SaveWebDAVConfig(syncer.WebDAVInput{
		Enabled: true, URL: "http://bad.example.com/dav/", Username: "alice",
	}); err == nil {
		t.Fatal("明文 http 应被拒绝")
	}

	view, err := svc.GetWebDAVConfig()
	if err != nil {
		t.Fatal(err)
	}
	if view.URL != "https://good.example.com/dav/" {
		t.Errorf("原有配置被坏配置覆盖了: %+v", view)
	}
	if !view.HasPassword {
		t.Error("原有密码丢了")
	}
}

func TestClearConfig(t *testing.T) {
	svc, _, _ := newSyncService(t)
	if err := svc.SaveWebDAVConfig(syncer.WebDAVInput{
		Enabled: true, URL: "https://a.example/dav/", Username: "alice", Password: "pw",
	}); err != nil {
		t.Fatal(err)
	}

	if err := svc.ClearWebDAVConfig(); err != nil {
		t.Fatalf("ClearWebDAVConfig: %v", err)
	}
	view, err := svc.GetWebDAVConfig()
	if err != nil {
		t.Fatal(err)
	}
	if view != (syncer.WebDAVView{}) {
		t.Errorf("清除后仍有残留: %+v", view)
	}
	if _, err := svc.SyncNow(); !errors.Is(err, syncer.ErrNotConfigured) {
		t.Errorf("清除后 SyncNow err = %v, want ErrNotConfigured", err)
	}
}

/* ---------- 测试连接 ---------- */

// TestConnectionResultReportsStep 「测试连接」以数据形式回报哪一步失败，
// 而不是抛 error：Wails 把 error 压成字符串，前端无法据此分支渲染建议。
func TestConnectionResultReportsStep(t *testing.T) {
	svc, _, _ := newSyncService(t)

	// 没有密码 → connect 步失败，且带上「请输入密码」的建议。
	res, err := svc.TestWebDAVConnection(syncer.WebDAVInput{
		URL: "https://dav.example.com/dav/", Username: "alice",
	})
	if err != nil {
		t.Fatalf("TestWebDAVConnection 不该返回 error: %v", err)
	}
	if res.OK {
		t.Fatal("无密码时应报失败")
	}
	if res.Step != "connect" {
		t.Errorf("step = %q, want connect", res.Step)
	}
	if res.Hint == "" {
		t.Error("应给出可操作建议")
	}
}

// TestConnectionResultHintsAppPassword 401 要指向应用密码 ——
// 坚果云与开了两步验证的 Nextcloud 都不能用登录密码，这是最常见的卡点。
func TestConnectionResultHintsAppPassword(t *testing.T) {
	if got := hintFor(webdav.ErrUnauthorized); !strings.Contains(got, "应用密码") {
		t.Errorf("401 的建议未提到应用密码: %q", got)
	}
	if got := hintFor(webdav.ErrNotCollection); !strings.Contains(got, "remote.php") {
		t.Errorf("409 的建议应给出 Nextcloud 的地址范例: %q", got)
	}
	// 无对应建议时返回空串，前端据此不渲染建议区。
	if got := hintFor(errors.New("某个未分类的错误")); got != "" {
		t.Errorf("未分类错误的建议 = %q, want 空", got)
	}
}

// TestProbePathMatchesSyncDir 探针必须写在真正同步用的目录里，
// 否则测的是别处的写权限，通过了也不代表同步能用。
func TestProbePathMatchesSyncDir(t *testing.T) {
	if !strings.HasPrefix(syncer.RemoteFile(), syncer.RemoteDir()) {
		t.Errorf("同步文件 %q 不在同步目录 %q 下",
			syncer.RemoteFile(), syncer.RemoteDir())
	}
}

/* ---------- 变更回调与抑制 ---------- */

// TestSettingsChangeTriggersObserver 改设置应通知观察者，
// 否则「配置变更后 debounce 推送」这条触发时机根本不会发生。
func TestSettingsChangeTriggersObserver(t *testing.T) {
	st := newTestStore(t)
	settings := NewSettingsService(st, nil, nil)
	spy := &observerSpy{}
	ObserveSettings(settings, spy)

	cfg, err := settings.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Theme = "dark"
	if err := settings.UpdateSettings(cfg); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	if spy.count() != 1 {
		t.Errorf("观察者被通知 %d 次, want 1", spy.count())
	}
}

// TestObserverNotNotifiedWhenSaveFails 设置没保存成功就不该通知 ——
// 否则会推送一份并不存在的配置。
func TestObserverNotNotifiedWhenSaveFails(t *testing.T) {
	st := newTestStore(t)
	settings := NewSettingsService(st, nil, nil)
	spy := &observerSpy{}
	ObserveSettings(settings, spy)

	cfg, err := settings.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DefaultUpdateInterval = 45 // store 只接受 0/15/30/60
	if err := settings.UpdateSettings(cfg); err == nil {
		t.Fatal("非法更新间隔应被拒绝")
	}
	if spy.count() != 0 {
		t.Errorf("保存失败却通知了 %d 次", spy.count())
	}
}

// TestPullDoesNotTriggerPush 拉取时引擎经 UpdateSettings 写库，
// 那会触发变更回调。必须抑制：刚拉下来的配置正是远端那份，推回去毫无意义，
// 在两台机器间还可能来回打转。
func TestPullDoesNotTriggerPush(t *testing.T) {
	svc, settings, _ := newSyncService(t)

	// 直接走引擎用的那条写入路径（settingsWriter），模拟一次拉取写库。
	writer := settingsWriter{svc: settings, owner: svc}

	// 先装上 debouncer，否则 notifySettingsChanged 会因 debounce 为 nil 直接返回，
	// 测不出抑制是否生效。
	var pushes int32
	var mu sync.Mutex
	svc.mu.Lock()
	svc.debounce = syncer.NewDebouncer(10*time.Millisecond, func() {
		mu.Lock()
		pushes++
		mu.Unlock()
	})
	svc.mu.Unlock()
	defer StopSync(svc)

	cfg, err := writer.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Theme = "sepia"
	if err := writer.UpdateSettings(cfg); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	time.Sleep(60 * time.Millisecond) // 给 debounce 足够时间触发（若未被抑制）

	mu.Lock()
	got := pushes
	mu.Unlock()
	if got != 0 {
		t.Errorf("拉取写库触发了 %d 次推送，应被抑制", got)
	}

	// 抑制必须已解除：用户自己改设置仍要能触发推送。
	svc.mu.Lock()
	suppress := svc.suppress
	svc.mu.Unlock()
	if suppress != 0 {
		t.Errorf("抑制计数 = %d, want 0（未解除会永久失去自动推送）", suppress)
	}
}

// TestUserChangeStillTriggersPush 抑制不能把正常路径也一起挡掉。
func TestUserChangeStillTriggersPush(t *testing.T) {
	svc, settings, _ := newSyncService(t)

	fired := make(chan struct{}, 4)
	svc.mu.Lock()
	svc.debounce = syncer.NewDebouncer(10*time.Millisecond, func() {
		fired <- struct{}{}
	})
	svc.mu.Unlock()
	defer StopSync(svc)

	cfg, err := settings.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Theme = "dark"
	if err := settings.UpdateSettings(cfg); err != nil {
		t.Fatal(err)
	}

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Error("用户改设置未触发推送")
	}
}

// TestStopPreventsFurtherPushes 退出后不该再发起网络推送。
func TestStopPreventsFurtherPushes(t *testing.T) {
	svc, settings, _ := newSyncService(t)

	var mu sync.Mutex
	var pushes int
	svc.mu.Lock()
	svc.debounce = syncer.NewDebouncer(10*time.Millisecond, func() {
		mu.Lock()
		pushes++
		mu.Unlock()
	})
	svc.mu.Unlock()

	StopSync(svc)

	cfg, _ := settings.GetSettings()
	cfg.Theme = "dark"
	if err := settings.UpdateSettings(cfg); err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)

	mu.Lock()
	got := pushes
	mu.Unlock()
	if got != 0 {
		t.Errorf("Stop 后仍推送了 %d 次", got)
	}
}

/* ---------- 设置页开关同步的会话内接线 ---------- */
//
// 这三条覆盖的是「阶段 F 的设置页第一次真正走到」的路径。此前 debounce 只在
// StartSync（应用启动）装一次，而那时同步通常还是关的。

// TestEnablingSyncMidSessionArmsAutoPush 在设置页首次开启同步后，
// 本次会话内改设置就该能触发推送。
//
// 少了 SaveWebDAVConfig 里的 arm，用户开启同步 → 改主题 → 什么都不会上传，
// 直到下次重启。而这在界面上完全无声：状态区显示「有改动待推送」，
// 却永远等不到那次推送。
func TestEnablingSyncMidSessionArmsAutoPush(t *testing.T) {
	svc, _, _ := newSyncService(t)

	// 启动时同步是关的 —— 绝大多数用户的初始状态。
	StartSync(svc)
	svc.mu.Lock()
	armedAtStart := svc.debounce != nil
	svc.mu.Unlock()
	if armedAtStart {
		t.Fatal("同步未启用时不该装 debounce")
	}

	// 用户在设置页填好并开启。地址只做解析校验，不发网络请求。
	if err := svc.SaveWebDAVConfig(syncer.WebDAVInput{
		Enabled: true, URL: "https://dav.example.com/dav/", Username: "alice", Password: "pw",
	}); err != nil {
		t.Fatalf("SaveWebDAVConfig: %v", err)
	}
	defer StopSync(svc)

	svc.mu.Lock()
	armed := svc.debounce != nil
	svc.mu.Unlock()
	if !armed {
		t.Error("设置页开启同步后未装 debounce —— 本次会话内改设置不会推送")
	}
}

// TestDisablingSyncDisarmsAutoPush 停用后不该再触发推送。
//
// 不卸的后果是每次改设置都去推一次、失败一次，白刷日志与网盘请求配额。
func TestDisablingSyncDisarmsAutoPush(t *testing.T) {
	svc, _, _ := newSyncService(t)

	if err := svc.SaveWebDAVConfig(syncer.WebDAVInput{
		Enabled: true, URL: "https://dav.example.com/dav/", Username: "alice", Password: "pw",
	}); err != nil {
		t.Fatalf("开启: %v", err)
	}
	if err := svc.SaveWebDAVConfig(syncer.WebDAVInput{
		Enabled: false, URL: "https://dav.example.com/dav/", Username: "alice",
	}); err != nil {
		t.Fatalf("停用: %v", err)
	}

	svc.mu.Lock()
	armed := svc.debounce != nil
	svc.mu.Unlock()
	if armed {
		t.Error("停用后 debounce 仍在，改设置会反复推送并失败")
	}
}

// TestSaveDisabledConfigSucceeds 「停用并保存」必须是成功的。
//
// refreshRemote 对未启用的表达就是 ErrNotConfigured。原样返回会让用户点完
// 保存看到「尚未配置同步服务器」，以为没保存上 —— 实际已经存了，于是用户
// 往往会再点几次，或者干脆认为功能坏了。
func TestSaveDisabledConfigSucceeds(t *testing.T) {
	svc, _, _ := newSyncService(t)

	if err := svc.SaveWebDAVConfig(syncer.WebDAVInput{
		Enabled: true, URL: "https://dav.example.com/dav/", Username: "alice", Password: "pw",
	}); err != nil {
		t.Fatalf("开启: %v", err)
	}

	// 停用。密码留空 = 保持原密码，用户重新开启时不必再输一遍。
	if err := svc.SaveWebDAVConfig(syncer.WebDAVInput{
		Enabled: false, URL: "https://dav.example.com/dav/", Username: "alice",
	}); err != nil {
		t.Fatalf("停用并保存不该报错: %v", err)
	}

	view, err := svc.GetWebDAVConfig()
	if err != nil {
		t.Fatal(err)
	}
	if view.Enabled {
		t.Error("enabled = true, want false")
	}
	if view.URL != "https://dav.example.com/dav/" {
		t.Errorf("url = %q, 停用不该丢地址", view.URL)
	}
	if !view.HasPassword {
		t.Error("停用丢了密码 —— 重新开启时用户得再输一遍")
	}
}

// TestRemoteFilePathIsWithinSyncDir 设置页展示的路径必须就是真正同步用的那个。
//
// 前端据此拼出完整位置告知用户。在前端写死一份会漂移。
func TestRemoteFilePathIsWithinSyncDir(t *testing.T) {
	svc, _, _ := newSyncService(t)
	if got := svc.RemoteFilePath(); got != syncer.RemoteFile() {
		t.Errorf("RemoteFilePath() = %q, want %q", got, syncer.RemoteFile())
	}
}

// observerSpy 记录通知次数。
type observerSpy struct {
	mu sync.Mutex
	n  int
}

func (o *observerSpy) notifySettingsChanged() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.n++
}

func (o *observerSpy) count() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.n
}
