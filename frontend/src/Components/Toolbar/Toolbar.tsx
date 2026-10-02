import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { usePlatform, type Platform } from '../../Hooks'
import { modKey } from '../../Utils'
import {
  useArticleStore,
  useUpdateStore,
  useSearchHistoryStore,
} from '../../Stores'
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
  const updateAvailable = useUpdateStore((s) => s.updateAvailable)

  const searchQuery = useArticleStore((s) => s.searchQuery)
  const setSearchQuery = useArticleStore((s) => s.setSearchQuery)
  const runSearch = useArticleStore((s) => s.runSearch)
  const clearSearch = useArticleStore((s) => s.clearSearch)

  const inputRef = useRef<HTMLInputElement>(null)
  const debounceRef = useRef<number | undefined>(undefined)
  const [searchFocused, setSearchFocused] = useState(false)
  const searchExpanded = searchFocused || searchQuery !== ''

  const history = useSearchHistoryStore((s) => s.history)
  const clearHistory = useSearchHistoryStore((s) => s.clear)
  const historyOpen = searchFocused && searchQuery === '' && history.length > 0

  useEffect(() => () => window.clearTimeout(debounceRef.current), [])

  function handleSearchChange(value: string): void {
    setSearchQuery(value)
    window.clearTimeout(debounceRef.current)
    debounceRef.current = window.setTimeout(
      () => runSearch(),
      SEARCH_DEBOUNCE_MS,
    )
  }

  function handlePickHistory(query: string): void {
    window.clearTimeout(debounceRef.current)
    setSearchQuery(query)
    void runSearch()
    inputRef.current?.blur()
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
          {historyOpen ? (
            <div
              className={styles.historyPanel}
              onMouseDown={(e) => e.preventDefault()}
            >
              <div className={styles.historyHeader}>
                <span>{t('toolbar.searchHistory')}</span>
                <button
                  type="button"
                  className={styles.historyClear}
                  onClick={clearHistory}
                  title={t('toolbar.clearSearchHistory')}
                  aria-label={t('toolbar.clearSearchHistory')}
                >
                  {t('toolbar.clearSearchHistory')}
                </button>
              </div>
              {history.map((q) => (
                <button
                  key={q}
                  type="button"
                  className={styles.historyItem}
                  onClick={() => handlePickHistory(q)}
                  title={q}
                >
                  {q}
                </button>
              ))}
            </div>
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
          className={styles.iconButton}
          onClick={onOpenSettings}
          title={settingsTitle}
          aria-label={t('toolbar.settings')}
          style={updateAvailable ? { position: 'relative' } : undefined}
        >
          <SettingsIcon
            className={updateAvailable ? styles.settingsSpinning : undefined}
          />
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

function SettingsIcon({ className }: { className?: string }): JSX.Element {
  return (
    <svg
      className={className}
      width="20"
      height="20"
      viewBox="0 0 15 15"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M7.92834 0.650246C8.3253 0.650336 8.66968 0.92521 8.75745 1.31236L8.99475 2.36216C9.37407 2.47236 9.73655 2.62282 10.0758 2.80943L10.9869 2.23521C11.3226 2.02392 11.76 2.07297 12.0406 2.35337L12.6471 2.95982C12.9277 3.24057 12.977 3.67868 12.7653 4.0145L12.191 4.92466C12.3775 5.2637 12.5281 5.62568 12.6383 6.00474L13.6881 6.24302C14.0752 6.33086 14.3502 6.67514 14.3502 7.07212V7.92857C14.3502 8.32553 14.0752 8.66981 13.6881 8.75767L12.6383 8.99595C12.5281 9.37514 12.3767 9.73689 12.1901 10.076L12.7643 10.9872C12.9759 11.323 12.9268 11.7602 12.6461 12.0409L12.0406 12.6473C11.7599 12.9281 11.3218 12.9772 10.986 12.7655L10.0748 12.1913C9.73582 12.3777 9.37376 12.5284 8.99475 12.6385L8.75745 13.6883C8.66963 14.0754 8.32526 14.3504 7.92834 14.3504H7.07092C6.67397 14.3504 6.32965 14.0754 6.24182 13.6883L6.00354 12.6376C5.625 12.5274 5.26403 12.3765 4.92542 12.1903L4.01428 12.7655C3.67846 12.9772 3.24035 12.9279 2.95959 12.6473L2.35315 12.0409C2.07276 11.7603 2.02372 11.3229 2.23499 10.9872L2.81018 10.0751C2.62379 9.73614 2.47209 9.37484 2.36194 8.99595L1.31213 8.75767C0.924966 8.66986 0.650024 8.32557 0.650024 7.92857V7.07212C0.650024 6.6751 0.924951 6.33082 1.31213 6.24302L2.36194 6.00474C2.47203 5.62592 2.6229 5.26449 2.8092 4.92564L2.23499 4.0145C2.02326 3.67863 2.0724 3.24056 2.35315 2.95982L2.95862 2.35435C3.23936 2.0736 3.67744 2.02446 4.01331 2.23618L4.92444 2.8104C5.26333 2.62395 5.62465 2.47237 6.00354 2.36216L6.24182 1.31236C6.3296 0.925173 6.67392 0.650284 7.07092 0.650246H7.92834ZM6.71545 3.06821C6.05669 3.18406 5.44722 3.44325 4.92053 3.81236L3.56018 2.95493L2.95374 3.56138L3.81213 4.92173C3.44325 5.44841 3.18369 6.05799 3.06799 6.71665L1.49963 7.07212V7.92857L3.06799 8.28403C3.18375 8.94266 3.44322 9.55232 3.81213 10.079L2.95471 11.4403L3.56018 12.0458L4.92151 11.1883C5.44803 11.5571 6.05701 11.8167 6.71545 11.9325L7.07092 13.4999H7.92834L8.28381 11.9325C8.94261 11.8168 9.55196 11.5573 10.0787 11.1883L11.4391 12.0467L12.0455 11.4403L11.1881 10.0799C11.5573 9.55311 11.8164 8.94299 11.9323 8.28403L13.4996 7.92857V7.07212L11.9323 6.71665C11.8165 6.05777 11.5572 5.44756 11.1881 4.92075L12.0455 3.5604L11.4401 2.95493L10.0797 3.81236C9.55281 3.44318 8.94285 3.18394 8.28381 3.06821L7.92834 1.49986H7.07092L6.71545 3.06821ZM7.49963 5.07505C8.8388 5.07505 9.92529 6.16072 9.92542 7.49986C9.92542 8.8391 8.83888 9.92466 7.49963 9.92466C6.16061 9.9244 5.07483 8.83894 5.07483 7.49986C5.07495 6.16088 6.16069 5.07531 7.49963 5.07505ZM7.49963 5.97544C6.65774 5.9757 5.97534 6.65793 5.97522 7.49986C5.97522 8.34188 6.65767 9.02499 7.49963 9.02525C8.34182 9.02525 9.02502 8.34204 9.02502 7.49986C9.0249 6.65777 8.34174 5.97544 7.49963 5.97544Z"
        fill="currentColor"
      />
    </svg>
  )
}

export default Toolbar
