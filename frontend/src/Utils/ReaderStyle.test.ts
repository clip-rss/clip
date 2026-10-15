import { describe, it, expect } from 'vitest'
import {
  readerContentStyle,
  readerBackgroundClass,
  cssFontFamily,
  normalizeFontSize,
  READER_FONT_SIZE_MIN,
  READER_FONT_SIZE_MAX,
} from './ReaderStyle'
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

describe('cssFontFamily', () => {
  it('预设三档走既有字体栈', () => {
    expect(cssFontFamily('sans')).toContain('--font-family')
    expect(cssFontFamily('serif')).toContain('serif')
    expect(cssFontFamily('mono')).toContain('monospace')
  })

  // 已下载字体 id：用合成家族名 + 无衬线兜底（缺字落回界面同一套字体，不跳 Georgia）。
  it('已下载字体 id 生成合成家族名并回落到无衬线栈', () => {
    const css = cssFontFamily('lxgw-wenkai')
    expect(css).toBe('"clip-font-lxgw-wenkai", var(--font-family)')
    expect(css).not.toContain('Georgia')
  })

  // id 来自远端清单，含引号等字符会破坏 CSS —— 最后一道白名单把它当作预设栈。
  it('非法字体 id 被视为预设（不拼进 CSS）', () => {
    expect(cssFontFamily('bad"id')).toBe('var(--font-family)')
    expect(cssFontFamily('a/b')).toBe('var(--font-family)')
  })
})

describe('readerBackgroundClass', () => {
  it('default 返回 null（继承主题）', () => {
    expect(readerBackgroundClass('default')).toBeNull()
  })

  it('其他映射到对应全局主题类名（theme- 前缀，避免与 Tailwind 工具类冲突）', () => {
    expect(readerBackgroundClass('light')).toBe('theme-light')
    expect(readerBackgroundClass('sepia')).toBe('theme-sepia')
    expect(readerBackgroundClass('dark')).toBe('theme-dark')
  })
})

describe('normalizeFontSize', () => {
  it('范围内原样通过', () => {
    expect(normalizeFontSize(16, 16)).toBe(16)
    expect(normalizeFontSize(READER_FONT_SIZE_MIN, 16)).toBe(
      READER_FONT_SIZE_MIN,
    )
    expect(normalizeFontSize(READER_FONT_SIZE_MAX, 16)).toBe(
      READER_FONT_SIZE_MAX,
    )
  })

  it('越界收窄到边界', () => {
    expect(normalizeFontSize(99, 16)).toBe(READER_FONT_SIZE_MAX)
    expect(normalizeFontSize(11.4, 16)).toBe(11)
    expect(normalizeFontSize(0.5, 16)).toBe(READER_FONT_SIZE_MIN)
  })

  // 0 是「后端 JSON 里没有这个 key」的零值，不是用户意图，必须回落而不是收窄到 10。
  it('0 / 负数回落 fallback', () => {
    expect(normalizeFontSize(0, 16)).toBe(16)
    expect(normalizeFontSize(-3, 16)).toBe(16)
  })

  it('非数回落 fallback', () => {
    expect(normalizeFontSize('18', 16)).toBe(16)
    expect(normalizeFontSize(null, 16)).toBe(16)
    expect(normalizeFontSize(undefined, 16)).toBe(16)
    expect(normalizeFontSize(Number.NaN, 16)).toBe(16)
    expect(normalizeFontSize(Number.POSITIVE_INFINITY, 16)).toBe(16)
  })
})
