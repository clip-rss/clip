import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as Dialog from '@radix-ui/react-dialog'
import { sanitizeHtml, SystemService, toApiError } from '../../Utils'
import Skeleton from '../Skeleton/Skeleton'
import styles from './ChangelogModal.module.scss'

interface ChangelogModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

/** 轻度 Markdown → HTML 转换，仅处理 CHANGELOG.md 中使用的标记。 */
function markdownToHTML(md: string): string {
  // 按段落分割（空行分隔）
  const blocks = md.split(/\n\n+/)
  return blocks
    .map((block) => {
      const trimmed = block.trim()
      if (!trimmed) return ''

      // ## 标题
      if (trimmed.startsWith('## ')) {
        return `<h2>${escapeHTML(trimmed.slice(3))}</h2>`
      }
      // #### 日期
      if (trimmed.startsWith('#### ')) {
        return `<h4>${inline(trimmed.slice(5))}</h4>`
      }

      // 无序列表
      if (trimmed.startsWith('- ')) {
        const items = trimmed
          .split('\n')
          .filter((l) => l.trim().startsWith('- '))
          .map((l) => `<li>${inline(l.trim().slice(2))}</li>`)
          .join('')
        return `<ul>${items}</ul>`
      }

      // 代码块
      if (trimmed.startsWith('```')) {
        const inner = trimmed.replace(/^```\w*\n?/, '').replace(/\n?```$/, '')
        return `<pre><code>${escapeHTML(inner)}</code></pre>`
      }

      // 普通段落
      return `<p>${inline(trimmed)}</p>`
    })
    .join('\n')
}

/** 行内格式：加粗、链接、行内代码 */
function inline(text: string): string {
  return (
    text
      // 加粗
      .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
      // 链接
      .replace(
        /\[([^\]]+)\]\(([^)]+)\)/g,
        '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>',
      )
      // 行内代码
      .replace(/`([^`]+)`/g, '<code>$1</code>')
  )
}

function escapeHTML(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
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
        const raw = markdownToHTML(md)
        setHtml(sanitizeHtml(raw))
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
