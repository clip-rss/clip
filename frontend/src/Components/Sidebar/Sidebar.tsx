import { useTranslation } from 'react-i18next'
import { useEffect, useMemo, useState } from 'react'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import * as Tooltip from '@radix-ui/react-tooltip'
import clsx from 'clsx'
import { useSidebarStore } from '../../Stores'
import {
  buildFeedTree,
  formatRelativeTime,
  latestUpdated,
  onFeedError,
  onItemsUpdated,
} from '../../Utils'
import FolderItem from './FolderItem'
import FeedItem from './FeedItem'
import UnreadBadge from './UnreadBadge'
import RenameInput from './RenameInput'
import { InboxIcon, PlusIcon, RefreshIcon } from './Icons'
import { rowPaddingLeft, FEED_DRAG_TYPE } from './layout'
import styles from './Sidebar.module.scss'

interface SidebarProps {
  /** 「添加订阅」入口（阶段 12 接入弹窗）；未提供时该菜单项禁用。 */
  onAddFeed?: () => void
}

function Sidebar(props: SidebarProps): JSX.Element {
  const { t } = useTranslation()
  const { onAddFeed } = props
  const categories = useSidebarStore((s) => s.categories)
  const feeds = useSidebarStore((s) => s.feeds)
  const selection = useSidebarStore((s) => s.selection)
  const load = useSidebarStore((s) => s.load)
  const select = useSidebarStore((s) => s.select)
  const addCategory = useSidebarStore((s) => s.addCategory)
  const moveFeed = useSidebarStore((s) => s.moveFeed)
  const refreshSelected = useSidebarStore((s) => s.refreshSelected)

  const [creatingFolder, setCreatingFolder] = useState(false)
  const [uncatDragOver, setUncatDragOver] = useState(false)
  const [refreshing, setRefreshing] = useState(false)

  // 初次加载 + 订阅后端事件，新文章/抓取错误时刷新未读与结构
  useEffect(() => {
    load()
    const offItems = onItemsUpdated(() => load())
    const offError = onFeedError(() => load())
    return () => {
      offItems()
      offError()
    }
  }, [load])

  // 每分钟刷新一次「上次更新」相对时间文案
  const [, setTick] = useState(0)
  useEffect(() => {
    const timer = window.setInterval(() => setTick((n) => n + 1), 60_000)
    return () => window.clearInterval(timer)
  }, [])

  const tree = useMemo(
    () => buildFeedTree(categories, feeds),
    [categories, feeds],
  )
  const lastUpdated = useMemo(() => latestUpdated(feeds), [feeds])
  const allSelected = selection.kind === 'all'

  function handleUncatDragOver(e: React.DragEvent): void {
    if (!e.dataTransfer.types.includes(FEED_DRAG_TYPE)) return
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    setUncatDragOver(true)
  }

  function handleUncatDrop(e: React.DragEvent): void {
    const raw = e.dataTransfer.getData(FEED_DRAG_TYPE)
    setUncatDragOver(false)
    if (!raw) return
    e.preventDefault()
    const feedId = Number(raw)
    if (Number.isFinite(feedId)) moveFeed(feedId, 0) // 0 = 移出到未分类
  }

  return (
    <div
      className={styles.sidebar}
      role="navigation"
      aria-label={t('sidebar.title')}
    >
      <header className={styles.header}>
        <span className={styles.headerTitle}>{t('sidebar.title')}</span>
        <AddMenu
          onNewFolder={() => setCreatingFolder(true)}
          onAddFeed={onAddFeed}
        />
      </header>

      <div className={styles.tree} role="tree" aria-label={t('sidebar.title')}>
        <div
          className={clsx(
            styles.row,
            styles.feedRow,
            allSelected && styles.selected,
          )}
          style={{ paddingLeft: rowPaddingLeft(0) }}
          onClick={() => select({ kind: 'all' })}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault()
              select({ kind: 'all' })
            }
          }}
          role="treeitem"
          aria-selected={allSelected}
          aria-label={t('sidebar.allArticles')}
          tabIndex={0}
        >
          <span className={styles.chevronSlot} />
          <InboxIcon size={16} className={styles.feedIcon} />
          <span className={styles.rowName}>{t('sidebar.allArticles')}</span>
          <UnreadBadge count={tree.totalUnread} />
        </div>

        {creatingFolder ? (
          <div
            className={clsx(styles.row, styles.folderRow)}
            style={{ paddingLeft: rowPaddingLeft(0) }}
          >
            <span className={styles.chevronSlot} />
            <RenameInput
              initialValue={t('sidebar.newFolder')}
              onSubmit={(v) => {
                setCreatingFolder(false)
                addCategory(v)
              }}
              onCancel={() => setCreatingFolder(false)}
            />
          </div>
        ) : null}

        {tree.roots.map((node) => (
          <FolderItem key={`c-${node.category.id}`} node={node} depth={0} />
        ))}

        <div
          className={clsx(
            styles.uncategorized,
            uncatDragOver && styles.dragOver,
          )}
          onDragOver={handleUncatDragOver}
          onDragLeave={() => setUncatDragOver(false)}
          onDrop={handleUncatDrop}
        >
          {tree.uncategorized.map((feed) => (
            <FeedItem key={`f-${feed.id}`} feed={feed} depth={0} />
          ))}
        </div>
      </div>

      <footer className={styles.footer}>
        <div className={styles.footerStatus}>
          <span className={styles.lastUpdated}>
            {t('sidebar.lastUpdated')}
            {formatRelativeTime(lastUpdated)}
          </span>
          <IconAction
            label={t('sidebar.refresh')}
            onClick={handleRefresh}
            disabled={refreshing}
          >
            <RefreshIcon
              size={15}
              className={refreshing ? styles.spinning : undefined}
            />
          </IconAction>
        </div>
      </footer>
    </div>
  )

  async function handleRefresh(): Promise<void> {
    if (refreshing) return
    setRefreshing(true)
    try {
      await refreshSelected()
    } finally {
      setRefreshing(false)
    }
  }
}

/** 头部「＋」下拉菜单。 */
function AddMenu(props: {
  onNewFolder: () => void
  onAddFeed?: () => void
}): JSX.Element {
  const { t } = useTranslation()
  const { onNewFolder, onAddFeed } = props
  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <button
          type="button"
          className={styles.headerAdd}
          title={t('sidebar.addMenu')}
          aria-label={t('sidebar.addMenu')}
        >
          <PlusIcon size={18} />
        </button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          className={styles.menuContent}
          align="end"
          sideOffset={4}
        >
          <DropdownMenu.Item className={styles.menuItem} onSelect={onNewFolder}>
            {t('sidebar.contextMenu.newFolder')}
          </DropdownMenu.Item>
          <DropdownMenu.Item
            className={styles.menuItem}
            disabled={!onAddFeed}
            onSelect={() => onAddFeed?.()}
          >
            {t('sidebar.contextMenu.addFeed')}
          </DropdownMenu.Item>
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  )
}

/** 底部带 Tooltip 的图标按钮。 */
function IconAction(props: {
  label: string
  onClick: () => void
  children: React.ReactNode
  disabled?: boolean
}): JSX.Element {
  const { label, onClick, children, disabled } = props
  return (
    <Tooltip.Provider delayDuration={300}>
      <Tooltip.Root>
        <Tooltip.Trigger asChild>
          <button
            type="button"
            className={styles.footerButton}
            onClick={onClick}
            disabled={disabled}
            aria-label={label}
          >
            {children}
          </button>
        </Tooltip.Trigger>
        <Tooltip.Portal>
          <Tooltip.Content className={styles.tooltip} sideOffset={6}>
            {label}
            <Tooltip.Arrow className={styles.tooltipArrow} />
          </Tooltip.Content>
        </Tooltip.Portal>
      </Tooltip.Root>
    </Tooltip.Provider>
  )
}

export default Sidebar
