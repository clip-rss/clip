import { useTranslation } from 'react-i18next'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import clsx from 'clsx'
import { useArticleStore, useLayoutStore, useReaderStore, useSidebarStore } from '../../Stores'
import { useArticleNavigation, usePlatform } from '../../Hooks'
import { readerBackgroundClass, readerContentStyle } from '../../Utils'
import { ReaderArticle, Lightbox, NotePanel } from '../ReadingView'
import FocusControlBar from './FocusControlBar'
import styles from './FocusMode.module.scss'

/** 进入/退出过渡时长，需与 FocusMode.module.scss 中 .overlay 过渡一致。 */
const TRANSITION_MS = 300
/** 触发控制条显示的顶部热区高度（px）。 */
const TOP_ZONE = 40
/** 无操作后控制条自动隐藏延迟（ms）。 */
const BAR_HIDE_MS = 3000
/** 专注模式固定阅读宽度。 */
const FOCUS_MAX_WIDTH = '680px'

/**
 * 专注阅读模式：全屏单栏覆盖层。
 *
 * 自管挂载/过渡生命周期——`focusMode` 关闭后保留 300ms 以播放退出动画再卸载。
 */
function FocusMode(): JSX.Element | null {
  const { t } = useTranslation()
  const focusMode = useLayoutStore((s) => s.focusMode)
  const exitFocus = useLayoutStore((s) => s.exitFocus)
  const notePanelOpen = useLayoutStore((s) => s.notePanelOpen)
  const closeNotePanel = useLayoutStore((s) => s.closeNotePanel)
  const platform = usePlatform()

  const item = useArticleStore((s) => s.items.find((it) => it.id === s.selectedItemId) ?? null)
  const feeds = useSidebarStore((s) => s.feeds)
  const prefs = useReaderStore()
  const nav = useArticleNavigation()

  // ===== 挂载 / 过渡生命周期 =====
  const [mounted, setMounted] = useState(focusMode)
  const [active, setActive] = useState(false)

  useEffect(() => {
    if (focusMode) {
      setMounted(true)
      const raf = requestAnimationFrame(() => setActive(true))
      return () => cancelAnimationFrame(raf)
    }
    setActive(false)
    const t = window.setTimeout(() => setMounted(false), TRANSITION_MS)
    return () => window.clearTimeout(t)
  }, [focusMode])

  // ===== 浮动控制条显隐 =====
  const [barVisible, setBarVisible] = useState(true)
  const hideTimer = useRef<number | null>(null)
  const hoveringBar = useRef(false)

  const scheduleHide = useCallback(() => {
    if (hideTimer.current) window.clearTimeout(hideTimer.current)
    hideTimer.current = window.setTimeout(() => {
      if (!hoveringBar.current) setBarVisible(false)
    }, BAR_HIDE_MS)
  }, [])

  const flashBar = useCallback(() => {
    setBarVisible(true)
    scheduleHide()
  }, [scheduleHide])

  // 进入时显示控制条并启动自动隐藏；卸载时清理计时器
  useEffect(() => {
    if (!mounted) return
    flashBar()
    return () => {
      if (hideTimer.current) window.clearTimeout(hideTimer.current)
    }
  }, [mounted, flashBar])

  // ===== 图片灯箱 =====
  const [lightboxSrc, setLightboxSrc] = useState<string | null>(null)
  const lightboxRef = useRef<string | null>(null)
  lightboxRef.current = lightboxSrc

  // ===== 切换文章：回到顶部 + 标题闪现 =====
  const scrollRef = useRef<HTMLDivElement>(null)
  const itemId = item?.id ?? null
  useEffect(() => {
    if (!mounted) return
    if (scrollRef.current) scrollRef.current.scrollTop = 0
    flashBar()
  }, [itemId, mounted, flashBar])

  // ===== 键盘：Esc 退出 / J·↓ 下一篇 / K·↑ 上一篇 =====
  const navRef = useRef(nav)
  navRef.current = nav
  useEffect(() => {
    if (!mounted) return
    function onKey(e: KeyboardEvent): void {
      const tag = (e.target as HTMLElement | null)?.tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA') return
      switch (e.key) {
        case 'Escape':
          if (lightboxRef.current) return // 灯箱开启时优先关闭灯箱
          e.preventDefault()
          exitFocus()
          break
        case 'j':
        case 'J':
        case 'ArrowDown':
          e.preventDefault()
          navRef.current.goNext()
          break
        case 'k':
        case 'K':
        case 'ArrowUp':
          e.preventDefault()
          navRef.current.goPrev()
          break
        default:
          break
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [mounted, exitFocus])

  const contentStyle = useMemo(() => {
    const base = readerContentStyle({
      fontFamily: prefs.fontFamily,
      fontSize: prefs.fontSize,
      lineHeight: prefs.lineHeight,
      width: prefs.width,
      background: prefs.background,
    })
    return { ...base, maxWidth: FOCUS_MAX_WIDTH }
  }, [prefs.fontFamily, prefs.fontSize, prefs.lineHeight, prefs.width, prefs.background])

  const bgClass = readerBackgroundClass(prefs.background)

  function handleMouseMove(e: React.MouseEvent): void {
    if (e.clientY <= TOP_ZONE) setBarVisible(true)
    scheduleHide()
  }

  if (!mounted) return null

  const sourceName = item ? feeds.find((f) => f.id === item.feedId)?.title ?? '' : ''

  return (
    <div
      className={clsx(styles.overlay, bgClass, active && styles.active)}
      onMouseMove={handleMouseMove}
      role="dialog"
      aria-modal="true"
      aria-label={t('toolbar.focusMode')}
    >
      <FocusControlBar
        item={item}
        visible={barVisible}
        platform={platform}
        onExit={exitFocus}
        onBarEnter={() => {
          hoveringBar.current = true
          if (hideTimer.current) window.clearTimeout(hideTimer.current)
        }}
        onBarLeave={() => {
          hoveringBar.current = false
          scheduleHide()
        }}
      />

      <div ref={scrollRef} className={styles.scroll} data-reader-scroll="focus">
        {item ? (
          <div key={item.id} className={styles.fadeIn}>
            <ReaderArticle
              item={item}
              sourceName={sourceName}
              contentStyle={contentStyle}
              onImageClick={setLightboxSrc}
            />
          </div>
        ) : (
          <div className={styles.empty}>{t('focus.empty')}</div>
        )}
      </div>

      {item && notePanelOpen ? <NotePanel item={item} onClose={closeNotePanel} /> : null}

      <Lightbox src={lightboxSrc} onClose={() => setLightboxSrc(null)} />
    </div>
  )
}

export default FocusMode
