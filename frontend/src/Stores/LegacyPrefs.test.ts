import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest'
import type { Settings } from '../Types'

vi.mock('../Utils', () => ({
  SettingsService: {
    GetSettings: vi.fn(),
    UpdateSettings: vi.fn(),
  },
  toApiError: (e: unknown) => String(e),
}))

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
 * 载入被测模块。
 *
 * LegacyPrefs 在模块加载时就取 localStorage 快照，所以每个用例必须先布置
 * localStorage、再重置模块注册表重新 import，否则拿到的是上一个用例的快照。
 */
async function loadModules(seed: () => void) {
  vi.resetModules()
  localStorage.clear()
  seed()
  const settingsMod = await import('./SettingsStore')
  const legacyMod = await import('./LegacyPrefs')
  return { useSettingsStore: settingsMod.useSettingsStore, ...legacyMod }
}

/** 旧格式 = zustand persist 的形状。 */
function seedLegacyTheme(preference: string): void {
  localStorage.setItem(
    'clip-theme',
    JSON.stringify({ state: { preference }, version: 0 }),
  )
}

function seedLegacyReader(state: Record<string, unknown>): void {
  localStorage.setItem('clip-reader', JSON.stringify({ state, version: 0 }))
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('migrateLegacyPrefs', () => {
  it('把旧主题与旧排版搬到后端', async () => {
    const m = await loadModules(() => {
      seedLegacyTheme('dark')
      seedLegacyReader({
        fontFamily: 'serif',
        fontSize: 18,
        lineHeight: 2.0,
        width: 'full',
        background: 'sepia',
      })
    })
    const { SettingsService } = await import('../Utils')
    const UpdateSettings = SettingsService.UpdateSettings as Mock
    UpdateSettings.mockResolvedValue(undefined)
    m.useSettingsStore.setState({ settings: baseSettings })

    expect(m.hasLegacyPrefs()).toBe(true)
    await m.migrateLegacyPrefs()

    expect(UpdateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        theme: 'dark',
        readerFontFamily: 'serif',
        readerFontSize: 18,
        readerLineHeight: 2.0,
        readerWidth: 'full',
        readerBackground: 'sepia',
      }),
    )
    // clip-reader 迁移后删除；clip-theme 保留作首帧缓存（见 PrefsCache）
    expect(localStorage.getItem('clip-reader')).toBeNull()
  })

  it('全新安装为无操作', async () => {
    const m = await loadModules(() => {})
    const { SettingsService } = await import('../Utils')
    const UpdateSettings = SettingsService.UpdateSettings as Mock
    m.useSettingsStore.setState({ settings: baseSettings })

    expect(m.hasLegacyPrefs()).toBe(false)
    await m.migrateLegacyPrefs()
    expect(UpdateSettings).not.toHaveBeenCalled()
  })

  it('新版缓存格式不被误判为待迁移', async () => {
    const m = await loadModules(() => {
      localStorage.setItem(
        'clip-theme',
        JSON.stringify({ v: 2, preference: 'dark' }),
      )
    })
    expect(m.hasLegacyPrefs()).toBe(false)
  })

  // 关键防线：先删 key 再写后端，一旦写失败用户设置就永久丢了。
  it('后端写入失败时不清理旧 key', async () => {
    const m = await loadModules(() => {
      seedLegacyReader({ fontFamily: 'mono' })
    })
    const { SettingsService } = await import('../Utils')
    const UpdateSettings = SettingsService.UpdateSettings as Mock
    UpdateSettings.mockRejectedValue(new Error('backend down'))
    m.useSettingsStore.setState({ settings: baseSettings })

    await m.migrateLegacyPrefs()

    expect(localStorage.getItem('clip-reader')).not.toBeNull()
    expect(m.hasLegacyPrefs()).toBe(true)
  })

  it('后端设置未载入时跳过本轮', async () => {
    const m = await loadModules(() => {
      seedLegacyTheme('sepia')
    })
    const { SettingsService } = await import('../Utils')
    const UpdateSettings = SettingsService.UpdateSettings as Mock
    m.useSettingsStore.setState({ settings: null })

    await m.migrateLegacyPrefs()

    expect(UpdateSettings).not.toHaveBeenCalled()
    expect(m.hasLegacyPrefs()).toBe(true) // 下次启动再试
  })

  it('损坏的 JSON 不致崩', async () => {
    const m = await loadModules(() => {
      localStorage.setItem('clip-theme', '{ not json')
      localStorage.setItem('clip-reader', 'garbage')
    })
    expect(m.hasLegacyPrefs()).toBe(false)
  })

  it('旧排版里的非法值按默认值迁移', async () => {
    const m = await loadModules(() => {
      seedLegacyReader({ fontFamily: 'wingdings', fontSize: 42 })
    })
    const { SettingsService } = await import('../Utils')
    const UpdateSettings = SettingsService.UpdateSettings as Mock
    UpdateSettings.mockResolvedValue(undefined)
    m.useSettingsStore.setState({ settings: baseSettings })

    await m.migrateLegacyPrefs()

    expect(UpdateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        readerFontFamily: 'sans',
        readerFontSize: 16,
      }),
    )
  })
})
