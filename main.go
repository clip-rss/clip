package main

import (
	"context"
	"embed"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/clip-rss/clip/api"
	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/i18n"
	"github.com/clip-rss/clip/internal/notify"
	"github.com/clip-rss/clip/internal/scheduler"
	"github.com/clip-rss/clip/internal/secret"
	"github.com/clip-rss/clip/internal/store"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

//go:embed all:frontend/dist
var assets embed.FS

// softwareUpdateHTML 是「Software Update」子窗口的页面模板。我们自建窗口、自驱 UI，而
// Updater 的内置模板 HTML 未导出，故自带一份。内容复制自 wails/v3
// pkg/updater/assets/window.html：监听 wails:updater:* 状态事件、通过
// AllowSimpleEventEmit 回发 wails:updater:user:* 动作事件。升级 Wails 时需同步此文件。
//
// 其中的 i18n 字典与当前语言由 Go 在建窗时注入（见 buildSoftwareUpdateHTML），模板里以
// __CLIP_I18N_DICT__ / __CLIP_I18N_LANG__ 两个占位符标记，避免手抄字典到 HTML 里。
//
//go:embed build/updater/window.html
var softwareUpdateHTML string

// updaterLocaleEN / updaterLocaleZH 是前端 locale 文件，作为更新窗口 i18n 的**唯一数据源**。
// 启动时取其中的 "updater" 段注入窗口，不再在 HTML 里手抄字典。
//
//go:embed frontend/src/I18n/locales/en.json
var updaterLocaleEN []byte

//go:embed frontend/src/I18n/locales/zh.json
var updaterLocaleZH []byte

const (
	updI18nDictMarker = "__CLIP_I18N_DICT__"
	updI18nLangMarker = "__CLIP_I18N_LANG__"
)

const (
	currentVersion = "0.1.0"
	repo           = "clip-rss/clip"
	changelogURL   = "https://raw.githubusercontent.com/clip-rss/clip/main/CHANGELOG.md"
)

// updaterI18nDict 解析出 en/zh 两份 locale 的 "updater" 段，拼成 {en:{...},zh:{...}} 的
// JSON（注入窗口用）。任一 locale 缺 "updater" 段则 panic —— 属于开发期集成错误，早失败。
func updaterI18nDict() string {
	extract := func(raw []byte, lang string) map[string]any {
		var all map[string]any
		if err := json.Unmarshal(raw, &all); err != nil {
			log.Fatalf("updater i18n: parse %s locale: %v", lang, err)
		}
		seg, ok := all["updater"].(map[string]any)
		if !ok {
			log.Fatalf("updater i18n: %s locale 缺少 \"updater\" 段", lang)
		}
		return seg
	}
	dict := map[string]any{
		"en": extract(updaterLocaleEN, "en"),
		"zh": extract(updaterLocaleZH, "zh"),
	}
	// json.Marshal 默认转义 <>& 为 \uXXXX，可安全内嵌进 <script>。
	b, err := json.Marshal(dict)
	if err != nil {
		log.Fatalf("updater i18n: marshal dict: %v", err)
	}
	return string(b)
}

// buildSoftwareUpdateHTML 把 i18n 字典与选定语言注入模板，返回可用于建窗的完整 HTML。
// 两个占位符必须都被替换，否则视为模板与代码不同步，panic 早失败。
func buildSoftwareUpdateHTML(dict, lang string) string {
	if !strings.Contains(softwareUpdateHTML, updI18nDictMarker) ||
		!strings.Contains(softwareUpdateHTML, updI18nLangMarker) {
		log.Fatalf("updater i18n: window.html 缺少占位符 %s / %s", updI18nDictMarker, updI18nLangMarker)
	}
	html := strings.Replace(softwareUpdateHTML, updI18nDictMarker, dict, 1)
	html = strings.Replace(html, updI18nLangMarker, lang, 1)
	return html
}

const (
	defaultWindowWidth  = 1200
	defaultWindowHeight = 800
	minWindowWidth      = 800
	minWindowHeight     = 600
)

// wailsEmitter 用 Wails 运行时事件总线实现 scheduler.Emitter，
// 通过全局 application.Get() 解耦，避免与 App 实例的构造顺序耦合。
type wailsEmitter struct{}

func (wailsEmitter) Emit(name string, data any) {
	if app := application.Get(); app != nil {
		app.Event.Emit(name, data)
	}
}

// wailsNotifSender 用 Wails notifications 服务实现 notify.Sender。
type wailsNotifSender struct {
	ns *notifications.NotificationService
}

func (s wailsNotifSender) Send(msg notify.Message) error {
	return s.ns.SendNotification(notifications.NotificationOptions{
		ID:    msg.ID,
		Title: msg.Title,
		Body:  msg.Body,
		Data:  map[string]interface{}{"articleId": msg.ID},
	})
}

// 窗口尺寸：改编自 Wails 内置更新窗口的常量（那些常量未导出）。
// full = 有新版/下载/安装等需要展示 release notes 与进度时；compact = 无新版/出错。
const (
	updWinFullWidth     = 520
	updWinFullHeight    = 500
	updWinCompactWidth  = 570
	updWinCompactHeight = 275
)

// softwareUpdateWindow 管理「Software Update」子窗口：用 app.Window.NewWithOptions
// 自建，页面是内置的更新 UI 模板（build/updater/window.html）。窗口靠事件总线与
// updateController 交互——窗口 JS 监听 wails:updater:* 状态事件、通过
// AllowSimpleEventEmit 回发 wails:updater:user:* 动作事件。
//
// 每轮检查都 rebuild 一个全新窗口：Wails 的 WindowClosing 会无条件销毁窗口，销毁后的
// *WebviewWindow 无法再 Show() 复活；且 Reload() 在 macOS 是空实现，无法重置页面里
// 单调递增的状态守卫（rank/errored）。所以复用旧窗口会卡在上一轮状态，必须新建。
//
// i18n：语言在**建窗时**由 langFn 现读并注入 HTML（连同 dict）。因窗口每轮 rebuild，
// 下次打开即用最新语言；代价是弹窗开着时切换 App 语言不会实时生效（短命对话框可接受）。
type softwareUpdateWindow struct {
	app    *application.App
	dict   string        // 注入的 i18n 字典 JSON（{en:{...},zh:{...}}），启动时算好、不变
	langFn func() string // 现读当前 App 语言（"zh"/"en"）

	mu  sync.Mutex
	win *application.WebviewWindow // 当前窗口；被销毁后置 nil
}

// ensure 返回一个可用窗口：不存在或已被销毁时新建。
func (s *softwareUpdateWindow) ensure() *application.WebviewWindow {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.win != nil {
		return s.win
	}
	lang := "en"
	if s.langFn != nil {
		if l := s.langFn(); l != "" {
			lang = l
		}
	}
	win := s.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   "software-update",
		Title:  i18n.T(lang, "updater.title"),
		Width:  updWinFullWidth,
		Height: updWinFullHeight,
		HTML:   buildSoftwareUpdateHTML(s.dict, lang),
		// 必须开启：窗口内 JS 靠 postMessage 回发 user:* 动作事件，否则按钮无效。
		AllowSimpleEventEmit: true,
		DisableResize:        true,
		MaximiseButtonState:  application.ButtonDisabled,
	})
	// 窗口被关闭（用户点 X 或我们调 Close）后会被销毁，清空引用以便下轮重建。
	win.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		s.mu.Lock()
		s.win = nil
		s.mu.Unlock()
	})
	s.win = win
	return win
}

// rebuild 关掉旧窗口并新建一个，保证每轮检查都是干净的页面状态。
func (s *softwareUpdateWindow) rebuild() {
	s.mu.Lock()
	old := s.win
	s.win = nil
	s.mu.Unlock()
	if old != nil {
		old.Close() // InvokeSync 同步销毁；WindowClosing 回调会再置 nil（幂等）
	}
	s.ensure().Show()
}

func (s *softwareUpdateWindow) Close() {
	s.mu.Lock()
	win := s.win
	s.mu.Unlock()
	if win != nil {
		win.Close()
	}
}

func (s *softwareUpdateWindow) SetSize(width, height int) {
	s.mu.Lock()
	win := s.win
	s.mu.Unlock()
	if win != nil {
		win.SetSize(width, height)
	}
}

// updateController 编排「先展示、再由用户决定」的更新流程，替代 updater.CheckAndInstall
// （后者检查到新版会立即下载）。它自己把窗口按钮事件接到 Updater 的导出方法上：
//
//	点检查 → Show 窗口 + 只调 Check（不下载）→ 窗口展示 release notes/状态
//	  ├─ Install → DownloadAndInstall → …→ update-ready → Restart & Apply
//	  └─ Close   → 关窗
//
// 事件走应用事件总线：Updater 用 app.Event.Emit 广播 wails:updater:* 状态，窗口 JS 监听；
// 窗口按钮通过 AllowSimpleEventEmit 回发 wails:updater:user:*，这里用 app.Event.On 收。
type updateController struct {
	app            *application.App
	updater        *updater.Updater
	win            *softwareUpdateWindow
	currentVersion string

	mu          sync.Mutex
	lastRelease *updater.Release // 最近一次 Check 命中的新版（供状态重放）
	lastStatus  func()           // 窗口 ready 时重放最近状态的闭包
}

func newUpdateController(app *application.App, up *updater.Updater, win *softwareUpdateWindow, version string) *updateController {
	c := &updateController{app: app, updater: up, win: win, currentVersion: version}
	c.wire()
	return c
}

// wire 只在启动时注册一次全部监听器（长期存活，幂等），避免每轮重复订阅导致泄漏。
func (c *updateController) wire() {
	on := c.app.Event.On

	// —— 用户动作 ——
	on(updater.EventUserInstall, func(*application.CustomEvent) {
		go func() {
			if err := c.updater.DownloadAndInstall(context.Background()); err != nil {
				c.app.Logger.Error("update", "stage", "download", "error", err)
			}
		}()
	})
	on(updater.EventUserRestart, func(*application.CustomEvent) {
		go func() {
			if err := c.updater.Restart(context.Background()); err != nil {
				c.app.Logger.Error("update", "stage", "restart", "error", err)
			}
		}()
	})
	on(updater.EventUserCancel, func(*application.CustomEvent) { c.win.Close() })

	// —— 状态事件：调整窗口尺寸（我们不走 CheckAndInstall，Updater 的自动 SetSize
	//    不会触发，这里自己按状态放大/缩小），并记录“最近状态”供 ready 重放。——
	full := func() { c.win.SetSize(updWinFullWidth, updWinFullHeight) }
	compact := func() { c.win.SetSize(updWinCompactWidth, updWinCompactHeight) }

	on(updater.EventUpdateAvailable, func(*application.CustomEvent) {
		c.setLastStatus(func() { c.app.Event.Emit(updater.EventUpdateAvailable, c.currentRelease()) })
		full()
	})
	on(updater.EventUpdateReady, func(*application.CustomEvent) {
		c.setLastStatus(func() { c.app.Event.Emit(updater.EventUpdateReady, c.currentRelease()) })
		full()
	})
	on(updater.EventNoUpdate, func(*application.CustomEvent) {
		c.setLastStatus(func() { c.app.Event.Emit(updater.EventNoUpdate) })
		compact()
	})
	on(updater.EventError, func(e *application.CustomEvent) {
		// 保留错误 payload 以便窗口 ready 时重放同样的错误横幅。
		var data any
		if e != nil {
			data = e.Data
		}
		c.setLastStatus(func() { c.app.Event.Emit(updater.EventError, data) })
		compact()
	})

	// —— 窗口就绪：每轮新建的窗口 JS 载入后会发 window:ready。先补发 Meta（Check 本身
	//    不发，否则“当前版本 → 新版本”里的当前版本渲染不出来），再重放最近状态，解决
	//    “窗口还没订阅上、Check 就已经发过事件”的竞态。——
	on(updater.EventWindowReady, func(*application.CustomEvent) {
		// 语言已在建窗时注入 HTML，无需再下发；这里只补 Meta（当前版本号，Check 不发它）。
		c.app.Event.Emit(updater.EventMeta, updater.Meta{
			CurrentVersion: c.currentVersion,
			SkippedVersion: c.updater.SkippedVersion(),
		})
		c.mu.Lock()
		replay := c.lastStatus
		c.mu.Unlock()
		if replay != nil {
			replay()
		}
	})
}

func (c *updateController) setLastStatus(f func()) {
	c.mu.Lock()
	c.lastStatus = f
	c.mu.Unlock()
}

func (c *updateController) currentRelease() *updater.Release {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastRelease
}

// check 是菜单「Check for Updates」的入口：新建窗口并只做检查（不下载）。
func (c *updateController) check() {
	// 重置本轮状态，rebuild 全新窗口（干净 JS 状态）。
	c.mu.Lock()
	c.lastRelease = nil
	c.lastStatus = func() { c.app.Event.Emit(updater.EventCheckStarted) }
	c.mu.Unlock()
	c.win.rebuild()

	go func() {
		rel, err := c.updater.Check(context.Background())
		if err != nil {
			// Check 已发过 EventError；记录重放（携带错误信息由 Updater 内部 emit 决定）。
			c.app.Logger.Error("update", "stage", "check", "error", err)
			return
		}
		if rel != nil {
			c.mu.Lock()
			c.lastRelease = rel
			c.mu.Unlock()
		}
	}()
}

// checkSilent 是静默更新检查（启动时后台运行）：只检查、不弹窗，若有新版本则通过
// "clip:update:available" 事件通知主窗口前端。
func (c *updateController) checkSilent() {
	go func() {
		rel, err := c.updater.Check(context.Background())
		if err != nil {
			c.app.Logger.Error("update", "stage", "check-silent", "error", err)
			return
		}
		if rel != nil {
			c.mu.Lock()
			c.lastRelease = rel
			c.mu.Unlock()
			// 通知主窗口前端：有新版本可用
			c.app.Event.Emit("clip:update:available", rel)
		}
	}()
}

// cleanOrphanedUpdateDirs 清理系统临时目录下遗留的 Wails 更新包目录。
//
// 问题：用户「下载完更新 → 关弹窗 → 不重启 → 退出 App」后，已下载的更新包
// （wails-update-* 目录）会残留在系统临时目录（macOS $TMPDIR、Windows %TEMP%），
// 因为关闭更新弹窗不会删除 staging 目录，而 Updater 的 stagingDir 字段只存在内存中，
// 下次启动时已丢失引用，无法通过 discardStaging() 清理。
//
// 清理策略：
// - 启动时扫描 os.TempDir() 下的 wails-update-* 目录
// - 保留最近 1 份（按修改时间）+ 24 小时内的（可能是其他实例正在下载）
// - 删除超过 24 小时且非最新的孤儿目录
//
// 并发安全：通过时间戳过滤避免删除其他 Clip 实例正在使用的目录。
func cleanOrphanedUpdateDirs() {
	cleanOrphanedUpdateDirsIn(os.TempDir())
}

// cleanOrphanedUpdateDirsIn 是 cleanOrphanedUpdateDirs 的可测试版本，接受目录参数。
func cleanOrphanedUpdateDirsIn(tmpDir string) {
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		log.Printf("clean orphaned updates: read temp dir: %v", err)
		return
	}

	const updateDirPrefix = "wails-update-"
	const retentionThreshold = 24 * time.Hour
	now := time.Now()

	type candidate struct {
		path    string
		modTime time.Time
	}
	var dirs []candidate

	// 收集所有 wails-update-* 目录及其修改时间
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), updateDirPrefix) {
			continue
		}
		fullPath := filepath.Join(tmpDir, entry.Name())
		info, err := os.Stat(fullPath)
		if err != nil {
			continue // 无法访问，跳过
		}
		dirs = append(dirs, candidate{path: fullPath, modTime: info.ModTime()})
	}

	if len(dirs) == 0 {
		return // 无孤儿目录
	}

	// 按修改时间降序排序（最新的在前）
	for i := 0; i < len(dirs); i++ {
		for j := i + 1; j < len(dirs); j++ {
			if dirs[j].modTime.After(dirs[i].modTime) {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}

	// 清理策略：保留最新 1 份 + 24 小时内的所有目录
	for i, dir := range dirs {
		age := now.Sub(dir.modTime)
		// 保留最新的 1 份（i == 0）或 24 小时内的（可能是其他实例正在下载）
		if i == 0 || age < retentionThreshold {
			continue
		}
		// 删除超过 24 小时且非最新的孤儿目录
		if err := os.RemoveAll(dir.path); err != nil {
			log.Printf("clean orphaned updates: remove %s: %v", dir.path, err)
		} else {
			log.Printf("clean orphaned updates: removed %s (age: %v)", dir.path, age.Round(time.Minute))
		}
	}
}

func savedWindowSize(settings store.Settings) (int, int) {
	width := settings.WindowWidth
	height := settings.WindowHeight
	if width < minWindowWidth {
		width = defaultWindowWidth
	}
	if height < minWindowHeight {
		height = defaultWindowHeight
	}
	return width, height
}

func saveWindowSize(st *store.Store, window application.Window) {
	if window == nil {
		return
	}
	width, height := window.Size()
	if width < minWindowWidth || height < minWindowHeight {
		return
	}
	settings, err := st.GetSettings()
	if err != nil {
		log.Printf("failed to load settings before saving window size: %v", err)
		return
	}
	settings.WindowWidth = width
	settings.WindowHeight = height
	if err := st.UpdateSettings(settings); err != nil {
		log.Printf("failed to save window size: %v", err)
	}
}

func main() {
	// 清理遗留的更新包目录（孤儿临时文件）。
	cleanOrphanedUpdateDirs()

	// 数据层。
	st, err := store.New()
	if err != nil {
		log.Fatalf("failed to init store: %v", err)
	}

	// 读取设置以决定窗口启动行为（最小化等）。读取失败时退回默认值。
	settings, _ := st.GetSettings()

	// 通知服务（需同时注册为 application.Service 并注入调度器）。
	notifSvc := notifications.New()
	notifSender := wailsNotifSender{ns: notifSvc}
	notifier := notify.NewService(st, notifSender)

	dockService := dock.New()

	// 抓取与调度层。
	ft := fetcher.New()
	if settings.ProxyHost != "" && settings.ProxyPort > 0 {
		ft.Client().SetProxy(settings.ProxyHost, settings.ProxyPort)
	}
	sch := scheduler.New(st, ft,
		scheduler.WithEmitter(wailsEmitter{}),
		scheduler.WithNotifier(notifier),
		scheduler.WithConfig(scheduler.Config{
			DefaultInterval: time.Duration(settings.DefaultUpdateInterval) * time.Minute,
		}),
	)

	// 凭据加密器。密钥与数据库同目录（<configDir>/clip/.synckey）。
	//
	// 失败不致命：只关掉备份功能，其余功能与它无关。WebDAVConfigService 收到 nil
	// 会让涉及凭据的方法返回明确错误，而不是崩在 nil 解引用上。
	var cipher *secret.Cipher
	if c, err := secret.NewCipher(filepath.Join(filepath.Dir(st.Path()), secret.KeyFileName)); err != nil {
		log.Printf("failed to init credential cipher, backup disabled: %v", err)
	} else {
		cipher = c
	}

	// 绑定服务（暴露给前端）。
	//
	// settingsSvc 提成变量：需要它来下发更新间隔到订阅源与调度器。
	settingsSvc := api.NewSettingsService(st, sch, ft.Client())
	webdavConfigSvc := api.NewWebDAVConfigService(st, cipher)
	opmlSvc := api.NewOPMLService(st)
	opmlBackupSvc := api.NewOPMLBackupService(st, webdavConfigSvc, opmlSvc)

	sysSvc := &api.SystemService{
		AppVersion:      currentVersion,
		ChangelogURL:    changelogURL,
		OnlineChangedFn: func(online bool) { sch.SetOfflineMode(!online) },
		LanguageFn: func() string {
			if current, err := st.GetSettings(); err == nil {
				return current.Language
			}
			return "en"
		},
	}
	app := application.New(application.Options{
		Name:        "clip",
		Description: i18n.T(settings.Language, "app.description"),
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Services: []application.Service{
			application.NewService(sysSvc),
			application.NewService(api.NewFeedService(st, ft, sch)),
			application.NewService(api.NewItemService(st)),
			application.NewService(api.NewCategoryService(st)),
			application.NewService(settingsSvc),
			application.NewService(webdavConfigSvc),
			application.NewService(opmlSvc),
			application.NewService(opmlBackupSvc),
			application.NewService(notifSvc),
			application.NewService(dockService),
		},
	})

	gh, err := github.New(github.Config{
		Repository:    repo,
		ChecksumAsset: "SHA256SUMS",
	})
	if err != nil {
		log.Fatalf("github.New: %v", err)
	}

	// langFn 每次现读设置里的语言（用户可能在运行时切换），建窗时注入更新窗口做 i18n。
	updLangFn := func() string {
		if s, err := st.GetSettings(); err == nil && s.Language != "" {
			return s.Language
		}
		return "en"
	}

	// 自建「Software Update」子窗口 + 控制器。Window 用 WindowNone：Updater 只做无头的
	// 检查/下载/安装并广播事件，UI 完全由我们自己驱动（先展示、由用户决定是否更新）。
	// i18n 字典启动时算好、语言建窗时现读注入 HTML。
	updWin := &softwareUpdateWindow{app: app, dict: updaterI18nDict(), langFn: updLangFn}

	if err := app.Updater.Init(updater.Config{
		CurrentVersion: currentVersion,
		Providers:      []updater.Provider{gh},
		Window:         updater.WindowNone,
	}); err != nil {
		log.Fatalf("Updater.Init: %v", err)
	}

	updCtrl := newUpdateController(app, app.Updater, updWin, currentVersion)
	sysSvc.CheckUpdateFn = updCtrl.check
	sysSvc.CheckSilentFn = updCtrl.checkSilent

	menu := application.DefaultApplicationMenu()

	// 「检查更新…」点击处理：与前端 SystemService.CheckForUpdates() 走同一条路。
	checkForUpdates := func(*application.Context) {
		updCtrl.check()
	}

	if appMenu := menu.FindByRole(application.AppMenu); appMenu != nil {
		sub := appMenu.GetSubmenu()
		sub.Clear()
		sub.AddRole(application.About)
		sub.Add("Check for Updates…").OnClick(checkForUpdates)
		sub.AddSeparator()
		sub.AddRole(application.ServicesMenu)
		sub.AddSeparator()
		sub.AddRole(application.Hide)
		sub.AddRole(application.HideOthers)
		sub.AddRole(application.UnHide)
		sub.AddSeparator()
		sub.AddRole(application.Quit)
	} else if help := menu.FindByRole(application.HelpMenu); help != nil {
		help.GetSubmenu().Add("Check for Updates…").OnClick(checkForUpdates)
	}

	app.Menu.SetApplicationMenu(menu)

	// 点击通知 → 调起窗口 + 向前端推送 article ID，前端自行定位。
	var mainWindow application.Window
	notifSvc.OnNotificationResponse(func(result notifications.NotificationResult) {
		if result.Error != nil {
			log.Printf("notification response error: %v", result.Error)
			return
		}
		rawID, _ := result.Response.UserInfo["articleId"].(string)
		if rawID == "" {
			return
		}
		idStr := strings.TrimPrefix(rawID, "article:")
		articleID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return
		}
		if mainWindow != nil {
			mainWindow.UnMinimise()
			mainWindow.Restore()
			mainWindow.Focus()
		}
		app.Event.Emit("notification:open", map[string]any{"articleId": articleID})
	})

	// macOS：应用启动完毕后请求通知权限（用户授权后通知才能弹出）。
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(event *application.ApplicationEvent) {
		granted, err := notifSvc.RequestNotificationAuthorization()
		if err != nil {
			log.Printf("notification authorization failed: %v", err)
			return
		}
		if !granted {
			log.Printf("notification authorization was not granted")
		}
	})

	// 启动后台调度（此时 application.Get() 已可用于事件推送）。
	// WebView 尚未上报 navigator.onLine 前按离线处理，避免断网启动时先发出一轮请求。
	sch.SetOfflineMode(true)
	sch.Start(context.Background())

	// OPML 云备份为纯手动触发，无后台任务需要启动。

	// 退出时优雅停机：先停调度，打断可能在进行的手动备份，再关数据库。
	app.OnShutdown(func() {
		sch.Stop()
		api.StopOPMLBackup(opmlBackupSvc)
		_ = st.Close()
	})

	windowWidth, windowHeight := savedWindowSize(settings)

	mainWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Clip",
		Width:     windowWidth,
		Height:    windowHeight,
		MinWidth:  minWindowWidth,
		MinHeight: minWindowHeight,
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTranslucent,
			TitleBar: application.MacTitleBarHiddenInset,
		},
		URL: "/",
	})
	mainWindow.OnWindowEvent(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		saveWindowSize(st, mainWindow)
	})

	sysSvc.Window = mainWindow

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
