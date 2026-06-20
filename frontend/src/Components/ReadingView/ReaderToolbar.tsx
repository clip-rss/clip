import clsx from 'clsx'
import { useArticleStore, useLayoutStore } from '../../Stores'
import { openURL } from '../../Utils'
import type { Item } from '../../Types'
import { ReadIcon, UnreadIcon, StarIcon, NoteIcon, ExternalLinkIcon } from './Icons'
import ReaderSettingsMenu from './ReaderSettingsMenu'
import styles from './ReadingView.module.scss'

interface ReaderToolbarProps {
  item: Item
}

/** 阅读视图浮动工具栏。 */
function ReaderToolbar(props: ReaderToolbarProps): JSX.Element {
  const { item } = props
  const markRead = useArticleStore((s) => s.markRead)
  const markUnread = useArticleStore((s) => s.markUnread)
  const toggleStar = useArticleStore((s) => s.toggleStar)
  const notePanelOpen = useLayoutStore((s) => s.notePanelOpen)
  const toggleNotePanel = useLayoutStore((s) => s.toggleNotePanel)
  const hasNote = item.note.trim() !== ''

  return (
    <div className={styles.toolbar}>
      <div className={styles.toolbarTitle} title={item.title}>
        {item.title}
      </div>
      <div className={styles.toolbarActions}>
        <button
          type="button"
          className={styles.toolbarBtn}
          onClick={() => (item.isRead ? markUnread(item.id) : markRead(item.id))}
          title={item.isRead ? '标记为未读' : '标记为已读'}
          aria-label={item.isRead ? '标记为未读' : '标记为已读'}
        >
          {item.isRead ? <ReadIcon size={18} /> : <UnreadIcon size={18} />}
        </button>
        <button
          type="button"
          className={clsx(styles.toolbarBtn, item.isStarred && styles.starred)}
          onClick={() => toggleStar(item.id)}
          title={item.isStarred ? '取消星标' : '星标'}
          aria-label={item.isStarred ? '取消星标' : '星标'}
        >
          <StarIcon size={18} filled={item.isStarred} />
        </button>
        <button
          type="button"
          className={clsx(
            styles.toolbarBtn,
            notePanelOpen && styles.noteActive,
            hasNote && styles.hasNote,
          )}
          onClick={toggleNotePanel}
          title={notePanelOpen ? '关闭笔记' : hasNote ? '查看笔记' : '添加笔记'}
          aria-label="笔记"
          aria-pressed={notePanelOpen}
        >
          <NoteIcon size={18} />
        </button>
        <button
          type="button"
          className={styles.toolbarBtn}
          onClick={() => openURL(item.url)}
          title="在浏览器打开"
          aria-label="在浏览器打开"
        >
          <ExternalLinkIcon size={18} />
        </button>
        <ReaderSettingsMenu />
      </div>
    </div>
  )
}

export default ReaderToolbar
