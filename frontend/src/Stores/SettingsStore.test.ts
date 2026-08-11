import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest'

vi.mock('../Utils', () => ({
  SettingsService: {
    GetSettings: vi.fn(),
    UpdateSettings: vi.fn(),
  },
  toApiError: (e: unknown) => String(e),
}))

import { SettingsService } from '../Utils'
import { useSettingsStore } from './SettingsStore'
import type { Settings } from '../Types'

const GetSettings = SettingsService.GetSettings as Mock
const UpdateSettings = SettingsService.UpdateSettings as Mock

const defaults: Settings = {
  theme: 'system',
  language: 'zh',
  defaultUpdateInterval: 30,
  defaultMaxItems: 100,
  notificationMode: 'each',
  showUnreadBadge: true,
  autoMarkReadDelay: 0,
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

beforeEach(() => {
  vi.clearAllMocks()
  useSettingsStore.setState({ settings: null, loading: false, error: null })
})

describe('SettingsStore', () => {
  it('load 从后端拉取设置', async () => {
    GetSettings.mockResolvedValue({ ...defaults, language: 'en' })
    await useSettingsStore.getState().load()
    expect(useSettingsStore.getState().settings?.language).toBe('en')
    expect(useSettingsStore.getState().loading).toBe(false)
  })

  it('load 失败时保存错误', async () => {
    GetSettings.mockRejectedValue(new Error('db down'))
    await useSettingsStore.getState().load()
    expect(useSettingsStore.getState().error).toMatch(/db down/)
  })

  it('update 乐观写入并调用后端', async () => {
    useSettingsStore.setState({ settings: defaults })
    UpdateSettings.mockResolvedValue(undefined)
    await useSettingsStore.getState().update({ language: 'en' })
    expect(useSettingsStore.getState().settings?.language).toBe('en')
    expect(UpdateSettings).toHaveBeenCalledWith(
      expect.objectContaining({ language: 'en' }),
    )
  })

  it('update 失败时回滚', async () => {
    useSettingsStore.setState({ settings: defaults })
    UpdateSettings.mockRejectedValue(new Error('fail'))
    await useSettingsStore.getState().update({ language: 'en' })
    // 乐观先设英文，失败后回退到中文
    expect(useSettingsStore.getState().settings?.language).toBe('zh')
    expect(useSettingsStore.getState().error).toMatch(/fail/)
  })

  it('setNotificationMode 基于 update 实现', async () => {
    useSettingsStore.setState({ settings: defaults })
    UpdateSettings.mockResolvedValue(undefined)
    await useSettingsStore.getState().setNotificationMode('off')
    expect(useSettingsStore.getState().settings?.notificationMode).toBe('off')
  })
})
