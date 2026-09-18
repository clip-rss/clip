import Hls from 'hls.js'
import { useEffect, useMemo, useRef } from 'react'
import { sanitizeHtml, openURL, type ReaderContentStyle } from '../../Utils'
import styles from './ReadingView.module.scss'

interface ReaderContentProps {
  html: string
  baseURL?: string
  style: ReaderContentStyle
  onImageClick: (src: string) => void
  onLinkHover?: (url: string | null) => void
}

/** 渲染清洗后的正文 HTML，委托处理链接（系统浏览器）与图片（灯箱）点击。 */
function ReaderContent(props: ReaderContentProps): JSX.Element {
  const { html, baseURL, style, onImageClick, onLinkHover } = props
  const clean = useMemo(() => sanitizeHtml(html, baseURL), [html, baseURL])
  const contentRef = useRef<HTMLElement>(null)

  useEffect(() => {
    const root = contentRef.current
    if (!root) return

    const lazyMediaNames = [
      'data-video-src',
      'data-video-url',
      'data-hls',
      'data-mp4',
    ]
    for (const holder of Array.from(
      root.querySelectorAll<HTMLElement>(
        '[data-video-src], [data-video-url], [data-hls], [data-mp4]',
      ),
    )) {
      if (['VIDEO', 'AUDIO', 'SOURCE', 'IFRAME'].includes(holder.tagName)) {
        continue
      }
      const src = lazyMediaNames
        .map((name) => holder.getAttribute(name))
        .find((value): value is string => Boolean(value))
      if (!src) continue
      let resolvedSrc = src
      if (baseURL) {
        try {
          resolvedSrc = new URL(src, baseURL).toString()
        } catch {
          continue
        }
      }
      const video = document.createElement('video')
      video.controls = true
      video.preload = 'metadata'
      video.src = resolvedSrc
      holder.replaceWith(video)
    }

    const players: Hls[] = []
    const videos = Array.from(root.querySelectorAll('video'))
    for (const video of videos) {
      const src =
        video.getAttribute('src') ??
        video.querySelector('source')?.getAttribute('src') ??
        ''
      if (!/\.m3u8(?:$|[?#])/i.test(src)) continue

      if (Hls.isSupported()) {
        const hls = new Hls()
        hls.loadSource(src)
        hls.attachMedia(video)
        players.push(hls)
      } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
        video.src = src
      }
    }

    return () => {
      for (const player of players) player.destroy()
    }
  }, [clean])

  function handleClick(e: React.MouseEvent<HTMLElement>): void {
    const target = e.target as HTMLElement
    const anchor = target.closest('a')
    if (anchor) {
      e.preventDefault()
      const href = anchor.getAttribute('href')
      if (href) openURL(href)
      return
    }
    const img = target.closest('img')
    if (img) {
      const src =
        (img as HTMLImageElement).currentSrc || (img as HTMLImageElement).src
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
