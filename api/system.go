package api

import (
	"runtime"

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
