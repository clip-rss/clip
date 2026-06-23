import { useTranslation } from 'react-i18next'
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

function FocusControlBar(props: FocusControlBarProps): JSX.Element {
  const { t } = useTranslation()
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
        <button type="button" className={styles.exitBtn} onClick={onExit} aria-label={t('focus.exit')}>
          <BackIcon size={18} />
          <span>{t('focus.exit')}</span>
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
              title={item.isStarred ? t('reader.toolbar.unstar') : t('reader.toolbar.star')}
              aria-label={item.isStarred ? t('reader.toolbar.unstar') : t('reader.toolbar.star')}
            >
              <StarIcon size={18} filled={item.isStarred} />
            </button>
            <button
              type="button"
              className={styles.barBtn}
              onClick={() => (item.isRead ? markUnread(item.id) : markRead(item.id))}
              title={item.isRead ? t('reader.toolbar.markUnread') : t('reader.toolbar.markRead')}
              aria-label={item.isRead ? t('reader.toolbar.markUnread') : t('reader.toolbar.markRead')}
            >
              {item.isRead ? <ReadIcon size={18} /> : <UnreadIcon size={18} />}
            </button>
            <button
              type="button"
              className={styles.barBtn}
              onClick={() => openURL(item.url)}
              title={t('reader.toolbar.openInBrowser')}
              aria-label={t('reader.toolbar.openInBrowser')}
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
              title={notePanelOpen ? t('reader.toolbar.closeNote') : t('note.title')}
              aria-label={t('note.title')}
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
