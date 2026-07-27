import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { ThemeToggle } from '../ThemeToggle'
import { usePlatform, type Platform } from '../../Hooks'
import { modKey } from '../../Utils'
import { useArticleStore, useLayoutStore, useUpdateStore } from '../../Stores'
import styles from './Toolbar.module.scss'

const SEARCH_DEBOUNCE_MS = 300

interface ToolbarProps {
  onAddFeed?: () => void
  onOpenSettings?: () => void
}

function Toolbar(props: ToolbarProps): JSX.Element {
  const { t } = useTranslation()
  const { onAddFeed, onOpenSettings } = props
  const platform = usePlatform()
  const focusMode = useLayoutStore((s) => s.focusMode)
  const toggleFocus = useLayoutStore((s) => s.toggleFocus)
  const hasSelection = useArticleStore((s) => s.selectedItemId !== null)
  const updateAvailable = useUpdateStore((s) => s.updateAvailable)

  const searchQuery = useArticleStore((s) => s.searchQuery)
  const setSearchQuery = useArticleStore((s) => s.setSearchQuery)
  const runSearch = useArticleStore((s) => s.runSearch)
  const clearSearch = useArticleStore((s) => s.clearSearch)

  const inputRef = useRef<HTMLInputElement>(null)
  const debounceRef = useRef<number | undefined>(undefined)
  const [searchFocused, setSearchFocused] = useState(false)
  const searchExpanded = searchFocused || searchQuery !== ''

  useEffect(() => () => window.clearTimeout(debounceRef.current), [])

  function handleSearchChange(value: string): void {
    setSearchQuery(value)
    window.clearTimeout(debounceRef.current)
    debounceRef.current = window.setTimeout(
      () => runSearch(),
      SEARCH_DEBOUNCE_MS,
    )
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

  const addTitle = `${t('toolbar.addFeed')} (${modKey(platform)}N)`
  const focusShortcut = platform === 'mac' ? '⇧F' : '+Shift+F'
  const focusTitle = `${t('toolbar.focusMode')} (${modKey(platform)}${focusShortcut})`
  const settingsShortcut = platform === 'mac' ? '，' : ','
  const settingsTitle = `${t('toolbar.settings')} (${modKey(platform)}${settingsShortcut})`

  return (
    <div
      className={styles.toolbar}
      style={
        { '--wails-draggable': platform === 'mac' ? 'drag' : 'none' } as any
      }
    >
      <div className={styles.left}>
        <WindowControls platform={platform} />
        <div
          className={clsx(
            styles.search,
            searchExpanded && styles.searchExpanded,
          )}
        >
          <SearchIcon />
          <input
            ref={inputRef}
            id="toolbar-search"
            type="text"
            placeholder={t('toolbar.search.placeholder')}
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
              title={t('toolbar.clearSearch')}
              aria-label={t('toolbar.clearSearch')}
            >
              <ClearIcon />
            </button>
          ) : null}
        </div>
      </div>
      <div className={styles.right}>
        <button
          className={styles.addButton}
          title={addTitle}
          aria-label={t('toolbar.addFeed')}
          onClick={onAddFeed}
        >
          {t('toolbar.addFeed')}
        </button>
        <button
          className={clsx(
            styles.iconButton,
            focusMode && styles.iconButtonActive,
          )}
          onClick={toggleFocus}
          disabled={!focusMode && !hasSelection}
          title={focusTitle}
          aria-label={t('toolbar.focusMode')}
          aria-pressed={focusMode}
        >
          <LayoutIcon />
        </button>
        <ThemeToggle />
        <button
          className={styles.iconButton}
          onClick={onOpenSettings}
          title={settingsTitle}
          aria-label={t('toolbar.settings')}
          style={updateAvailable ? { position: 'relative' } : undefined}
        >
          <SettingsIcon className={updateAvailable ? styles.settingsSpinning : undefined} />
          {updateAvailable && <span className={styles.updateBadge} />}
        </button>
      </div>
    </div>
  )
}

function WindowControls(props: {
  platform: Platform | null
}): JSX.Element | null {
  const { platform } = props

  // 平台未解析前不渲染，避免在 Windows 下闪现多余占位
  if (platform === null) {
    return null
  }

  // macOS：原生红绿灯由系统在标题栏内绘制，此处仅预留空间避免内容被遮挡
  if (platform === 'mac') {
    return <div className={styles.macSpacer} aria-hidden="true" />
  }

  // Windows
  return <div className={styles.winSpacer} aria-hidden="true" />
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

function SettingsIcon({ className }: { className?: string }): JSX.Element {
  return (
    <svg
      className={className}
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
