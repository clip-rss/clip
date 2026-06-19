import { useEffect, useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import clsx from 'clsx'
import { useSidebarStore } from '../../Stores'
import { FeedService, flattenCategories, toApiError } from '../../Utils'
import type { FeedPreview } from '../../Types'
import styles from './AddFeedModal.module.scss'

interface AddFeedModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type Status = 'idle' | 'detecting' | 'detected' | 'error' | 'submitting'

/**
 * 添加订阅弹窗：两阶段交互——先检测订阅地址（支持网页自动发现），
 * 检测成功后填写名称、选择归属文件夹并确认添加。
 */
function AddFeedModal(props: AddFeedModalProps): JSX.Element {
  const { open, onOpenChange } = props
  const categories = useSidebarStore((s) => s.categories)
  const reload = useSidebarStore((s) => s.load)

  const [url, setUrl] = useState('')
  const [status, setStatus] = useState<Status>('idle')
  const [preview, setPreview] = useState<FeedPreview | null>(null)
  const [name, setName] = useState('')
  const [categoryId, setCategoryId] = useState(0)
  const [errorMsg, setErrorMsg] = useState('')

  // 每次打开弹窗时重置全部状态。
  useEffect(() => {
    if (open) reset()
  }, [open])

  const options = flattenCategories(categories)
  const detected = status === 'detected' && preview !== null
  const canAdd = detected && !preview!.alreadyAdded

  function reset(): void {
    setUrl('')
    setStatus('idle')
    setPreview(null)
    setName('')
    setCategoryId(0)
    setErrorMsg('')
  }

  function handleUrlChange(value: string): void {
    setUrl(value)
    // 改地址后需重新检测，清掉上一次结果。
    if (status !== 'idle') {
      setStatus('idle')
      setPreview(null)
      setErrorMsg('')
    }
  }

  async function detect(): Promise<void> {
    const trimmed = url.trim()
    if (!trimmed || status === 'detecting') return
    setStatus('detecting')
    setErrorMsg('')
    try {
      const result = await FeedService.PreviewFeed(trimmed)
      if (!result) throw new Error('未在该地址找到可订阅的源')
      setPreview(result)
      setName(result.title)
      setStatus('detected')
    } catch (err) {
      setErrorMsg(toApiError(err))
      setStatus('error')
    }
  }

  async function submit(): Promise<void> {
    if (!canAdd || !preview) return
    setStatus('submitting')
    try {
      const created = await FeedService.AddFeed(preview.url, categoryId)
      const finalName = name.trim()
      if (created && finalName && finalName !== preview.title) {
        await FeedService.UpdateFeed({ ...created, title: finalName })
      }
      await reload()
      onOpenChange(false)
    } catch (err) {
      setErrorMsg(toApiError(err))
      setStatus('detected') // 退回可重试态
    }
  }

  function handleUrlKeyDown(e: React.KeyboardEvent): void {
    if (e.key === 'Enter') {
      e.preventDefault()
      detect()
    }
  }

  function handleNameKeyDown(e: React.KeyboardEvent): void {
    if (e.key === 'Enter') {
      e.preventDefault()
      submit()
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
            <div className={styles.headerLeft}>
              <RssIcon />
              <Dialog.Title className={styles.title}>添加订阅</Dialog.Title>
            </div>
            <Dialog.Close asChild>
              <button type="button" className={styles.closeBtn} aria-label="关闭">
                <CloseIcon />
              </button>
            </Dialog.Close>
          </header>

          <div className={styles.body}>
            <label className={styles.label} htmlFor="add-feed-url">
              RSS / Atom 订阅地址
            </label>
            <div className={styles.urlRow}>
              <input
                id="add-feed-url"
                type="text"
                className={styles.urlInput}
                placeholder="请输入"
                value={url}
                onChange={(e) => handleUrlChange(e.target.value)}
                onKeyDown={handleUrlKeyDown}
                readOnly={status === 'detecting'}
                autoFocus
              />
              <button
                type="button"
                className={styles.detectBtn}
                onClick={detect}
                disabled={!url.trim() || status === 'detecting'}
              >
                {status === 'detecting' ? '检测中…' : '检测'}
              </button>
            </div>

            {detected ? (
              <div className={clsx(styles.resultBar, styles.resultOk)}>
                <CheckIcon />
                <span>
                  检测成功，找到订阅源：{preview!.title}
                  {preview!.itemCount > 0 ? `（${preview!.itemCount} 篇文章）` : ''}
                </span>
              </div>
            ) : null}

            {status === 'error' ? (
              <div className={clsx(styles.resultBar, styles.resultError)}>
                <CrossIcon />
                <span>{errorMsg || '无法解析该地址，请检查 URL'}</span>
              </div>
            ) : null}

            {detected && preview!.alreadyAdded ? (
              <div className={clsx(styles.resultBar, styles.resultError)}>
                <CrossIcon />
                <span>该订阅源已存在，无需重复添加</span>
              </div>
            ) : null}

            {detected && !preview!.alreadyAdded ? (
              <>
                <div className={styles.fieldGroup}>
                  <label className={styles.label} htmlFor="add-feed-name">
                    订阅源名称（可修改）
                  </label>
                  <input
                    id="add-feed-name"
                    type="text"
                    className={styles.textInput}
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    onKeyDown={handleNameKeyDown}
                  />
                </div>

                <div className={styles.fieldGroup}>
                  <label className={styles.label} htmlFor="add-feed-folder">
                    归属文件夹
                  </label>
                  <select
                    id="add-feed-folder"
                    className={styles.select}
                    value={categoryId}
                    onChange={(e) => setCategoryId(Number(e.target.value))}
                  >
                    <option value={0}>未分类</option>
                    {options.map((c) => (
                      <option key={c.id} value={c.id}>
                        {'　'.repeat(c.depth)}
                        {c.name}
                      </option>
                    ))}
                  </select>
                </div>

                {errorMsg ? <p className={styles.inlineError}>{errorMsg}</p> : null}
              </>
            ) : null}
          </div>

          <footer className={styles.footer}>
            <Dialog.Close asChild>
              <button type="button" className={styles.cancelBtn}>
                取消
              </button>
            </Dialog.Close>
            <button
              type="button"
              className={styles.addBtn}
              onClick={submit}
              disabled={!canAdd}
            >
              {status === 'submitting' ? '添加中…' : '添加订阅'}
            </button>
          </footer>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

function RssIcon(): JSX.Element {
  return (
    <svg
      className={styles.rssIcon}
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M4 11a9 9 0 0 1 9 9" />
      <path d="M4 4a16 16 0 0 1 16 16" />
      <circle cx="5" cy="19" r="1" fill="currentColor" />
    </svg>
  )
}

function CloseIcon(): JSX.Element {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden="true">
      <path d="M18 6 6 18M6 6l12 12" />
    </svg>
  )
}

function CheckIcon(): JSX.Element {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M20 6 9 17l-5-5" />
    </svg>
  )
}

function CrossIcon(): JSX.Element {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" aria-hidden="true">
      <circle cx="12" cy="12" r="9" />
      <path d="m15 9-6 6M9 9l6 6" />
    </svg>
  )
}

export default AddFeedModal
