import * as Dialog from '@radix-ui/react-dialog'
import clsx from 'clsx'
import styles from './Sidebar.module.scss'

interface ConfirmDialogProps {
  open: boolean
  title: string
  description?: string
  confirmText?: string
  cancelText?: string
  /** 危险操作（如删除）使用 --danger 配色。 */
  danger?: boolean
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}

/** 通用确认对话框，基于 Radix Dialog；删除等不可逆操作复用。 */
function ConfirmDialog(props: ConfirmDialogProps): JSX.Element {
  const {
    open,
    title,
    description,
    confirmText = '确认',
    cancelText = '取消',
    danger = false,
    onConfirm,
    onOpenChange,
  } = props

  function handleConfirm(): void {
    onConfirm()
    onOpenChange(false)
  }

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className={styles.dialogOverlay} />
        <Dialog.Content className={styles.dialogContent}>
          <Dialog.Title className={styles.dialogTitle}>{title}</Dialog.Title>
          {description ? (
            <Dialog.Description className={styles.dialogDesc}>{description}</Dialog.Description>
          ) : null}
          <div className={styles.dialogFooter}>
            <Dialog.Close asChild>
              <button type="button" className={styles.dialogCancel}>
                {cancelText}
              </button>
            </Dialog.Close>
            <button
              type="button"
              className={clsx(styles.dialogConfirm, danger && styles.dialogConfirmDanger)}
              onClick={handleConfirm}
            >
              {confirmText}
            </button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

export default ConfirmDialog
