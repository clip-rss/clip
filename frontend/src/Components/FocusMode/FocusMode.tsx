import { useTranslation } from 'react-i18next'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import clsx from 'clsx'
import {
  useArticleStore,
  useFocusReaderStore,
  useLayoutStore,
  useSidebarStore,
} from '../../Stores'
import { useArticleNavigation, usePlatform, useSelectedItem } from '../../Hooks'
import {
  hasArticleBody,
  readerBackgroundClass,
  readerContentStyle,
  restoreScrollFraction,
} from '../../Utils'
import {
  ReaderArticle,
  Lightbox,
  NotePanel,
  type LightboxKind,
} from '../ReadingView'
import FocusControlBar from './FocusControlBar'
import styles from './FocusMode.module.scss'

/** 进入/退出过渡时长，需与 FocusMode.module.scss 中 .overlay 过渡一致。 */
const TRANSITION_MS = 300
/** 触发控制条显示的顶部热区高度（px）。 */
const TOP_ZONE = 40
/** 无操作后控制条自动隐藏延迟（ms）。 */
const BAR_HIDE_MS = 3000

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
  const readerReturnRatio = useLayoutStore((s) => s.readerReturnRatio)
  const closeNotePanel = useLayoutStore((s) => s.closeNotePanel)
  const platform = usePlatform()

  const item = useSelectedItem()
  const feeds = useSidebarStore((s) => s.feeds)
  // 专注模式读写自己那套偏好，与三栏阅读的互不影响（无 width：宽度下面固定 680px）
  const prefs = useFocusReaderStore()
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
  // 阅读设置菜单展开时也不能隐藏控制条：菜单是 portal 出去的，鼠标多半停在菜单上，
  // 控制条会先收到 mouseleave，若只看 hoveringBar 就会在菜单开着的时候淡出。
  const settingsMenuOpen = useRef(false)

  const scheduleHide = useCallback(() => {
    if (hideTimer.current) window.clearTimeout(hideTimer.current)
    hideTimer.current = window.setTimeout(() => {
      if (!hoveringBar.current && !settingsMenuOpen.current)
        setBarVisible(false)
    }, BAR_HIDE_MS)
  }, [])

  const flashBar = useCallback(() => {
    setBarVisible(true)
    scheduleHide()
  }, [scheduleHide])

  const handleSettingsMenuOpenChange = useCallback(
    (open: boolean) => {
      settingsMenuOpen.current = open
      if (open) {
        setBarVisible(true)
        if (hideTimer.current) window.clearTimeout(hideTimer.current)
        return
      }
      scheduleHide()
    },
    [scheduleHide],
  )

  // 进入时显示控制条并启动自动隐藏；卸载时清理计时器
  useEffect(() => {
    if (!mounted) return
    flashBar()
    return () => {
      if (hideTimer.current) window.clearTimeout(hideTimer.current)
    }
  }, [mounted, flashBar])

  // ===== 媒体灯箱（图片/视频） =====
  const [lightbox, setLightbox] = useState<{
    kind: LightboxKind
    src: string
  } | null>(null)
  const lightboxRef = useRef(lightbox)
  lightboxRef.current = lightbox

  // ===== 链接预览条（与阅读视图同款：hover 段落链接时显示目标地址） =====
  const [previewUrl, setPreviewUrl] = useState('')
  const [previewVisible, setPreviewVisible] = useState(false)

  // item 有正文才渲染文章体，否则展示空状态。
  // 与阅读视图共用 hasArticleBody，两处对「提取到全文后」的判断不会分叉；
  // 显示模式也要一起传，否则空状态判定会与 ReaderArticle 实际渲染的正文对不上。
  const showSummary = useArticleStore((s) => s.showSummary)
  const hasBody = item ? hasArticleBody(item, showSummary) : false

  // ===== 切换文章：回到顶部 + 标题闪现 =====
  const scrollRef = useRef<HTMLDivElement>(null)
  const itemId = item?.id ?? null
  useEffect(() => {
    if (!mounted) return
    if (scrollRef.current) scrollRef.current.scrollTop = 0
    flashBar()
    // j/k 切文时不经过 mouseout，残留的链接预览会指向旧文章，直接清掉。
    setPreviewVisible(false)
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
          // 阅读设置菜单开着时，这一下 Escape 已经被 Radix 消费掉了——它在 document
          // 捕获阶段就响应（早于这里的 window 冒泡监听），关闭菜单的同时 preventDefault。
          // 不认这个标记的话，一次 Escape 会既关菜单又退出专注模式。
          if (e.defaultPrevented) return
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

  // 宽度不走偏好设置：专注模式正文铺满窗口，等同阅读视图的「全宽」。
  // 两侧留白由 ReaderArticle 的 .article padding 提供。
  const contentStyle = useMemo(
    () =>
      readerContentStyle({
        fontFamily: prefs.fontFamily,
        fontSize: prefs.fontSize,
        lineHeight: prefs.lineHeight,
        width: 'full',
        background: prefs.background,
      }),
    [prefs.fontFamily, prefs.fontSize, prefs.lineHeight, prefs.background],
  )

  const bgClass = readerBackgroundClass(prefs.background)

  function handleMouseMove(e: React.MouseEvent): void {
    if (e.clientY <= TOP_ZONE) setBarVisible(true)
    scheduleHide()
  }

  // 正文段落链接 hover：显示目标地址，与阅读视图同一套逻辑（见 ReadingView）。
  function handleLinkHover(url: string | null): void {
    if (url) {
      setPreviewUrl(url)
      setPreviewVisible(true)
    } else {
      setPreviewVisible(false)
    }
  }

  // 「返回上一个阅读位置」：一次性回到锚点跳转前的进度，之后清空记忆（按钮消失）。
  // 记忆在切文章时由常驻挂载的 ReadingView 清空（见 ReadingView 的 itemId effect），
  // 专注模式只是 overlay，此处只管恢复。
  function handleBackToPosition(): void {
    const ratio = useLayoutStore.getState().readerReturnRatio
    const el = scrollRef.current
    if (ratio === null || !el) return
    restoreScrollFraction(el, ratio)
    useLayoutStore.getState().clearReaderReturnRatio()
  }

  if (!mounted) return null

  const sourceName = item
    ? (feeds.find((f) => f.id === item.feedId)?.title ?? '')
    : ''

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
        canBackToPosition={readerReturnRatio !== null}
        onBackToPosition={handleBackToPosition}
        onExit={exitFocus}
        onBarEnter={() => {
          hoveringBar.current = true
          if (hideTimer.current) window.clearTimeout(hideTimer.current)
        }}
        onBarLeave={() => {
          hoveringBar.current = false
          scheduleHide()
        }}
        onSettingsMenuOpenChange={handleSettingsMenuOpenChange}
      />

      <div ref={scrollRef} className={styles.scroll} data-reader-scroll="focus">
        {item && hasBody ? (
          <div key={item.id} className={styles.fadeIn}>
            <ReaderArticle
              item={item}
              sourceName={sourceName}
              contentStyle={contentStyle}
              onImageClick={(src) => setLightbox({ kind: 'image', src })}
              onVideoClick={(src) => setLightbox({ kind: 'video', src })}
              onLinkHover={handleLinkHover}
            />
          </div>
        ) : (
          <div className={styles.empty}>
            {item ? t('reader.noContent') : t('focus.empty')}
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

      {item && notePanelOpen ? (
        <NotePanel item={item} onClose={closeNotePanel} />
      ) : null}

      <Lightbox
        kind={lightbox?.kind}
        src={lightbox?.src ?? null}
        articleUrl={item?.url ?? ''}
        onClose={() => setLightbox(null)}
      />
    </div>
  )
}

export default FocusMode
