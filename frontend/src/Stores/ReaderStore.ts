import { create } from 'zustand'
import type {
  ReaderBackground,
  ReaderFontFamily,
  ReaderFontSize,
  ReaderLineHeight,
  ReaderPrefs,
  ReaderWidth,
  Settings,
} from '../Types'
import { useSettingsStore } from './SettingsStore'

interface ReaderState extends ReaderPrefs {
  setFontFamily: (fontFamily: ReaderFontFamily) => void
  setFontSize: (fontSize: ReaderFontSize) => void
  setLineHeight: (lineHeight: ReaderLineHeight) => void
  setWidth: (width: ReaderWidth) => void
  setBackground: (background: ReaderBackground) => void
}

/** 出厂默认排版，与后端 store.DefaultSettings() 的 Reader* 字段保持一致。 */
export const DEFAULT_READER_PREFS: ReaderPrefs = {
  fontFamily: 'sans',
  fontSize: 16,
  lineHeight: 1.8,
  width: '640',
  background: 'default',
}

/* ---------- 入站取值校验 ---------- */

// 后端字段是宽类型（string / number / float64），Go 没有联合类型。
// 且配置同步允许拉取到更高版本客户端写入的载荷，其中可能含本端不认识的取值。
// 排版值直接进 CSS，非法值会渲染错乱，故逐字段按白名单收窄，越界回落默认。
const FONT_FAMILIES: ReaderFontFamily[] = ['sans', 'serif', 'mono']
const FONT_SIZES: ReaderFontSize[] = [14, 16, 18]
const LINE_HEIGHTS: ReaderLineHeight[] = [1.5, 1.8, 2.0]
const WIDTHS: ReaderWidth[] = ['640', '800', 'full']
const BACKGROUNDS: ReaderBackground[] = ['default', 'light', 'sepia', 'dark']

function pick<T>(allowed: T[], value: unknown, fallback: T): T {
  return allowed.includes(value as T) ? (value as T) : fallback
}

/** 后端设置中与排版相关的字段。取 Pick 而非整个 Settings，便于迁移复用本校验。 */
export type ReaderSettingsFields = Pick<
  Settings,
  | 'readerFontFamily'
  | 'readerFontSize'
  | 'readerLineHeight'
  | 'readerWidth'
  | 'readerBackground'
>

/** 把后端设置收窄为合法排版偏好。 */
export function toReaderPrefs(settings: ReaderSettingsFields): ReaderPrefs {
  const d = DEFAULT_READER_PREFS
  return {
    fontFamily: pick(FONT_FAMILIES, settings.readerFontFamily, d.fontFamily),
    fontSize: pick(FONT_SIZES, settings.readerFontSize, d.fontSize),
    lineHeight: pick(LINE_HEIGHTS, settings.readerLineHeight, d.lineHeight),
    width: pick(WIDTHS, settings.readerWidth, d.width),
    background: pick(BACKGROUNDS, settings.readerBackground, d.background),
  }
}

/* ---------- store ---------- */

/** 写后端。失败时 SettingsStore 自行回滚，随后订阅会把旧值同步回本 store。 */
function persist(partial: Partial<Settings>): void {
  void useSettingsStore.getState().update(partial)
}

export const useReaderStore = create<ReaderState>()((set) => ({
  ...DEFAULT_READER_PREFS,

  setFontFamily(fontFamily) {
    set({ fontFamily })
    persist({ readerFontFamily: fontFamily })
  },
  setFontSize(fontSize) {
    set({ fontSize })
    persist({ readerFontSize: fontSize })
  },
  setLineHeight(lineHeight) {
    set({ lineHeight })
    persist({ readerLineHeight: lineHeight })
  },
  setWidth(width) {
    set({ width })
    persist({ readerWidth: width })
  },
  setBackground(background) {
    set({ background })
    persist({ readerBackground: background })
  },
}))

/* ---------- 后端 → store 单向同步 ---------- */

// 仅在取值真正变化时 setState，避免与写后端形成回环。
function syncFromSettings(settings: Settings | null): void {
  if (!settings) return
  const next = toReaderPrefs(settings)
  const cur = useReaderStore.getState()
  if (
    next.fontFamily === cur.fontFamily &&
    next.fontSize === cur.fontSize &&
    next.lineHeight === cur.lineHeight &&
    next.width === cur.width &&
    next.background === cur.background
  ) {
    return
  }
  useReaderStore.setState(next)
}

useSettingsStore.subscribe((state) => {
  syncFromSettings(state.settings)
})
