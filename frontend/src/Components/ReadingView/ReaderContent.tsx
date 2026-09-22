import { useEffect, useMemo, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import {
  sanitizeHtml,
  openURL,
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
      const href = anchor.getAttribute('href')
      if (href) openURL(href)
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

  function handleMouseOver(e: React.MouseEvent<HTMLElement>): void {
    const anchor = (e.target as HTMLElement).closest('a')
    if (anchor) {
      const href = anchor.getAttribute('href')
      if (href) onLinkHover?.(href)
    }
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
