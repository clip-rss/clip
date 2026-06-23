import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { formatRelativeTime, highlightText, openURL } from '../../Utils'
import type { Item } from '../../Types'
import { StarIcon, ExternalLinkIcon } from './Icons'
import styles from './ArticleList.module.scss'

interface ArticleRowProps {
  item: Item
  sourceName: string
  selected: boolean
  onSelect: (id: number) => void
  onToggleStar: (id: number) => void
  query?: string
}

function htmlToText(html: string): string {
  if (!html) return ''
  const doc = new DOMParser().parseFromString(html, 'text/html')
  return (doc.body.textContent ?? '').replace(/\s+/g, ' ').trim()
}

function ArticleRow(props: ArticleRowProps): JSX.Element {
  const { t } = useTranslation()
  const { item, sourceName, selected, onSelect, onToggleStar, query } = props
  const summary = htmlToText(item.summary)
  const q = query?.trim()
  const titleNode = q ? highlightText(item.title, q, styles.mark) : item.title
  const summaryNode = q ? highlightText(summary, q, styles.mark) : summary

  function handleStar(e: React.MouseEvent): void {
    e.stopPropagation()
    onToggleStar(item.id)
  }

  function handleOpen(e: React.MouseEvent): void {
    e.stopPropagation()
    openURL(item.url)
  }

  return (
    <div
      className={clsx(styles.row, selected && styles.rowSelected)}
      onClick={() => onSelect(item.id)}
      role="option"
      aria-selected={selected}
      title={item.title}
    >
      <span className={clsx(styles.dot, item.isRead && styles.dotRead)} aria-hidden="true" />

      <div className={styles.body}>
        <div className={clsx(styles.title, !item.isRead && styles.titleUnread)}>{titleNode}</div>
        {summary ? <div className={styles.summary}>{summaryNode}</div> : null}
        <div className={styles.meta}>
          {sourceName ? <span className={styles.source}>{sourceName}</span> : null}
          {sourceName ? <span className={styles.metaDot}>·</span> : null}
          <span>{formatRelativeTime(item.publishedAt)}</span>
        </div>
      </div>

      <div className={styles.actions}>
        <button
          type="button"
          className={clsx(styles.actionBtn, item.isStarred && styles.starred)}
          onClick={handleStar}
          title={item.isStarred ? t('reader.toolbar.unstar') : t('reader.toolbar.star')}
          aria-label={item.isStarred ? t('reader.toolbar.unstar') : t('reader.toolbar.star')}
        >
          <StarIcon size={16} filled={item.isStarred} />
        </button>
        <button
          type="button"
          className={styles.actionBtn}
          onClick={handleOpen}
          title={t('reader.toolbar.openInBrowser')}
          aria-label={t('reader.toolbar.openInBrowser')}
        >
          <ExternalLinkIcon size={16} />
        </button>
      </div>
    </div>
  )
}

export default ArticleRow
