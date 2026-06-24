import { useTranslation } from 'react-i18next'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import clsx from 'clsx'
import type { ArticleFilter, ArticleSort } from '../../Types'
import { ChevronDownIcon, CheckIcon, SortIcon, MoreIcon } from './Icons'
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

  const sortLabel =
    sort === 'time' ? t('article.sort.time') : t('article.sort.source')
  const sortTitle =
    sort === 'time' ? t('article.sort.byTime') : t('article.sort.bySource')

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
          onClick={() => onSortChange(sort === 'time' ? 'source' : 'time')}
          title={sortTitle}
          aria-label="切换排序"
        >
          <SortIcon size={16} />
          <span className={styles.sortLabel}>{sortLabel}</span>
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
