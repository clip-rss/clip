import { useCallback, useRef } from 'react'
import styles from './Divider.module.scss'

interface DividerProps {
  onResize: (delta: number) => void
}

function Divider(props: DividerProps): JSX.Element {
  const { onResize } = props
  const startXRef = useRef(0)
  const draggingRef = useRef(false)

  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault()
      startXRef.current = e.clientX
      draggingRef.current = true

      function handleMouseMove(ev: MouseEvent): void {
        if (!draggingRef.current) return
        const delta = ev.clientX - startXRef.current
        startXRef.current = ev.clientX
        onResize(delta)
      }

      function handleMouseUp(): void {
        draggingRef.current = false
        document.removeEventListener('mousemove', handleMouseMove)
        document.removeEventListener('mouseup', handleMouseUp)
        document.body.style.cursor = ''
        document.body.style.userSelect = ''
      }

      document.body.style.cursor = 'col-resize'
      document.body.style.userSelect = 'none'
      document.addEventListener('mousemove', handleMouseMove)
      document.addEventListener('mouseup', handleMouseUp)
    },
    [onResize],
  )

  return (
    <div className={styles.divider} onMouseDown={handleMouseDown}>
      <div className={styles.handle}>
        <span />
        <span />
        <span />
      </div>
    </div>
  )
}

export default Divider
