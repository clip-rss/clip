import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest'
import { THEME_CACHE_KEY } from './PrefsCache'
import type { Settings } from '../Types'

// ThemeStore 从 '../Utils/Api' 取 SystemService，SettingsStore 从 '../Utils' 取
// SettingsService，两条路径都要挡掉，否则会真的去调 Wails 绑定。
vi.mock('../Utils/Api', () => ({
  SystemService: { SetTheme: vi.fn() },
}))
vi.mock('../Utils', () => ({
  SettingsService: { GetSettings: vi.fn(), UpdateSettings: vi.fn() },
  toApiError: (e: unknown) => String(e),
}))

/* ---------- matchMedia 替身 ---------- */

// 本版 jsdom 不提供 window.matchMedia，必须自备。
// 同时留出改 matches 与手动触发 change 的钩子，用于验证「跟随系统」分支。
const media = { matches: false, listeners: [] as Array<() => void> }

function installMatchMedia(): void {
  window.matchMedia = ((query: string) => ({
    media: query,
    get matches() {
      return media.matches
    },
    onchange: null,
    addEventListener: (_: string, cb: () => void) => {
      media.listeners.push(cb)
    },
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia
}

installMatchMedia()

/** 触发一次系统主题变化。 */
function emitSystemThemeChange(dark: boolean): void {
  media.matches = dark
  media.listeners.forEach((cb) => cb())
}

const baseSettings: Settings = {
  theme: 'system',
  language: 'zh',
  defaultUpdateInterval: 30,
  defaultMaxItems: 100,
  notificationMode: 'each',
  showUnreadBadge: true,
  autoMarkReadDelay: 0,
  launchMinimized: false,
  windowWidth: 1200,
  windowHeight: 800,
  proxyHost: '',
  proxyPort: 0,
  reduceMotion: false,
  showFocusIndicator: true,
  readerFontFamily: 'sans',
  readerFontSize: 16,
  readerLineHeight: 1.8,
  readerWidth: '640',
  readerBackground: 'default',
}

/**
 * ThemeStore 在模块加载时就取缓存定初值、涂 DOM、订阅 SettingsStore，
 * 所以每个用例都要拿一份全新模块，且必须与同批次的 SettingsStore 配对
 * —— resetModules 后两者都是新实例，跨批次引用订阅不到。
 */
async function freshModules() {
  vi.resetModules()
  const themeMod = await import('./ThemeStore')
  const settingsMod = await import('./SettingsStore')
  return {
    useThemeStore: themeMod.useThemeStore,
    useSettingsStore: settingsMod.useSettingsStore,
  }
}

const cls = () => document.documentElement.classList

beforeEach(async () => {
  vi.clearAllMocks()
  localStorage.clear()
  cls().remove('theme-dark', 'theme-sepia', 'light', 'dark', 'sepia')
  media.matches = false
  media.listeners = []
  installMatchMedia()
  const { SettingsService } = await import('../Utils')
  ;(SettingsService.UpdateSettings as Mock).mockResolvedValue(undefined)
})

describe('ThemeStore 初值', () => {
  it('无缓存时为 system', async () => {
    const { useThemeStore } = await freshModules()
    expect(useThemeStore.getState().preference).toBe('system')
  })

  it('取自 v2 缓存', async () => {
    localStorage.setItem(THEME_CACHE_KEY, JSON.stringify({ v: 2, preference: 'sepia' }))
    const { useThemeStore } = await freshModules()
    expect(useThemeStore.getState().preference).toBe('sepia')
  })

  // 老用户升级后的第一帧：迁移还没跑，缓存仍是 zustand persist 的形状。
  it('取自旧格式缓存（升级首帧不闪）', async () => {
    localStorage.setItem(
      THEME_CACHE_KEY,
      JSON.stringify({ state: { preference: 'dark' }, version: 0 }),
    )
    const { useThemeStore } = await freshModules()
    expect(useThemeStore.getState().preference).toBe('dark')
    expect(cls().contains('theme-dark')).toBe(true)
  })
})

describe('setPreference', () => {
  it('涂 DOM class、写缓存、写后端', async () => {
    const { useThemeStore, useSettingsStore } = await freshModules()
    useSettingsStore.setState({ settings: baseSettings })
    const { SettingsService } = await import('../Utils')

    useThemeStore.getState().setPreference('dark')

    expect(cls().contains('theme-dark')).toBe(true)
    expect(JSON.parse(localStorage.getItem(THEME_CACHE_KEY)!)).toEqual({
      v: 2,
      preference: 'dark',
    })
    await vi.waitFor(() =>
      expect(SettingsService.UpdateSettings).toHaveBeenCalledWith(
        expect.objectContaining({ theme: 'dark' }),
      ),
    )
  })

  it('切走时清掉上一个主题的 class', async () => {
    const { useThemeStore } = await freshModules()
    useThemeStore.getState().setPreference('dark')
    expect(cls().contains('theme-dark')).toBe(true)

    useThemeStore.getState().setPreference('sepia')
    expect(cls().contains('theme-dark')).toBe(false)
    expect(cls().contains('theme-sepia')).toBe(true)
  })

  // 回归防线：裸类名 'dark'/'sepia' 与 Tailwind 内置滤镜工具类同名，
  // 残留会给整页叠加真实滤镜。index.html 早先就是错涂了裸类名。
  it('清理历史遗留的裸类名', async () => {
    cls().add('dark', 'sepia')
    const { useThemeStore } = await freshModules()
    useThemeStore.getState().setPreference('light')
    expect(cls().contains('dark')).toBe(false)
    expect(cls().contains('sepia')).toBe(false)
  })

  // 本地已涂好，写后端触发订阅时不该再涂一遍：applyTheme 内含一次
  // SystemService.SetTheme 原生调用，重复调用是白费的 IPC。
  it('不因写后端而重复应用主题', async () => {
    const { useThemeStore, useSettingsStore } = await freshModules()
    useSettingsStore.setState({ settings: baseSettings })
    const { SystemService } = await import('../Utils/Api')
    const { SettingsService } = await import('../Utils')
    ;(SystemService.SetTheme as Mock).mockClear()

    useThemeStore.getState().setPreference('dark')
    await vi.waitFor(() =>
      expect(SettingsService.UpdateSettings).toHaveBeenCalled(),
    )

    expect(SystemService.SetTheme).toHaveBeenCalledTimes(1)
  })

  // 上面那条去重不能把回滚一起吃掉：后端写失败时 SettingsStore 回退到旧值，
  // 订阅必须把界面也改回去，否则用户看到的主题与实际存储不一致。
  it('后端写入失败时回滚到旧主题', async () => {
    const { useThemeStore, useSettingsStore } = await freshModules()
    useSettingsStore.setState({ settings: { ...baseSettings, theme: 'light' } })
    const { SettingsService } = await import('../Utils')
    ;(SettingsService.UpdateSettings as Mock).mockRejectedValue(
      new Error('disk full'),
    )

    useThemeStore.getState().setPreference('dark')
    expect(cls().contains('theme-dark')).toBe(true) // 乐观应用

    await vi.waitFor(() =>
      expect(useThemeStore.getState().preference).toBe('light'),
    )
    expect(cls().contains('theme-dark')).toBe(false)
  })
})

describe('后端设置 → 主题', () => {
  it('载入后应用后端主题', async () => {
    const { useThemeStore, useSettingsStore } = await freshModules()
    useSettingsStore.setState({ settings: { ...baseSettings, theme: 'sepia' } })

    expect(useThemeStore.getState().preference).toBe('sepia')
    expect(cls().contains('theme-sepia')).toBe(true)
    // 后端值同时回写首帧缓存，下次启动即可直接涂对。
    expect(JSON.parse(localStorage.getItem(THEME_CACHE_KEY)!).preference).toBe('sepia')
  })

  // 同步会拉到更高版本客户端写入的载荷，可能含本端不认识的主题名。
  it('非法主题值回落 system', async () => {
    const { useThemeStore, useSettingsStore } = await freshModules()
    useSettingsStore.setState({
      settings: { ...baseSettings, theme: 'hologram' },
    })
    expect(useThemeStore.getState().preference).toBe('system')
  })

  it('偏好与缓存值相同时首次载入也会应用一次', async () => {
    localStorage.setItem(THEME_CACHE_KEY, JSON.stringify({ v: 2, preference: 'dark' }))
    const { useThemeStore, useSettingsStore } = await freshModules()
    cls().remove('theme-dark') // 抹掉模块加载时涂的，验证订阅确实会补涂

    useSettingsStore.setState({ settings: { ...baseSettings, theme: 'dark' } })

    expect(cls().contains('theme-dark')).toBe(true)
    expect(useThemeStore.getState().preference).toBe('dark')
  })
})

describe('跟随系统', () => {
  it('preference 为 system 时随系统切换', async () => {
    const { useThemeStore } = await freshModules()
    expect(useThemeStore.getState().resolved).toBe('light')

    emitSystemThemeChange(true)

    expect(useThemeStore.getState().resolved).toBe('dark')
    expect(cls().contains('theme-dark')).toBe(true)
  })

  it('已显式选定主题时忽略系统变化', async () => {
    const { useThemeStore } = await freshModules()
    useThemeStore.getState().setPreference('light')

    emitSystemThemeChange(true)

    expect(useThemeStore.getState().resolved).toBe('light')
    expect(cls().contains('theme-dark')).toBe(false)
  })
})
