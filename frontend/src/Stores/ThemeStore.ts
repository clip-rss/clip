import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { ThemeMode, ThemePreference } from '../Types'

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
  root.classList.remove('dark', 'sepia')
  if (mode === 'dark') {
    root.classList.add('dark')
  } else if (mode === 'sepia') {
    root.classList.add('sepia')
  }
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
