import clsx from 'clsx'
import { useArticleStore, useLayoutStore } from '../../Stores'
import { openURL } from '../../Utils'
import type { Item } from '../../Types'
import type { Platform } from '../../Hooks'
import {
  BackIcon,
  ReadIcon,
  UnreadIcon,
  StarIcon,
  ExternalLinkIcon,
  NoteIcon,
} from '../ReadingView/Icons'
import styles from './FocusMode.module.scss'

interface FocusControlBarProps {
  item: Item | null
  visible: boolean
  platform: Platform | null
  onExit: () => void
  onBarEnter: () => void
  onBarLeave: () => void
}

/** 专注模式顶部浮动控制条：退出 / 标题 / 操作。 */
function FocusControlBar(props: FocusControlBarProps): JSX.Element {
  const { item, visible, platform, onExit, onBarEnter, onBarLeave } = props
  const markRead = useArticleStore((s) => s.markRead)
  const markUnread = useArticleStore((s) => s.markUnread)
  const toggleStar = useArticleStore((s) => s.toggleStar)
  const notePanelOpen = useLayoutStore((s) => s.notePanelOpen)
  const toggleNotePanel = useLayoutStore((s) => s.toggleNotePanel)

  return (
    <div
      className={clsx(styles.bar, visible && styles.barVisible)}
      onMouseEnter={onBarEnter}
      onMouseLeave={onBarLeave}
    >
      <div className={clsx(styles.barLeft, platform === 'mac' && styles.barLeftMac)}>
        <button type="button" className={styles.exitBtn} onClick={onExit} aria-label="退出专注">
          <BackIcon size={18} />
          <span>退出专注</span>
        </button>
      </div>

      <div className={styles.barTitle} title={item?.title}>
        {item?.title ?? ''}
      </div>

      <div className={styles.barActions}>
        {item ? (
          <>
            <button
              type="button"
              className={clsx(styles.barBtn, item.isStarred && styles.starred)}
              onClick={() => toggleStar(item.id)}
              title={item.isStarred ? '取消星标' : '星标'}
              aria-label={item.isStarred ? '取消星标' : '星标'}
            >
              <StarIcon size={18} filled={item.isStarred} />
            </button>
            <button
              type="button"
              className={styles.barBtn}
              onClick={() => (item.isRead ? markUnread(item.id) : markRead(item.id))}
              title={item.isRead ? '标记为未读' : '标记为已读'}
              aria-label={item.isRead ? '标记为未读' : '标记为已读'}
            >
              {item.isRead ? <ReadIcon size={18} /> : <UnreadIcon size={18} />}
            </button>
            <button
              type="button"
              className={styles.barBtn}
              onClick={() => openURL(item.url)}
              title="在浏览器打开"
              aria-label="在浏览器打开"
            >
              <ExternalLinkIcon size={18} />
            </button>
            <button
              type="button"
              className={clsx(
                styles.barBtn,
                notePanelOpen && styles.noteActive,
                item.note.trim() !== '' && styles.hasNote,
              )}
              onClick={toggleNotePanel}
              title={notePanelOpen ? '关闭笔记' : '笔记'}
              aria-label="笔记"
              aria-pressed={notePanelOpen}
            >
              <NoteIcon size={18} />
            </button>
          </>
        ) : null}
      </div>
    </div>
  )
}

export default FocusControlBar
