import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as Dialog from '@radix-ui/react-dialog'
import {
  markdownToHtml,
  sanitizeHtml,
  SystemService,
  toApiError,
} from '../../Utils'
import Skeleton from '../Skeleton/Skeleton'
import styles from './ChangelogModal.module.scss'

interface ChangelogModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

function CloseIcon(): JSX.Element {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
    >
      <path d="M4 4l8 8M12 4l-8 8" />
    </svg>
  )
}

export function ChangelogModal(props: ChangelogModalProps): JSX.Element {
  const { t } = useTranslation()
  const { open, onOpenChange } = props
  const [html, setHtml] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return
    // 重置状态并立即进入 loading（避免打开瞬间闪空白内容）
    setHtml('')
    setError('')
    setLoading(true)

    SystemService.FetchChangelog()
      .then((md: string) => {
        setHtml(sanitizeHtml(markdownToHtml(md)))
      })
      .catch((err: unknown) => {
        setError(toApiError(err))
      })
      .finally(() => setLoading(false))
  }, [open])

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
              {t('settings.about.links.changelog')}
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

          {loading ? (
            <div className={styles.skeleton}>
              <Skeleton
                width="60%"
                height={20}
                className={styles.skeletonTitle}
              />
              <Skeleton
                width="35%"
                height={14}
                className={styles.skeletonDate}
              />
              <Skeleton width="100%" height={14} />
              <Skeleton width="85%" height={14} />
              <Skeleton width="70%" height={14} />
              <Skeleton width="55%" height={14} />
            </div>
          ) : error ? (
            <div className={styles.errorMsg}>
              {t('settings.about.changelogError', 'Failed to load changelog')}:{' '}
              {error}
            </div>
          ) : (
            <div
              className={styles.body}
              dangerouslySetInnerHTML={{ __html: html }}
            />
          )}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
