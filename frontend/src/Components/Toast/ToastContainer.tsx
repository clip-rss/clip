import { useToastStore } from '../../Stores'
import styles from './Toast.module.scss'

/**
 * Toast 通知容器。
 *
 * 固定在窗口顶部居中，浮在所有内容之上（z-index: 110）。
 * 每条 toast 从顶部滑入，显示一段时间后自动滑出，也可手动关闭。
 * 鼠标悬停时暂停自动消失，移出后恢复。
 * 超过 3 条时旧 toast 折叠为堆叠指示器。
 *
 * 使用：调用 `useToastStore.getState().addToast(msg, type)`。
 */
function ToastContainer(): JSX.Element | null {
  const queue = useToastStore((s) => s.queue)
  const dismiss = useToastStore((s) => s.dismiss)
  const pauseAll = useToastStore((s) => s.pauseAll)
  const resumeAll = useToastStore((s) => s.resumeAll)

  if (queue.length === 0) return null

  const VISIBLE = 3
  const visible = queue.slice(0, VISIBLE)
  const overflow = queue.slice(VISIBLE)
  const overflowCount = overflow.length

  return (
    <div
      className={styles.container}
      onMouseEnter={pauseAll}
      onMouseLeave={resumeAll}
      aria-live="polite"
      role="status"
    >
      {visible.map((t) => {
        /* 类型对应的图标与颜色 */
        const iconClass =
          t.type === 'error'
            ? styles.iconError
            : t.type === 'success'
              ? styles.iconSuccess
              : styles.iconInfo

        const icon = t.type === 'error' ? '✕' : t.type === 'success' ? '✓' : 'ℹ'

        return (
          <div
            key={t.id}
            className={`${styles.toast} ${t.exiting ? styles.exiting : ''}`}
            role="alert"
          >
            <span className={`${styles.icon} ${iconClass}`} aria-hidden="true">
              {icon}
            </span>
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
        )
      })}

      {overflowCount > 0 ? (
        <div className={styles.stackArea} aria-hidden="true">
          {/* 堆叠装饰线：模拟下方还压着几张卡片 */}
          <div className={styles.stackLine} />
          <div className={styles.stackLine} />
          <div className={styles.stackCount}>+{overflowCount}</div>
        </div>
      ) : null}
    </div>
  )
}

export default ToastContainer
