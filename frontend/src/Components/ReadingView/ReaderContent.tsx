import { useMemo } from 'react'
import { sanitizeHtml, openURL, type ReaderContentStyle } from '../../Utils'
import styles from './ReadingView.module.scss'

interface ReaderContentProps {
  html: string
  style: ReaderContentStyle
  onImageClick: (src: string) => void
}

/** 渲染清洗后的正文 HTML，委托处理链接（系统浏览器）与图片（灯箱）点击。 */
function ReaderContent(props: ReaderContentProps): JSX.Element {
  const { html, style, onImageClick } = props
  const clean = useMemo(() => sanitizeHtml(html), [html])

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
      const src = (img as HTMLImageElement).currentSrc || (img as HTMLImageElement).src
      if (src) onImageClick(src)
    }
  }

  return (
    <article
      className={styles.content}
      style={{
        fontFamily: style.fontFamily,
        fontSize: style.fontSize,
        lineHeight: style.lineHeight,
      }}
      onClick={handleClick}
      dangerouslySetInnerHTML={{ __html: clean }}
    />
  )
}

export default ReaderContent
