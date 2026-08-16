import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { CloseIcon } from './Icons'
import { SystemService } from '../../Utils'
import styles from './ReadingView.module.scss'

interface LightboxProps {
  src: string | null
  onClose: () => void
}

const SCALE_STEP = 0.25
const SCALE_MIN = 0.25
const SCALE_MAX = 8
const ROTATION_STEP = 90

interface Position {
  x: number
  y: number
}

/** 下载按钮的状态：空闲 / 下载中 / 成功 / 失败。 */
type DownloadState = 'idle' | 'downloading' | 'saved' | 'failed'

/** 图片灯箱：鼠标滚轮缩放、拖动平移、工具栏操作，Esc 或点击遮罩关闭。 */
function Lightbox(props: LightboxProps): JSX.Element | null {
  const { src, onClose } = props
  const { t } = useTranslation()
  const [scale, setScale] = useState(1)
  const [rotation, setRotation] = useState(0)
  const [pos, setPos] = useState<Position>({ x: 0, y: 0 })
  const [dragging, setDragging] = useState(false)
  const [downloadState, setDownloadState] = useState<DownloadState>('idle')

  // 用 ref 持有最新 pos，避免 handleImgMouseDown 依赖 pos 导致每次拖动都重建回调
  const posRef = useRef(pos)
  posRef.current = pos

  // 拖动原点（避免闭包中读取过期 state）
  const dragRef = useRef<{ origin: Position; originPos: Position } | null>(null)

  // 下载状态提示的自动隐藏定时器
  const statusTimerRef = useRef<number | null>(null)

  // 切换图片时重置所有变换
  useEffect(() => {
    setScale(1)
    setRotation(0)
    setPos({ x: 0, y: 0 })
    setDownloadState('idle')
    if (statusTimerRef.current !== null) {
      window.clearTimeout(statusTimerRef.current)
      statusTimerRef.current = null
    }
  }, [src])

  // Esc 关闭
  useEffect(() => {
    if (!src) return
    function onKey(e: KeyboardEvent): void {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [src, onClose])

  // ---- 鼠标滚轮缩放 ----
  const handleWheel = useCallback((e: React.WheelEvent) => {
    e.stopPropagation()
    const delta = e.deltaY < 0 ? SCALE_STEP : -SCALE_STEP
    setScale((prev) => Math.min(SCALE_MAX, Math.max(SCALE_MIN, prev + delta)))
  }, [])

  // ---- 工具栏操作 ----
  const zoomIn = useCallback(
    () => setScale((s) => Math.min(SCALE_MAX, s + SCALE_STEP)),
    [],
  )
  const zoomOut = useCallback(
    () => setScale((s) => Math.max(SCALE_MIN, s - SCALE_STEP)),
    [],
  )
  // 逆时针旋转 90 度。用负数角度累积而非取模：CSS transition 对 transform 做
  // 线性插值，若把 0 取模成 270，补间会沿顺时针弧线走 270 度（视觉上先往顺时针转）；
  // 而 0 → -90 → -180 … 每一步都是 -90 增量，动画始终逆时针。
  const rotateCcw = useCallback(
    () => setRotation((r) => r - ROTATION_STEP),
    [],
  )
  const reset = useCallback(() => {
    setScale(1)
    setRotation(0)
    setPos({ x: 0, y: 0 })
  }, [])

  // 下载状态提示几秒后自动消失
  const scheduleStatusReset = useCallback(() => {
    if (statusTimerRef.current !== null) {
      window.clearTimeout(statusTimerRef.current)
    }
    statusTimerRef.current = window.setTimeout(() => {
      statusTimerRef.current = null
      setDownloadState('idle')
    }, 2500)
  }, [])

  // ---- 下载图片到用户指定目录（后端弹保存对话框） ----
  const handleDownload = useCallback(async () => {
    if (!src || downloadState === 'downloading') return
    setDownloadState('downloading')
    try {
      const saved = await SystemService.DownloadImage(src)
      // 用户取消（saved=false）不打扰，回到空闲即可
      setDownloadState(saved ? 'saved' : 'idle')
      if (saved) scheduleStatusReset()
    } catch {
      setDownloadState('failed')
      scheduleStatusReset()
    }
  }, [src, downloadState, scheduleStatusReset])

  // ---- 图片拖动平移 ----
  const handleImgMouseDown = useCallback((e: React.MouseEvent) => {
    e.stopPropagation()
    e.preventDefault()
    setDragging(true)
    dragRef.current = {
      origin: { x: e.clientX, y: e.clientY },
      originPos: posRef.current,
    }
  }, []) // 通过 posRef 读取最新位置，无需依赖 pos

  useEffect(() => {
    if (!dragging) return

    function onMove(e: MouseEvent): void {
      if (!dragRef.current) return
      const dx = e.clientX - dragRef.current.origin.x
      const dy = e.clientY - dragRef.current.origin.y
      setPos({
        x: dragRef.current.originPos.x + dx,
        y: dragRef.current.originPos.y + dy,
      })
    }
    function onUp(): void {
      setDragging(false)
      dragRef.current = null
    }

    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
    return () => {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
  }, [dragging])

  // 阻止工具栏点击冒泡到遮罩层 —— 必须在所有 hook 之后、条件返回之前
  const stopToolbarClick = useCallback(
    (e: React.MouseEvent) => e.stopPropagation(),
    [],
  )

  if (!src) return null

  const cursorClass = dragging
    ? styles.lightboxImgDragging
    : scale > 1
      ? styles.lightboxImgGrab
      : undefined

  return (
    <div
      className={styles.lightbox}
      onClick={onClose}
      onWheel={handleWheel}
      role="dialog"
      aria-modal="true"
    >
      <button
        type="button"
        className={styles.lightboxClose}
        onClick={onClose}
        aria-label="关闭"
      >
        <CloseIcon size={20} />
      </button>

      <div className={styles.lightboxToolbar} onClick={stopToolbarClick}>
        <button
          type="button"
          className={styles.lightboxBtn}
          onClick={zoomOut}
          aria-label="缩小"
        >
          <ZoomOutIcon />
        </button>
        <button
          type="button"
          className={styles.lightboxBtn}
          onClick={zoomIn}
          aria-label="放大"
        >
          <ZoomInIcon />
        </button>
        <button
          type="button"
          className={styles.lightboxBtn}
          onClick={rotateCcw}
          aria-label="旋转"
        >
          <RotateIcon />
        </button>
        <button
          type="button"
          className={styles.lightboxBtn}
          onClick={reset}
          aria-label="复位"
        >
          <ResetIcon />
        </button>
        <button
          type="button"
          className={styles.lightboxBtn}
          onClick={handleDownload}
          disabled={downloadState === 'downloading'}
          aria-label={t('reader.lightbox.download')}
          title={t('reader.lightbox.download')}
        >
          <DownloadIcon />
        </button>
      </div>

      {downloadState === 'saved' || downloadState === 'failed' ? (
        <p className={styles.lightboxStatus} role="status">
          {downloadState === 'saved'
            ? t('reader.lightbox.saved')
            : t('reader.lightbox.failed')}
        </p>
      ) : null}

      <img
        src={src}
        alt=""
        className={`${styles.lightboxImg} ${cursorClass ?? ''}`}
        style={{
          transform: `translate(${pos.x}px, ${pos.y}px) scale(${scale}) rotate(${rotation}deg)`,
        }}
        draggable={false}
        onClick={(e) => e.stopPropagation()}
        onMouseDown={handleImgMouseDown}
      />
    </div>
  )
}

/* ---- 内联 toolbar 图标 ---- */

function ZoomInIcon(): JSX.Element {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
    >
      <circle cx="11" cy="11" r="8" />
      <path d="m21 21-4.3-4.3" />
      <path d="M11 8v6M8 11h6" />
    </svg>
  )
}

function ZoomOutIcon(): JSX.Element {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
    >
      <circle cx="11" cy="11" r="8" />
      <path d="m21 21-4.3-4.3" />
      <path d="M8 11h6" />
    </svg>
  )
}

/** 旋转图标：逆时针箭头，与按钮「逆时针旋转图片」的行为语义一致。 */
function RotateIcon(): JSX.Element {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 15 15"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M9.65332 4.00808C10.4097 4.08488 11 4.72361 11 5.50027V12.5003L10.9922 12.6536C10.9205 13.3596 10.3593 13.9208 9.65332 13.9925L9.5 14.0003H2.5L2.34668 13.9925C1.64071 13.9208 1.07952 13.3596 1.00781 12.6536L1 12.5003V5.50027C1 4.72361 1.59028 4.08488 2.34668 4.00808L2.5 4.00027H9.5L9.65332 4.00808ZM2.5 5.00027C2.22386 5.00027 2 5.22413 2 5.50027V12.5003C2.00003 12.7764 2.22388 13.0003 2.5 13.0003H9.5C9.77612 13.0003 9.99997 12.7764 10 12.5003V5.50027C10 5.22413 9.77614 5.00027 9.5 5.00027H2.5ZM7.59668 0.0637466C7.76077 -0.0636113 7.99965 0.0533557 8 0.261012V1.00027L8.37988 1.00418C10.2476 1.04836 11.6658 1.43172 12.6172 2.38308C13.632 3.39793 14 4.94411 14 7.00027C14 7.27638 13.7761 7.50027 13.5 7.50027C13.2239 7.50027 13 7.27638 13 7.00027C13 5.01892 12.6359 3.81588 11.9102 3.09011C11.2297 2.40969 10.1298 2.04681 8.3623 2.00418L8 2.00027V2.73953C7.99971 2.94726 7.76079 3.06422 7.59668 2.93679L6.00391 1.69754C5.87524 1.59744 5.87523 1.40309 6.00391 1.303L7.59668 0.0637466Z"
        fill="currentColor"
      />
    </svg>
  )
}

function ResetIcon(): JSX.Element {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect x="4" y="4" width="16" height="16" rx="4" ry="4" />
    </svg>
  )
}

/** 下载图标：箭头向下指向横线的下载符号，fill 风格跟随按钮颜色。 */
function DownloadIcon(): JSX.Element {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 15 15"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M12.5 10.0002C12.7761 10.0002 13 10.224 13 10.5002V12.0002C12.9999 13.1042 12.106 14.0002 11.001 14.0002H3.99609C2.89012 14 2.0001 13.103 2 12.0002V10.5002C2 10.224 2.22386 10.0002 2.5 10.0002C2.77614 10.0002 3 10.224 3 10.5002V12.0002C3.0001 12.5539 3.44557 13 3.99609 13.0002H11.001C11.5527 13.0002 11.9999 12.5529 12 12.0002V10.5002C12 10.224 12.2239 10.0002 12.5 10.0002ZM7.5 1.04999C7.74853 1.04999 7.9502 1.25165 7.9502 1.50018V8.41327L10.1816 6.18182C10.3574 6.00609 10.6426 6.00609 10.8184 6.18182C10.994 6.35757 10.9941 6.64283 10.8184 6.81854L7.81836 9.81854C7.64264 9.99413 7.35734 9.99416 7.18164 9.81854L4.18164 6.81854C4.00595 6.64285 4.00603 6.35757 4.18164 6.18182C4.35738 6.00609 4.64262 6.00609 4.81836 6.18182L7.0498 8.41327V1.50018C7.0498 1.25167 7.25149 1.05001 7.5 1.04999Z"
        fill="currentColor"
      />
    </svg>
  )
}

export default Lightbox
