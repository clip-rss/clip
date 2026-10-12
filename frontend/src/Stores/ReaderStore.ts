import { create } from 'zustand'
import type {
  FocusReaderPrefs,
  ReaderBackground,
  ReaderFontFamily,
  ReaderFontSize,
  ReaderLineHeight,
  ReaderPrefs,
  ReaderWidth,
  Settings,
} from '../Types'
import { READER_FONT_PRESETS } from '../Types'
import { useSettingsStore } from './SettingsStore'
import { installedFontIds, useFontStore } from './FontStore'
import { normalizeFontSize } from '../Utils/ReaderStyle'
import { isSafeFontSegment } from '../Utils/fonts'

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
// 字号是连续量（10–32），走 normalizeFontSize 收窄而非白名单。
// 字体是**动态白名单**：预设三档 ∪ 已装字体 id。installedIds 进每个收窄入口，
// 缺省为空（纯预设）——阶段 C 时序保证：清单到齐后再基于后端原始值重收窄一次。
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

/** 后端设置中专注模式排版的相关字段（无宽度项）。 */
export type FocusReaderSettingsFields = Pick<
  Settings,
  'focusFontFamily' | 'focusFontSize' | 'focusLineHeight' | 'focusBackground'
>

/** 一组待收窄的原始排版取值（不含宽度）。 */
export interface RawReaderPrefs {
  fontFamily: unknown
  fontSize: unknown
  lineHeight: unknown
  background: unknown
}

/**
 * 逐字段收窄一组排版取值。阅读视图与专注模式两套偏好共用同一套收窄逻辑，
 * 区别只在字段来源（reader* / focus*）与默认值，故收在这里一份。
 * 字体的动态白名单由调用方传 installedIds（缺省空 = 只有预设三档）。
 */
export function narrowReaderPrefs(
  raw: RawReaderPrefs,
  fallback: FocusReaderPrefs,
  installedIds: string[] = [],
): FocusReaderPrefs {
  return {
    fontFamily: pick(
      // 已装 id 进白名单前过滤非法字符：id 来自远端清单，非法值直接视为不在白名单。
      [...READER_FONT_PRESETS, ...installedIds.filter(isSafeFontSegment)],
      raw.fontFamily,
      fallback.fontFamily,
    ),
    fontSize: normalizeFontSize(raw.fontSize, fallback.fontSize),
    lineHeight: pick(LINE_HEIGHTS, raw.lineHeight, fallback.lineHeight),
    background: pick(BACKGROUNDS, raw.background, fallback.background),
  }
}

/** 把后端设置收窄为合法排版偏好。 */
export function toReaderPrefs(
  settings: ReaderSettingsFields,
  installedIds: string[] = [],
): ReaderPrefs {
  return {
    ...narrowReaderPrefs(
      {
        fontFamily: settings.readerFontFamily,
        fontSize: settings.readerFontSize,
        lineHeight: settings.readerLineHeight,
        background: settings.readerBackground,
      },
      DEFAULT_READER_PREFS,
      installedIds,
    ),
    width: pick(WIDTHS, settings.readerWidth, DEFAULT_READER_PREFS.width),
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
// 收窄用动态白名单：预设 ∪ 已装字体（清单未到齐时为空，id 值会被回落——见 reNarrowFonts）。
function syncFromSettings(settings: Settings | null): void {
  if (!settings) return
  const next = toReaderPrefs(settings, installedFontIds())
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

// 阶段 C 时序：偏好**启动时**从后端同步，而「已装清单」是**异步**拿到的。若清单
// 未到齐就按动态白名单收窄，用户上次选的下载字体会被回落成 sans。因此清单到齐
// 后要基于**后端原始值**再收窄一次（此时 id 已在白名单内，值被救回），且这一步
// 绝不写回后端（保持 syncFromSettings 只 setState 的语义，与「删除使用中字体才
// 写回」是两件事）。
function reNarrowFonts(): void {
  const settings = useSettingsStore.getState().settings
  if (!settings) return
  const next = toReaderPrefs(settings, installedFontIds())
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

// 已装清单变化（下载 / 删除 / 外部删文件）都会触发重收窄。幂等：值没变就不动。
useFontStore.subscribe(() => {
  reNarrowFonts()
})

useSettingsStore.subscribe((state) => {
  syncFromSettings(state.settings)
})
