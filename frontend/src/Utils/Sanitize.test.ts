import { describe, it, expect } from 'vitest'
import { sanitizeHtml } from './Sanitize'

describe('sanitizeHtml', () => {
  it('移除 script 标签', () => {
    const out = sanitizeHtml('<p>hi</p><script>alert(1)</script>')
    expect(out).toContain('<p>hi</p>')
    expect(out.toLowerCase()).not.toContain('<script')
  })

  it('移除事件处理器属性', () => {
    const out = sanitizeHtml('<img src="x" onerror="alert(1)">')
    expect(out.toLowerCase()).not.toContain('onerror')
  })

  it('移除内联 style', () => {
    const out = sanitizeHtml('<p style="color:red">x</p>')
    expect(out).not.toContain('style=')
  })

  it('保留安全标签与链接', () => {
    const out = sanitizeHtml('<p>a<strong>b</strong><a href="https://x.com">l</a></p>')
    expect(out).toContain('<strong>b</strong>')
    expect(out).toContain('href="https://x.com"')
  })

  it('图片加 loading=lazy，外链加 rel/target', () => {
    const img = sanitizeHtml('<img src="https://x.com/a.png">')
    expect(img).toContain('loading="lazy"')
    const a = sanitizeHtml('<a href="https://x.com">l</a>')
    expect(a).toContain('rel="noopener noreferrer"')
    expect(a).toContain('target="_blank"')
  })

  it('剔除 javascript: 协议链接', () => {
    const out = sanitizeHtml('<a href="javascript:alert(1)">x</a>')
    expect(out.toLowerCase()).not.toContain('javascript:')
  })

  it('空输入返回空串', () => {
    expect(sanitizeHtml('')).toBe('')
  })
})
