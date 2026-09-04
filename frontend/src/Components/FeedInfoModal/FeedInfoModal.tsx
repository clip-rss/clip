import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as Dialog from '@radix-ui/react-dialog'
import { useSidebarStore } from '../../Stores'
import { formatRelativeTime } from '../../Utils'
import type { FeedWithUnread } from '../../Types'
import styles from './FeedInfoModal.module.scss'

interface FeedInfoModalProps {
  feed: FeedWithUnread
  open: boolean
  onOpenChange: (open: boolean) => void
}

function FeedInfoModal(props: FeedInfoModalProps): JSX.Element {
  const { t } = useTranslation()
  const { feed, open, onOpenChange } = props
  const categories = useSidebarStore((s) => s.categories)

  const categoryName = useMemo(() => {
    if (!feed.categoryId) return t('sidebar.allArticles')
    const cat = categories.find((c) => c.id === feed.categoryId)
    return cat ? cat.name : t('sidebar.allArticles')
  }, [feed.categoryId, categories, t])

  const statusLabel = useMemo(() => {
    switch (feed.status) {
      case 'active':
        return t('feed.info.status.active')
      case 'paused':
        return t('feed.info.status.paused')
      case 'error':
        return t('feed.info.status.error')
      default:
        return feed.status
    }
  }, [feed.status, t])

  const hasError =
    feed.status === 'error' || (feed.errorCount > 0 && feed.lastError)

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className={styles.overlay} />
        <Dialog.Content className={styles.content} aria-describedby={undefined}>
          <header className={styles.header}>
            <Dialog.Title className={styles.title}>
              {t('feed.info.title')}
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
            {/* favicon */}
            <div className={styles.faviconRow}>
              <FeedFavicon icon={feed.icon} />
              <span className={styles.faviconTitle}>{feed.title}</span>
            </div>

            {/* 基本信息 */}
            <div className={styles.section}>
              <div className={styles.sectionTitle}>
                {t('feed.info.section.basic')}
              </div>
              {feed.description ? (
                <InfoRow
                  label={t('feed.info.description')}
                  value={feed.description}
                />
              ) : null}
              <InfoRow label={t('feed.info.feedUrl')} value={feed.url} isLink />
              {feed.link ? (
                <InfoRow
                  label={t('feed.info.siteLink')}
                  value={feed.link}
                  isLink
                />
              ) : null}
              <InfoRow label={t('feed.info.category')} value={categoryName} />
              <div className={styles.row}>
                <span className={styles.label}>
                  {t('feed.info.statusLabel')}
                </span>
                <span className={styles.value}>
                  <span
                    className={`${styles.statusBadge} ${styles[feed.status] || styles.active}`}
                  >
                    <span
                      className={`${styles.statusBadgeDot} ${styles[feed.status] || styles.active}`}
                    />
                    {statusLabel}
                  </span>
                </span>
              </div>
            </div>

            {/* 统计信息 */}
            <div className={styles.section}>
              <div className={styles.sectionTitle}>
                {t('feed.info.section.stats')}
              </div>
              <InfoRow
                label={t('feed.info.updateInterval')}
                value={t('feed.info.intervalUnit', {
                  count: feed.updateInterval,
                })}
              />
              <InfoRow
                label={t('feed.info.maxItems')}
                value={t('feed.info.itemUnit', { count: feed.maxItems })}
              />
              <InfoRow
                label={t('feed.info.unreadCount')}
                value={t('feed.info.itemUnit', { count: feed.unreadCount })}
              />
            </div>

            {/* 时间信息 */}
            <div className={styles.section}>
              <div className={styles.sectionTitle}>
                {t('feed.info.section.time')}
              </div>
              <InfoRow
                label={t('feed.info.createdAt')}
                value={formatRelativeTime(
                  feed.createdAt as unknown as string | null,
                )}
              />
              <InfoRow
                label={t('feed.info.lastUpdated')}
                value={
                  feed.lastUpdated
                    ? formatRelativeTime(feed.lastUpdated as unknown as string)
                    : t('time.never')
                }
              />
            </div>

            {/* 错误信息 */}
            {hasError ? (
              <div className={styles.section}>
                <div className={styles.sectionTitle}>
                  {t('feed.info.section.error')}
                </div>
                <InfoRow
                  label={t('feed.info.errorCount')}
                  value={String(feed.errorCount)}
                />
                {feed.lastError ? (
                  <div className={styles.row}>
                    <span className={styles.label}>
                      {t('feed.info.lastError')}
                    </span>
                    <span className={styles.errorText}>{feed.lastError}</span>
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>

          <footer className={styles.footer}>
            <Dialog.Close asChild>
              <button type="button" className={styles.closeFooterBtn}>
                {t('confirm.confirm')}
              </button>
            </Dialog.Close>
          </footer>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

/** 标签-值行。 */
function InfoRow(props: {
  label: string
  value: string
  isLink?: boolean
}): JSX.Element {
  const { label, value, isLink } = props
  return (
    <div className={styles.row}>
      <span className={styles.label}>{label}</span>
      {isLink ? (
        <a
          className={`${styles.value} ${styles.link}`}
          href={value}
          target="_blank"
          rel="noopener noreferrer"
          onClick={(e) => e.stopPropagation()}
        >
          {value}
        </a>
      ) : (
        <span className={styles.value}>{value}</span>
      )}
    </div>
  )
}

/** favicon 加载失败时回退到 globe 图标。 */
function FeedFavicon(props: { icon: string }): JSX.Element {
  const { icon } = props
  const [failed, setFailed] = useState(false)
  if (!icon || failed) {
    return (
      <svg
        className={styles.faviconImg}
        width="32"
        height="32"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <circle cx="12" cy="12" r="10" />
        <path d="M2 12h20" />
        <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
      </svg>
    )
  }
  return (
    <img
      src={icon}
      alt=""
      width={32}
      height={32}
      draggable={false}
      className={styles.faviconImg}
      onError={() => setFailed(true)}
    />
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

export default FeedInfoModal
