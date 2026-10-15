// 阅读视图排版偏好类型。

/** 内置三档字体。设定值的子集，也是动态白名单的底座。 */
export const READER_FONT_PRESETS = ['sans', 'serif', 'mono'] as const
export type ReaderFontPreset = (typeof READER_FONT_PRESETS)[number]

/**
 * 排版字体取值：内置三档（sans / serif / mono）或**已下载字体 id**。
 *
 * 设成宽 string 而非三段联合：阶段 C 之后白名单是动态的（预设 ∪ 已装字体），
 * 联合类型装不下运行时才知道的字体 id。合法取值由 ReaderStore 的动态收窄保证
 * （预设三档 + 已装清单，非法值回落 sans）。
 */
export type ReaderFontFamily = string
/** 正文字号（px）。取值域由 Utils/ReaderStyle.ts 的 READER_FONT_SIZE_* 约束。 */
export type ReaderFontSize = number
export type ReaderLineHeight = 1.5 | 1.8 | 2.0
export type ReaderWidth = '640' | '800' | 'full'
export type ReaderBackground = 'default' | 'light' | 'sepia' | 'dark'

/** 阅读排版偏好聚合。 */
export interface ReaderPrefs {
  fontFamily: ReaderFontFamily
  fontSize: ReaderFontSize
  lineHeight: ReaderLineHeight
  width: ReaderWidth
  background: ReaderBackground
}

/**
 * 专注模式的排版偏好：与阅读视图那套独立存储，互不影响。
 */
export type FocusReaderPrefs = Omit<ReaderPrefs, 'width'>
