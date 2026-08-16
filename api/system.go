package api

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/i18n"
	"github.com/wailsapp/wails/v3/pkg/application"
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

	// AppVersion 由 main.go 注入当前应用版本号（与更新检查使用的版本一致）。
	AppVersion string

	// ChangelogURL 由 main.go 注入，指向 CHANGELOG.md 的 raw 地址。
	ChangelogURL string

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

// FetchChangelog 从 ChangelogURL 拉取原始 Markdown 文本返回给前端渲染。
func (s *SystemService) FetchChangelog() (string, error) {
	lang := i18n.English
	if s.LanguageFn != nil {
		lang = s.LanguageFn()
	}
	if s.ChangelogURL == "" {
		return "", errors.New(i18n.T(lang, "changelog.notConfigured"))
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(s.ChangelogURL)
	if err != nil {
		return "", i18n.Error(lang, "changelog.fetchFailed", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New(i18n.T(lang, "changelog.badStatus", resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10)) // 512 KB cap
	if err != nil {
		return "", i18n.Error(lang, "changelog.readFailed", err)
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
	if base == "" || base == "." || base == "/" {
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
