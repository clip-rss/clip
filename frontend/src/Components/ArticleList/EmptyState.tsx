import type { ArticleFilter } from '../../Types'
import styles from './ArticleList.module.scss'

interface EmptyStateProps {
  filter: ArticleFilter
}

const EMPTY_TEXT: Record<ArticleFilter, string> = {
  all: '暂无文章',
  unread: '暂无未读文章',
  read: '暂无已读文章',
  starred: '暂无星标文章',
  today: '今日暂无文章',
}

function EmptyState(props: EmptyStateProps): JSX.Element {
  return (
    <div className={styles.empty}>
      <svg
        width="96"
        height="96"
        viewBox="0 0 96 96"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        className={styles.emptyArt}
        aria-hidden="true"
      >
        <rect x="20" y="26" width="56" height="44" rx="4" />
        <path d="M20 38h56" />
        <path d="M30 52h22M30 60h14" />
        <circle cx="64" cy="58" r="10" />
        <path d="M71 65l6 6" />
      </svg>
      <p className={styles.emptyText}>{EMPTY_TEXT[props.filter]}</p>
    </div>
  )
}

export default EmptyState
