import { create } from 'zustand'
import type { ThemeMode, ThemePreference } from '../Types'
import { SystemService } from '../Utils/Api'
import { readThemeCache, writeThemeCache } from './PrefsCache'
import { useSettingsStore } from './SettingsStore'

interface ThemeState {
  preference: ThemePreference
  resolved: ThemeMode
  setPreference: (preference: ThemePreference) => void
}

const PREFERENCES: ThemePreference[] = ['light', 'dark', 'sepia', 'system']

/** 校验后端传入的主题偏好，非法值回退 system。 */
function normalizePreference(raw: string): ThemePreference {
  return PREFERENCES.includes(raw as ThemePreference)
    ? (raw as ThemePreference)
    : 'system'
}

function resolveTheme(preference: ThemePreference): ThemeMode {
  if (preference === 'system') {
    const isDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    return isDark ? 'dark' : 'light'
  }
  return preference
}

function applyTheme(mode: ThemeMode): void {
  const root = document.documentElement
  // 同时清理历史遗留的裸类名（'light'/'dark'/'sepia'）：它们与 Tailwind 内置滤镜
  // 工具类同名，若残留在 <html> 上会给整页叠加真实滤镜。
  root.classList.remove('theme-dark', 'theme-sepia', 'light', 'dark', 'sepia')
  if (mode === 'dark') {
    root.classList.add('theme-dark')
  } else if (mode === 'sepia') {
    root.classList.add('theme-sepia')
  }
  // 同步 Windows 原生窗口标题栏主题
  SystemService.SetTheme(mode)
}

// 初值取自 localStorage 首帧缓存，与 index.html 内联脚本已经涂上的 class 保持一致；
// 后端设置载入后由下方订阅覆盖为真值。
const initialPreference = readThemeCache() ?? 'system'

export const useThemeStore = create<ThemeState>()((set) => ({
  preference: initialPreference,
  resolved: resolveTheme(initialPreference),

  setPreference(preference: ThemePreference) {
    const resolved = resolveTheme(preference)
    applyTheme(resolved)
    writeThemeCache(preference)
    set({ preference, resolved })
    // 本地已应用，记下来免得下面写后端触发订阅时再涂一遍（applyTheme 含一次
    // SetTheme 原生调用）。写失败回滚时订阅拿到的是旧值，与此处不等，仍会正确改回。
    lastAppliedTheme = preference
    // 后端为数据源；失败时 SettingsStore 自行回滚，订阅会把本地改回去。
    void useSettingsStore.getState().update({ theme: preference })
  },
}))

// 首帧：把缓存值应用到 DOM 与原生标题栏。index.html 只涂了 class，
// SystemService.SetTheme 仍需在此补一次。
applyTheme(useThemeStore.getState().resolved)

// 后端设置 → 本地：载入与跨端同步后的变更都经由这里落到 DOM。
// null 初值保证首次载入必定应用一次，即便偏好与缓存值相同。
let lastAppliedTheme: string | null = null

useSettingsStore.subscribe((state) => {
  const theme = state.settings?.theme
  if (theme === undefined || theme === lastAppliedTheme) return
  lastAppliedTheme = theme

  const preference = normalizePreference(theme)
  const resolved = resolveTheme(preference)
  applyTheme(resolved)
  writeThemeCache(preference)
  useThemeStore.setState({ preference, resolved })
})

// 监听系统主题变化，当 preference 为 system 时自动切换
function initSystemThemeListener(): void {
  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  mediaQuery.addEventListener('change', () => {
    const { preference } = useThemeStore.getState()
    if (preference === 'system') {
      const resolved = resolveTheme('system')
      applyTheme(resolved)
      useThemeStore.setState({ resolved })
    }
  })
}

initSystemThemeListener()
