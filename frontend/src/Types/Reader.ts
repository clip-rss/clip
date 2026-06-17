// 阅读视图排版偏好类型。

export type ReaderFontFamily = 'sans' | 'serif' | 'mono'
export type ReaderFontSize = 14 | 16 | 18
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
