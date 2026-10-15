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
import { useFontStore } from './FontStore'
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
  // 字体清单也要归零：上一用例塞进去的已装字体会污染动态白名单。
  useFontStore.setState({
    catalog: null,
    catalogFailed: false,
    installed: [],
    ready: false,
    refreshing: false,
  })
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

describe('动态字体白名单（阶段 C）', () => {
  it('已装字体 id 在传入白名单时通过收窄', () => {
    const got = toReaderPrefs(
      { ...baseReader, readerFontFamily: 'lxgw-wenkai' },
      ['lxgw-wenkai'],
    )
    expect(got.fontFamily).toBe('lxgw-wenkai')
  })

  it('缺省白名单（清单未到齐）时下载字体 id 回落默认', () => {
    const got = toReaderPrefs({
      ...baseReader,
      readerFontFamily: 'lxgw-wenkai',
    })
    expect(got.fontFamily).toBe(DEFAULT_READER_PREFS.fontFamily)
  })

  // 已装清单里混进含非法字符的 id（远端清单可能带），白名单要过滤掉它。
  it('白名单过滤含非法字符的已装 id', () => {
    const got = toReaderPrefs({ ...baseReader, readerFontFamily: 'bad"id' }, [
      'bad"id',
    ])
    expect(got.fontFamily).toBe(DEFAULT_READER_PREFS.fontFamily)
  })

  // 阶段 C 的核心时序雷区：偏好启动时同步、已装清单异步到达。清单未到齐时若照收窄
  // 会把用户选的下载字体回落成 sans；清单到齐后必须基于后端原始值再收窄一次救回，
  // 且这一步只 setState、不写回后端。
  it('清单到齐后基于后端原始值重收窄，救回 id 且不写回', () => {
    UpdateSettings.mockResolvedValue(undefined)
    useFontStore.setState({ installed: [], ready: false })
    useSettingsStore.setState({
      settings: { ...baseSettings, readerFontFamily: 'lxgw-wenkai' },
      loading: false,
      error: null,
    })
    // 清单未到齐 → 回落 sans（此时后端仍是 id）。
    expect(useReaderStore.getState().fontFamily).toBe('sans')

    // 清单到齐 → 订阅触发重收窄，值被救回。
    useFontStore.setState({
      installed: [{ id: 'lxgw-wenkai', files: [], totalSize: 0 }],
      ready: true,
    })
    expect(useReaderStore.getState().fontFamily).toBe('lxgw-wenkai')
    // 重收窄只 setState，绝不写回后端。
    expect(UpdateSettings).not.toHaveBeenCalled()
  })
})
