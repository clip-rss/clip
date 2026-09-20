// 正文 HTML 清洗：去除脚本/事件处理器/内联样式，杜绝 XSS，并统一图片懒加载、外链属性
// 与视频贴片装饰。

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
      el.setAttribute('referrerpolicy', 'no-referrer')
    }
    if (el.tagName === 'A') {
      el.setAttribute('rel', 'noopener noreferrer')
      el.setAttribute('target', '_blank')
    }
    if (el.tagName === 'VIDEO') {
      // 正文里的视频不保留原生控件：播放统一走视频灯箱（ReaderContent 点击委托），
      // 控件由视频灯箱里的播放器提供。这里 RemoveAttribute 而非从白名单排除，
      // 避免影响已入库内容与新内容的渲染一致性。
      el.removeAttribute('controls')
      el.removeAttribute('width')
      el.removeAttribute('height')
      // 无 controls 的视频 Chromium 可能会退化为整文件预载，显式钉住 metadata：
      // 只取首帧做贴片预览，避免滚动到文章就把大 mp4 全下载了。
      el.setAttribute('preload', 'metadata')
    }
  })
}

// videoFailedPlaceholder 生成「视频加载失败」的文案占位元素。
// 类名 video-failed 供 ReadingView.module.scss 命中（提示类文案：斜体 + 次级色）。
//
// 导出供两处复用：清洗期（无视频源，见 decorateVideos）与运行时（有源但加载失败，
// 见 ReaderContent 的 error 事件）。共用同一工厂可避免两边各自拼类名导致样式落空。
export function videoFailedPlaceholder(doc: Document, label: string): Element {
  const span = doc.createElement('span')
  span.className = 'video-failed'
  span.textContent = label
  return span
}

// decorateVideos 把正文视频统一处理成「贴片」：
//   - 有视频源的：包一层容器（class video-box / data-video-box），叠加居中播放按钮，
//     播放由 ReaderContent 的点击委托交给视频灯箱；
//   - 没有视频源的（正文抓不到地址，如靠 JS 动态填充的页面）：根本无法播放，
//     直接用「视频加载失败」文案代替。
function decorateVideos(
  html: string,
  opts: { playLabel: string; failedLabel: string },
): string {
  if (!html.includes('<video')) return html
  const label = opts.playLabel.trim()
  if (!label) return html
  const failedLabel = opts.failedLabel.trim()
  const doc = new DOMParser().parseFromString(html, 'text/html')
  const videos = Array.from(doc.querySelectorAll('video'))
  if (videos.length === 0) return html

  let changed = false
  for (const video of videos) {
    if (video.closest('[data-video-box]')) continue
    const hasSrc =
      video.getAttribute('src') !== null ||
      video.querySelector('source[src]') !== null

    if (!hasSrc) {
      if (!failedLabel) continue
      video.replaceWith(videoFailedPlaceholder(doc, failedLabel))
      changed = true
      continue
    }

    const wrap = doc.createElement('div')
    // 类名供 ReadingView.module.scss 的 `.video-box` 样式命中；data 属性供
    // ReaderContent 的点击委托（closest('[data-video-box]')）使用。两者缺一不可：
    // 只加 data 属性会让样式全部落空（按钮退化成浏览器默认样式）。
    wrap.className = 'video-box'
    wrap.setAttribute('data-video-box', '')
    // 文字按钮：可见文案即无障碍名称，无需再设 aria-label；title 供鼠标悬停提示。
    const play = doc.createElement('button')
    play.type = 'button'
    play.className = 'video-play'
    play.setAttribute('title', label)
    play.textContent = label

    video.parentNode?.insertBefore(wrap, video)
    wrap.appendChild(video)
    wrap.appendChild(play)
    changed = true
  }
  return changed ? doc.body.innerHTML : html
}

export interface SanitizeOptions {
  /** 把正文 <video> 装饰成贴片：有源的加播放按钮，无源的替换为失败文案。 */
  videoSticker?: boolean
  /** videoSticker 时播放按钮的文案（可见文本与 title 共用；空则不做装饰）。 */
  playLabel?: string
  /** videoSticker 时「视频加载失败」占位文案（空则无源视频保持原样）。 */
  failedLabel?: string
}

/** 清洗正文 HTML：移除 script/style 与内联样式、on* 事件，返回安全字符串。 */
export function sanitizeHtml(html: string, options?: SanitizeOptions): string {
  if (!html) return ''
  ensureHooks()
  const clean = DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
    FORBID_TAGS: ['style'],
    FORBID_ATTR: ['style'],
  })
  return options?.videoSticker
    ? decorateVideos(clean, {
        playLabel: options.playLabel ?? '',
        failedLabel: options.failedLabel ?? '',
      })
    : clean
}
