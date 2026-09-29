import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest'

vi.mock('../Utils', () => ({
  SettingsService: {
    GetSettings: vi.fn(),
    UpdateSettings: vi.fn(),
  },
  toApiError: (e: unknown) => String(e),
}))

import { SettingsService } from '../Utils'
// 走深路径而非 '../Utils'：本文件把那个 barrel mock 掉了，从这里拿不到真值。
import {
  READER_FONT_SIZE_MAX,
  READER_FONT_SIZE_MIN,
} from '../Utils/ReaderStyle'
import {
  useReaderStore,
  DEFAULT_READER_PREFS,
  toReaderPrefs,
} from './ReaderStore'
import type { ReaderSettingsFields } from './ReaderStore'
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
  readerFontFamily: 'sans',
  readerFontSize: 16,
  readerLineHeight: 1.8,
  readerWidth: '640',
  readerBackground: 'default',
  focusFontFamily: 'sans',
  focusFontSize: 16,
  focusLineHeight: 1.8,
  focusBackground: 'default',
}

/** toReaderPrefs 的入参基线：各字段均合法，便于按需覆盖单个字段。 */
const baseReader: ReaderSettingsFields = {
  readerFontFamily: baseSettings.readerFontFamily,
  readerFontSize: baseSettings.readerFontSize,
  readerLineHeight: baseSettings.readerLineHeight,
  readerWidth: baseSettings.readerWidth,
  readerBackground: baseSettings.readerBackground,
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
  // 排版值直接进 CSS，必须收窄而非照搬。
  it('越界值回落默认（白名单字段）', () => {
    const got = toReaderPrefs({
      readerFontFamily: 'comic-sans',
      readerFontSize: DEFAULT_READER_PREFS.fontSize,
      readerLineHeight: 3.5,
      readerWidth: '1920',
      readerBackground: 'neon',
    })
    expect(got).toEqual(DEFAULT_READER_PREFS)
  })

  // 字号是连续量，越界收窄到边界，与设置页 InputNumber 的输入行为一致。
  it('字号越界收窄到边界而非回落默认', () => {
    expect(toReaderPrefs({ ...baseReader, readerFontSize: 99 }).fontSize).toBe(
      READER_FONT_SIZE_MAX,
    )
    expect(toReaderPrefs({ ...baseReader, readerFontSize: 5 }).fontSize).toBe(
      READER_FONT_SIZE_MIN,
    )
  })

  it('字号范围内原样通过（含非 14/16/18 的取值）', () => {
    expect(toReaderPrefs({ ...baseReader, readerFontSize: 21 }).fontSize).toBe(
      21,
    )
    expect(toReaderPrefs({ ...baseReader, readerFontSize: 10 }).fontSize).toBe(
      10,
    )
    expect(toReaderPrefs({ ...baseReader, readerFontSize: 32 }).fontSize).toBe(
      32,
    )
  })

  // 设置在后端是一整个 JSON blob，旧版本写下的行没有 readerFontSize 这个 key，
  // Go 反序列化后是 int 零值 0 —— 那是「字段缺失」，不能当成「用户要 0px」收窄到 10。
  it('字号 0 / 负数 / 非数回落默认（字段缺失或非法）', () => {
    expect(toReaderPrefs({ ...baseReader, readerFontSize: 0 }).fontSize).toBe(
      DEFAULT_READER_PREFS.fontSize,
    )
    expect(toReaderPrefs({ ...baseReader, readerFontSize: -3 }).fontSize).toBe(
      DEFAULT_READER_PREFS.fontSize,
    )
    expect(
      toReaderPrefs({
        ...baseReader,
        readerFontSize: 'big' as unknown as number,
      }).fontSize,
    ).toBe(DEFAULT_READER_PREFS.fontSize)
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
