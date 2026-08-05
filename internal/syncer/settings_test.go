package syncer

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/clip-rss/clip/internal/store"
)

// syncedFields 参与同步的 store.Settings 字段。
// notSyncedFields 有意排除的字段，附上原因。
//
// 这两份名单是 TestSyncableSubsetIsExhaustive 的输入：新增设置字段时若忘了
// 归类，测试会红并指出字段名。名单写在测试里而非生产代码里，是为了让「分类」
// 这件事成为一道必须显式通过的关卡 —— 生产代码里的白名单可以被顺手改坏，
// 而这里改坏就是测试红。
var syncedFields = []string{
	"Theme",
	"Language",
	"DefaultUpdateInterval",
	"DefaultMaxItems",
	"NotificationMode",
	"ShowUnreadBadge",
	"AutoMarkReadDelay",
	"LaunchMinimized",
	"ReduceMotion",
	"ShowFocusIndicator",
	"ReaderFontFamily",
	"ReaderFontSize",
	"ReaderLineHeight",
	"ReaderWidth",
	"ReaderBackground",
}

var notSyncedFields = map[string]string{
	"WindowWidth":  "机器相关：两台不同分辨率的机器会互相覆盖窗口尺寸",
	"WindowHeight": "机器相关：同上",
	"ProxyHost":    "网络相关：家里的代理地址同步到公司会让全部抓取失败",
	"ProxyPort":    "网络相关：同上",
}

// TestSyncableSubsetIsExhaustive store.Settings 的每个字段都必须被显式归类。
//
// 这是本阶段的关键防线：漏归类的字段会静默不同步，而「某项设置没跟着同步」
// 这种问题在真机上极难发现 —— 用户只会觉得功能时好时坏。
func TestSyncableSubsetIsExhaustive(t *testing.T) {
	synced := make(map[string]bool, len(syncedFields))
	for _, name := range syncedFields {
		synced[name] = true
	}

	settingsType := reflect.TypeOf(store.Settings{})
	seen := make(map[string]bool, settingsType.NumField())

	for i := range settingsType.NumField() {
		f := settingsType.Field(i)
		seen[f.Name] = true
		switch {
		case synced[f.Name] && notSyncedFields[f.Name] != "":
			t.Errorf("字段 %s 同时出现在两份名单里", f.Name)
		case synced[f.Name], notSyncedFields[f.Name] != "":
			// 已归类
		default:
			t.Errorf("store.Settings 新增字段 %s 未归类："+
				"要同步就加进 syncedFields 与 SyncableSettings，"+
				"不同步就加进 notSyncedFields 并写明原因", f.Name)
		}
	}

	// 反向检查，防止字段被改名后名单里留下死条目。
	for _, name := range syncedFields {
		if !seen[name] {
			t.Errorf("syncedFields 里的 %s 已不存在于 store.Settings", name)
		}
	}
	for name := range notSyncedFields {
		if !seen[name] {
			t.Errorf("notSyncedFields 里的 %s 已不存在于 store.Settings", name)
		}
	}
}

// TestSyncableFieldsMatchSettingsShape 同步子集的字段类型与 JSON 标签必须与
// store.Settings 一致。
//
// JSON 标签一致是跨版本可读的前提：载荷由不同版本的客户端互相读写，标签打错
// 一个字母，对方读到的就是缺省值 —— 而这不会报错，只会静默丢配置。
func TestSyncableFieldsMatchSettingsShape(t *testing.T) {
	settingsType := reflect.TypeOf(store.Settings{})
	syncableType := reflect.TypeOf(SyncableSettings{})

	if got, want := syncableType.NumField(), len(syncedFields); got != want {
		t.Errorf("SyncableSettings 有 %d 个字段，syncedFields 列了 %d 个", got, want)
	}

	for _, name := range syncedFields {
		src, ok := settingsType.FieldByName(name)
		if !ok {
			continue // 由上一个测试报告
		}
		dst, ok := syncableType.FieldByName(name)
		if !ok {
			t.Errorf("SyncableSettings 缺少字段 %s", name)
			continue
		}
		if src.Type != dst.Type {
			t.Errorf("字段 %s 类型不一致：Settings 是 %s，SyncableSettings 是 %s",
				name, src.Type, dst.Type)
		}
		if s, d := src.Tag.Get("json"), dst.Tag.Get("json"); s != d {
			t.Errorf("字段 %s 的 json 标签不一致：Settings 是 %q，SyncableSettings 是 %q",
				name, s, d)
		}
	}
}

/* ---------- From / Apply ---------- */

// settingsA / syncableB 两组取值：每个字段都合法，且两组在**每个**字段上都不同。
// 用它们做往返，任何字段在 From 或 Apply 里漏抄都会暴露成不相等。
//
// ⚠️ settingsA 的每个字段都必须是非零值（bool 全 true、数值避开 0）。
// 若某字段取零值，From 漏抄该字段后得到的也是零值，测试便看不出区别 ——
// 这个坑是变异验证撞出来的：最初 ReduceMotion 写的是 false，
// 删掉 From 里对应的那行，测试照样绿。
func settingsA() store.Settings {
	return store.Settings{
		Theme:                 "light",
		Language:              "zh",
		DefaultUpdateInterval: 15,
		DefaultMaxItems:       50,
		NotificationMode:      store.NotifyEach,
		ShowUnreadBadge:       true,
		AutoMarkReadDelay:     2000,
		LaunchMinimized:       true,
		ReduceMotion:          true,
		ShowFocusIndicator:    true,
		ReaderFontFamily:      "sans",
		ReaderFontSize:        14,
		ReaderLineHeight:      1.5,
		ReaderWidth:           "640",
		ReaderBackground:      "light",

		// 不同步的字段，用于验证 Apply 不动它们。
		WindowWidth:  1440,
		WindowHeight: 900,
		ProxyHost:    "127.0.0.1",
		ProxyPort:    7890,
	}
}

func syncableB() SyncableSettings {
	return SyncableSettings{
		Theme:                 "dark",
		Language:              "en",
		DefaultUpdateInterval: 60,
		DefaultMaxItems:       200,
		NotificationMode:      store.NotifyOff,
		ShowUnreadBadge:       false,
		AutoMarkReadDelay:     -1,
		LaunchMinimized:       false,
		ReduceMotion:          false,
		ShowFocusIndicator:    false,
		ReaderFontFamily:      "mono",
		ReaderFontSize:        18,
		ReaderLineHeight:      2.0,
		ReaderWidth:           "full",
		ReaderBackground:      "dark",
	}
}

// TestFixturesDifferOnEveryField 两组测试取值必须逐字段不同，且 A 无零值。
//
// 这是下面几个测试的前提，不是可选的讲究：任一字段上两组取值相同，
// 对应的漏抄就检测不出来，测试会假绿。新增字段时若忘了给 fixture 赋值，
// 这里会红并点出字段名。
func TestFixturesDifferOnEveryField(t *testing.T) {
	// 直接读 store.Settings，不经 From：否则 From 本身有漏抄时，
	// 这里会报成「fixture 写错了」，把矛头指向错误的地方。
	a := reflect.ValueOf(settingsA())
	b := reflect.ValueOf(syncableB())

	for _, name := range syncedFields {
		av, bv := a.FieldByName(name), b.FieldByName(name)
		if !av.IsValid() || !bv.IsValid() {
			continue
		}
		if av.Interface() == bv.Interface() {
			t.Errorf("字段 %s 在两组 fixture 中取值相同（均为 %v），漏抄该字段将检测不出",
				name, av.Interface())
		}
		if av.IsZero() {
			t.Errorf("字段 %s 在 settingsA 中是零值，From 漏抄它将检测不出", name)
		}
	}
}

// TestFromCopiesEverySyncedField From 漏抄字段会让该字段停留零值。
func TestFromCopiesEverySyncedField(t *testing.T) {
	base := settingsA()
	got := From(base)

	baseVal := reflect.ValueOf(base)
	gotVal := reflect.ValueOf(got)
	for _, name := range syncedFields {
		want := baseVal.FieldByName(name)
		have := gotVal.FieldByName(name)
		if !have.IsValid() || !want.IsValid() {
			continue // 由结构一致性测试报告
		}
		if want.Interface() != have.Interface() {
			t.Errorf("From 未复制字段 %s：得到 %v，应为 %v",
				name, have.Interface(), want.Interface())
		}
	}
}

// TestApplyRoundTrip 合法载荷经 Apply 后应逐字段生效，且不动非同步字段。
func TestApplyRoundTrip(t *testing.T) {
	base := settingsA()
	in := syncableB()

	out := Apply(base, in)
	if got := From(out); got != in {
		t.Errorf("Apply 后的可同步子集 = %+v，应为 %+v", got, in)
	}

	// 非同步字段必须保持 base 原值。
	if out.WindowWidth != base.WindowWidth || out.WindowHeight != base.WindowHeight {
		t.Errorf("窗口尺寸被同步覆盖：%dx%d，应为 %dx%d",
			out.WindowWidth, out.WindowHeight, base.WindowWidth, base.WindowHeight)
	}
	if out.ProxyHost != base.ProxyHost || out.ProxyPort != base.ProxyPort {
		t.Errorf("代理设置被同步覆盖：%s:%d，应为 %s:%d",
			out.ProxyHost, out.ProxyPort, base.ProxyHost, base.ProxyPort)
	}
}

// TestApplyFallsBackOnUnknownValues 越界取值回落到本机原值，不写进设置。
//
// 载荷可能来自更高版本客户端，含本端不认识的取值。排版值会直接进 CSS，
// 更新间隔非法还会让 store.UpdateSettings 报错、整次同步失败。
func TestApplyFallsBackOnUnknownValues(t *testing.T) {
	base := settingsA()
	in := SyncableSettings{
		Theme:                 "hologram", // 未来版本的新主题
		Language:              "ja",
		DefaultUpdateInterval: 45,
		DefaultMaxItems:       9999,
		NotificationMode:      "digest",
		AutoMarkReadDelay:     1234,
		ReaderFontFamily:      "comic",
		ReaderFontSize:        72,
		ReaderLineHeight:      3.3,
		ReaderWidth:           "1200",
		ReaderBackground:      "neon",
	}

	out := Apply(base, in)

	checks := []struct {
		field string
		got   any
		want  any
	}{
		{"Theme", out.Theme, base.Theme},
		{"Language", out.Language, base.Language},
		{"DefaultUpdateInterval", out.DefaultUpdateInterval, base.DefaultUpdateInterval},
		{"DefaultMaxItems", out.DefaultMaxItems, base.DefaultMaxItems},
		{"NotificationMode", out.NotificationMode, base.NotificationMode},
		{"AutoMarkReadDelay", out.AutoMarkReadDelay, base.AutoMarkReadDelay},
		{"ReaderFontFamily", out.ReaderFontFamily, base.ReaderFontFamily},
		{"ReaderFontSize", out.ReaderFontSize, base.ReaderFontSize},
		{"ReaderLineHeight", out.ReaderLineHeight, base.ReaderLineHeight},
		{"ReaderWidth", out.ReaderWidth, base.ReaderWidth},
		{"ReaderBackground", out.ReaderBackground, base.ReaderBackground},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s 未回落：得到 %v，应为本机原值 %v", c.field, c.got, c.want)
		}
	}
}

// TestApplyAcceptsEveryWhitelistedValue 白名单里的每个取值都必须能被采纳。
// 防止白名单与 pick 的比较逻辑之间出现「列了但用不上」的死值。
func TestApplyAcceptsEveryWhitelistedValue(t *testing.T) {
	base := settingsA()

	for _, v := range themes {
		if got := Apply(base, SyncableSettings{Theme: v}).Theme; got != v {
			t.Errorf("主题 %q 未被采纳，得到 %q", v, got)
		}
	}
	for _, v := range updateIntervals {
		if got := Apply(base, SyncableSettings{DefaultUpdateInterval: v}).DefaultUpdateInterval; got != v {
			t.Errorf("更新间隔 %d 未被采纳，得到 %d", v, got)
		}
	}
	for _, v := range readerLineHeights {
		if got := Apply(base, SyncableSettings{ReaderLineHeight: v}).ReaderLineHeight; got != v {
			t.Errorf("行高 %v 未被采纳，得到 %v", v, got)
		}
	}
	for _, v := range readerBackgrounds {
		if got := Apply(base, SyncableSettings{ReaderBackground: v}).ReaderBackground; got != v {
			t.Errorf("阅读背景 %q 未被采纳，得到 %q", v, got)
		}
	}
}

// TestUpdateIntervalWhitelistMatchesStore updateIntervals 是 store 内部校验
// （validGlobalUpdateInterval，未导出）的一份副本。两者若漂移，同步会把
// store 不接受的值喂给 UpdateSettings，整次拉取失败。
//
// 这里用真库逐个验证，把「副本」与「原本」钉在一起。
func TestUpdateIntervalWhitelistMatchesStore(t *testing.T) {
	st := newTestStore(t)
	base, err := st.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}

	for _, v := range updateIntervals {
		s := base
		s.DefaultUpdateInterval = v
		if err := st.UpdateSettings(s); err != nil {
			t.Errorf("store 拒绝了白名单内的更新间隔 %d: %v", v, err)
		}
	}

	// 反向：Apply 不能放过 store 会拒绝的值。
	out := Apply(base, SyncableSettings{DefaultUpdateInterval: 45})
	if err := st.UpdateSettings(out); err != nil {
		t.Errorf("Apply 放过了非法更新间隔，导致 store 报错: %v", err)
	}
}

/* ---------- Hash ---------- */

func TestHashIsStableAndSensitive(t *testing.T) {
	a := syncableB()
	if a.Hash() != a.Hash() {
		t.Error("同一份配置两次求哈希结果不同")
	}
	if a.Hash() == "" {
		t.Fatal("哈希为空")
	}

	// 每个字段的改动都必须反映到哈希上，否则该字段的改动不会触发推送。
	base := settingsA()
	for _, name := range syncedFields {
		mutated := reflect.New(reflect.TypeOf(SyncableSettings{})).Elem()
		mutated.Set(reflect.ValueOf(From(base)))
		field := mutated.FieldByName(name)
		if !field.IsValid() {
			continue
		}
		other := reflect.ValueOf(syncableB()).FieldByName(name)
		field.Set(other)

		got := mutated.Interface().(SyncableSettings)
		if got == From(base) {
			continue // A 与 B 该字段取值相同（不应发生，由构造保证）
		}
		if got.Hash() == From(base).Hash() {
			t.Errorf("改动字段 %s 后哈希未变化", name)
		}
	}
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.NewWithPath(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}
