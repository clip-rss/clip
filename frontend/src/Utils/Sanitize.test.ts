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
    const out = sanitizeHtml(
      '<p>a<strong>b</strong><a href="https://x.com">l</a></p>',
    )
    expect(out).toContain('<strong>b</strong>')
    expect(out).toContain('href="https://x.com"')
  })

  it('图片加 loading=lazy/decoding=async/referrerpolicy=no-referrer，外链加 rel/target', () => {
    const img = sanitizeHtml('<img src="https://x.com/a.png">')
    expect(img).toContain('loading="lazy"')
    expect(img).toContain('decoding="async"')
    expect(img).toContain('referrerpolicy="no-referrer"')
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

  it('preserves trusted video embeds', () => {
    const out = sanitizeHtml(
      '<iframe src="https://www.youtube.com/embed/demo" allowfullscreen></iframe>',
    )
    expect(out).toContain('<iframe')
    expect(out).toContain('src="https://www.youtube.com/embed/demo"')
  })

  it('removes untrusted video embeds', () => {
    const out = sanitizeHtml(
      '<iframe src="https://evil.example/embed/demo"></iframe>',
    )
    expect(out).not.toContain('evil.example')
  })

  it('promotes trusted lazy media attributes', () => {
    const out = sanitizeHtml(
      '<video data-video-src="https://cloudvideo.thepaper.cn/demo.mp4"></video>' +
        '<iframe data-src="https://www.thepaper.cn/video/player?vid=1"></iframe>',
    )
    expect(out).toContain('src="https://cloudvideo.thepaper.cn/demo.mp4"')
    expect(out).toContain('src="https://www.thepaper.cn/video/player?vid=1"')
  })

  it('keeps lazy media containers for the reader player', () => {
    const out = sanitizeHtml(
      '<div class="video-player" data-video-url="/media/demo.m3u8"></div>',
    )
    expect(out).toContain('data-video-url="/media/demo.m3u8"')
  })

  it('resolves media URLs against the article URL', () => {
    const out = sanitizeHtml(
      '<video src="../media/demo.mp4"></video>',
      'https://www.thepaper.cn/news/123',
    )
    expect(out).toContain('src="https://www.thepaper.cn/media/demo.mp4"')
  })

})
