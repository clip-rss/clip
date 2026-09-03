import { useTranslation } from 'react-i18next'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import clsx from 'clsx'
import type { ArticleFilter, ArticleSort } from '../../Types'
import {
  ChevronDownIcon,
  CheckIcon,
  ArrowDownIcon,
  ArrowUpIcon,
  MoreIcon,
} from './Icons'
import styles from './ArticleList.module.scss'

interface ListHeaderProps {
  filter: ArticleFilter
  sort: ArticleSort
  onFilterChange: (filter: ArticleFilter) => void
  onSortChange: (sort: ArticleSort) => void
  onMarkAllRead: () => void
  onBatchStar: () => void
  searchActive?: boolean
  resultCount?: number
}

function ListHeader(props: ListHeaderProps): JSX.Element {
  const { t } = useTranslation()
  const {
    filter,
    sort,
    onFilterChange,
    onSortChange,
    onMarkAllRead,
    onBatchStar,
    searchActive = false,
    resultCount = 0,
  } = props

  const filterOptions: { value: ArticleFilter; label: string }[] = [
    { value: 'all', label: t('article.filter.all') },
    { value: 'unread', label: t('article.filter.unread') },
    { value: 'read', label: t('article.filter.read') },
    { value: 'starred', label: t('article.filter.starred') },
    { value: 'today', label: t('article.filter.today') },
  ]

  const filterLabel: Record<ArticleFilter, string> = {
    all: t('article.filter.all'),
    unread: t('article.filter.unread'),
    read: t('article.filter.read'),
    starred: t('article.filter.starred'),
    today: t('article.filter.today'),
  }

  // 按时间排序：按钮仅为方向箭头图标，完整含义放在 title / aria-label，点击在新→旧 / 旧→新间切换。
  const isDesc = sort === 'timeDesc'
  const sortDirLabel = isDesc
    ? t('article.sort.newest')
    : t('article.sort.oldest')
  const sortTitle = `${t('article.sort.byTime')} · ${sortDirLabel}`

  if (searchActive) {
    return (
      <div className={styles.header}>
        <span className={styles.resultCount}>
          {t('article.searchResult', { count: resultCount })}
        </span>
      </div>
    )
  }

  return (
    <div className={styles.header}>
      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild>
          <button type="button" className={styles.filterButton}>
            {filterLabel[filter]}
            <ChevronDownIcon size={14} />
          </button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            className={styles.menuContent}
            align="start"
            sideOffset={4}
          >
            {filterOptions.map((opt) => (
              <DropdownMenu.Item
                key={opt.value}
                className={styles.menuItem}
                onSelect={() => onFilterChange(opt.value)}
              >
                <span className={styles.menuCheck}>
                  {filter === opt.value ? <CheckIcon size={14} /> : null}
                </span>
                {opt.label}
              </DropdownMenu.Item>
            ))}
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>

      <div className={styles.headerRight}>
        <button
          type="button"
          className={styles.iconButton}
          onClick={() => onSortChange(isDesc ? 'timeAsc' : 'timeDesc')}
          title={sortTitle}
          aria-label={sortTitle}
        >
          {isDesc ? <ArrowDownIcon size={16} /> : <ArrowUpIcon size={16} />}
        </button>

        <DropdownMenu.Root>
          <DropdownMenu.Trigger asChild>
            <button
              type="button"
              className={styles.iconButton}
              title="批量操作"
              aria-label="批量操作"
            >
              <MoreIcon size={18} />
            </button>
          </DropdownMenu.Trigger>
          <DropdownMenu.Portal>
            <DropdownMenu.Content
              className={styles.menuContent}
              align="end"
              sideOffset={4}
            >
              <DropdownMenu.Item
                className={clsx(styles.menuItem, styles.menuItemPlain)}
                onSelect={onMarkAllRead}
              >
                {t('article.actions.markAllRead')}
              </DropdownMenu.Item>
              <DropdownMenu.Item
                className={clsx(styles.menuItem, styles.menuItemPlain)}
                onSelect={onBatchStar}
              >
                {t('article.actions.batchStar')}
              </DropdownMenu.Item>
            </DropdownMenu.Content>
          </DropdownMenu.Portal>
        </DropdownMenu.Root>
      </div>
    </div>
  )
}

export default ListHeader
