import clsx from 'clsx'
import { badgeWarnProgress } from '../../Utils'
import styles from './Sidebar.module.scss'

interface UnreadBadgeProps {
  count: number
  /**
   * 未读相对保留上限的负载（0~1，Utils/badgeLoad 的结果）。
   * null/缺省表示未设上限，badge 保持常态 accent 配色。
   */
  load?: number | null
}

/**
 * 未读计数胶囊：count 为 0 时不渲染。
 * 负载达到阈值（80%）后进入警告配色，颜色随逼近 100% 由 --warning 渐变到 --danger。
 */
function UnreadBadge(props: UnreadBadgeProps): JSX.Element | null {
  const { count, load } = props
  if (count <= 0) return null
  const progress = badgeWarnProgress(load ?? null)
  return (
    <span
      className={clsx(styles.badge, progress !== null && styles.warn)}
      style={
        progress !== null
          ? ({
              '--badge-warn': `${Math.round(progress * 100)}%`,
            } as React.CSSProperties)
          : undefined
      }
    >
      {count > 999 ? '999+' : count}
    </span>
  )
}

export default UnreadBadge
