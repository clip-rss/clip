// Package syncer 把应用配置同步到用户自己的 WebDAV 服务器。
//
// 只同步配置，不同步订阅源与文章 —— 订阅源用 OPML 导入导出覆盖，整库同步在
// WebDAV 上做不安全（无部分写入也无可靠锁，两端各自上传整库等于稳定丢数据）。
//
// 包名是 syncer 而非 sync：与标准库 sync 撞名会让每个引用点都得起别名。
package syncer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/clip-rss/clip/internal/store"
)

// SyncableSettings 参与跨机器同步的配置子集。
//
// ⚠️ 这是显式白名单，不是 store.Settings 的别名。整个 Settings 丢上去会把
// 「机器相关」的配置也带过去：窗口尺寸在两台不同分辨率的机器间互相覆盖，
// 代理设置更糟 —— 家里的代理地址同步到公司机器上会让所有抓取直接失败。
//
// 新增字段时必须同时更新 settings_test.go 里的两份白名单，否则守卫测试会红。
// JSON 标签必须与 store.Settings 对应字段一致（守卫测试也会检查），
// 以便载荷在不同版本客户端间保持可读。
type SyncableSettings struct {
	Theme                 string `json:"theme"`
	Language              string `json:"language"`
	DefaultUpdateInterval int    `json:"defaultUpdateInterval"`
	DefaultMaxItems       int    `json:"defaultMaxItems"`
	NotificationMode      string `json:"notificationMode"`
	ShowUnreadBadge       bool   `json:"showUnreadBadge"`
	AutoMarkReadDelay     int    `json:"autoMarkReadDelay"`
	LaunchMinimized       bool   `json:"launchMinimized"`
	ReduceMotion          bool   `json:"reduceMotion"`
	ShowFocusIndicator    bool   `json:"showFocusIndicator"`

	ReaderFontFamily string  `json:"readerFontFamily"`
	ReaderFontSize   int     `json:"readerFontSize"`
	ReaderLineHeight float64 `json:"readerLineHeight"`
	ReaderWidth      string  `json:"readerWidth"`
	ReaderBackground string  `json:"readerBackground"`
}

// From 从完整设置中摘出可同步子集。
func From(s store.Settings) SyncableSettings {
	return SyncableSettings{
		Theme:                 s.Theme,
		Language:              s.Language,
		DefaultUpdateInterval: s.DefaultUpdateInterval,
		DefaultMaxItems:       s.DefaultMaxItems,
		NotificationMode:      s.NotificationMode,
		ShowUnreadBadge:       s.ShowUnreadBadge,
		AutoMarkReadDelay:     s.AutoMarkReadDelay,
		LaunchMinimized:       s.LaunchMinimized,
		ReduceMotion:          s.ReduceMotion,
		ShowFocusIndicator:    s.ShowFocusIndicator,
		ReaderFontFamily:      s.ReaderFontFamily,
		ReaderFontSize:        s.ReaderFontSize,
		ReaderLineHeight:      s.ReaderLineHeight,
		ReaderWidth:           s.ReaderWidth,
		ReaderBackground:      s.ReaderBackground,
	}
}

// Apply 把同步子集盖到 base 上，返回新的完整设置。base 的其余字段原样保留。
//
// 每个取值都按白名单收窄，越界时回落到 base 的当前值（而非出厂默认值）：
// 载荷可能来自更高版本客户端，含本端不认识的取值。这时保留用户本机已有的
// 选择比重置成默认值更接近用户意图，也避免非法值直接进到 CSS 或喂给
// store.UpdateSettings（后者对非法更新间隔会直接报错，让整次同步失败）。
func Apply(base store.Settings, in SyncableSettings) store.Settings {
	out := base
	out.Theme = pickString(themes, in.Theme, base.Theme)
	out.Language = pickString(languages, in.Language, base.Language)
	out.DefaultUpdateInterval = pickInt(updateIntervals, in.DefaultUpdateInterval, base.DefaultUpdateInterval)
	out.DefaultMaxItems = pickInt(maxItems, in.DefaultMaxItems, base.DefaultMaxItems)
	out.NotificationMode = pickString(notificationModes, in.NotificationMode, base.NotificationMode)
	out.AutoMarkReadDelay = pickInt(autoMarkReadDelays, in.AutoMarkReadDelay, base.AutoMarkReadDelay)
	out.ReaderFontFamily = pickString(readerFontFamilies, in.ReaderFontFamily, base.ReaderFontFamily)
	out.ReaderFontSize = pickInt(readerFontSizes, in.ReaderFontSize, base.ReaderFontSize)
	out.ReaderLineHeight = pickFloat(readerLineHeights, in.ReaderLineHeight, base.ReaderLineHeight)
	out.ReaderWidth = pickString(readerWidths, in.ReaderWidth, base.ReaderWidth)
	out.ReaderBackground = pickString(readerBackgrounds, in.ReaderBackground, base.ReaderBackground)

	// 布尔字段无需校验：JSON 的 bool 只有两个合法取值，且缺字段时
	// Decode 已用本端当前值作基底（见 payload.go 的 Decode）。
	out.ShowUnreadBadge = in.ShowUnreadBadge
	out.LaunchMinimized = in.LaunchMinimized
	out.ReduceMotion = in.ReduceMotion
	out.ShowFocusIndicator = in.ShowFocusIndicator
	return out
}

// Hash 返回配置子集的内容哈希（SHA-256 十六进制）。
//
// 用内容哈希而非 dirty 标记位判断本地是否改过：标记位要在每条改配置的路径上
// 记得置位，漏一处就静默失同步；哈希是从当前配置直接算出来的，不存在漏置。
//
// 结构体（而非 map）序列化的字段顺序由声明顺序固定，同一份配置的哈希稳定。
func (s SyncableSettings) Hash() string {
	// 定长结构体且全为标量，json.Marshal 不会失败。
	b, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

/* ---------- 取值白名单 ---------- */

// 与前端各处的取值集合一一对应：
//   - themes / languages：Types 里的 ThemePreference 与语言选项
//   - updateIntervals：store.validGlobalUpdateInterval（未导出，此处复制一份；
//     两者不一致时 syncer 会把非法值喂给 UpdateSettings 而报错，
//     TestApplyRejectsIntervalStoreWouldReject 守住这条）
//   - maxItems / autoMarkReadDelays：Sections.tsx 的 MAX_ITEMS_OPTIONS / autoMarkOptions
//   - reader*：ReaderStore.ts 的 FONT_FAMILIES / FONT_SIZES / LINE_HEIGHTS / WIDTHS / BACKGROUNDS
var (
	themes             = []string{"system", "light", "dark", "sepia"}
	languages          = []string{"zh", "en"}
	updateIntervals    = []int{0, 15, 30, 60}
	maxItems           = []int{50, 100, 200, 500}
	notificationModes  = []string{store.NotifyEach, store.NotifySummary, store.NotifyOff}
	autoMarkReadDelays = []int{-1, 0, 2000, 5000}
	readerFontFamilies = []string{"sans", "serif", "mono"}
	readerFontSizes    = []int{14, 16, 18}
	readerLineHeights  = []float64{1.5, 1.8, 2.0}
	readerWidths       = []string{"640", "800", "full"}
	readerBackgrounds  = []string{"default", "light", "sepia", "dark"}
)

func pickString(allowed []string, value, fallback string) string {
	return pick(allowed, value, fallback)
}

func pickInt(allowed []int, value, fallback int) int {
	return pick(allowed, value, fallback)
}

// pickFloat 按精确相等匹配。取值来自固定选项集（1.5 / 1.8 / 2.0）而非计算结果，
// 同一字面量经 JSON 往返后仍是同一个 float64，不需要容差比较。
func pickFloat(allowed []float64, value, fallback float64) float64 {
	return pick(allowed, value, fallback)
}

// pick 在白名单内则采用 value，否则回落 fallback。
func pick[T comparable](allowed []T, value, fallback T) T {
	for _, a := range allowed {
		if value == a {
			return value
		}
	}
	return fallback
}
