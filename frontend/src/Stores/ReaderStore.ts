import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type {
  ReaderBackground,
  ReaderFontFamily,
  ReaderFontSize,
  ReaderLineHeight,
  ReaderWidth,
} from '../Types'

interface ReaderState {
  fontFamily: ReaderFontFamily
  fontSize: ReaderFontSize
  lineHeight: ReaderLineHeight
  width: ReaderWidth
  background: ReaderBackground

  setFontFamily: (fontFamily: ReaderFontFamily) => void
  setFontSize: (fontSize: ReaderFontSize) => void
  setLineHeight: (lineHeight: ReaderLineHeight) => void
  setWidth: (width: ReaderWidth) => void
  setBackground: (background: ReaderBackground) => void
}

export const useReaderStore = create<ReaderState>()(
  persist(
    (set) => ({
      fontFamily: 'sans',
      fontSize: 16,
      lineHeight: 1.8,
      width: '640',
      background: 'default',

      setFontFamily(fontFamily) {
        set({ fontFamily })
      },
      setFontSize(fontSize) {
        set({ fontSize })
      },
      setLineHeight(lineHeight) {
        set({ lineHeight })
      },
      setWidth(width) {
        set({ width })
      },
      setBackground(background) {
        set({ background })
      },
    }),
    { name: 'clip-reader' },
  ),
)
