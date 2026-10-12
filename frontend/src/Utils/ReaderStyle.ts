// 阅读排版偏好 → CSS 取值映射（纯函数，便于测试）。

import type {
  ReaderBackground,
  ReaderFontFamily,
  ReaderFontPreset,
  ReaderPrefs,
} from '../Types'
import { fontFaceFamily, isSafeFontSegment } from './fonts'

/** 正文字号可选范围（px）。设置页的 InputNumber 与入站校验共用这一对边界。 */
export const READER_FONT_SIZE_MIN = 10
export const READER_FONT_SIZE_MAX = 32

/**
 * 收窄入站字号。后端字段是宽类型（int），且配置同步会拉取到更高版本客户端写入的载荷，
 * 其中可能含本端不认识的取值；字号直接进 CSS，故必须收进范围。
 *
 * 0 / 负数 / 非数一律回落 fallback：设置在后端是一整个 JSON blob，
 * 旧版本写下的行里没有 readerFontSize 这个 key，Go 反序列化后就是 int 的零值 0，
 * 那是「字段缺失」而不是「用户要 0px」。其余越界值收窄到边界而非回落，
 * 与设置页 InputNumber 的输入行为一致。
 */
export function normalizeFontSize(value: unknown, fallback: number): number {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
    return fallback
  }
  return Math.min(
    READER_FONT_SIZE_MAX,
    Math.max(READER_FONT_SIZE_MIN, Math.round(value)),
  )
}

const FONT_FAMILY: Record<ReaderFontPreset, string> = {
  sans: 'var(--font-family)',
  serif: 'Georgia, "Times New Roman", "Songti SC", "Noto Serif CJK SC", serif',
  mono: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
}

/**
 * 字体取值 → CSS font-family。
 *
 * 预设三档走既有字体栈；已下载字体 id 用「合成家族名 + 无衬线兜底」——
 * 自定义字体缺字时落回界面同一套 `--font-family`，而不是跳到衬线/等宽栈
 * （下载的都是正文用中文字体，落到 Georgia 会非常突兀）。
 * id 来自远端清单，进 CSS 前再走一次字符白名单；不合法的当作预设栈处理。
 */
export function cssFontFamily(fontFamily: ReaderFontFamily): string {
  if (fontFamily in FONT_FAMILY) {
    return FONT_FAMILY[fontFamily as ReaderFontPreset]
  }
  if (!isSafeFontSegment(fontFamily)) {
    return FONT_FAMILY.sans
  }
  return `"${fontFaceFamily(fontFamily)}", var(--font-family)`
}

export interface ReaderContentStyle {
  fontFamily: string
  fontSize: string
  lineHeight: string
  maxWidth: string
}

/** 正文排版样式取值。 */
export function readerContentStyle(prefs: ReaderPrefs): ReaderContentStyle {
  return {
    fontFamily: cssFontFamily(prefs.fontFamily),
    fontSize: `${prefs.fontSize}px`,
    lineHeight: String(prefs.lineHeight),
    maxWidth: prefs.width === 'full' ? '100%' : `${prefs.width}px`,
  }
}

/**
 * 阅读区独立背景对应的全局主题类名（在子树内重置 CSS 变量 token，使标题/正文/边框一并适配）。
 * 'default' 返回 null（继承应用主题）；其余映射到 global.css 中的 `.theme-light/.theme-sepia/.theme-dark`。
 * 注意加 theme- 前缀：避免与 Tailwind 内置滤镜工具类（.sepia 等）同名而给子树叠加真实滤镜。
 */
export function readerBackgroundClass(bg: ReaderBackground): string | null {
  return bg === 'default' ? null : `theme-${bg}`
}
