import { useEffect, useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import clsx from 'clsx'
import { useSettingsStore } from '../../Stores'
import {
  DataSection,
  GeneralSection,
  NotificationSection,
  ProxySection,
  ReadingSection,
  ShortcutSection,
  ThemeSection,
} from './Sections'
import styles from './SettingsModal.module.scss'

interface SettingsModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type SectionId = 'general' | 'reading' | 'theme' | 'notification' | 'proxy' | 'shortcuts' | 'data'

const NAV: { id: SectionId; label: string }[] = [
  { id: 'general', label: '通用' },
  { id: 'reading', label: '阅读' },
  { id: 'theme', label: '主题' },
  { id: 'notification', label: '通知' },
  { id: 'proxy', label: '代理' },
  { id: 'shortcuts', label: '快捷键' },
  { id: 'data', label: '数据管理' },
]

/**
 * 应用设置面板：左侧分区导航 + 右侧内容。聚合后端设置（通用/通知/自动已读）、
 * localStorage 偏好（主题/阅读排版）与数据管理（清理缓存、OPML、数据库备份恢复）。
 */
function SettingsModal(props: SettingsModalProps): JSX.Element {
  const { open, onOpenChange } = props
  const [active, setActive] = useState<SectionId>('general')

  // 打开时兜底刷新后端设置（启动时通常已加载）。
  useEffect(() => {
    if (open) void useSettingsStore.getState().load()
  }, [open])

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className={styles.overlay} />
        <Dialog.Content className={styles.content} aria-describedby={undefined}>
          <header className={styles.header}>
            <Dialog.Title className={styles.title}>设置</Dialog.Title>
            <Dialog.Close asChild>
              <button type="button" className={styles.closeBtn} aria-label="关闭">
                <CloseIcon />
              </button>
            </Dialog.Close>
          </header>

          <div className={styles.main}>
            <nav className={styles.nav}>
              {NAV.map((n) => (
                <button
                  key={n.id}
                  type="button"
                  className={clsx(styles.navItem, active === n.id && styles.navItemActive)}
                  onClick={() => setActive(n.id)}
                >
                  {n.label}
                </button>
              ))}
            </nav>

            <div className={styles.panel}>
              {active === 'general' ? <GeneralSection /> : null}
              {active === 'reading' ? <ReadingSection /> : null}
              {active === 'theme' ? <ThemeSection /> : null}
              {active === 'notification' ? <NotificationSection /> : null}
              {active === 'proxy' ? <ProxySection /> : null}
              {active === 'shortcuts' ? <ShortcutSection /> : null}
              {active === 'data' ? <DataSection /> : null}
            </div>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

function CloseIcon(): JSX.Element {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden="true">
      <path d="M18 6 6 18M6 6l12 12" />
    </svg>
  )
}

export default SettingsModal
