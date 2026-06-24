import { useTranslation } from 'react-i18next'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useArticleStore } from '../../Stores'
import type { Item } from '../../Types'
import { CloseIcon } from './Icons'
import styles from './ReadingView.module.scss'

/** 笔记自动保存防抖间隔（毫秒）。 */
const SAVE_DEBOUNCE_MS = 500

interface NotePanelProps {
  item: Item
  onClose: () => void
}

type SaveStatus = 'idle' | 'saving' | 'saved'

/**
 * 阅读区底部笔记抽屉：纯文本（支持 Markdown 语法）编辑，防抖 500ms 自动保存。
 *
 * 草稿为本地态，切换文章或关闭/卸载时立即冲刷未保存的改动，避免丢失。
 */
function NotePanel(props: NotePanelProps): JSX.Element {
  const { t } = useTranslation()
  const { item, onClose } = props
  const saveNote = useArticleStore((s) => s.saveNote)

  const [draft, setDraft] = useState(item.note)
  const [status, setStatus] = useState<SaveStatus>('idle')

  const timerRef = useRef<number | undefined>(undefined)
  // 待保存的最新草稿（按文章 id 归属），供切换/卸载时冲刷。
  const pendingRef = useRef<{ id: number; text: string } | null>(null)
  // saveNote 引用随 store 变化，用 ref 持有以便在稳定的回调里调用。
  const saveRef = useRef(saveNote)
  saveRef.current = saveNote

  /** 立即写入待保存草稿（清除计时器）。 */
  const flush = useCallback(() => {
    window.clearTimeout(timerRef.current)
    const pending = pendingRef.current
    if (!pending) return
    pendingRef.current = null
    void saveRef
      .current(pending.id, pending.text)
      .then(() => setStatus('saved'))
  }, [])

  // 仅在切换文章（item.id 变化）时：冲刷上一篇待保存草稿，再载入当前文章笔记。
  // 不依赖 item.note——自身保存会回写 item.note，若据此重置会覆盖正在输入的内容。
  useEffect(() => {
    flush()
    setDraft(item.note)
    setStatus('idle')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [item.id, flush])

  // 卸载时冲刷未保存草稿。
  useEffect(() => () => flush(), [flush])

  function handleChange(value: string): void {
    setDraft(value)
    pendingRef.current = { id: item.id, text: value }
    setStatus('saving')
    window.clearTimeout(timerRef.current)
    timerRef.current = window.setTimeout(() => {
      const pending = pendingRef.current
      if (!pending) return
      pendingRef.current = null
      void saveRef
        .current(pending.id, pending.text)
        .then(() => setStatus('saved'))
    }, SAVE_DEBOUNCE_MS)
  }

  function handleClose(): void {
    flush()
    onClose()
  }

  return (
    <div className={styles.notePanel}>
      <div className={styles.noteHeader}>
        <span className={styles.noteTitle}>{t('note.title')}</span>
        <span className={styles.noteHint}>{t('note.placeholder')}</span>
        <span className={styles.noteStatus}>
          {status === 'saving'
            ? t('note.saving')
            : status === 'saved'
              ? t('note.saved')
              : ''}
        </span>
        <button
          type="button"
          className={styles.noteClose}
          onClick={handleClose}
          title={t('reader.toolbar.closeNote')}
          aria-label={t('reader.toolbar.closeNote')}
        >
          <CloseIcon size={16} />
        </button>
      </div>
      <textarea
        className={styles.noteEditor}
        value={draft}
        onChange={(e) => handleChange(e.target.value)}
        placeholder={t('note.placeholder')}
        spellCheck={false}
        autoFocus
      />
    </div>
  )
}

export default NotePanel
