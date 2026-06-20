import { useEffect, useRef } from 'react'
import clsx from 'clsx'
import { ThemeToggle } from '../ThemeToggle'
import { usePlatform, useSearchHotkey, type Platform } from '../../Hooks'
import { useArticleStore, useLayoutStore } from '../../Stores'
import styles from './Toolbar.module.scss'

/** 搜索输入防抖间隔（毫秒）。 */
const SEARCH_DEBOUNCE_MS = 300

interface ToolbarProps {
  /** 「添加订阅」入口；点击「＋ 订阅」按钮触发。 */
  onAddFeed?: () => void
}

function Toolbar(props: ToolbarProps): JSX.Element {
  const { onAddFeed } = props
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

  useSearchHotkey(() => inputRef.current?.focus())

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
        <div className={styles.search}>
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
          title="添加订阅"
          aria-label="添加订阅"
          onClick={onAddFeed}
        >
          ＋ 订阅
        </button>
        <button
          className={clsx(
            styles.iconButton,
            focusMode && styles.iconButtonActive,
          )}
          onClick={toggleFocus}
          disabled={!focusMode && !hasSelection}
          title="专注模式 (Ctrl+Shift+F)"
          aria-label="专注模式"
          aria-pressed={focusMode}
        >
          <LayoutIcon />
        </button>
        <ThemeToggle />
      </div>
    </div>
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

export default Toolbar
