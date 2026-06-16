import { useState } from 'react'
import * as ContextMenu from '@radix-ui/react-context-menu'
import clsx from 'clsx'
import { useSidebarStore } from '../../Stores'
import type { FeedTreeNode } from '../../Types'
import UnreadBadge from './UnreadBadge'
import ConfirmDialog from './ConfirmDialog'
import RenameInput from './RenameInput'
import FeedItem from './FeedItem'
import { ChevronIcon, FolderIcon } from './Icons'
import { rowPaddingLeft, FEED_DRAG_TYPE } from './layout'
import styles from './Sidebar.module.scss'

interface FolderItemProps {
  node: FeedTreeNode
  depth: number
}

function FolderItem(props: FolderItemProps): JSX.Element {
  const { node, depth } = props
  const { category, children, feeds, unreadCount } = node

  const expanded = useSidebarStore((s) => s.expanded.has(category.id))
  const selected = useSidebarStore(
    (s) => s.selection.kind === 'category' && s.selection.id === category.id,
  )
  const toggleExpand = useSidebarStore((s) => s.toggleExpand)
  const select = useSidebarStore((s) => s.select)
  const renameCategory = useSidebarStore((s) => s.renameCategory)
  const deleteCategory = useSidebarStore((s) => s.deleteCategory)
  const moveFeed = useSidebarStore((s) => s.moveFeed)

  const [editing, setEditing] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [dragOver, setDragOver] = useState(false)

  function handleChevronClick(e: React.MouseEvent): void {
    e.stopPropagation()
    toggleExpand(category.id)
  }

  function handleDragOver(e: React.DragEvent): void {
    if (!e.dataTransfer.types.includes(FEED_DRAG_TYPE)) return
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    setDragOver(true)
  }

  function handleDrop(e: React.DragEvent): void {
    const raw = e.dataTransfer.getData(FEED_DRAG_TYPE)
    setDragOver(false)
    if (!raw) return
    e.preventDefault()
    const feedId = Number(raw)
    if (Number.isFinite(feedId)) moveFeed(feedId, category.id)
  }

  return (
    <>
      <ContextMenu.Root>
        <ContextMenu.Trigger asChild>
          <div
            className={clsx(
              styles.row,
              styles.folderRow,
              selected && styles.selected,
              dragOver && styles.dragOver,
            )}
            style={{ paddingLeft: rowPaddingLeft(depth) }}
            onClick={() => select({ kind: 'category', id: category.id })}
            onDragOver={handleDragOver}
            onDragLeave={() => setDragOver(false)}
            onDrop={handleDrop}
            role="treeitem"
            aria-expanded={expanded}
            aria-selected={selected}
            title={category.name}
          >
            <button
              type="button"
              className={clsx(styles.chevronSlot, styles.chevronButton, expanded && styles.expanded)}
              onClick={handleChevronClick}
              aria-label={expanded ? '折叠' : '展开'}
              tabIndex={-1}
            >
              <ChevronIcon />
            </button>
            <FolderIcon size={16} className={styles.folderIcon} />
            {editing ? (
              <RenameInput
                initialValue={category.name}
                onSubmit={(v) => {
                  setEditing(false)
                  if (v !== category.name) renameCategory(category.id, v)
                }}
                onCancel={() => setEditing(false)}
              />
            ) : (
              <span className={styles.rowName}>{category.name}</span>
            )}
            {!editing ? <UnreadBadge count={unreadCount} /> : null}
          </div>
        </ContextMenu.Trigger>
        <ContextMenu.Portal>
          <ContextMenu.Content className={styles.menuContent}>
            <ContextMenu.Item className={styles.menuItem} onSelect={() => setEditing(true)}>
              重命名
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

      {expanded ? (
        <div role="group">
          {children.map((child) => (
            <FolderItem key={`c-${child.category.id}`} node={child} depth={depth + 1} />
          ))}
          {feeds.map((feed) => (
            <FeedItem key={`f-${feed.id}`} feed={feed} depth={depth + 1} />
          ))}
        </div>
      ) : null}

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title="删除文件夹"
        description={`确定删除文件夹「${category.name}」吗？其子文件夹也会被删除，文件夹内的订阅源将变为未分类。`}
        confirmText="删除"
        danger
        onConfirm={() => deleteCategory(category.id)}
      />
    </>
  )
}

export default FolderItem
