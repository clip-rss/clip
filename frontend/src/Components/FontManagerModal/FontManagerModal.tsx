import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as Dialog from '@radix-ui/react-dialog'
import Skeleton from '../Skeleton/Skeleton'
import {
  FontService,
  onFontDownloadProgress,
  showToast,
  toApiError,
} from '../../Utils'
import { useReaderStore, useSettingsStore } from '../../Stores'
import type { FontCatalog, FontDef, InstalledFont } from '../../Types'
import styles from './FontManagerModal.module.scss'

interface FontManagerModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

/** 单个字体族的下载状态：无记录且不在已装清单即未开始。 */
interface DownloadState {
  /** 「已下载」不在这里，由 installed 集合表达——它来自后端的跨会话清单。 */
  status: 'downloading' | 'failed'
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

/** 单个字体族的卡片：名称 / 字重 chips / 格式 / 体积 / 许可证 + 下载 / 删除。 */
function FontCard(props: {
  font: FontDef
  /** 是否已在本机装好（来自后端的跨会话清单，跨关闭重开保持）。 */
  installed: boolean
  /** 正在删除的这张卡片：禁用删除按钮，防重复点击。 */
  deleting: boolean
  state?: DownloadState
  onDownload: (id: string) => void
  onDelete: (id: string) => void
}): JSX.Element {
  const { font, installed, deleting, state, onDownload, onDelete } = props
  const { t } = useTranslation()
  const totalBytes = font.files.reduce((sum, f) => sum + f.size, 0)

  let label = t('settings.fonts.download')
  if (state?.status === 'downloading') {
    label = state.known
      ? t('settings.fonts.downloading', { percent: state.percent })
      : t('settings.fonts.downloadingIndeterminate')
  } else if (state?.status === 'failed') {
    label = t('settings.fonts.downloadFailed')
  }

  // 内联两段式删除：第一次点击进入「确认删除」待命态，3 秒不操作自动复位，
  // 再点一次才真正删除。体感等同连点两次，免弹窗。
  const [armed, setArmed] = useState(false)
  useEffect(() => {
    if (!armed) return
    const timer = window.setTimeout(() => setArmed(false), 3000)
    return () => window.clearTimeout(timer)
  }, [armed])

  const deleteLabel = deleting
    ? t('settings.fonts.deleting')
    : armed
      ? t('settings.fonts.deleteConfirm')
      : t('settings.fonts.delete')

  function handleDelete(): void {
    if (armed) onDelete(font.id)
    else setArmed(true)
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
        {installed ? (
          <div className={styles.downloadActions}>
            <span className={styles.installedLabel}>
              {t('settings.fonts.downloaded')}
            </span>
            <button
              type="button"
              className={styles.deleteBtn}
              data-status={armed ? 'confirm' : 'idle'}
              disabled={deleting}
              title={deleteLabel}
              aria-label={deleteLabel}
              onClick={handleDelete}
            >
              {deleteLabel}
            </button>
          </div>
        ) : (
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
        )}
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
  // 已装字体 id 集合——来自后端的真实跨会话清单，不是本会话临时加工。
  const [installed, setInstalled] = useState<Set<string>>(new Set())
  // 正在删除的字体 id：删除请求进行中，禁用该卡片的删除按钮。
  const [deleting, setDeleting] = useState<string | null>(null)

  // 每次打开都重新拉取：字体目录可能随版本更新，不缓存到关闭；已装清单同样重取，
  // 覆盖「别的会话/外部文件变更后」的状态漂移。
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
    // 已装清单失败不阻塞面板——最坏是「已下载」标不出来，目录本身还能看。
    FontService.ListInstalledFonts()
      .then((list: InstalledFont[]) =>
        setInstalled(new Set(list.map((f) => f.id))),
      )
      .catch(() => {})
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
      // 成功后即视为已装：文件已落盘且逐个 SHA256 通过，无需再回查一遍清单。
      setInstalled((prev) => new Set(prev).add(id))
      setDownloads((prev) => {
        const next = { ...prev }
        delete next[id]
        return next
      })
    } catch (e) {
      setDownloads((prev) => ({
        ...prev,
        [id]: { status: 'failed', percent: 0, known: false },
      }))
      console.error(toApiError(e))
    }
  }

  /** 卸载字体，成功后从已装清单移除；若删的正是当前使用的字体则回落预设并写回。 */
  async function deleteFont(id: string): Promise<void> {
    setDeleting(id)
    try {
      await FontService.DeleteFont(id)
      setInstalled((prev) => {
        const next = new Set(prev)
        next.delete(id)
        return next
      })
      if (useSettingsStore.getState().settings?.readerFontFamily === id) {
        // 删的正是当前使用的字体：回落到预设并**写回后端**（用户偏好确实变了）。
        // 目前字体 id 还不是合法 fontFamily 值（阶段 C 才动态化白名单），但后端字段
        // 是宽 string，这里防御性兜底，阶段 C 之后自动生效。
        useReaderStore.getState().setFontFamily('sans')
        showToast(t('settings.fonts.reverted'))
      } else {
        showToast(t('settings.fonts.deleted'))
      }
    } catch (e) {
      showToast(toApiError(e), 'error')
    } finally {
      setDeleting(null)
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
                  installed={installed.has(font.id)}
                  deleting={deleting === font.id}
                  state={downloads[font.id]}
                  onDownload={download}
                  onDelete={(id) => void deleteFont(id)}
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
