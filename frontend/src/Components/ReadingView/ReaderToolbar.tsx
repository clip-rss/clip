import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { useArticleStore, useLayoutStore } from '../../Stores'
import {
  modKey,
  openURL,
  fullTextButtonMode,
  FULL_TEXT_TITLE_KEY,
} from '../../Utils'
import { usePlatform } from '../../Hooks'
import type { Item } from '../../Types'
import {
  ReadIcon,
  UnreadIcon,
  StarIcon,
  NoteIcon,
  ExternalLinkIcon,
  EnterFullScreenIcon,
  FullTextIcon,
} from './Icons'
import ReaderSettingsMenu from './ReaderSettingsMenu'
import styles from './ReadingView.module.scss'

interface ReaderToolbarProps {
  item: Item
}

function ReaderToolbar(props: ReaderToolbarProps): JSX.Element {
  const { t } = useTranslation()
  const { item } = props
  const platform = usePlatform()
  const markRead = useArticleStore((s) => s.markRead)
  const markUnread = useArticleStore((s) => s.markUnread)
  const toggleStar = useArticleStore((s) => s.toggleStar)
  const notePanelOpen = useLayoutStore((s) => s.notePanelOpen)
  const toggleNotePanel = useLayoutStore((s) => s.toggleNotePanel)
  const focusMode = useLayoutStore((s) => s.focusMode)
  const toggleFocus = useLayoutStore((s) => s.toggleFocus)
  const fetchFullContent = useArticleStore((s) => s.fetchFullContent)
  const fullTextLoadingId = useArticleStore((s) => s.fullTextLoadingId)
  const showSummary = useArticleStore((s) => s.showSummary)
  const toggleBodyMode = useArticleStore((s) => s.toggleBodyMode)
  const hasNote = item.note.trim() !== ''

  // 全文按钮：未提取时是抓取按钮，提取完成后变成摘要/全文开关（形态判定见 Utils/ArticleBody）。
  const fullTextMode = fullTextButtonMode(
    item,
    fullTextLoadingId === item.id,
    showSummary,
  )
  const canToggleBody = fullTextMode === 'full' || fullTextMode === 'summary'
  const fullTextTitle = t(FULL_TEXT_TITLE_KEY[fullTextMode])

  const focusShortcut = platform === 'mac' ? '⇧F' : '+Shift+F'
  const focusTitle = `${t('toolbar.focusMode')} (${modKey(platform)}${focusShortcut})`

  return (
    <div className={styles.toolbar}>
      <div className={styles.toolbarTitle} title={item.title}>
        {item.title}
      </div>
      <div className={styles.toolbarActions}>
        <button
          type="button"
          className={styles.toolbarBtn}
          onClick={() =>
            item.isRead ? markUnread(item.id) : markRead(item.id)
          }
          title={
            item.isRead
              ? t('reader.toolbar.markUnread')
              : t('reader.toolbar.markRead')
          }
          aria-label={
            item.isRead
              ? t('reader.toolbar.markUnread')
              : t('reader.toolbar.markRead')
          }
        >
          {item.isRead ? <ReadIcon size={18} /> : <UnreadIcon size={18} />}
        </button>
        <button
          type="button"
          className={clsx(styles.toolbarBtn, item.isStarred && styles.starred)}
          onClick={() => toggleStar(item.id)}
          title={
            item.isStarred
              ? t('reader.toolbar.unstar')
              : t('reader.toolbar.star')
          }
          aria-label={
            item.isStarred
              ? t('reader.toolbar.unstar')
              : t('reader.toolbar.star')
          }
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
          title={
            notePanelOpen
              ? t('reader.toolbar.closeNote')
              : hasNote
                ? t('reader.toolbar.viewNote')
                : t('reader.toolbar.note')
          }
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
        <button
          type="button"
          className={clsx(
            styles.toolbarBtn,
            // 开关形态下高亮表示「当前正在看全文」；没有摘要可切时保持原来的完成态。
            canToggleBody && fullTextMode === 'full' && styles.fullTextActive,
            fullTextMode === 'done' && styles.fullTextDone,
          )}
          onClick={() =>
            canToggleBody ? toggleBodyMode() : void fetchFullContent(item.id)
          }
          disabled={fullTextMode === 'fetching' || fullTextMode === 'done'}
          title={fullTextTitle}
          aria-label={fullTextTitle}
          aria-pressed={canToggleBody ? fullTextMode === 'full' : undefined}
        >
          <FullTextIcon size={18} />
        </button>
        <ReaderSettingsMenu />
        <button
          type="button"
          className={clsx(styles.toolbarBtn, focusMode && styles.focusActive)}
          onClick={toggleFocus}
          title={focusTitle}
          aria-label={t('toolbar.focusMode')}
          aria-pressed={focusMode}
        >
          <EnterFullScreenIcon size={18} />
        </button>
      </div>
    </div>
  )
}

export default ReaderToolbar
