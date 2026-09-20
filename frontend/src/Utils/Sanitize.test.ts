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

  it('视频贴片：去控件，有源视频包贴片加播放按钮，无源视频替换为失败文案', () => {
    const out = sanitizeHtml(
      '<p><video src="https://x.com/v.mp4" controls width="640" height="360"></video></p>' +
        '<p><video controls></video></p>',
      {
        videoSticker: true,
        playLabel: '播放视频',
        failedLabel: '[视频加载失败]',
      },
    )
    // controls / width / height 被移除
    expect(out.toLowerCase()).not.toContain('controls')
    expect(out).not.toContain('width="640"')
    // 有源视频被包一层贴片并叠加文字播放按钮
    // ⚠️ 必须断言 class="video-box"：样式表按 `.video-box` 类命中，只给 data 属性
    // 会让所有样式落空（按钮退化成浏览器默认样式），而 data 属性断言发现不了。
    expect(out).toContain('class="video-box"')
    expect(out).toContain('data-video-box')
    expect(out).toContain('video-play')
    expect(out).toContain('title="播放视频"')
    expect(out).toContain('>播放视频</button>')
    expect(out.match(/data-video-box/g)?.length).toBe(1)
    // 无源视频：整个 <video> 被失败文案占位替换（不能只剩个播不了的贴片）
    expect(out).not.toContain('<video controls')
    expect(out).toContain('class="video-failed"')
    expect(out).toContain('[视频加载失败]')
    expect(out.match(/<video\b/g)?.length).toBe(1)
  })

  it('缺少 failedLabel 时无源视频保持原样', () => {
    const out = sanitizeHtml('<video></video>', {
      videoSticker: true,
      playLabel: '播放视频',
    })
    expect(out).toContain('<video')
    expect(out).not.toContain('video-failed')
  })

  it('不开启 videoSticker 时不装饰视频', () => {
    const out = sanitizeHtml('<video src="https://x.com/v.mp4"></video>')
    expect(out).not.toContain('data-video-box')
    expect(out).not.toContain('video-failed')
  })

  it('videoSticker 但文案为空时不装饰（避免出现无字按钮）', () => {
    const out = sanitizeHtml('<video src="https://x.com/v.mp4"></video>', {
      videoSticker: true,
    })
    expect(out).not.toContain('data-video-box')
  })
})
