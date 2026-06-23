import { useTranslation } from 'react-i18next'
import * as Dialog from '@radix-ui/react-dialog'
import clsx from 'clsx'
import styles from './Sidebar.module.scss'

interface ConfirmDialogProps {
  open: boolean
  title: string
  description?: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}

function ConfirmDialog(props: ConfirmDialogProps): JSX.Element {
  const { t } = useTranslation()
  const {
    open,
    title,
    description,
    confirmText = t('confirm.confirm'),
    cancelText = t('confirm.cancel'),
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
        <Dialog.Content
          className={styles.dialogContent}
          onInteractOutside={(e) => e.preventDefault()}
        >
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
