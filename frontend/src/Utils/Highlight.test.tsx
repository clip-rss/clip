import { describe, it, expect } from 'vitest'
import { isValidElement, type ReactNode } from 'react'
import { highlightText } from './Highlight'

/** 取出高亮结果里 <mark> 包裹的文本片段。 */
function marks(node: ReactNode): string[] {
  if (!Array.isArray(node)) return []
  const out: string[] = []
  for (const part of node) {
    if (isValidElement(part) && part.type === 'mark') {
      out.push((part.props as { children: string }).children)
    }
  }
  return out
}

describe('highlightText', () => {
  it('无 query 时原样返回文本', () => {
    expect(highlightText('科技爱好者周刊', '', 'm')).toBe('科技爱好者周刊')
    expect(highlightText('hello', '   ', 'm')).toBe('hello')
  })

  it('无匹配时返回原文本字符串', () => {
    expect(highlightText('科技爱好者周刊', 'xyz', 'm')).toBe('科技爱好者周刊')
  })

  it('中文子串高亮', () => {
    const node = highlightText('科技爱好者周刊', '周刊', 'm')
    expect(marks(node)).toEqual(['周刊'])
  })

  it('英文大小写不敏感高亮', () => {
    const node = highlightText('Swift Concurrency', 'swift', 'm')
    expect(marks(node)).toEqual(['Swift'])
  })

  it('多关键词分别高亮', () => {
    const node = highlightText('周刊 Swift 专题', '周刊 swift', 'm')
    expect(marks(node)).toEqual(['周刊', 'Swift'])
  })

  it('正则元字符按字面量匹配，不报错', () => {
    const node = highlightText('a (b) c.d', '(b)', 'm')
    expect(marks(node)).toEqual(['(b)'])
  })
})
