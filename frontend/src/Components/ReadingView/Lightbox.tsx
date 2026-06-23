import { useCallback, useEffect, useRef, useState } from 'react'
import { CloseIcon } from './Icons'
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

/** 图片放大灯箱：鼠标滚轮缩放、拖动平移、工具栏操作，Esc 或点击遮罩关闭。 */
function Lightbox(props: LightboxProps): JSX.Element | null {
  const { src, onClose } = props
  const [scale, setScale] = useState(1)
  const [rotation, setRotation] = useState(0)
  const [pos, setPos] = useState<Position>({ x: 0, y: 0 })
  const [dragging, setDragging] = useState(false)

  // 用 ref 持有最新 pos，避免 handleImgMouseDown 依赖 pos 导致每次拖动都重建回调
  const posRef = useRef(pos)
  posRef.current = pos

  // 拖动原点（避免闭包中读取过期 state）
  const dragRef = useRef<{ origin: Position; originPos: Position } | null>(null)

  // 切换图片时重置所有变换
  useEffect(() => {
    setScale(1)
    setRotation(0)
    setPos({ x: 0, y: 0 })
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
  const rotateCw = useCallback(
    () => setRotation((r) => (r + ROTATION_STEP) % 360),
    [],
  )
  const reset = useCallback(() => {
    setScale(1)
    setRotation(0)
    setPos({ x: 0, y: 0 })
  }, [])

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
          onClick={rotateCw}
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
      </div>

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

function RotateIcon(): JSX.Element {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M17.8 21H22v1h-6v-6h1v4.508a9.861 9.861 0 1 0-5 1.373v.837A10.748 10.748 0 1 1 17.8 21zM11 11v2h2v-2z" />
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

export default Lightbox
