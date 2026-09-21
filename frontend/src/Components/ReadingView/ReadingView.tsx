import { useTranslation } from 'react-i18next'
import { useLayoutEffect, useMemo, useRef, useState } from 'react'
import clsx from 'clsx'
import {
  useArticleStore,
  useSidebarStore,
  useReaderStore,
  useLayoutStore,
} from '../../Stores'
import { useSelectedItem } from '../../Hooks'
import { readerBackgroundClass, readerContentStyle } from '../../Utils'
import ReaderToolbar from './ReaderToolbar'
import ReaderArticle from './ReaderArticle'
import Lightbox, { type LightboxKind } from './Lightbox'
import NotePanel from './NotePanel'
import styles from './ReadingView.module.scss'

/** 灯箱状态：媒体类型 + 来源 URL，图片与视频共用一个弹层位。 */
interface MediaLightbox {
  kind: LightboxKind
  src: string
}

function ReadingView(): JSX.Element {
  const { t } = useTranslation()
  const item = useSelectedItem()
  const loadingContentId = useArticleStore((s) => s.loadingContentId)
  const feeds = useSidebarStore((s) => s.feeds)
  const prefs = useReaderStore()
  const notePanelOpen = useLayoutStore((s) => s.notePanelOpen)
  const closeNotePanel = useLayoutStore((s) => s.closeNotePanel)

  const [lightbox, setLightbox] = useState<MediaLightbox | null>(null)
  const [previewUrl, setPreviewUrl] = useState('')
  const [previewVisible, setPreviewVisible] = useState(false)

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

  function handleLinkHover(url: string | null): void {
    if (url) {
      setPreviewUrl(url)
      setPreviewVisible(true)
    } else {
      setPreviewVisible(false)
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
    [
      prefs.fontFamily,
      prefs.fontSize,
      prefs.lineHeight,
      prefs.width,
      prefs.background,
    ],
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

  // content 正在加载中（首次点击文章，后端拉取完整正文）
  const isLoadingContent = loadingContentId === item.id && !item.content
  // 加载完成后仍无正文（该条目本身没有正文）时展示空状态
  const hasBody = item.content.trim() !== ''

  return (
    <div className={styles.reader}>
      <ReaderToolbar item={item} />
      <div
        ref={scrollRef}
        className={clsx(styles.scroll, bgClass)}
        onScroll={handleScroll}
        data-reader-scroll="main"
      >
        {isLoadingContent ? (
          <div className={styles.loading}>
            <p>{t('reader.loadingContent')}</p>
          </div>
        ) : hasBody ? (
          <ReaderArticle
            item={item}
            sourceName={sourceName}
            contentStyle={contentStyle}
            onImageClick={(src) => setLightbox({ kind: 'image', src })}
            onVideoClick={(src) => setLightbox({ kind: 'video', src })}
            onLinkHover={handleLinkHover}
          />
        ) : (
          <div className={styles.empty}>
            <p>{t('reader.noContent')}</p>
          </div>
        )}
      </div>
      <div
        className={clsx(
          styles.linkPreview,
          previewVisible && styles.linkPreviewVisible,
        )}
      >
        {previewUrl}
      </div>
      {notePanelOpen ? (
        <NotePanel item={item} onClose={closeNotePanel} />
      ) : null}
      <Lightbox
        kind={lightbox?.kind}
        src={lightbox?.src ?? null}
        onClose={() => setLightbox(null)}
      />
    </div>
  )
}

export default ReadingView
