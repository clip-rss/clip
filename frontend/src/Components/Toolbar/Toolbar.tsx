import { useEffect, useRef, useState } from 'react'
import clsx from 'clsx'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { ThemeToggle } from '../ThemeToggle'
import { usePlatform, type Platform } from '../../Hooks'
import { modKey } from '../../Utils'
import { useArticleStore, useLayoutStore, useSettingsStore } from '../../Stores'
import styles from './Toolbar.module.scss'

/** 搜索输入防抖间隔（毫秒）。 */
const SEARCH_DEBOUNCE_MS = 300

interface ToolbarProps {
  /** 「添加订阅」入口；点击「＋ 订阅」按钮触发。 */
  onAddFeed?: () => void
  /** 打开设置面板；点击齿轮按钮触发。 */
  onOpenSettings?: () => void
}

function Toolbar(props: ToolbarProps): JSX.Element {
  const { onAddFeed, onOpenSettings } = props
  const platform = usePlatform()
  const focusMode = useLayoutStore((s) => s.focusMode)
  const toggleFocus = useLayoutStore((s) => s.toggleFocus)
  const hasSelection = useArticleStore((s) => s.selectedItemId !== null)

  const searchQuery = useArticleStore((s) => s.searchQuery)
  const setSearchQuery = useArticleStore((s) => s.setSearchQuery)
  const runSearch = useArticleStore((s) => s.runSearch)
  const clearSearch = useArticleStore((s) => s.clearSearch)

  const inputRef = useRef<HTMLInputElement>(null)
  const debounceRef = useRef<number | undefined>(undefined)
  const [searchFocused, setSearchFocused] = useState(false)
  // 有焦点或已有查询内容时保持展开，避免活动搜索被收起截断。
  const searchExpanded = searchFocused || searchQuery !== ''

  const settings = useSettingsStore((s) => s.settings)
  const setNotificationMode = useSettingsStore((s) => s.setNotificationMode)
  const mode: NotifMode = (settings?.notificationMode as NotifMode) || 'each'

  // 应用启动时拉取设置。
  useEffect(() => { useSettingsStore.getState().load() }, [])

  // 卸载时清掉未触发的防抖计时器。
  useEffect(() => () => window.clearTimeout(debounceRef.current), [])

  function handleSearchChange(value: string): void {
    setSearchQuery(value)
    window.clearTimeout(debounceRef.current)
    debounceRef.current = window.setTimeout(() => runSearch(), SEARCH_DEBOUNCE_MS)
  }

  function handleClear(): void {
    window.clearTimeout(debounceRef.current)
    clearSearch()
    inputRef.current?.focus()
  }

  function handleSearchKeyDown(e: React.KeyboardEvent): void {
    if (e.key === 'Escape') {
      e.preventDefault()
      window.clearTimeout(debounceRef.current)
      clearSearch()
      inputRef.current?.blur()
    }
  }

  return (
    <div
      className={styles.toolbar}
      style={{ '--wails-draggable': 'drag' } as any}
    >
      <div className={styles.left}>
        <WindowControls platform={platform} />
        <span className={styles.title}>Clip</span>
        <div className={clsx(styles.search, searchExpanded && styles.searchExpanded)}>
          <SearchIcon />
          <input
            ref={inputRef}
            id="toolbar-search"
            type="text"
            placeholder="搜索文章、笔记…"
            className={styles.searchInput}
            value={searchQuery}
            onChange={(e) => handleSearchChange(e.target.value)}
            onKeyDown={handleSearchKeyDown}
            onFocus={() => setSearchFocused(true)}
            onBlur={() => setSearchFocused(false)}
            data-wails-no-drag
          />
          {searchQuery ? (
            <button
              type="button"
              className={styles.searchClear}
              onClick={handleClear}
              title="清除搜索"
              aria-label="清除搜索"
            >
              <ClearIcon />
            </button>
          ) : null}
        </div>
      </div>
      <div className={styles.right}>
        <button
          className={styles.addButton}
          title={`添加订阅 (${modKey(platform)}N)`}
          aria-label="添加订阅"
          onClick={onAddFeed}
        >
          ＋ 订阅
        </button>
        <NotifDropdown
          mode={mode}
          onSelect={setNotificationMode}
        />
        <button
          className={clsx(
            styles.iconButton,
            focusMode && styles.iconButtonActive,
          )}
          onClick={toggleFocus}
          disabled={!focusMode && !hasSelection}
          title={`专注模式 (${modKey(platform)}${platform === 'mac' ? '⇧F' : '+Shift+F'})`}
          aria-label="专注模式"
          aria-pressed={focusMode}
        >
          <LayoutIcon />
        </button>
        <ThemeToggle />
        <button
          className={styles.iconButton}
          onClick={onOpenSettings}
          title={`设置 (${modKey(platform)}${platform === 'mac' ? '，' : ','})`}
          aria-label="设置"
        >
          <SettingsIcon />
        </button>
      </div>
    </div>
  )
}

type NotifMode = 'each' | 'summary' | 'off'

const NOTIF_LABEL: Record<NotifMode, string> = { each: '每篇新文章', summary: '仅摘要', off: '关闭' }

function NotifDropdown(props: { mode: NotifMode; onSelect: (m: NotifMode) => void }): JSX.Element {
  const { mode, onSelect } = props
  const modes: NotifMode[] = ['each', 'summary', 'off']
  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <button
          type="button"
          className={styles.iconButton}
          title={`通知：${NOTIF_LABEL[mode]}`}
          aria-label={`通知：${NOTIF_LABEL[mode]}`}
        >
          <BellIcon active={mode !== 'off'} />
        </button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content className={styles.notifMenu} align="end" sideOffset={6}>
          {modes.map((m) => (
            <DropdownMenu.CheckboxItem
              key={m}
              className={styles.notifItem}
              checked={mode === m}
              onSelect={() => onSelect(m)}
            >
              <DropdownMenu.ItemIndicator className={styles.notifCheck}>
                <CheckMark />
              </DropdownMenu.ItemIndicator>
              {NOTIF_LABEL[m]}
            </DropdownMenu.CheckboxItem>
          ))}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  )
}

function BellIcon(props: { active: boolean }): JSX.Element {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
      <path d="M13.73 21a2 2 0 0 1-3.46 0" />
      {!props.active ? <line x1="2" y1="2" x2="22" y2="22" /> : null}
    </svg>
  )
}

function CheckMark(): JSX.Element {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M20 6 9 17l-5-5" />
    </svg>
  )
}

function WindowControls(props: { platform: Platform | null }): JSX.Element | null {
  const { platform } = props

  // 平台未解析前不渲染，避免在 Windows 下闪现多余占位
  if (platform === null) {
    return null
  }

  // macOS：原生红绿灯由系统在标题栏内绘制，此处仅预留空间避免内容被遮挡
  if (platform === 'mac') {
    return <div className={styles.macSpacer} aria-hidden="true" />
  }

  // Windows：以应用图标占位
  return (
    <div className={styles.winIcon} aria-hidden="true">
      <AppIcon />
    </div>
  )
}

function AppIcon(): JSX.Element {
  return (
    <svg
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M4 11a9 9 0 0 1 9 9" />
      <path d="M4 4a16 16 0 0 1 16 16" />
      <circle cx="5" cy="19" r="1" fill="currentColor" />
    </svg>
  )
}

function SearchIcon(): JSX.Element {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="11" cy="11" r="8" />
      <path d="m21 21-4.35-4.35" />
    </svg>
  )
}

function ClearIcon(): JSX.Element {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      aria-hidden="true"
    >
      <path d="M18 6 6 18M6 6l12 12" />
    </svg>
  )
}

function LayoutIcon(): JSX.Element {
  return (
    <svg
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect x="3" y="3" width="7" height="18" />
      <rect x="14" y="3" width="7" height="7" />
      <rect x="14" y="14" width="7" height="7" />
    </svg>
  )
}

function SettingsIcon(): JSX.Element {
  return (
    <svg
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
  )
}

export default Toolbar
