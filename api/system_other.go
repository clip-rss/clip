//go:build !windows

package api

// SetTheme 在非 Windows 平台上是空操作。
//
// macOS 使用隐藏式标题栏（MacTitleBarHiddenInset），无需切换原生主题。
func (s *SystemService) SetTheme(mode string) {
	// no-op on non-Windows platforms
}
