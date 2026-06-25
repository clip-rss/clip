//go:build windows

package api

import "github.com/wailsapp/wails/v3/pkg/w32"

// SetTheme 将原生窗口标题栏切换为指定的主题。
//
// mode 的值：
//   - "dark"  — 暗色主题
//   - 其它值  — 亮色主题（默认）
func (s *SystemService) SetTheme(mode string) {
	if s.Window == nil {
		return
	}
	isDark := mode == "dark"
	hwnd := uintptr(s.Window.NativeWindow())
	w32.SetTheme(hwnd, isDark)
}
