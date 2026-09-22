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

  it('给出 articleUrl 时把正文图片改写成代理地址，并留下原始地址', () => {
    const out = sanitizeHtml('<img src="https://cdn.com/a.png" alt="图">', {
      articleUrl: 'https://site.com/post/1',
    })

    // src 指向代理，原始地址挪到 data-origin-src（灯箱与「下载图片」要用它）
    expect(out).toMatch(/\ssrc="\/__clip\/media\?u=/)
    expect(out).toContain('data-origin-src="https://cdn.com/a.png"')
    expect(out).toContain(`r=${encodeURIComponent('https://site.com')}`)
  })

  it('给出 articleUrl 时 srcset 的每个候选都被改写（否则浏览器会挑到未代理的那个）', () => {
    const out = sanitizeHtml(
      '<img src="https://cdn.com/a.png" srcset="https://cdn.com/a.png 1x, https://cdn.com/b.png 2x">',
      { articleUrl: 'https://site.com/post/1' },
    )

    const srcset = out.match(/srcset="([^"]*)"/)?.[1] ?? ''
    expect(srcset).toContain(encodeURIComponent('https://cdn.com/a.png'))
    expect(srcset).toContain(encodeURIComponent('https://cdn.com/b.png'))
  })

  it('相对图片地址按 articleUrl 解析后再代理（RSS 正文没解析过相对地址）', () => {
    const out = sanitizeHtml('<img src="/img/a.png">', {
      articleUrl: 'https://site.com/post/1',
    })

    expect(out).toContain(encodeURIComponent('https://site.com/img/a.png'))
  })

  it('没有 articleUrl 时不改写地址（更新日志弹窗等）', () => {
    const out = sanitizeHtml('<img src="https://cdn.com/a.png">')

    expect(out).toContain('src="https://cdn.com/a.png"')
    expect(out).not.toContain('__clip/media')
    expect(out).not.toContain('data-origin-src')
  })

  it('data: 内联图片既不代理，也不把整串 base64 抄进 data-origin-src', () => {
    const inline = 'data:image/gif;base64,R0lGODlhAQABAAAAACw='
    const out = sanitizeHtml(`<img src="${inline}">`, {
      articleUrl: 'https://site.com/post/1',
    })

    expect(out).toContain(`src="${inline}"`)
    expect(out).not.toContain('__clip/media')
    expect(out).not.toContain('data-origin-src')
  })

  it('视频的 src 与 poster 同样走代理', () => {
    const out = sanitizeHtml(
      '<video src="https://cdn.com/v.mp4" poster="https://cdn.com/p.jpg"></video>',
      { articleUrl: 'https://site.com/post/1' },
    )

    expect(out).toContain(encodeURIComponent('https://cdn.com/v.mp4'))
    expect(out).toContain(encodeURIComponent('https://cdn.com/p.jpg'))
  })

  // 阅读视图的真实调用组合：videoSticker 会把 HTML 过一遍 DOMParser 再序列化回来，
  // 代理地址必须能扛过这一趟往返。
  it('videoSticker 与 articleUrl 同时开启时，代理地址在贴片重排后仍完好', () => {
    const out = sanitizeHtml(
      '<p><img src="https://cdn.com/a.png" alt="图"></p>' +
        '<p><video src="https://cdn.com/v.mp4"></video></p>',
      {
        videoSticker: true,
        playLabel: '播放视频',
        failedLabel: '[视频加载失败]',
        articleUrl: 'https://site.com/post/1',
      },
    )

    expect(out).toContain('video-box')
    expect(out).toMatch(/\ssrc="\/__clip\/media\?u=/)
    expect(out).toContain(encodeURIComponent('https://cdn.com/a.png'))
    expect(out).toContain(encodeURIComponent('https://cdn.com/v.mp4'))
    expect(out).toContain('data-origin-src="https://cdn.com/a.png"')
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
