import { useEffect } from 'react'
import { CloseIcon } from './Icons'
import styles from './ReadingView.module.scss'

interface LightboxProps {
  src: string | null
  onClose: () => void
}

/** 图片放大灯箱：点击遮罩或按 Esc 关闭。 */
function Lightbox(props: LightboxProps): JSX.Element | null {
  const { src, onClose } = props

  useEffect(() => {
    if (!src) return
    function onKey(e: KeyboardEvent): void {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [src, onClose])

  if (!src) return null

  return (
    <div className={styles.lightbox} onClick={onClose} role="dialog" aria-modal="true">
      <button type="button" className={styles.lightboxClose} onClick={onClose} aria-label="关闭">
        <CloseIcon size={20} />
      </button>
      <img src={src} alt="" className={styles.lightboxImg} onClick={(e) => e.stopPropagation()} />
    </div>
  )
}

export default Lightbox
