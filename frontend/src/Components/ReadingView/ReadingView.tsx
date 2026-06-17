import { useLayoutEffect, useMemo, useRef, useState } from 'react'
import clsx from 'clsx'
import { useArticleStore, useSidebarStore, useReaderStore } from '../../Stores'
import { formatRelativeTime, readerBackgroundClass, readerContentStyle } from '../../Utils'
import ReaderToolbar from './ReaderToolbar'
import ReaderContent from './ReaderContent'
import Lightbox from './Lightbox'
import styles from './ReadingView.module.scss'

function ReadingView(): JSX.Element {
  const item = useArticleStore((s) => s.items.find((it) => it.id === s.selectedItemId) ?? null)
  const feeds = useSidebarStore((s) => s.feeds)
  const prefs = useReaderStore()

  const [lightboxSrc, setLightboxSrc] = useState<string | null>(null)

  const scrollRef = useRef<HTMLDivElement>(null)
  const positionsRef = useRef<Map<number, number>>(new Map())
  const currentIdRef = useRef<number | null>(null)

  const itemId = item?.id ?? null
  // 切换文章时恢复该文的滚动位置
  useLayoutEffect(() => {
    currentIdRef.current = itemId
    if (itemId !== null && scrollRef.current) {
      scrollRef.current.scrollTop = positionsRef.current.get(itemId) ?? 0
    }
  }, [itemId])

  function handleScroll(): void {
    const id = currentIdRef.current
    if (id !== null && scrollRef.current) {
      positionsRef.current.set(id, scrollRef.current.scrollTop)
    }
  }

  const contentStyle = useMemo(
    () =>
      readerContentStyle({
        fontFamily: prefs.fontFamily,
        fontSize: prefs.fontSize,
        lineHeight: prefs.lineHeight,
        width: prefs.width,
        background: prefs.background,
      }),
    [prefs.fontFamily, prefs.fontSize, prefs.lineHeight, prefs.width, prefs.background],
  )
  const bgClass = readerBackgroundClass(prefs.background)

  if (!item) {
    return (
      <div className={styles.reader}>
        <div className={styles.empty}>
          <p>选择一篇文章以阅读</p>
        </div>
      </div>
    )
  }

  const sourceName = feeds.find((f) => f.id === item.feedId)?.title ?? ''

  return (
    <div className={styles.reader}>
      <ReaderToolbar item={item} />
      <div
        ref={scrollRef}
        className={clsx(styles.scroll, bgClass)}
        onScroll={handleScroll}
      >
        <div className={styles.article} style={{ maxWidth: contentStyle.maxWidth }}>
          <h1 className={styles.title} style={{ fontFamily: contentStyle.fontFamily }}>
            {item.title}
          </h1>
          <div className={styles.meta}>
            {item.author ? <span>{item.author}</span> : null}
            {item.author ? <span className={styles.metaDot}>·</span> : null}
            <span>{formatRelativeTime(item.publishedAt)}</span>
            {sourceName ? <span className={styles.metaDot}>·</span> : null}
            {sourceName ? <span>{sourceName}</span> : null}
          </div>
          <div className={styles.divider} />
          <ReaderContent html={item.content} style={contentStyle} onImageClick={setLightboxSrc} />
          <div className={styles.endHint}>已是全部内容</div>
        </div>
      </div>
      <Lightbox src={lightboxSrc} onClose={() => setLightboxSrc(null)} />
    </div>
  )
}

export default ReadingView
