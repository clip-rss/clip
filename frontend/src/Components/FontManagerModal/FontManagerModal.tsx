import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as Dialog from '@radix-ui/react-dialog'
import Skeleton from '../Skeleton/Skeleton'
import { FontService, onFontDownloadProgress, toApiError } from '../../Utils'
import type { FontCatalog, FontDef } from '../../Types'
import styles from './FontManagerModal.module.scss'

interface FontManagerModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

/** 单个字体族的下载状态：无记录即未开始。 */
interface DownloadState {
  status: 'downloading' | 'done' | 'failed'
  /** 0–100；清单没给文件大小时恒为 0，此时只展示不确定态文案。 */
  percent: number
  /** 是否拿得到总量，用于决定要不要显示百分比。 */
  known: boolean
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

/** 单个字体族的卡片：名称 / 字重 chips / 格式 / 体积 / 许可证 + 下载。 */
function FontCard(props: {
  font: FontDef
  state?: DownloadState
  onDownload: (id: string) => void
}): JSX.Element {
  const { font, state, onDownload } = props
  const { t } = useTranslation()
  const totalBytes = font.files.reduce((sum, f) => sum + f.size, 0)

  let label = t('settings.fonts.download')
  if (state?.status === 'downloading') {
    label = state.known
      ? t('settings.fonts.downloading', { percent: state.percent })
      : t('settings.fonts.downloadingIndeterminate')
  } else if (state?.status === 'done') {
    label = t('settings.fonts.downloaded')
  } else if (state?.status === 'failed') {
    label = t('settings.fonts.downloadFailed')
  }

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
      <div className={styles.cardFoot}>
        <span className={styles.cardLicense}>{font.license}</span>
        <button
          type="button"
          className={styles.downloadBtn}
          data-status={state?.status ?? 'idle'}
          disabled={state?.status === 'downloading'}
          title={label}
          aria-label={label}
          onClick={() => onDownload(font.id)}
        >
          {label}
        </button>
      </div>
    </div>
  )
}

export function FontManagerModal(props: FontManagerModalProps): JSX.Element {
  const { t } = useTranslation()
  const { open, onOpenChange } = props
  const [catalog, setCatalog] = useState<FontCatalog | null>(null)
  const [loading, setLoading] = useState(false)
  const [failed, setFailed] = useState(false)
  const [downloads, setDownloads] = useState<Record<string, DownloadState>>({})

  // 每次打开都重新拉取：字体目录可能随版本更新，不缓存到关闭。
  const load = useCallback((): void => {
    setLoading(true)
    setFailed(false)
    setCatalog(null)
    // 下载态一并清空：后端对已校验通过的文件会跳过，重开面板再点也不会重下。
    setDownloads({})
    FontService.FetchFontCatalog()
      .then((cat) => setCatalog(cat))
      .catch(() => setFailed(true))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    if (open) load()
  }, [open, load])

  // 进度事件按 id 分发；只保留在下载中的那一条，避免完成后的迟到事件把状态改回去。
  useEffect(() => {
    const unsub = onFontDownloadProgress((p) => {
      setDownloads((prev) => {
        if (prev[p.id]?.status !== 'downloading') return prev
        return {
          ...prev,
          [p.id]: {
            status: 'downloading',
            percent: p.percent,
            known: p.total > 0,
          },
        }
      })
    })
    return unsub
  }, [])

  async function download(id: string): Promise<void> {
    setDownloads((prev) => ({
      ...prev,
      [id]: { status: 'downloading', percent: 0, known: false },
    }))
    try {
      await FontService.DownloadFont(id)
      setDownloads((prev) => ({
        ...prev,
        [id]: { status: 'done', percent: 100, known: true },
      }))
    } catch (e) {
      setDownloads((prev) => ({
        ...prev,
        [id]: { status: 'failed', percent: 0, known: false },
      }))
      console.error(toApiError(e))
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
                <FontCard
                  key={font.id}
                  font={font}
                  state={downloads[font.id]}
                  onDownload={download}
                />
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
