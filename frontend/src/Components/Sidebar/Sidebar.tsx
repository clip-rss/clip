import { useEffect, useMemo, useRef, useState } from 'react'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import * as Tooltip from '@radix-ui/react-tooltip'
import * as Dialog from '@radix-ui/react-dialog'
import clsx from 'clsx'
import { useSidebarStore } from '../../Stores'
import { usePlatform, HOTKEY_OPML_IMPORT, HOTKEY_OPML_EXPORT } from '../../Hooks'
import {
  buildFeedTree,
  formatRelativeTime,
  latestUpdated,
  onFeedError,
  onItemsUpdated,
  OPMLService,
  shortcutHint,
  toApiError,
} from '../../Utils'
import FolderItem from './FolderItem'
import FeedItem from './FeedItem'
import UnreadBadge from './UnreadBadge'
import RenameInput from './RenameInput'
import { InboxIcon, PlusIcon, ImportIcon, ExportIcon } from './Icons'
import { rowPaddingLeft, FEED_DRAG_TYPE } from './layout'
import styles from './Sidebar.module.scss'

interface SidebarProps {
  /** 「添加订阅」入口（阶段 12 接入弹窗）；未提供时该菜单项禁用。 */
  onAddFeed?: () => void
}

function Sidebar(props: SidebarProps): JSX.Element {
  const { onAddFeed } = props
  const categories = useSidebarStore((s) => s.categories)
  const feeds = useSidebarStore((s) => s.feeds)
  const selection = useSidebarStore((s) => s.selection)
  const load = useSidebarStore((s) => s.load)
  const select = useSidebarStore((s) => s.select)
  const addCategory = useSidebarStore((s) => s.addCategory)
  const moveFeed = useSidebarStore((s) => s.moveFeed)

  const [creatingFolder, setCreatingFolder] = useState(false)
  const [uncatDragOver, setUncatDragOver] = useState(false)
  const [importMsg, setImportMsg] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const platform = usePlatform()

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

  // OPML 导入/导出快捷键经全局事件触发，复用本组件既有 UI 流程。
  useEffect(() => {
    const onImport = (): void => fileInputRef.current?.click()
    const onExport = (): void => void handleExport()
    window.addEventListener(HOTKEY_OPML_IMPORT, onImport)
    window.addEventListener(HOTKEY_OPML_EXPORT, onExport)
    return () => {
      window.removeEventListener(HOTKEY_OPML_IMPORT, onImport)
      window.removeEventListener(HOTKEY_OPML_EXPORT, onExport)
    }
  }, [])

  // 每分钟刷新一次「上次更新」相对时间文案
  const [, setTick] = useState(0)
  useEffect(() => {
    const timer = window.setInterval(() => setTick((n) => n + 1), 60_000)
    return () => window.clearInterval(timer)
  }, [])

  const tree = useMemo(() => buildFeedTree(categories, feeds), [categories, feeds])
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
    <div className={styles.sidebar}>
      <header className={styles.header}>
        <span className={styles.headerTitle}>源</span>
        <AddMenu
          onNewFolder={() => setCreatingFolder(true)}
          onAddFeed={onAddFeed}
        />
      </header>

      <div className={styles.tree} role="tree">
        <div
          className={clsx(styles.row, styles.feedRow, allSelected && styles.selected)}
          style={{ paddingLeft: rowPaddingLeft(0) }}
          onClick={() => select({ kind: 'all' })}
          role="treeitem"
          aria-selected={allSelected}
        >
          <span className={styles.chevronSlot} />
          <InboxIcon size={16} className={styles.feedIcon} />
          <span className={styles.rowName}>全部文章</span>
          <UnreadBadge count={tree.totalUnread} />
        </div>

        {creatingFolder ? (
          <div className={clsx(styles.row, styles.folderRow)} style={{ paddingLeft: rowPaddingLeft(0) }}>
            <span className={styles.chevronSlot} />
            <RenameInput
              initialValue="新建文件夹"
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
          className={clsx(styles.uncategorized, uncatDragOver && styles.dragOver)}
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
        <span className={styles.lastUpdated}>
          上次更新：{formatRelativeTime(lastUpdated)}
        </span>
        <div className={styles.footerActions}>
          <IconAction
            label={`导入 OPML ${shortcutHint(platform, ['Shift', 'I'])}`}
            onClick={() => fileInputRef.current?.click()}
          >
            <ImportIcon size={16} />
          </IconAction>
          <IconAction
            label={`导出 OPML ${shortcutHint(platform, ['Shift', 'E'])}`}
            onClick={handleExport}
          >
            <ExportIcon size={16} />
          </IconAction>
        </div>
      </footer>

      <input
        ref={fileInputRef}
        type="file"
        accept=".opml,.xml,text/xml,application/xml"
        className={styles.hiddenInput}
        onChange={handleImportFile}
      />

      <Dialog.Root open={importMsg !== null} onOpenChange={(o) => !o && setImportMsg(null)}>
        <Dialog.Portal>
          <Dialog.Overlay className={styles.dialogOverlay} />
          <Dialog.Content
            className={styles.dialogContent}
            aria-describedby={undefined}
            onInteractOutside={(e) => e.preventDefault()}
          >
            <Dialog.Title className={styles.dialogTitle}>导入 OPML</Dialog.Title>
            <p className={styles.dialogDesc}>{importMsg}</p>
            <div className={styles.dialogFooter}>
              <button
                type="button"
                className={styles.dialogConfirm}
                onClick={() => setImportMsg(null)}
              >
                知道了
              </button>
            </div>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </div>
  )

  async function handleImportFile(e: React.ChangeEvent<HTMLInputElement>): Promise<void> {
    const file = e.target.files?.[0]
    e.target.value = '' // 允许再次选择同一文件
    if (!file) return
    try {
      const content = await file.text()
      const res = await OPMLService.ImportOPML(content)
      await load()
      setImportMsg(
        `导入完成：新增 ${res.feeds} 个订阅源，跳过 ${res.skipped} 个重复，新建 ${res.categories} 个文件夹。`,
      )
    } catch (err) {
      setImportMsg(`OPML 导入失败：${toApiError(err)}`)
    }
  }

  async function handleExport(): Promise<void> {
    try {
      const content = await OPMLService.ExportOPML()
      const blob = new Blob([content], { type: 'text/xml;charset=utf-8' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'clip-feeds.opml'
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
    } catch (err) {
      console.error('OPML 导出失败：', toApiError(err))
    }
  }
}

/** 头部「＋」下拉菜单。 */
function AddMenu(props: { onNewFolder: () => void; onAddFeed?: () => void }): JSX.Element {
  const { onNewFolder, onAddFeed } = props
  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <button type="button" className={styles.headerAdd} title="新建" aria-label="新建文件夹或订阅">
          <PlusIcon size={18} />
        </button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content className={styles.menuContent} align="end" sideOffset={4}>
          <DropdownMenu.Item className={styles.menuItem} onSelect={onNewFolder}>
            新建文件夹
          </DropdownMenu.Item>
          <DropdownMenu.Item
            className={styles.menuItem}
            disabled={!onAddFeed}
            onSelect={() => onAddFeed?.()}
          >
            添加订阅
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
}): JSX.Element {
  const { label, onClick, children } = props
  return (
    <Tooltip.Provider delayDuration={300}>
      <Tooltip.Root>
        <Tooltip.Trigger asChild>
          <button type="button" className={styles.footerButton} onClick={onClick} aria-label={label}>
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
