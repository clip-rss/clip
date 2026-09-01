import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as ContextMenu from '@radix-ui/react-context-menu'
import clsx from 'clsx'
import { useSidebarStore } from '../../Stores'
import {
  badgeLoad,
  erroredFeedIds,
  isFeedErrored,
  showToast,
} from '../../Utils'
import type { FeedWithUnread } from '../../Types'
import { EditFeedModal } from '../EditFeedModal'
import UnreadBadge from './UnreadBadge'
import ConfirmDialog from './ConfirmDialog'
import RenameInput from './RenameInput'
import { GlobeIcon, PauseIcon, SpinnerIcon } from './Icons'
import { rowPaddingLeft, FEED_DRAG_TYPE } from './layout'
import styles from './Sidebar.module.scss'

interface FeedItemProps {
  feed: FeedWithUnread
  /** 树深度，决定左缩进。 */
  depth: number
}

function FeedItem(props: FeedItemProps): JSX.Element {
  const { t } = useTranslation()
  const { feed, depth } = props
  const selected = useSidebarStore(
    (s) => s.selection.kind === 'feed' && s.selection.id === feed.id,
  )
  const select = useSidebarStore((s) => s.select)
  const renameFeed = useSidebarStore((s) => s.renameFeed)
  const deleteFeed = useSidebarStore((s) => s.deleteFeed)
  const deleteFeeds = useSidebarStore((s) => s.deleteFeeds)
  const pauseFeed = useSidebarStore((s) => s.pauseFeed)
  const resumeFeed = useSidebarStore((s) => s.resumeFeed)
  const refreshFeed = useSidebarStore((s) => s.refreshFeed)
  const batchMode = useSidebarStore((s) => s.batchMode)
  const enterBatchMode = useSidebarStore((s) => s.enterBatchMode)
  const multiSelected = useSidebarStore((s) => s.multiSelectIds.has(feed.id))
  const toggleMultiSelect = useSidebarStore((s) => s.toggleMultiSelect)

  const [editing, setEditing] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [erroredConfirmOpen, setErroredConfirmOpen] = useState(false)
  /** 右键时快照的异常源 id 列表；确认删除按此执行，避免与弹窗期间的状态漂移不一致。 */
  const [erroredIds, setErroredIds] = useState<number[]>([])

  const feeds = useSidebarStore((s) => s.feeds)
  const errorIds = useMemo(() => erroredFeedIds(feeds), [feeds])

  const paused = feed.status === 'paused'
  const hasError = isFeedErrored(feed)
  const refreshing = useSidebarStore((s) => s.refreshingFeeds.has(feed.id))

  function handleDragStart(e: React.DragEvent): void {
    e.dataTransfer.setData(FEED_DRAG_TYPE, String(feed.id))
    e.dataTransfer.effectAllowed = 'move'
  }

  /** 批量删除异常订阅源，并按结果给出 toast 反馈。 */
  async function deleteErroredFeeds(ids: number[]): Promise<void> {
    const ok = await deleteFeeds(ids)
    if (ok) {
      showToast(
        t('sidebar.deleteErrored.done', { count: ids.length }),
        'success',
      )
    } else {
      const err = useSidebarStore.getState().error
      if (err) showToast(err, 'error')
    }
  }

  return (
    <>
      <ContextMenu.Root>
        <ContextMenu.Trigger asChild>
          <div
            className={clsx(
              styles.row,
              styles.feedRow,
              selected && styles.selected,
              multiSelected && styles.multiSelected,
            )}
            style={{ paddingLeft: rowPaddingLeft(depth) }}
            draggable={!editing}
            onDragStart={handleDragStart}
            onClick={() => select({ kind: 'feed', id: feed.id })}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                select({ kind: 'feed', id: feed.id })
              }
            }}
            role="treeitem"
            aria-selected={selected}
            aria-label={feed.title}
            title={feed.title}
            tabIndex={0}
          >
            <span className={styles.chevronSlot}>
              {batchMode ? (
                <input
                  type="checkbox"
                  className={styles.multiSelectCheckbox}
                  checked={multiSelected}
                  onChange={() => toggleMultiSelect(feed.id)}
                  onClick={(e) => e.stopPropagation()}
                  onKeyDown={(e) => e.stopPropagation()}
                  aria-label={t('sidebar.toggleMultiSelect')}
                />
              ) : null}
            </span>
            {refreshing ? (
              <SpinnerIcon size={16} className={styles.spinning} />
            ) : (
              <FeedFavicon icon={feed.icon} title={feed.title} />
            )}
            {editing ? (
              <RenameInput
                initialValue={feed.title}
                onSubmit={(v) => {
                  setEditing(false)
                  if (v !== feed.title) renameFeed(feed.id, v)
                }}
                onCancel={() => setEditing(false)}
              />
            ) : (
              <span className={styles.rowName}>{feed.title}</span>
            )}
            {paused ? (
              <PauseIcon size={12} className={styles.pausedMark} />
            ) : null}
            {hasError ? (
              <span
                className={styles.errorMark}
                title={feed.lastError || t('sidebar.feedError')}
                aria-label={t('sidebar.feedError')}
              >
                ⚠
              </span>
            ) : null}
            {!editing ? (
              <UnreadBadge
                count={feed.unreadCount}
                load={badgeLoad(feed.unreadCount, feed.maxItems)}
              />
            ) : null}
          </div>
        </ContextMenu.Trigger>
        <ContextMenu.Portal>
          <ContextMenu.Content className={styles.menuContent}>
            <ContextMenu.Item
              className={styles.menuItem}
              onSelect={() => setEditing(true)}
            >
              {t('sidebar.contextMenu.rename')}
            </ContextMenu.Item>
            <ContextMenu.Item
              className={styles.menuItem}
              onSelect={() => setEditOpen(true)}
            >
              {t('sidebar.contextMenu.edit')}
            </ContextMenu.Item>
            <ContextMenu.Item
              className={styles.menuItem}
              onSelect={() => {
                void refreshFeed(feed.id)
              }}
            >
              {t('sidebar.contextMenu.refresh')}
            </ContextMenu.Item>
            <ContextMenu.Item
              className={styles.menuItem}
              onSelect={() =>
                paused ? resumeFeed(feed.id) : pauseFeed(feed.id)
              }
            >
              {paused
                ? t('sidebar.contextMenu.resume')
                : t('sidebar.contextMenu.pause')}
            </ContextMenu.Item>
            <ContextMenu.Separator className={styles.menuSeparator} />
            <ContextMenu.Item
              className={clsx(styles.menuItem, styles.menuItemDanger)}
              onSelect={() => setConfirmOpen(true)}
            >
              {t('sidebar.contextMenu.delete')}
            </ContextMenu.Item>
            <ContextMenu.Item
              className={clsx(styles.menuItem, styles.menuItemDanger)}
              onSelect={() => enterBatchMode()}
            >
              {t('sidebar.contextMenu.deleteBatch')}
            </ContextMenu.Item>
            <ContextMenu.Item
              className={clsx(styles.menuItem, styles.menuItemDanger)}
              disabled={errorIds.length === 0}
              onSelect={() => {
                setErroredIds(errorIds)
                setErroredConfirmOpen(true)
              }}
            >
              {t('sidebar.contextMenu.deleteErrored', {
                count: errorIds.length,
              })}
            </ContextMenu.Item>
          </ContextMenu.Content>
        </ContextMenu.Portal>
      </ContextMenu.Root>

      <EditFeedModal feed={feed} open={editOpen} onOpenChange={setEditOpen} />

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('sidebar.deleteFeed.title')}
        description={t('sidebar.deleteFeed.description', { title: feed.title })}
        confirmText={t('sidebar.deleteFeed.confirm')}
        danger
        onConfirm={() => deleteFeed(feed.id)}
      />

      <ConfirmDialog
        open={erroredConfirmOpen}
        onOpenChange={setErroredConfirmOpen}
        title={t('sidebar.deleteFeed.title')}
        description={t('sidebar.deleteFeed.erroredDescription', {
          count: erroredIds.length,
        })}
        confirmText={t('sidebar.deleteFeed.confirm')}
        danger
        onConfirm={() => void deleteErroredFeeds(erroredIds)}
      />
    </>
  )
}

/** favicon 加载失败时回退到内置 globe 图标。 */
function FeedFavicon(props: { icon: string; title: string }): JSX.Element {
  const { icon, title } = props
  const [failed, setFailed] = useState(false)
  if (!icon || failed) {
    return <GlobeIcon size={16} className={styles.feedIcon} />
  }
  return (
    <img
      src={icon}
      alt=""
      width={16}
      height={16}
      className={styles.favicon}
      onError={() => setFailed(true)}
      aria-label={title}
    />
  )
}

export default FeedItem
