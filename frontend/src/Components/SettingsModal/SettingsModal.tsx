import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as Dialog from '@radix-ui/react-dialog'
import clsx from 'clsx'
import { useSettingsStore } from '../../Stores'
import {
  AboutSection,
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

type SectionId =
  | 'general'
  | 'reading'
  | 'theme'
  | 'notification'
  | 'proxy'
  | 'shortcuts'
  | 'data'
  | 'about'

const NAV_KEYS: { id: SectionId; labelKey: string }[] = [
  { id: 'general', labelKey: 'settings.tabs.general' },
  { id: 'reading', labelKey: 'settings.tabs.reading' },
  { id: 'theme', labelKey: 'settings.tabs.theme' },
  { id: 'notification', labelKey: 'settings.tabs.notification' },
  { id: 'proxy', labelKey: 'settings.tabs.proxy' },
  { id: 'shortcuts', labelKey: 'settings.tabs.shortcuts' },
  { id: 'data', labelKey: 'settings.tabs.data' },
  { id: 'about', labelKey: 'settings.tabs.about' },
]

function SettingsModal(props: SettingsModalProps): JSX.Element {
  const { t } = useTranslation()
  const { open, onOpenChange } = props
  const [active, setActive] = useState<SectionId>('general')

  useEffect(() => {
    if (open) void useSettingsStore.getState().load()
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
              {t('settings.title')}
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

          <div className={styles.main}>
            <nav className={styles.nav}>
              {NAV_KEYS.map((n) => (
                <button
                  key={n.id}
                  type="button"
                  className={clsx(
                    styles.navItem,
                    active === n.id && styles.navItemActive,
                  )}
                  onClick={() => setActive(n.id)}
                >
                  {t(n.labelKey)}
                </button>
              ))}
            </nav>

            <div className={styles.panel}>
              <div key={active} className={styles.panelContent}>
                {active === 'general' ? <GeneralSection /> : null}
                {active === 'reading' ? <ReadingSection /> : null}
                {active === 'theme' ? <ThemeSection /> : null}
                {active === 'notification' ? <NotificationSection /> : null}
                {active === 'proxy' ? <ProxySection /> : null}
                {active === 'shortcuts' ? <ShortcutSection /> : null}
                {active === 'data' ? <DataSection /> : null}
                {active === 'about' ? <AboutSection /> : null}
              </div>
            </div>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
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

export default SettingsModal
