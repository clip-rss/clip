import { useLayoutEffect, useRef, useState } from 'react'
import { useToastStore } from '../../Stores'
import styles from './Toast.module.scss'

/**
 * Toast 通知容器。
 *
 * 固定在窗口顶部居中，浮在所有内容之上（z-index: 300）。
 * 每条 toast 从顶部滑入，显示一段时间后自动滑出，也可手动关闭。
 * 鼠标悬停时暂停自动消失，移出后恢复。
 * 超过 3 条时折叠为堆叠态：卡片保持完整尺寸按 z-index 层叠，
 * 最早一条在最前面完整显示，新来的压在下方、只露出底缘一条边；
 * 鼠标悬停时展开为完整列表。
 *
 * 使用：调用 `useToastStore.getState().addToast(msg, type)`。
 */

/** 超过该数量时进入堆叠态。 */
const STACK_LIMIT = 3
/** 堆叠态下相邻卡片底缘的错位间距（px）。 */
const PEEK = 6
/** 展开态下卡片之间的间距（px）。 */
const GAP = 6

function ToastContainer(): JSX.Element | null {
  const queue = useToastStore((s) => s.queue)
  const hovering = useToastStore((s) => s.hovering)
  const dismiss = useToastStore((s) => s.dismiss)
  const pauseAll = useToastStore((s) => s.pauseAll)
  const resumeAll = useToastStore((s) => s.resumeAll)

  /* 每张卡片的实测高度，用于计算堆叠/展开两种排布的 y 偏移 */
  const [heights, setHeights] = useState<Map<number, number>>(new Map())
  const cardRefs = useRef(new Map<number, HTMLDivElement>())

  useLayoutEffect(() => {
    const next = new Map<number, number>()
    let changed = cardRefs.current.size !== heights.size
    cardRefs.current.forEach((el, id) => {
      const h = el.offsetHeight
      next.set(id, h)
      if (heights.get(id) !== h) changed = true
    })
    if (changed) setHeights(next)
  })

  if (queue.length === 0) return null

  const stacked = queue.length > STACK_LIMIT
  const expanded = stacked && hovering

  /*
   * 逐张累计 y 偏移：
   * - 展开（含未堆叠）时按「卡片高度 + 间距」正常列表排布；
   * - 堆叠时让下一张的底缘恰好压在这张底缘下方 PEEK 处，
   *   即 y(i+1) + H(i+1) = y(i) + H(i) + PEEK，卡片高度不齐也只露一条边。
   */
  const offsets: number[] = []
  let y = 0
  for (let i = 0; i < queue.length; i++) {
    offsets.push(y)
    const h = heights.get(queue[i].id) ?? 0
    if (stacked && !expanded) {
      const hNext =
        i + 1 < queue.length ? (heights.get(queue[i + 1].id) ?? 0) : 0
      y += h + PEEK - hNext
    } else {
      y += h + GAP
    }
  }
  /* 展开态末尾多算了一个 GAP，扣掉 */
  const containerHeight = expanded ? y - GAP : y

  return (
    <div
      className={styles.container}
      style={{ height: `${containerHeight}px` }}
      onMouseEnter={pauseAll}
      onMouseLeave={resumeAll}
      aria-live="polite"
      role="status"
    >
      {queue.map((t, i) => {
        /* 类型对应的背景色调 */
        const typeClass =
          t.type === 'error'
            ? styles.toastError
            : t.type === 'success'
              ? styles.toastSuccess
              : styles.toastInfo

        return (
          <div
            key={t.id}
            ref={(el) => {
              if (el) cardRefs.current.set(t.id, el)
              else cardRefs.current.delete(t.id)
            }}
            className={styles.cardSlot}
            style={{
              transform: `translateY(${offsets[i]}px)`,
              zIndex: queue.length - i,
            }}
          >
            <div
              className={`${styles.toast} ${typeClass} ${t.exiting ? styles.exiting : ''}`}
              role="alert"
            >
              <span className={styles.message}>{t.message}</span>
              <button
                className={styles.close}
                onClick={() => dismiss(t.id)}
                title="关闭"
                aria-label="关闭"
              >
                ✕
              </button>
            </div>
          </div>
        )
      })}
    </div>
  )
}

export default ToastContainer
