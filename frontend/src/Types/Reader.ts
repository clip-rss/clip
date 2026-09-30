// 阅读视图排版偏好类型。

export type ReaderFontFamily = 'sans' | 'serif' | 'mono'
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
