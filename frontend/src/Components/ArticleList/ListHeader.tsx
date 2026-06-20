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
  /** 搜索模式：头部替换为结果数提示，隐藏筛选/排序/批量。 */
  searchActive?: boolean
  resultCount?: number
}

const FILTER_OPTIONS: { value: ArticleFilter; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'unread', label: '未读' },
  { value: 'read', label: '已读' },
  { value: 'starred', label: '星标' },
  { value: 'today', label: '今日' },
]

const FILTER_LABEL: Record<ArticleFilter, string> = {
  all: '全部',
  unread: '未读',
  read: '已读',
  starred: '星标',
  today: '今日',
}

function ListHeader(props: ListHeaderProps): JSX.Element {
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

  // 搜索模式：头部仅显示结果数，不展示筛选/排序/批量操作。
  if (searchActive) {
    return (
      <div className={styles.header}>
        <span className={styles.resultCount}>找到 {resultCount} 篇文章</span>
      </div>
    )
  }

  return (
    <div className={styles.header}>
      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild>
          <button type="button" className={styles.filterButton}>
            {FILTER_LABEL[filter]}
            <ChevronDownIcon size={14} />
          </button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content className={styles.menuContent} align="start" sideOffset={4}>
            {FILTER_OPTIONS.map((opt) => (
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
          title={sort === 'time' ? '按时间排序（点击切换为来源）' : '按来源排序（点击切换为时间）'}
          aria-label="切换排序"
        >
          <SortIcon size={16} />
          <span className={styles.sortLabel}>{sort === 'time' ? '时间' : '来源'}</span>
        </button>

        <DropdownMenu.Root>
          <DropdownMenu.Trigger asChild>
            <button type="button" className={styles.iconButton} title="批量操作" aria-label="批量操作">
              <MoreIcon size={18} />
            </button>
          </DropdownMenu.Trigger>
          <DropdownMenu.Portal>
            <DropdownMenu.Content className={styles.menuContent} align="end" sideOffset={4}>
              <DropdownMenu.Item className={clsx(styles.menuItem, styles.menuItemPlain)} onSelect={onMarkAllRead}>
                全部标记为已读
              </DropdownMenu.Item>
              <DropdownMenu.Item className={clsx(styles.menuItem, styles.menuItemPlain)} onSelect={onBatchStar}>
                批量星标
              </DropdownMenu.Item>
            </DropdownMenu.Content>
          </DropdownMenu.Portal>
        </DropdownMenu.Root>
      </div>
    </div>
  )
}

export default ListHeader
