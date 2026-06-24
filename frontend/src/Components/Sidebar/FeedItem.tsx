import { useState } from 'react'
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
  const { feed, depth } = props
  const selected = useSidebarStore(
    (s) => s.selection.kind === 'feed' && s.selection.id === feed.id,
  )
  const select = useSidebarStore((s) => s.select)
  const renameFeed = useSidebarStore((s) => s.renameFeed)
  const deleteFeed = useSidebarStore((s) => s.deleteFeed)
  const pauseFeed = useSidebarStore((s) => s.pauseFeed)
  const resumeFeed = useSidebarStore((s) => s.resumeFeed)

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
              重命名
            </ContextMenu.Item>
            <ContextMenu.Item
              className={styles.menuItem}
              onSelect={() => setEditOpen(true)}
            >
              编辑…
            </ContextMenu.Item>
            <ContextMenu.Item
              className={styles.menuItem}
              onSelect={() =>
                paused ? resumeFeed(feed.id) : pauseFeed(feed.id)
              }
            >
              {paused ? '恢复更新' : '暂停更新'}
            </ContextMenu.Item>
            <ContextMenu.Separator className={styles.menuSeparator} />
            <ContextMenu.Item
              className={clsx(styles.menuItem, styles.menuItemDanger)}
              onSelect={() => setConfirmOpen(true)}
            >
              删除
            </ContextMenu.Item>
          </ContextMenu.Content>
        </ContextMenu.Portal>
      </ContextMenu.Root>

      <EditFeedModal feed={feed} open={editOpen} onOpenChange={setEditOpen} />

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title="删除订阅源"
        description={`确定删除「${feed.title}」吗？该源下的全部文章也会被删除，此操作不可撤销。`}
        confirmText="删除"
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
