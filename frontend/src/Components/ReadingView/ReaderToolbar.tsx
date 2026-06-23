import { useTranslation } from 'react-i18next'
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

function ReaderToolbar(props: ReaderToolbarProps): JSX.Element {
  const { t } = useTranslation()
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
          title={item.isRead ? t('reader.toolbar.markUnread') : t('reader.toolbar.markRead')}
          aria-label={item.isRead ? t('reader.toolbar.markUnread') : t('reader.toolbar.markRead')}
        >
          {item.isRead ? <ReadIcon size={18} /> : <UnreadIcon size={18} />}
        </button>
        <button
          type="button"
          className={clsx(styles.toolbarBtn, item.isStarred && styles.starred)}
          onClick={() => toggleStar(item.id)}
          title={item.isStarred ? t('reader.toolbar.unstar') : t('reader.toolbar.star')}
          aria-label={item.isStarred ? t('reader.toolbar.unstar') : t('reader.toolbar.star')}
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
          title={notePanelOpen ? t('reader.toolbar.closeNote') : hasNote ? t('reader.toolbar.viewNote') : t('reader.toolbar.note')}
          aria-label={t('note.title')}
          aria-pressed={notePanelOpen}
        >
          <NoteIcon size={18} />
        </button>
        <button
          type="button"
          className={styles.toolbarBtn}
          onClick={() => openURL(item.url)}
          title={t('reader.toolbar.openInBrowser')}
          aria-label={t('reader.toolbar.openInBrowser')}
        >
          <ExternalLinkIcon size={18} />
        </button>
        <ReaderSettingsMenu />
      </div>
    </div>
  )
}

export default ReaderToolbar
