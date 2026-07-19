// 阅读排版偏好 → CSS 取值映射（纯函数，便于测试）。

import type { ReaderBackground, ReaderPrefs } from '../Types'

const FONT_FAMILY: Record<ReaderPrefs['fontFamily'], string> = {
  sans: 'var(--font-family)',
  serif: 'Georgia, "Times New Roman", "Songti SC", "Noto Serif CJK SC", serif',
  mono: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
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
    fontFamily: FONT_FAMILY[prefs.fontFamily],
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
