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
  useReaderStore,
  DEFAULT_READER_PREFS,
  toReaderPrefs,
} from './ReaderStore'
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

beforeEach(() => {
  vi.clearAllMocks()
  useReaderStore.setState({ ...DEFAULT_READER_PREFS })
  useSettingsStore.setState({ settings: null, loading: false, error: null })
})

describe('ReaderStore', () => {
  it('默认偏好与后端出厂值一致', () => {
    const s = useReaderStore.getState()
    expect(s.fontFamily).toBe('sans')
    expect(s.fontSize).toBe(16)
    expect(s.lineHeight).toBe(1.8)
    expect(s.width).toBe('640')
    expect(s.background).toBe('default')
  })

  it('setters 更新本地偏好', () => {
    const st = useReaderStore.getState()
    st.setFontFamily('serif')
    st.setFontSize(18)
    st.setLineHeight(2.0)
    st.setWidth('full')
    st.setBackground('sepia')

    const s = useReaderStore.getState()
    expect(s.fontFamily).toBe('serif')
    expect(s.fontSize).toBe(18)
    expect(s.lineHeight).toBe(2.0)
    expect(s.width).toBe('full')
    expect(s.background).toBe('sepia')
  })

  it('setters 写穿到后端', async () => {
    useSettingsStore.setState({ settings: baseSettings })
    UpdateSettings.mockResolvedValue(undefined)

    useReaderStore.getState().setFontFamily('mono')
    await vi.waitFor(() => expect(UpdateSettings).toHaveBeenCalled())

    expect(UpdateSettings).toHaveBeenCalledWith(
      expect.objectContaining({ readerFontFamily: 'mono' }),
    )
  })

  it('后端设置载入后同步到本 store', () => {
    useSettingsStore.setState({
      settings: {
        ...baseSettings,
        readerFontFamily: 'serif',
        readerFontSize: 14,
        readerBackground: 'dark',
      },
    })

    const s = useReaderStore.getState()
    expect(s.fontFamily).toBe('serif')
    expect(s.fontSize).toBe(14)
    expect(s.background).toBe('dark')
  })
})

describe('toReaderPrefs 入站校验', () => {
  it('合法值原样通过', () => {
    const got = toReaderPrefs({
      readerFontFamily: 'mono',
      readerFontSize: 18,
      readerLineHeight: 1.5,
      readerWidth: 'full',
      readerBackground: 'dark',
    })
    expect(got).toEqual({
      fontFamily: 'mono',
      fontSize: 18,
      lineHeight: 1.5,
      width: 'full',
      background: 'dark',
    })
  })

  // 同步会拉取到更高版本客户端写入的载荷，其中可能含本端不认识的取值；
  // 排版值直接进 CSS，必须回落而非照搬。
  it('越界值回落默认', () => {
    const got = toReaderPrefs({
      readerFontFamily: 'comic-sans',
      readerFontSize: 99,
      readerLineHeight: 3.5,
      readerWidth: '1920',
      readerBackground: 'neon',
    })
    expect(got).toEqual(DEFAULT_READER_PREFS)
  })

  it('空值与零值回落默认（旧库无这些字段）', () => {
    const got = toReaderPrefs({
      readerFontFamily: '',
      readerFontSize: 0,
      readerLineHeight: 0,
      readerWidth: '',
      readerBackground: '',
    })
    expect(got).toEqual(DEFAULT_READER_PREFS)
  })
})
