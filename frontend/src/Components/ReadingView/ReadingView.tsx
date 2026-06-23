import { useTranslation } from 'react-i18next'
import { useLayoutEffect, useMemo, useRef, useState } from 'react'
import clsx from 'clsx'
import { useArticleStore, useSidebarStore, useReaderStore, useLayoutStore } from '../../Stores'
import { readerBackgroundClass, readerContentStyle } from '../../Utils'
import ReaderToolbar from './ReaderToolbar'
import ReaderArticle from './ReaderArticle'
import Lightbox from './Lightbox'
import NotePanel from './NotePanel'
import styles from './ReadingView.module.scss'

function ReadingView(): JSX.Element {
  const { t } = useTranslation()
  const item = useArticleStore((s) => s.items.find((it) => it.id === s.selectedItemId) ?? null)
  const feeds = useSidebarStore((s) => s.feeds)
  const prefs = useReaderStore()
  const notePanelOpen = useLayoutStore((s) => s.notePanelOpen)
  const closeNotePanel = useLayoutStore((s) => s.closeNotePanel)

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
          <p>{t('reader.empty')}</p>
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
        data-reader-scroll="main"
      >
        <ReaderArticle
          item={item}
          sourceName={sourceName}
          contentStyle={contentStyle}
          onImageClick={setLightboxSrc}
        />
      </div>
      {notePanelOpen ? <NotePanel item={item} onClose={closeNotePanel} /> : null}
      <Lightbox src={lightboxSrc} onClose={() => setLightboxSrc(null)} />
    </div>
  )
}

export default ReadingView
