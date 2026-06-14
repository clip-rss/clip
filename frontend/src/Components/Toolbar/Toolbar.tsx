import { ThemeToggle } from '../ThemeToggle'
import styles from './Toolbar.module.scss'

function Toolbar(): JSX.Element {
  return (
    <div className={styles.toolbar} data-wails-drag>
      <div className={styles.left}>
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
          className={styles.iconButton}
          title="切换布局"
          aria-label="切换布局"
        >
          <LayoutIcon />
        </button>
        <ThemeToggle />
      </div>
    </div>
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
