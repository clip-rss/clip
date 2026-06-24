import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as Dialog from '@radix-ui/react-dialog'
import { useSidebarStore } from '../../Stores'
import { FeedService, flattenCategories, toApiError } from '../../Utils'
import type { FeedWithUnread } from '../../Types'
import styles from './EditFeedModal.module.scss'

interface EditFeedModalProps {
  feed: FeedWithUnread
  open: boolean
  onOpenChange: (open: boolean) => void
}

function EditFeedModal(props: EditFeedModalProps): JSX.Element {
  const { t } = useTranslation()
  const { feed, open, onOpenChange } = props
  const categories = useSidebarStore((s) => s.categories)
  const reload = useSidebarStore((s) => s.load)

  const [title, setTitle] = useState(feed.title)
  const [categoryId, setCategoryId] = useState(feed.categoryId ?? 0)
  const [updateInterval, setUpdateInterval] = useState(feed.updateInterval)
  const [maxItems, setMaxItems] = useState(feed.maxItems)
  const [saving, setSaving] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')

  // 每次打开时用最新 feed 初始化表单。
  useEffect(() => {
    if (open) {
      setTitle(feed.title)
      setCategoryId(feed.categoryId ?? 0)
      setUpdateInterval(feed.updateInterval)
      setMaxItems(feed.maxItems)
      setSaving(false)
      setErrorMsg('')
    }
  }, [open, feed])

  const options = flattenCategories(categories)

  async function save(): Promise<void> {
    const finalTitle = title.trim()
    if (!finalTitle || saving) return
    setSaving(true)
    setErrorMsg('')
    try {
      await FeedService.UpdateFeed({
        ...feed,
        title: finalTitle,
        categoryId: categoryId === 0 ? null : categoryId,
        updateInterval: Math.max(1, updateInterval),
        maxItems: Math.max(0, maxItems),
      })
      await reload()
      onOpenChange(false)
    } catch (err) {
      setErrorMsg(toApiError(err))
      setSaving(false)
    }
  }

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className={styles.overlay} />
        <Dialog.Content
          className={styles.content}
          aria-describedby={undefined}
          onInteractOutside={(e) => e.preventDefault()}
        >
          <header className={styles.header}>
            <Dialog.Title className={styles.title}>
              {t('feed.edit.title')}
            </Dialog.Title>
            <Dialog.Close asChild>
              <button
                type="button"
                className={styles.closeBtn}
                aria-label={t('confirm.cancel')}
              >
                <CloseIcon />
              </button>
            </Dialog.Close>
          </header>

          <div className={styles.body}>
            <div className={styles.fieldGroup}>
              <label className={styles.label} htmlFor="edit-feed-title">
                {t('feed.edit.label')}
              </label>
              <input
                id="edit-feed-title"
                type="text"
                className={styles.textInput}
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                autoFocus
              />
            </div>

            <div className={styles.fieldGroup}>
              <label className={styles.label} htmlFor="edit-feed-folder">
                {t('feed.edit.folder')}
              </label>
              <select
                id="edit-feed-folder"
                className={styles.select}
                value={categoryId}
                onChange={(e) => setCategoryId(Number(e.target.value))}
              >
                <option value={0}>{t('sidebar.allArticles')}</option>
                {options.map((c) => (
                  <option key={c.id} value={c.id}>
                    {'　'.repeat(c.depth)}
                    {c.name}
                  </option>
                ))}
              </select>
            </div>

            <div className={styles.fieldRow}>
              <div className={styles.fieldGroup}>
                <label className={styles.label} htmlFor="edit-feed-interval">
                  {t('feed.edit.interval')}
                </label>
                <input
                  id="edit-feed-interval"
                  type="number"
                  min={1}
                  className={styles.textInput}
                  value={updateInterval}
                  onChange={(e) => setUpdateInterval(Number(e.target.value))}
                />
              </div>
              <div className={styles.fieldGroup}>
                <label className={styles.label} htmlFor="edit-feed-max">
                  {t('feed.edit.maxItems')}
                </label>
                <input
                  id="edit-feed-max"
                  type="number"
                  min={0}
                  className={styles.textInput}
                  value={maxItems}
                  onChange={(e) => setMaxItems(Number(e.target.value))}
                />
              </div>
            </div>

            {errorMsg ? <p className={styles.inlineError}>{errorMsg}</p> : null}
          </div>

          <footer className={styles.footer}>
            <Dialog.Close asChild>
              <button type="button" className={styles.cancelBtn}>
                {t('feed.edit.cancel')}
              </button>
            </Dialog.Close>
            <button
              type="button"
              className={styles.saveBtn}
              onClick={save}
              disabled={!title.trim() || saving}
            >
              {saving ? `${t('note.saving')}` : t('feed.edit.save')}
            </button>
          </footer>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

function CloseIcon(): JSX.Element {
  return (
    <svg
      width="16"
      height="16"
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

export default EditFeedModal
