import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as Dialog from '@radix-ui/react-dialog'
import Skeleton from '../Skeleton/Skeleton'
import { FontService } from '../../Utils'
import type { FontCatalog, FontDef } from '../../Types'
import styles from './FontManagerModal.module.scss'

interface FontManagerModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

/** 字节 → 「x.x MB」。 */
function formatMB(bytes: number): string {
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
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
      aria-hidden="true"
    >
      <path d="M4 4l8 8M12 4l-8 8" />
    </svg>
  )
}

/** 单个字体族的卡片：名称 / 字重 chips / 格式 / 体积 / 许可证。 */
function FontCard({ font }: { font: FontDef }): JSX.Element {
  const totalBytes = font.files.reduce((sum, f) => sum + f.size, 0)
  return (
    <div className={styles.card}>
      <div className={styles.cardHead}>
        <span className={styles.cardName}>{font.name}</span>
        <span className={styles.cardFamily}>{font.family}</span>
      </div>
      <div className={styles.cardMeta}>
        {font.files.map((f) => (
          <span key={f.file} className={styles.chip}>
            {f.style || String(f.weight)}
          </span>
        ))}
        <span className={styles.chip}>{font.files[0]?.format ?? ''}</span>
        <span className={styles.chip}>{formatMB(totalBytes)}</span>
      </div>
      <div className={styles.cardLicense}>{font.license}</div>
    </div>
  )
}

export function FontManagerModal(props: FontManagerModalProps): JSX.Element {
  const { t } = useTranslation()
  const { open, onOpenChange } = props
  const [catalog, setCatalog] = useState<FontCatalog | null>(null)
  const [loading, setLoading] = useState(false)
  const [failed, setFailed] = useState(false)

  // 每次打开都重新拉取：字体目录可能随版本更新，不缓存到关闭。
  function load(): void {
    setLoading(true)
    setFailed(false)
    setCatalog(null)
    FontService.FetchFontCatalog()
      .then((cat) => setCatalog(cat))
      .catch(() => setFailed(true))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    if (open) load()
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
              {t('settings.fonts.title')}
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
            <div className={styles.list}>
              <Skeleton className={styles.skeletonCard} height={64} />
              <Skeleton className={styles.skeletonCard} height={64} />
            </div>
          ) : failed ? (
            <div className={styles.failed}>
              <p className={styles.failedText}>{t('settings.fonts.failed')}</p>
              <button type="button" className={styles.retryBtn} onClick={load}>
                {t('settings.fonts.retry')}
              </button>
            </div>
          ) : catalog && catalog.fonts.length > 0 ? (
            <div className={styles.list}>
              {catalog.fonts.map((font) => (
                <FontCard key={font.id} font={font} />
              ))}
            </div>
          ) : (
            <p className={styles.empty}>{t('settings.fonts.empty')}</p>
          )}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
