import styles from './Sidebar.module.scss'

interface UnreadBadgeProps {
  count: number
}

/** 未读计数胶囊：count 为 0 时不渲染。 */
function UnreadBadge(props: UnreadBadgeProps): JSX.Element | null {
  const { count } = props
  if (count <= 0) return null
  return <span className={styles.badge}>{count > 999 ? '999+' : count}</span>
}

export default UnreadBadge
