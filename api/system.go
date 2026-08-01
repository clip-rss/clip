package api

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"time"

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
	if s.ChangelogURL == "" {
		return "", fmt.Errorf("changelog url not configured")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(s.ChangelogURL)
	if err != nil {
		return "", fmt.Errorf("fetch changelog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch changelog: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10)) // 512 KB cap
	if err != nil {
		return "", fmt.Errorf("read changelog: %w", err)
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
