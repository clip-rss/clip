// 正文 HTML 清洗：去除脚本/事件处理器/内联样式，杜绝 XSS，并统一图片懒加载与外链属性。

import DOMPurify from 'dompurify'

let hooksReady = false
let sanitizeBaseURL: URL | null = null

const safeEmbedDomains = [
  'youtube.com',
  'youtube-nocookie.com',
  'youtu.be',
  'vimeo.com',
  'bilibili.com',
  'dailymotion.com',
  'twitch.tv',
  'thepaper.cn',
]

function isSafeEmbedURL(raw: string): boolean {
  try {
    const url = new URL(raw)
    if (
      (url.protocol !== 'http:' && url.protocol !== 'https:') ||
      url.username ||
      url.password
    ) {
      return false
    }
    const host = url.hostname.toLowerCase().replace(/\.$/, '')
    return safeEmbedDomains.some(
      (domain) => host === domain || host.endsWith(`.${domain}`),
    )
  } catch {
    return false
  }
}

function resolveMediaURL(raw: string): string {
  if (!sanitizeBaseURL) return raw
  try {
    return new URL(raw, sanitizeBaseURL).toString()
  } catch {
    return raw
  }
}

function ensureHooks(): void {
  if (hooksReady) return
  hooksReady = true
  DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    const el = node as Element
    if (['VIDEO', 'AUDIO', 'SOURCE', 'IFRAME'].includes(el.tagName)) {
      if (el.hasAttribute('src')) {
        el.setAttribute('src', resolveMediaURL(el.getAttribute('src') ?? ''))
      }
      if (!el.hasAttribute('src')) {
        for (const name of [
          'data-src',
          'data-video-src',
          'data-video-url',
          'data-video',
          'data-hls',
          'data-mp4',
          'data-url',
        ]) {
          const value = el.getAttribute(name)
          if (
            value &&
            (el.tagName !== 'IFRAME' || isSafeEmbedURL(resolveMediaURL(value)))
          ) {
            el.setAttribute('src', resolveMediaURL(value))
            break
          }
        }
      }
      if (el.tagName === 'IFRAME' && !el.getAttribute('src')) {
        el.remove()
        return
      }
    }
    if (el.tagName === 'IMG') {
      el.setAttribute('loading', 'lazy')
      el.setAttribute('decoding', 'async')
      el.setAttribute('referrerpolicy', 'no-referrer')
    }
    if (el.tagName === 'A') {
      el.setAttribute('rel', 'noopener noreferrer')
      el.setAttribute('target', '_blank')
    }
  })
  DOMPurify.addHook('uponSanitizeAttribute', (node, data) => {
    if (node.tagName === 'IFRAME' && data.attrName === 'src') {
      data.attrValue = resolveMediaURL(data.attrValue)
    }
    if (
      node.tagName === 'IFRAME' &&
      data.attrName === 'src' &&
      !isSafeEmbedURL(data.attrValue)
    ) {
      data.keepAttr = false
    }
  })
}

/** 清洗正文 HTML：移除 script/style 与内联样式、on* 事件，返回安全字符串。 */
export function sanitizeHtml(html: string, baseURL?: string): string {
  if (!html) return ''
  ensureHooks()
  sanitizeBaseURL = null
  if (baseURL) {
    try {
      sanitizeBaseURL = new URL(baseURL)
    } catch {
      sanitizeBaseURL = null
    }
  }
  try {
    return DOMPurify.sanitize(html, {
      USE_PROFILES: { html: true },
      ADD_TAGS: ['iframe'],
      ADD_ATTR: [
        'data-src',
        'data-video-src',
        'data-video-url',
        'data-video',
        'data-hls',
        'data-mp4',
        'data-url',
        'allow',
        'allowfullscreen',
        'frameborder',
        'loading',
        'referrerpolicy',
      ],
      FORBID_TAGS: ['style'],
      FORBID_ATTR: ['style'],
    })
  } finally {
    sanitizeBaseURL = null
  }
}
