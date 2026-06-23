import { useTranslation } from 'react-i18next'
import type { ArticleFilter } from '../../Types'
import styles from './ArticleList.module.scss'

interface EmptyStateProps {
  filter: ArticleFilter
  searchQuery?: string
}

function EmptyState(props: EmptyStateProps): JSX.Element {
  const { t } = useTranslation()
  const { filter, searchQuery } = props

  const emptyText: Record<ArticleFilter, string> = {
    all: t('article.empty.noArticles'),
    unread: t('article.empty.noUnread'),
    read: t('article.empty.noRead'),
    starred: t('article.empty.noStarred'),
    today: t('article.empty.noToday'),
  }

  const text =
    searchQuery !== undefined
      ? t('article.empty.noMatch', { query: searchQuery })
      : emptyText[filter]

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
      <p className={styles.emptyText}>{text}</p>
    </div>
  )
}

export default EmptyState
