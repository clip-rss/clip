import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as ContextMenu from '@radix-ui/react-context-menu'
import clsx from 'clsx'
import { useSidebarStore } from '../../Stores'
import { badgeLoad } from '../../Utils'
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
  const { t } = useTranslation()
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
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                select({ kind: 'category', id: category.id })
              } else if (e.key === 'ArrowLeft' && expanded) {
                e.preventDefault()
                toggleExpand(category.id)
              } else if (e.key === 'ArrowRight' && !expanded) {
                e.preventDefault()
                toggleExpand(category.id)
              }
            }}
            onDragOver={handleDragOver}
            onDragLeave={() => setDragOver(false)}
            onDrop={handleDrop}
            role="treeitem"
            aria-expanded={expanded}
            aria-selected={selected}
            aria-label={category.name}
            title={category.name}
            tabIndex={0}
          >
            <button
              type="button"
              className={clsx(
                styles.chevronSlot,
                styles.chevronButton,
                expanded && styles.expanded,
              )}
              onClick={handleChevronClick}
              aria-label={
                expanded
                  ? t('sidebar.chevron.collapse')
                  : t('sidebar.chevron.expand')
              }
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
            {!editing ? (
              <UnreadBadge
                count={unreadCount}
                load={badgeLoad(node.cappedUnread, node.capacity)}
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

      {expanded ? (
        <div role="group">
          {children.map((child) => (
            <FolderItem
              key={`c-${child.category.id}`}
              node={child}
              depth={depth + 1}
            />
          ))}
          {feeds.map((feed) => (
            <FeedItem key={`f-${feed.id}`} feed={feed} depth={depth + 1} />
          ))}
        </div>
      ) : null}

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('sidebar.deleteFolder.title')}
        description={t('sidebar.deleteFolder.description', {
          name: category.name,
        })}
        confirmText={t('sidebar.deleteFolder.confirm')}
        danger
        onConfirm={() => deleteCategory(category.id)}
      />
    </>
  )
}

export default FolderItem
