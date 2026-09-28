import { useEffect, useMemo, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import {
  sanitizeHtml,
  openURL,
  resolveLink,
  videoFailedPlaceholder,
  type ReaderContentStyle,
} from '../../Utils'
import styles from './ReadingView.module.scss'

interface ReaderContentProps {
  html: string
  style: ReaderContentStyle
  /** 文章原文地址：正文媒体走代理时用它作 Referer，并作为相对地址的解析基准。 */
  articleUrl: string
  onImageClick: (src: string) => void
  onVideoClick: (src: string) => void
  onLinkHover?: (url: string | null) => void
}

/** 系统「减弱动态」偏好或设置里的「关闭动画」生效时，锚点跳转不做平滑滚动。
 *  CSS 的 `scroll-behavior: auto` 管不到 `scrollIntoView` 显式传入的 behavior，只能在这判。 */
function prefersReducedMotion(): boolean {
  if (document.documentElement.classList.contains('reduce-motion')) return true
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/** 渲染清洗后的正文 HTML，委托处理链接（系统浏览器）、图片与视频（灯箱）点击。 */
function ReaderContent(props: ReaderContentProps): JSX.Element {
  const { t } = useTranslation()
  const { html, style, articleUrl, onImageClick, onVideoClick, onLinkHover } =
    props
  const videoFailedLabel = t('reader.videoFailed')
  const contentRef = useRef<HTMLElement>(null)

  const clean = useMemo(
    () =>
      sanitizeHtml(html, {
        videoSticker: true,
        playLabel: t('reader.videoPlay'),
        failedLabel: t('reader.videoFailed'),
        articleUrl,
      }),
    [html, t, articleUrl],
  )

  // 有视频源、但实际拉取失败（地址失效 / 403 / 断网）时，把贴片换成失败文案。
  // <video> 的 error 事件不冒泡，只能逐个元素挂监听；正文是 dangerouslySetInnerHTML
  // 一次性写入的，故在 effect 里按当前 HTML 重新绑定（clean 变化即重新渲染后再绑）。
  useEffect(() => {
    const root = contentRef.current
    if (!root) return
    const videos = Array.from(root.querySelectorAll('video'))
    if (videos.length === 0) return

    const replaceFailed = (video: HTMLVideoElement): void => {
      const target = video.closest('[data-video-box]') ?? video
      target.replaceWith(videoFailedPlaceholder(document, videoFailedLabel))
    }
    const onError = (e: Event): void => {
      replaceFailed(e.currentTarget as HTMLVideoElement)
    }

    for (const v of videos) {
      v.addEventListener('error', onError)
      // effect 挂载前就已失败的（同步失败 / 命中缓存）补一次
      if (v.error) replaceFailed(v)
    }
    return () => {
      for (const v of videos) v.removeEventListener('error', onError)
    }
  }, [clean, videoFailedLabel])

  function handleClick(e: React.MouseEvent<HTMLElement>): void {
    const target = e.target as HTMLElement
    const anchor = target.closest('a')
    if (anchor) {
      e.preventDefault()
      // 裸 href 不能直接丢给 openURL：`#toc` 这类锚点没有 scheme，会被后端判废并
      // reject，最终炸成整页崩溃。落点必须先判定（见 Utils/Links.ts）。
      const link = resolveLink(anchor.getAttribute('href') ?? '', articleUrl)
      if (link.kind === 'anchor') scrollToAnchor(link.id)
      else if (link.kind === 'external') openURL(link.url)
      // kind === 'invalid'：javascript: / mailto: / 解析不出来的地址，静默忽略。
      return
    }
    // 视频贴片：点击本体或叠加的播放按钮（按钮经事件冒泡到这里）都交给视频灯箱。
    // 已由 sanitizeHtml 去控件并包一层 [data-video-box]，这里取其中的 <video> 取源。
    const videoBox = target.closest('[data-video-box]')
    const video = (videoBox?.querySelector('video') ??
      (target.closest(
        'video',
      ) as HTMLVideoElement | null)) as HTMLVideoElement | null
    if (video) {
      // 拦截成「打开视频灯箱」：preventDefault 阻止内联播放/暂停的默认动作，
      // 再补一次 pause() 兜底（个别默认动作仍然触发的场景下保证内联视频保持暂停，
      // 避免灯箱关掉后正文里的视频已在静默播放）。
      e.preventDefault()
      video.pause()
      const src = video.currentSrc || video.src
      if (src) onVideoClick(src)
      return
    }
    const img = target.closest('img')
    if (img) {
      // 优先取 data-origin-src：清洗时图片 src 已被换成 /__clip/media?... 代理地址，
      // 而灯箱展示与「下载图片」要的是未代理的原始地址（后端会自己补 Referer）。
      const el = img as HTMLImageElement
      const src = el.dataset.originSrc || el.currentSrc || el.src
      if (src) onImageClick(src)
    }
  }

  // 正文里的「目录」锚点：滚到正文内对应的 id。限定在 contentRef 里查，
  // 免得撞上应用外壳的同名 id（如 #root 那一层的布局节点）。
  function scrollToAnchor(id: string): void {
    const el = contentRef.current?.querySelector(`#${CSS.escape(id)}`)
    if (!el) return
    el.scrollIntoView({
      behavior: prefersReducedMotion() ? 'auto' : 'smooth',
      block: 'start',
    })
  }

  function handleMouseOver(e: React.MouseEvent<HTMLElement>): void {
    const anchor = (e.target as HTMLElement).closest('a')
    if (!anchor) return
    // 预览条显示的是「点了会去哪」。页内锚点和打不开的地址都不去别处，
    // 显示 `#toc` 或解析失败的空串只会误导，直接清空。
    const link = resolveLink(anchor.getAttribute('href') ?? '', articleUrl)
    onLinkHover?.(link.kind === 'external' ? link.url : null)
  }

  function handleMouseOut(e: React.MouseEvent<HTMLElement>): void {
    const relatedTarget = e.relatedTarget as HTMLElement | null
    if (!relatedTarget || !relatedTarget.closest('a')) {
      onLinkHover?.(null)
    }
  }

  return (
    <article
      ref={contentRef}
      className={styles.content}
      style={{
        fontFamily: style.fontFamily,
        fontSize: style.fontSize,
        lineHeight: style.lineHeight,
      }}
      onClick={handleClick}
      onMouseOver={handleMouseOver}
      onMouseOut={handleMouseOut}
      dangerouslySetInnerHTML={{ __html: clean }}
    />
  )
}

export default ReaderContent
