// 正文 HTML 清洗：去除脚本/事件处理器/内联样式，杜绝 XSS，并统一图片懒加载与外链属性。

import DOMPurify from 'dompurify'

let hooksReady = false

function ensureHooks(): void {
  if (hooksReady) return
  hooksReady = true
  DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    const el = node as Element
    if (el.tagName === 'IMG') {
      el.setAttribute('loading', 'lazy')
      el.setAttribute('decoding', 'async')
    }
    if (el.tagName === 'A') {
      el.setAttribute('rel', 'noopener noreferrer')
      el.setAttribute('target', '_blank')
    }
  })
}

/** 清洗正文 HTML：移除 script/style 与内联样式、on* 事件，返回安全字符串。 */
export function sanitizeHtml(html: string): string {
  if (!html) return ''
  ensureHooks()
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
    FORBID_TAGS: ['style'],
    FORBID_ATTR: ['style'],
  })
}
