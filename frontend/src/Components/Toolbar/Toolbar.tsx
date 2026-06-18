import clsx from 'clsx'
import { ThemeToggle } from '../ThemeToggle'
import { usePlatform, type Platform } from '../../Hooks'
import { useArticleStore, useLayoutStore } from '../../Stores'
import styles from './Toolbar.module.scss'

function Toolbar(): JSX.Element {
  const platform = usePlatform()
  const focusMode = useLayoutStore((s) => s.focusMode)
  const toggleFocus = useLayoutStore((s) => s.toggleFocus)
  const hasSelection = useArticleStore((s) => s.selectedItemId !== null)

  return (
    <div className={styles.toolbar} data-wails-drag>
      <div className={styles.left}>
        <WindowControls platform={platform} />
        <span className={styles.title}>Clip</span>
        <div className={styles.search} data-wails-no-drag>
          <SearchIcon />
          <input
            type="text"
            placeholder="搜索文章..."
            className={styles.searchInput}
          />
        </div>
      </div>
      <div className={styles.right} data-wails-no-drag>
        <button
          className={styles.addButton}
          title="添加订阅"
          aria-label="添加订阅"
        >
          ＋ 订阅
        </button>
        <button
          className={clsx(styles.iconButton, focusMode && styles.iconButtonActive)}
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
