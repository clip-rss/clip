package api

import (
	"context"
	"errors"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/i18n"
	"github.com/clip-rss/clip/internal/store"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// 更新日志抓取的上限。fetcher.Client 自带重试，这里的超时覆盖含重试的整轮。
const (
	changelogFetchTimeout = 30 * time.Second
	changelogMaxBytes     = 512 << 10
)

// SystemService 提供与运行平台相关的信息，暴露给前端用于平台差异化渲染。
type SystemService struct {
	Window application.Window

	// CheckUpdateFn 由 main.go 延迟注入，触发「检查更新」流程（弹 Software Update
	// 窗口并只做检查）。前端调 SystemService.CheckForUpdates() 即可触发。
	CheckUpdateFn func()

	// CheckSilentFn 由 main.go 延迟注入，触发静默更新检查（后台运行，不弹窗）。
	// 前端调 SystemService.CheckForUpdatesSilent() 即可触发。
	CheckSilentFn func()

	// NoUpdateFn 由 main.go 延迟注入，报告最近一次更新检查是否明确确认「已是最新」。
	// 为 nil 时视为「未确认」，更新日志缓存不会被复用也不会被写入。
	NoUpdateFn func() bool

	// AppVersion 由 main.go 注入当前应用版本号（与更新检查使用的版本一致）。
	AppVersion string

	// ChangelogURL 由 main.go 注入，指向 CHANGELOG.md 的 raw 地址。
	ChangelogURL string

	// Store 由 main 注入，用于持久化更新日志缓存。
	// 为 nil 时（如单测）退化为每次都重新抓取。
	Store *store.Store

	// OnlineChangedFn 由 main 注入，把 WebView 的在线状态同步给后台调度器。
	OnlineChangedFn func(online bool)

	// LanguageFn 由 main 注入，每次生成用户可见提示时读取当前语言。
	LanguageFn func() string

	// HTTPClient 由 main 注入，用于下载图片等二进制资源（复用代理/超时/重试）。
	// 为 nil 时（如单测）下载功能不可用。
	HTTPClient *fetcher.Client
}

// Platform 返回当前运行的操作系统标识。
//
// 仅区分本项目支持的两个桌面平台：
//   - "mac"     — macOS（runtime.GOOS == "darwin"）
//   - "windows" — Windows 及其它
func (s *SystemService) Platform() string {
	if runtime.GOOS == "darwin" {
		return "mac"
	}
	return "windows"
}

// Version 返回当前应用版本号，供设置-关于页展示。
func (s *SystemService) Version() string {
	return s.AppVersion
}

// CheckForUpdates 供前端调用（如设置-关于页的「检查更新」按钮），触发与菜单
// 「Check for Updates…」相同的更新流程：弹出 Software Update 窗口 + 只做检查。
func (s *SystemService) CheckForUpdates() {
	if s.CheckUpdateFn != nil {
		s.CheckUpdateFn()
	}
}

// CheckForUpdatesSilent 供前端在启动时调用，触发静默更新检查（后台运行，不弹窗）。
// 若有新版本可用，会通过 "clip:update:available" 事件通知前端。
func (s *SystemService) CheckForUpdatesSilent() {
	if s.CheckSilentFn != nil {
		s.CheckSilentFn()
	}
}

// FetchChangelog 返回更新日志原始 Markdown 文本，供前端渲染。
//
// 优先读本地缓存：changelogURL 指向仓库 main 分支，内容就是「最新已发布版本」的日志，
// 所以当版本号与缓存一致、且更新检查已确认无新版时，远端内容不可能变，直接复用即可，
// 不再发请求。检出新版（或尚未检查）时缓存立即失效，保证升级前后都能看到对应的日志。
//
// 抓取失败时若存有本版本的缓存则回退返回缓存，让离线状态下仍能查看更新日志。
func (s *SystemService) FetchChangelog() (string, error) {
	cache, cached := s.changelogCache()
	noUpdate := s.NoUpdateFn != nil && s.NoUpdateFn()

	// 命中条件必须同时含 noUpdate：只比版本号不够——用户长时间不重启、期间上游发了新版
	// 并被检出时，本地版本号依然等于缓存里的版本，但远端日志已变。
	if cached && noUpdate {
		return cache.Markdown, nil
	}

	md, err := s.fetchChangelogRemote()
	if err != nil {
		if cached {
			return cache.Markdown, nil
		}
		return "", err
	}

	// 只在确认无新版时回写：有新版待装时抓到的是新版日志，缓存下来会在升级后被
	// 当作「当前版本」的日志复用（版本号那时才追上），反而把内容锁死在错误的一版。
	if noUpdate && s.Store != nil {
		if err := s.Store.SaveChangelogCache(store.ChangelogCache{
			Version:  s.AppVersion,
			Markdown: md,
		}); err != nil {
			log.Printf("changelog: save cache: %v", err)
		}
	}
	return md, nil
}

// changelogCache 读取与当前版本匹配的更新日志缓存。
// 缓存不存在、版本不符或已损坏时返回 (零值, false)——损坏只记日志，随后重新抓取即可自愈。
func (s *SystemService) changelogCache() (store.ChangelogCache, bool) {
	if s.Store == nil {
		return store.ChangelogCache{}, false
	}
	cache, found, err := s.Store.GetChangelogCache()
	if err != nil {
		log.Printf("changelog: read cache: %v", err)
		return store.ChangelogCache{}, false
	}
	if !found || cache.Version != s.AppVersion || cache.Markdown == "" {
		return store.ChangelogCache{}, false
	}
	return cache, true
}

// fetchChangelogRemote 从 ChangelogURL 抓取原始 Markdown 文本。
//
// 走注入的 fetcher.Client 而非自建裸客户端：ChangelogURL 指向 raw.githubusercontent.com，
// 在部分网络环境下需要代理才能访问，而代理配置只存在于这个共享客户端上。顺带也拿到了
// 它的重试与统一超时。
func (s *SystemService) fetchChangelogRemote() (string, error) {
	lang := i18n.English
	if s.LanguageFn != nil {
		lang = s.LanguageFn()
	}
	if s.ChangelogURL == "" {
		return "", errors.New(i18n.T(lang, "changelog.notConfigured"))
	}
	if s.HTTPClient == nil {
		return "", errors.New(i18n.T(lang, "changelog.clientUnavailable"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), changelogFetchTimeout)
	defer cancel()
	body, _, err := s.HTTPClient.Get(ctx, s.ChangelogURL)
	if err != nil {
		return "", i18n.Error(lang, "changelog.fetchFailed", err)
	}
	if len(body) > changelogMaxBytes {
		body = body[:changelogMaxBytes]
	}
	return string(body), nil
}

// IsOnline 探测网络连通性，返回 true 表示在线，false 表示离线。
//
// 实现方式：尝试连接 Google Public DNS (8.8.8.8:53)，超时 2 秒。
// 该方法简单快速，但无法区分"本地网络正常但外网不通"的情况。
func (s *SystemService) IsOnline() bool {
	conn, err := net.DialTimeout("udp", "8.8.8.8:53", 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

// SetOnline 接收前端 navigator.onLine 变化并同步后台网络模式。
func (s *SystemService) SetOnline(online bool) {
	if s.OnlineChangedFn != nil {
		s.OnlineChangedFn(online)
	}
}

// DownloadImage 下载图片到用户指定的目录：弹出保存对话框选择位置后写盘。
//
// 返回 (true, nil) 表示已保存；(false, nil) 表示用户取消；(false, err) 表示失败。
// 文件名从 URL 路径推断，无法推断时回退为 "image"；扩展名缺失时按 Content-Type 补全。
func (s *SystemService) DownloadImage(rawURL string) (bool, error) {
	lang := i18n.English
	if s.LanguageFn != nil {
		lang = s.LanguageFn()
	}

	if s.HTTPClient == nil {
		return false, errors.New(i18n.T(lang, "image.downloadUnavailable"))
	}
	if strings.TrimSpace(rawURL) == "" {
		return false, errors.New(i18n.T(lang, "image.urlEmpty"))
	}

	app := application.Get()
	if app == nil {
		return false, errors.New(i18n.T(lang, "app.unavailable"))
	}

	// 先下载，再弹保存对话框：避免用户选完路径后才发现下载失败。
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	body, contentType, err := s.HTTPClient.Get(ctx, rawURL)
	if err != nil {
		return false, i18n.Error(lang, "image.downloadFailed", err)
	}

	filename := imageFilename(rawURL, contentType)
	dest, err := app.Dialog.SaveFile().
		SetMessage(i18n.T(lang, "image.save")).
		SetFilename(filename).
		AddFilter(i18n.T(lang, "image.fileFilter"), "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.svg;*.avif;*.bmp").
		PromptForSingleSelection()
	if err != nil {
		return false, err
	}
	if dest == "" {
		return false, nil // 用户取消
	}
	if err := os.WriteFile(dest, body, 0o644); err != nil {
		return false, i18n.Error(lang, "image.writeFailed", err)
	}
	return true, nil
}

// imageFilename 从图片 URL 推断一个合理的保存文件名。
// 无法从路径推断时回退为 "image"，扩展名缺失时按 Content-Type 补全。
func imageFilename(rawURL, contentType string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "image"
	}
	base := filepath.Base(u.Path)
	if base == "" || base == "." || base == "/" || base == string(filepath.Separator) || base == "\\" {
		base = "image"
	}
	// 去掉查询串可能残留的非法字符（filepath.Base 已处理路径，这里兜底）。
	base = strings.TrimSpace(base)
	if base == "" {
		base = "image"
	}
	if filepath.Ext(base) == "" {
		if ext := extFromContentType(contentType); ext != "" {
			base += ext
		}
	}
	return base
}

// extFromContentType 把常见图片 Content-Type 映射为文件扩展名。
func extFromContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "image/avif":
		return ".avif"
	case "image/bmp":
		return ".bmp"
	default:
		return ""
	}
}
