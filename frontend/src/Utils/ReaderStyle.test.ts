import { describe, it, expect } from 'vitest'
import { readerContentStyle, readerBackgroundClass } from './ReaderStyle'
import type { ReaderPrefs } from '../Types'

const base: ReaderPrefs = {
  fontFamily: 'sans',
  fontSize: 16,
  lineHeight: 1.8,
  width: '640',
  background: 'default',
}

describe('readerContentStyle', () => {
  it('sans 使用主题字体变量', () => {
    expect(readerContentStyle(base).fontFamily).toContain('--font-family')
  })

  it('serif / mono 字体族', () => {
    expect(
      readerContentStyle({ ...base, fontFamily: 'serif' }).fontFamily,
    ).toContain('serif')
    expect(
      readerContentStyle({ ...base, fontFamily: 'mono' }).fontFamily,
    ).toContain('monospace')
  })

  it('字号 / 行高 / 宽度映射', () => {
    const s = readerContentStyle({
      ...base,
      fontSize: 18,
      lineHeight: 2.0,
      width: '800',
    })
    expect(s.fontSize).toBe('18px')
    expect(s.lineHeight).toBe('2')
    expect(s.maxWidth).toBe('800px')
  })

  it('全宽返回 100%', () => {
    expect(readerContentStyle({ ...base, width: 'full' }).maxWidth).toBe('100%')
  })
})

describe('readerBackgroundClass', () => {
  it('default 返回 null（继承主题）', () => {
    expect(readerBackgroundClass('default')).toBeNull()
  })

  it('其他映射到对应全局主题类名', () => {
    expect(readerBackgroundClass('light')).toBe('light')
    expect(readerBackgroundClass('sepia')).toBe('sepia')
    expect(readerBackgroundClass('dark')).toBe('dark')
  })
})
