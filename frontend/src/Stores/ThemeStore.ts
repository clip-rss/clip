import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { ThemeMode, ThemePreference } from '../Types'
import { SystemService } from '../Utils/Api'

interface ThemeState {
  preference: ThemePreference
  resolved: ThemeMode
  setPreference: (preference: ThemePreference) => void
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

export const useThemeStore = create<ThemeState>()(
  persist(
    (set) => ({
      preference: 'system',
      resolved: resolveTheme('system'),
      setPreference(preference: ThemePreference) {
        const resolved = resolveTheme(preference)
        applyTheme(resolved)
        set({ preference, resolved })
      },
    }),
    {
      name: 'clip-theme',
      onRehydrateStorage() {
        return (state) => {
          if (state) {
            const resolved = resolveTheme(state.preference)
            applyTheme(resolved)
            state.resolved = resolved
          }
        }
      },
    },
  ),
)

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
