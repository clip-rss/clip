import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest'

vi.mock('../Utils', () => ({
  SettingsService: {
    GetSettings: vi.fn(),
    UpdateSettings: vi.fn(),
  },
  toApiError: (e: unknown) => String(e),
}))

import { SettingsService } from '../Utils'
import {
  useFocusReaderStore,
  DEFAULT_FOCUS_READER_PREFS,
  toFocusReaderPrefs,
  focusPrefsUnset,
  seedFocusPrefsFromReader,
} from './FocusReaderStore'
import { useReaderStore, DEFAULT_READER_PREFS } from './ReaderStore'
import { useSettingsStore } from './SettingsStore'
import type { Settings } from '../Types'

const UpdateSettings = SettingsService.UpdateSettings as Mock

/** 完整后端设置基线，便于按需覆盖单个字段。 */
const baseSettings: Settings = {
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
  readerFontFamily: 'serif',
  readerFontSize: 20,
  readerLineHeight: 2.0,
  readerWidth: '800',
  readerBackground: 'sepia',
  focusFontFamily: 'sans',
  focusFontSize: 16,
  focusLineHeight: 1.8,
  focusBackground: 'default',
}

/**
 * 老库那份设置：JSON blob 里没有 focus* 这些 key，Go 反序列化后全是零值。
 * 播种逻辑认的就是这个形态。
 */
const legacySettings: Settings = {
  ...baseSettings,
  focusFontFamily: '',
  focusFontSize: 0,
  focusLineHeight: 0,
  focusBackground: '',
}

/** 阅读视图那套取默认值、专注模式取另一套值：用来验证两者互不影响。 */
const splitSettings: Settings = {
  ...baseSettings,
  readerFontFamily: 'sans',
  readerFontSize: 16,
  readerLineHeight: 1.8,
  readerWidth: '640',
  readerBackground: 'default',
  focusFontFamily: 'mono',
  focusFontSize: 24,
  focusLineHeight: 1.5,
  focusBackground: 'dark',
}

beforeEach(() => {
  vi.clearAllMocks()
  UpdateSettings.mockResolvedValue(undefined)
  useFocusReaderStore.setState({ ...DEFAULT_FOCUS_READER_PREFS })
  useReaderStore.setState({ ...DEFAULT_READER_PREFS })
  useSettingsStore.setState({ settings: null, loading: false, error: null })
})

describe('toFocusReaderPrefs 入站校验', () => {
  it('合法值原样通过', () => {
    expect(
      toFocusReaderPrefs({
        focusFontFamily: 'mono',
        focusFontSize: 22,
        focusLineHeight: 1.5,
        focusBackground: 'dark',
      }),
    ).toEqual({
      fontFamily: 'mono',
      fontSize: 22,
      lineHeight: 1.5,
      background: 'dark',
    })
  })

  it('白名单字段越界回落默认', () => {
    expect(
      toFocusReaderPrefs({
        focusFontFamily: 'comic-sans',
        focusFontSize: 16,
        focusLineHeight: 3.5,
        focusBackground: 'neon',
      }),
    ).toEqual(DEFAULT_FOCUS_READER_PREFS)
  })

  // 与阅读视图同一套语义：字号是连续量，越界收窄；零值是「字段缺失」而非用户意图。
  it('字号越界收窄，零值回落默认', () => {
    expect(
      toFocusReaderPrefs({ ...baseSettings, focusFontSize: 99 }).fontSize,
    ).toBe(32)
    expect(
      toFocusReaderPrefs({ ...baseSettings, focusFontSize: 0 }).fontSize,
    ).toBe(DEFAULT_FOCUS_READER_PREFS.fontSize)
  })

  it('老库的零值全部回落默认', () => {
    expect(toFocusReaderPrefs(legacySettings)).toEqual(
      DEFAULT_FOCUS_READER_PREFS,
    )
  })
})

describe('focusPrefsUnset', () => {
  it('四项全为零值才算未播种', () => {
    expect(focusPrefsUnset(legacySettings)).toBe(true)
  })

  it('用户动过任意一项就不再视为未播种', () => {
    expect(focusPrefsUnset({ ...legacySettings, focusFontSize: 20 })).toBe(
      false,
    )
    expect(
      focusPrefsUnset({ ...legacySettings, focusFontFamily: 'mono' }),
    ).toBe(false)
    expect(
      focusPrefsUnset({ ...legacySettings, focusBackground: 'dark' }),
    ).toBe(false)
    expect(focusPrefsUnset({ ...legacySettings, focusLineHeight: 2.0 })).toBe(
      false,
    )
  })
})

describe('FocusReaderStore', () => {
  it('默认偏好与出厂值一致', () => {
    const s = useFocusReaderStore.getState()
    expect(s.fontFamily).toBe('sans')
    expect(s.fontSize).toBe(16)
    expect(s.lineHeight).toBe(1.8)
    expect(s.background).toBe('default')
  })

  // update() 写的是合并后的整份 Settings（且 settings 为 null 时直接跳过），
  // 所以断言要用 objectContaining，别期望只收到那一个字段。
  it('改偏好会写进后端', async () => {
    useSettingsStore.setState({ settings: baseSettings })

    useFocusReaderStore.getState().setFontSize(24)
    await vi.waitFor(() => expect(UpdateSettings).toHaveBeenCalled())

    expect(UpdateSettings).toHaveBeenCalledWith(
      expect.objectContaining({ focusFontSize: 24 }),
    )
    expect(useFocusReaderStore.getState().fontSize).toBe(24)
  })

  it('后端设置载入后同步到本 store', () => {
    useSettingsStore.setState({
      settings: {
        ...baseSettings,
        focusFontFamily: 'mono',
        focusFontSize: 24,
        focusLineHeight: 1.5,
        focusBackground: 'dark',
      },
    })

    const s = useFocusReaderStore.getState()
    expect(s.fontFamily).toBe('mono')
    expect(s.fontSize).toBe(24)
    expect(s.lineHeight).toBe(1.5)
    expect(s.background).toBe('dark')
  })

  it('后端还是零值时同步为出厂默认，而不是 0', () => {
    useSettingsStore.setState({ settings: legacySettings })

    const s = useFocusReaderStore.getState()
    expect(s.fontFamily).toBe(DEFAULT_FOCUS_READER_PREFS.fontFamily)
    expect(s.fontSize).toBe(DEFAULT_FOCUS_READER_PREFS.fontSize)
    expect(s.lineHeight).toBe(DEFAULT_FOCUS_READER_PREFS.lineHeight)
    expect(s.background).toBe(DEFAULT_FOCUS_READER_PREFS.background)
  })

  // 两套偏好各自独立落在自己的 store 上，同一份后端设置同步下来互不覆盖。
  it('两套偏好互不影响', () => {
    useSettingsStore.setState({ settings: splitSettings })

    expect(useReaderStore.getState().fontFamily).toBe('sans')
    expect(useReaderStore.getState().fontSize).toBe(16)
    expect(useReaderStore.getState().width).toBe('640')

    expect(useFocusReaderStore.getState().fontFamily).toBe('mono')
    expect(useFocusReaderStore.getState().fontSize).toBe(24)
    expect(useFocusReaderStore.getState().lineHeight).toBe(1.5)
    expect(useFocusReaderStore.getState().background).toBe('dark')
  })
})

describe('seedFocusPrefsFromReader', () => {
  it('未播种时拿阅读视图的值播种（四项一起）', async () => {
    useSettingsStore.setState({ settings: legacySettings })

    await seedFocusPrefsFromReader()

    expect(UpdateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        focusFontFamily: 'serif',
        focusFontSize: 20,
        focusLineHeight: 2.0,
        focusBackground: 'sepia',
      }),
    )
  })

  // 后端里可能还是零值，直接照搬就会把「字段缺失」搬过去。
  it('播种的是收窄后的值', async () => {
    useSettingsStore.setState({
      settings: {
        ...legacySettings,
        readerFontFamily: '',
        readerFontSize: 99,
        readerLineHeight: 0,
        readerBackground: 'neon',
      },
    })

    await seedFocusPrefsFromReader()

    expect(UpdateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        focusFontFamily: 'sans',
        focusFontSize: 32,
        focusLineHeight: 1.8,
        focusBackground: 'default',
      }),
    )
  })

  it('已播种过则为无操作', async () => {
    useSettingsStore.setState({ settings: baseSettings })

    await seedFocusPrefsFromReader()

    expect(UpdateSettings).not.toHaveBeenCalled()
  })

  it('后端设置未载入时跳过', async () => {
    useSettingsStore.setState({ settings: null })

    await seedFocusPrefsFromReader()

    expect(UpdateSettings).not.toHaveBeenCalled()
  })
})
