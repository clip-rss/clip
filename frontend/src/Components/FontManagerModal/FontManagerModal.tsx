import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as Dialog from '@radix-ui/react-dialog'
import Skeleton from '../Skeleton/Skeleton'
import {
  FontService,
  onFontDownloadProgress,
  openURL,
  showToast,
  toApiError,
} from '../../Utils'
import { useFocusReaderStore, useFontStore, useReaderStore } from '../../Stores'
import type { FontDef } from '../../Types'
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

/** 许可证外链：licenseFile 是**仓库内相对路径**（如 lxgw-wenkai/OFL.txt），
 *  不是 release 资产名，不能做 basename 归一化。 */
function licenseURL(font: FontDef): string {
  return `https://github.com/clip-rss/clip-font/blob/main/${font.licenseFile}`
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
  onOpenDir: (id: string) => void
}): JSX.Element {
  const { font, installed, deleting, state, onDownload, onDelete, onOpenDir } =
    props
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

  const [confirming, setConfirming] = useState(false)

  function confirmDelete(): void {
    setConfirming(false)
    onDelete(font.id)
  }

  return (
    <div className={styles.card}>
      <div className={styles.cardHead}>
        <span className={styles.cardName}>{font.name}</span>
        <span className={styles.cardFamily}>{font.family}</span>
      </div>
      <div className={styles.cardMeta}>
        <span className={styles.chip}>{font.files[0]?.format ?? ''}</span>
        <span className={styles.chip}>{formatMB(totalBytes)}</span>
      </div>
      <div className={styles.cardFoot}>
        {font.licenseFile ? (
          <button
            type="button"
            className={styles.licenseLink}
            title={font.license}
            aria-label={font.license}
            onClick={() => openURL(licenseURL(font))}
          >
            {font.license}
          </button>
        ) : (
          <span className={styles.cardLicense}>{font.license}</span>
        )}
        {installed ? (
          confirming ? (
            <div className={styles.btnGroup}>
              <button
                type="button"
                className={`${styles.btn} ${styles.btnDanger}`}
                disabled={deleting}
                title={t('settings.fonts.deleteConfirm')}
                aria-label={t('settings.fonts.deleteConfirm')}
                onClick={confirmDelete}
              >
                {t('settings.fonts.deleteConfirm')}
              </button>
              <button
                type="button"
                className={styles.btn}
                title={t('confirm.cancel')}
                aria-label={t('confirm.cancel')}
                onClick={() => setConfirming(false)}
              >
                {t('confirm.cancel')}
              </button>
            </div>
          ) : (
            <div className={styles.downloadActions}>
              <span className={styles.installedLabel}>
                {t('settings.fonts.downloaded')}
              </span>
              <button
                type="button"
                className={styles.btn}
                title={t('settings.fonts.openDir')}
                aria-label={t('settings.fonts.openDir')}
                onClick={() => onOpenDir(font.id)}
              >
                {t('settings.fonts.openDir')}
              </button>
              <button
                type="button"
                className={styles.btn}
                disabled={deleting}
                title={t('settings.fonts.delete')}
                aria-label={t('settings.fonts.delete')}
                onClick={() => setConfirming(true)}
              >
                {deleting
                  ? t('settings.fonts.deleting')
                  : `${t('settings.fonts.delete')}…`}
              </button>
            </div>
          )
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
  // 目录与已装清单来自共享 FontStore：@font-face 注入、阅读器选择器分组与这里
  // 共用同一份，避免各处各拉一次、状态漂移。
  const catalog = useFontStore((s) => s.catalog)
  const catalogFailed = useFontStore((s) => s.catalogFailed)
  const refreshing = useFontStore((s) => s.refreshing)
  const installed = useFontStore((s) => s.installed)
  const refresh = useFontStore((s) => s.refresh)
  const [downloads, setDownloads] = useState<Record<string, DownloadState>>({})
  // 正在删除的字体 id：删除请求进行中，禁用该卡片的删除按钮。
  const [deleting, setDeleting] = useState<string | null>(null)

  const installedIds = useMemo(
    () => new Set(installed.map((f) => f.id)),
    [installed],
  )

  // 每次打开都重新拉取：字体目录可能随版本更新，不缓存到关闭；已装清单同样重取，
  // 覆盖「别的会话/外部文件变更后」的状态漂移。下载态一并清空——后端对已校验通过
  // 的文件会跳过，重开面板再点也不会重下。
  useEffect(() => {
    if (!open) return
    setDownloads({})
    void refresh()
  }, [open, refresh])

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
      // 成功后刷新共享清单：文件已落盘且逐个 SHA256 通过，以磁盘扫描结果为准。
      void refresh()
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

  /** 在系统文件管理器中打开该字体的安装目录。失败多半是目录被挪走/删除，改为报错提示。 */
  async function openDir(id: string): Promise<void> {
    try {
      await FontService.OpenFontDirectory(id)
    } catch (e) {
      showToast(toApiError(e), 'error')
    }
  }

  /** 卸载字体；若删的正是当前使用的字体（阅读视图或专注模式）则回落预设并写回后端。 */
  async function deleteFont(id: string): Promise<void> {
    setDeleting(id)
    try {
      await FontService.DeleteFont(id)
      void refresh()
      // 使用中的字体必须回落：两套偏好各自独立，逐套检查。这里**写回后端**（用户偏好
      // 确实变了），与阶段 C「清单变化后重收窄不写回」是两件事，别混。
      let fellBack = false
      if (useReaderStore.getState().fontFamily === id) {
        useReaderStore.getState().setFontFamily('sans')
        fellBack = true
      }
      if (useFocusReaderStore.getState().fontFamily === id) {
        useFocusReaderStore.getState().setFontFamily('sans')
        fellBack = true
      }
      showToast(
        fellBack ? t('settings.fonts.reverted') : t('settings.fonts.deleted'),
      )
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

          <div className={styles.feedbackHint}>
            <span>{t('settings.fonts.missingFontHint')}</span>
            <button
              type="button"
              className={styles.feedbackLink}
              title={t('settings.fonts.missingFontLink')}
              aria-label={t('settings.fonts.missingFontLink')}
              onClick={() =>
                openURL('https://github.com/clip-rss/clip-font/issues')
              }
            >
              {t('settings.fonts.missingFontLink')}
            </button>
          </div>

          {refreshing && !catalog ? (
            <div className={styles.list}>
              <Skeleton className={styles.skeletonCard} height={64} />
              <Skeleton className={styles.skeletonCard} height={64} />
            </div>
          ) : catalogFailed && !catalog ? (
            <div className={styles.failed}>
              <p className={styles.failedText}>{t('settings.fonts.failed')}</p>
              <button
                type="button"
                className={styles.retryBtn}
                onClick={() => void refresh()}
              >
                {t('settings.fonts.retry')}
              </button>
            </div>
          ) : catalog && catalog.fonts.length > 0 ? (
            <div className={styles.list}>
              {catalog.fonts.map((font) => (
                <FontCard
                  key={font.id}
                  font={font}
                  installed={installedIds.has(font.id)}
                  deleting={deleting === font.id}
                  state={downloads[font.id]}
                  onDownload={download}
                  onDelete={(id) => void deleteFont(id)}
                  onOpenDir={(id) => void openDir(id)}
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
