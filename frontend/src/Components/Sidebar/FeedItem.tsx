import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as ContextMenu from '@radix-ui/react-context-menu'
import clsx from 'clsx'
import { useSidebarStore } from '../../Stores'
import type { FeedWithUnread } from '../../Types'
import { EditFeedModal } from '../EditFeedModal'
import UnreadBadge from './UnreadBadge'
import ConfirmDialog from './ConfirmDialog'
import RenameInput from './RenameInput'
import { GlobeIcon, PauseIcon } from './Icons'
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
  const pauseFeed = useSidebarStore((s) => s.pauseFeed)
  const resumeFeed = useSidebarStore((s) => s.resumeFeed)
  const refreshFeed = useSidebarStore((s) => s.refreshFeed)

  const [editing, setEditing] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)

  const paused = feed.status === 'paused'

  function handleDragStart(e: React.DragEvent): void {
    e.dataTransfer.setData(FEED_DRAG_TYPE, String(feed.id))
    e.dataTransfer.effectAllowed = 'move'
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
            )}
            style={{ paddingLeft: rowPaddingLeft(depth) }}
            draggable={!editing}
            onDragStart={handleDragStart}
            onClick={() => select({ kind: 'feed', id: feed.id })}
            role="treeitem"
            aria-selected={selected}
            title={feed.title}
          >
            <span className={styles.chevronSlot} />
            <FeedFavicon icon={feed.icon} title={feed.title} />
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
            {!editing ? <UnreadBadge count={feed.unreadCount} /> : null}
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
