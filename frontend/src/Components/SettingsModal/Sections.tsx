import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import i18n from 'i18next'
import {
  useArticleStore,
  useReaderStore,
  useSettingsStore,
  useSidebarStore,
  useThemeStore,
} from '../../Stores'
import {
  SettingsService,
  exportOpmlToFile,
  importOpmlFromFile,
  toApiError,
} from '../../Utils'
import type {
  ReaderBackground,
  ReaderFontFamily,
  ReaderFontSize,
  ReaderLineHeight,
  ReaderWidth,
  ThemePreference,
} from '../../Types'
import { usePlatform } from '../../Hooks'
import type { Platform } from '../../Hooks'
import { SegmentedControl, SettingRow, Toggle } from './Controls'
import styles from './SettingsModal.module.scss'

/* ============================ 通用 ============================ */

const MAX_ITEMS_OPTIONS = [50, 100, 200, 500]

export function GeneralSection(): JSX.Element {
  const { t } = useTranslation()
  const settings = useSettingsStore((s) => s.settings)
  const update = useSettingsStore((s) => s.update)

  const intervalOptions = [
    { value: 15, label: t('settings.general.updateIntervalOptions.15m') },
    { value: 30, label: t('settings.general.updateIntervalOptions.30m') },
    { value: 60, label: t('settings.general.updateIntervalOptions.1h') },
    { value: 0, label: t('settings.general.updateIntervalOptions.manual') },
  ]

  return (
    <div>
      <h3 className={styles.sectionTitle}>{t('settings.general.title')}</h3>
      <SettingRow
        label={t('settings.general.updateInterval')}
        description={t('settings.general.updateIntervalDesc')}
      >
        <SegmentedControl
          value={settings?.defaultUpdateInterval ?? 30}
          options={intervalOptions}
          onChange={(v) => update({ defaultUpdateInterval: v })}
        />
      </SettingRow>

      <SettingRow
        label={t('settings.general.maxItems')}
        description={t('settings.general.maxItemsDesc')}
      >
        <select
          className={styles.select}
          value={settings?.defaultMaxItems ?? 100}
          onChange={(e) => update({ defaultMaxItems: Number(e.target.value) })}
        >
          {MAX_ITEMS_OPTIONS.map((n) => (
            <option key={n} value={n}>
              {n} {t('settings.general.itemsUnit')}
            </option>
          ))}
        </select>
      </SettingRow>

      <SettingRow
        label={t('settings.general.launchMinimized')}
        description={t('settings.general.launchMinimizedDesc')}
      >
        <Toggle
          checked={settings?.launchMinimized ?? false}
          onChange={(v) => update({ launchMinimized: v })}
          label={t('settings.general.launchMinimized')}
        />
      </SettingRow>

      <SettingRow
        label={t('settings.general.language')}
        description={t('settings.general.languageDesc')}
      >
        <select
          className={styles.select}
          value={settings?.language ?? 'zh'}
          onChange={(e) => {
            const lang = e.target.value
            i18n.changeLanguage(lang)
            update({ language: lang })
          }}
        >
          <option value="zh">简体中文</option>
          <option value="en">English</option>
        </select>
      </SettingRow>
    </div>
  )
}

/* ============================ 阅读 ============================ */

export function ReadingSection(): JSX.Element {
  const { t } = useTranslation()
  const reader = useReaderStore()
  const settings = useSettingsStore((s) => s.settings)
  const update = useSettingsStore((s) => s.update)

  const fontOptions = [
    { value: 'sans' as ReaderFontFamily, label: t('reader.font.sans') },
    { value: 'serif' as ReaderFontFamily, label: t('reader.font.serif') },
    { value: 'mono' as ReaderFontFamily, label: t('reader.font.mono') },
  ]
  const sizeOptions = [
    { value: 14 as ReaderFontSize, label: t('reader.size.small') },
    { value: 16 as ReaderFontSize, label: t('reader.size.medium') },
    { value: 18 as ReaderFontSize, label: t('reader.size.large') },
  ]
  const lineOptions = [
    { value: 1.5 as ReaderLineHeight, label: t('reader.lineHeight.compact') },
    { value: 1.8 as ReaderLineHeight, label: t('reader.lineHeight.moderate') },
    { value: 2.0 as ReaderLineHeight, label: t('reader.lineHeight.loose') },
  ]
  const widthOptions = [
    { value: '640' as ReaderWidth, label: t('reader.width.narrow') },
    { value: '800' as ReaderWidth, label: t('reader.width.wide') },
    { value: 'full' as ReaderWidth, label: t('reader.width.full') },
  ]
  const bgOptions = [
    {
      value: 'default' as ReaderBackground,
      label: t('reader.background.default'),
    },
    { value: 'light' as ReaderBackground, label: t('reader.background.light') },
    { value: 'sepia' as ReaderBackground, label: t('reader.background.sepia') },
    { value: 'dark' as ReaderBackground, label: t('reader.background.dark') },
  ]
  const autoMarkOptions = [
    { value: 0, label: t('settings.reading.autoMark.immediate') },
    { value: 2000, label: t('settings.reading.autoMark.2s') },
    { value: 5000, label: t('settings.reading.autoMark.5s') },
    { value: -1, label: t('settings.reading.autoMark.off') },
  ]

  return (
    <div>
      <h3 className={styles.sectionTitle}>{t('settings.tabs.reading')}</h3>
      <SettingRow label={t('reader.settings.font')}>
        <SegmentedControl
          value={reader.fontFamily}
          options={fontOptions}
          onChange={reader.setFontFamily}
        />
      </SettingRow>
      <SettingRow label={t('reader.settings.fontSize')}>
        <SegmentedControl
          value={reader.fontSize}
          options={sizeOptions}
          onChange={reader.setFontSize}
        />
      </SettingRow>
      <SettingRow label={t('reader.settings.lineHeight')}>
        <SegmentedControl
          value={reader.lineHeight}
          options={lineOptions}
          onChange={reader.setLineHeight}
        />
      </SettingRow>
      <SettingRow label={t('reader.settings.width')}>
        <SegmentedControl
          value={reader.width}
          options={widthOptions}
          onChange={reader.setWidth}
        />
      </SettingRow>
      <SettingRow
        label={t('reader.settings.background')}
        description={t('reader.backgroundDesc')}
      >
        <SegmentedControl
          value={reader.background}
          options={bgOptions}
          onChange={reader.setBackground}
        />
      </SettingRow>
      <SettingRow
        label={t('settings.reading.autoMarkRead')}
        description={t('settings.reading.autoMarkReadDesc')}
      >
        <SegmentedControl
          value={settings?.autoMarkReadDelay ?? 0}
          options={autoMarkOptions}
          onChange={(v) => update({ autoMarkReadDelay: v })}
        />
      </SettingRow>
    </div>
  )
}

/* ============================ 主题 ============================ */

export function ThemeSection(): JSX.Element {
  const { t } = useTranslation()
  const preference = useThemeStore((s) => s.preference)
  const setPreference = useThemeStore((s) => s.setPreference)

  const themeOptions: { value: ThemePreference; label: string }[] = [
    { value: 'light', label: t('settings.theme.mode.light') },
    { value: 'dark', label: t('settings.theme.mode.dark') },
    { value: 'sepia', label: t('theme.sepia') },
    { value: 'system', label: t('settings.theme.mode.system') },
  ]

  return (
    <div>
      <h3 className={styles.sectionTitle}>{t('settings.theme.title')}</h3>
      <SettingRow
        label={t('settings.theme.theme')}
        description={t('settings.theme.themeDesc')}
      >
        <SegmentedControl
          value={preference}
          options={themeOptions}
          onChange={setPreference}
        />
      </SettingRow>
    </div>
  )
}

/* ============================ 通知 ============================ */

export function NotificationSection(): JSX.Element {
  const { t } = useTranslation()
  const settings = useSettingsStore((s) => s.settings)
  const setNotificationMode = useSettingsStore((s) => s.setNotificationMode)

  const notifOptions = [
    { value: 'each', label: t('settings.notification.each') },
    { value: 'summary', label: t('settings.notification.summary') },
    { value: 'off', label: t('settings.notification.off') },
  ]

  return (
    <div>
      <h3 className={styles.sectionTitle}>
        {t('settings.notification.titleSection')}
      </h3>
      <SettingRow
        label={t('settings.notification.title')}
        description={t('settings.notification.modeDesc')}
      >
        <SegmentedControl
          value={settings?.notificationMode ?? 'each'}
          options={notifOptions}
          onChange={(v) => setNotificationMode(v as 'each' | 'summary' | 'off')}
        />
      </SettingRow>
    </div>
  )
}

/* ============================ 数据管理 ============================ */

export function DataSection(): JSX.Element {
  const { t } = useTranslation()
  const [dbPath, setDbPath] = useState('')
  const [confirmClear, setConfirmClear] = useState(false)
  const [feedback, setFeedback] = useState('')
  const [isError, setIsError] = useState(false)
  const [busy, setBusy] = useState(false)
  const importRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    SettingsService.DatabasePath()
      .then(setDbPath)
      .catch(() => setDbPath(t('settings.data.unavailable')))
  }, [t])

  function notify(msg: string, error = false): void {
    setFeedback(msg)
    setIsError(error)
  }

  async function handleClearCache(): Promise<void> {
    setConfirmClear(false)
    setBusy(true)
    try {
      const removed = await SettingsService.ClearCache()
      await useArticleStore.getState().reload()
      await useSidebarStore.getState().load()
      notify(t('settings.data.clearCacheResult', { count: removed }))
    } catch (err) {
      notify(`${t('settings.data.clearCacheError')}：${toApiError(err)}`, true)
    } finally {
      setBusy(false)
    }
  }

  async function handleImportFile(
    e: React.ChangeEvent<HTMLInputElement>,
  ): Promise<void> {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return
    setBusy(true)
    try {
      const res = await importOpmlFromFile(file)
      await useSidebarStore.getState().load()
      notify(
        t('settings.data.importSuccess', {
          feeds: res.feeds,
          skipped: res.skipped,
          categories: res.categories,
        }),
      )
    } catch (err) {
      notify(`${t('settings.data.importError')}：${toApiError(err)}`, true)
    } finally {
      setBusy(false)
    }
  }

  async function handleExportOpml(): Promise<void> {
    try {
      await exportOpmlToFile()
      notify(t('settings.data.exportSuccess'))
    } catch (err) {
      notify(`${t('settings.data.exportError')}：${toApiError(err)}`, true)
    }
  }

  async function handleBackup(): Promise<void> {
    setBusy(true)
    try {
      const ok = await SettingsService.BackupDatabase()
      notify(
        ok
          ? t('settings.data.backupSuccess')
          : t('settings.data.backupCancelled'),
      )
    } catch (err) {
      notify(`${t('settings.data.backupError')}：${toApiError(err)}`, true)
    } finally {
      setBusy(false)
    }
  }

  async function handleRestore(): Promise<void> {
    setBusy(true)
    try {
      const ok = await SettingsService.RestoreDatabase()
      notify(
        ok
          ? t('settings.data.restoreSuccess')
          : t('settings.data.restoreCancelled'),
      )
    } catch (err) {
      notify(`${t('settings.data.restoreError')}：${toApiError(err)}`, true)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <h3 className={styles.sectionTitle}>{t('settings.data.title')}</h3>

      <SettingRow
        label={t('settings.data.dbPath')}
        description={t('settings.data.dbPathDesc')}
      >
        <div className={styles.pathBox}>
          {dbPath || t('settings.data.loading')}
        </div>
      </SettingRow>

      <SettingRow
        label={t('settings.data.clearCache')}
        description={t('settings.data.clearCacheDesc')}
      >
        {confirmClear ? (
          <div className={styles.btnGroup}>
            <button
              type="button"
              className={`${styles.btn} ${styles.btnDanger}`}
              onClick={handleClearCache}
              disabled={busy}
            >
              {t('settings.data.clearCacheConfirm')}
            </button>
            <button
              type="button"
              className={styles.btn}
              onClick={() => setConfirmClear(false)}
            >
              {t('confirm.cancel')}
            </button>
          </div>
        ) : (
          <button
            type="button"
            className={styles.btn}
            onClick={() => setConfirmClear(true)}
            disabled={busy}
          >
            {t('settings.data.clearCacheBtn')}…
          </button>
        )}
      </SettingRow>

      <SettingRow
        label={t('settings.data.opml')}
        description={t('settings.data.opmlDesc')}
      >
        <div className={styles.btnGroup}>
          <button
            type="button"
            className={styles.btn}
            onClick={() => importRef.current?.click()}
            disabled={busy}
          >
            {t('settings.data.import')}
          </button>
          <button
            type="button"
            className={styles.btn}
            onClick={handleExportOpml}
            disabled={busy}
          >
            {t('settings.data.export')}
          </button>
        </div>
        <input
          ref={importRef}
          type="file"
          accept=".opml,.xml,text/xml,application/xml"
          style={{ display: 'none' }}
          onChange={handleImportFile}
        />
      </SettingRow>

      <SettingRow
        label={t('settings.data.backup')}
        description={t('settings.data.backupDesc')}
      >
        <div className={styles.btnGroup}>
          <button
            type="button"
            className={styles.btn}
            onClick={handleBackup}
            disabled={busy}
          >
            {t('settings.data.backupBtn')}…
          </button>
          <button
            type="button"
            className={styles.btn}
            onClick={handleRestore}
            disabled={busy}
          >
            {t('settings.data.restoreBtn')}…
          </button>
        </div>
      </SettingRow>

      {feedback ? (
        <p
          className={`${styles.feedback} ${isError ? styles.feedbackError : ''}`}
        >
          {feedback}
        </p>
      ) : null}
    </div>
  )
}

/* ============================ 代理 ============================ */

export function AboutSection(): JSX.Element {
  const { t } = useTranslation()

  return (
    <div className={styles.aboutSection}>
      <div className={styles.aboutAppName}>{t('settings.about.appName')}</div>
    </div>
  )
}

export function ProxySection(): JSX.Element {
  const { t } = useTranslation()
  const settings = useSettingsStore((s) => s.settings)
  const stored = useSettingsStore((s) => s.update)
  const [host, setHost] = useState(settings?.proxyHost ?? '')
  const [port, setPort] = useState(settings?.proxyPort?.toString() ?? '')
  const [testing, setTesting] = useState(false)
  const [saving, setSaving] = useState(false)
  const [status, setStatus] = useState<{ ok: boolean; msg: string } | null>(
    null,
  )

  useEffect(() => {
    setHost(settings?.proxyHost ?? '')
    setPort(settings?.proxyPort?.toString() ?? '')
  }, [settings?.proxyHost, settings?.proxyPort])

  async function handleTest(): Promise<void> {
    const portNum = parseInt(port, 10)
    if (!host || !portNum) {
      setStatus({ ok: false, msg: t('settings.proxy.error') })
      return
    }
    setTesting(true)
    setStatus(null)
    try {
      await SettingsService.TestProxy(host, portNum)
      setStatus({ ok: true, msg: t('settings.proxy.success') })
    } catch (err) {
      setStatus({
        ok: false,
        msg: `${t('settings.proxy.failed')}：${toApiError(err)}`,
      })
    } finally {
      setTesting(false)
    }
  }

  async function handleSave(): Promise<void> {
    const portNum = parseInt(port, 10) || 0
    setSaving(true)
    setStatus(null)
    try {
      await stored({ proxyHost: host, proxyPort: portNum })
      setStatus({ ok: true, msg: t('settings.proxy.saved') })
    } catch (err) {
      setStatus({
        ok: false,
        msg: `${t('settings.proxy.saveError')}：${toApiError(err)}`,
      })
    } finally {
      setSaving(false)
    }
  }

  return (
    <div>
      <h3 className={styles.sectionTitle}>{t('settings.proxy.title')}</h3>
      <SettingRow
        label={t('settings.proxy.host')}
        description={t('settings.proxy.hostDesc')}
      >
        <input
          className={styles.input}
          type="text"
          value={host}
          onChange={(e) => setHost(e.target.value)}
          placeholder="127.0.0.1"
        />
      </SettingRow>
      <SettingRow label={t('settings.proxy.port')}>
        <input
          className={styles.input}
          type="number"
          value={port}
          onChange={(e) => setPort(e.target.value)}
          placeholder="8080"
        />
      </SettingRow>

      <div className={styles.btnRow}>
        <button
          type="button"
          className={styles.btn}
          onClick={handleTest}
          disabled={testing}
        >
          {testing ? t('settings.proxy.testing') : t('settings.proxy.testBtn')}
        </button>
        <button
          type="button"
          className={`${styles.btn} ${styles.btnPrimary}`}
          onClick={handleSave}
          disabled={saving}
        >
          {saving ? t('settings.proxy.saving') : t('settings.proxy.save')}
        </button>
      </div>

      {status ? (
        <p
          className={`${styles.feedback} ${status.ok ? '' : styles.feedbackError}`}
        >
          {status.msg}
        </p>
      ) : null}
    </div>
  )
}

/* ============================ 快捷键 ============================ */

interface ShortcutDef {
  combo: string
  descKey: string
}

const MAC_KEY: Record<string, string> = {
  mod: '⌘',
  shift: '⇧',
  alt: '⌥',
  ctrl: '⌃',
  space: '␣',
  'shift+space': '⇧␣',
}
const WIN_KEY: Record<string, string> = {
  mod: 'Ctrl',
  shift: 'Shift',
  alt: 'Alt',
  ctrl: 'Ctrl',
  space: 'space',
  'shift+space': 'shiftSpace',
}

function formatCombo(combo: string, platform: Platform | null): string {
  if (combo.includes('/') || combo.includes('↑') || combo.includes('↓'))
    return combo
  if (combo === 'Esc') return 'Esc'

  const isMac = platform === 'mac'
  const parts = combo.split('+')
  return parts
    .map((p) => {
      const key = p.toLowerCase()
      if (isMac && MAC_KEY[key]) return MAC_KEY[key]
      if (!isMac && WIN_KEY[key]) {
        const winKey = WIN_KEY[key]
        // 可翻译的键名（space / shiftSpace）
        if (winKey === 'space') return i18n.t('key.space')
        if (winKey === 'shiftSpace') return i18n.t('key.shiftSpace')
        return winKey
      }
      return p.length === 1 ? p.toUpperCase() : p
    })
    .join(isMac ? ' ' : ' + ')
}

export function ShortcutSection(): JSX.Element {
  const { t } = useTranslation()
  const platform = usePlatform()

  const groups: { titleKey: string; items: ShortcutDef[] }[] = [
    {
      titleKey: 'settings.shortcuts.groups.general',
      items: [
        { combo: 'mod+n', descKey: 'settings.shortcuts.addFeed' },
        { combo: 'mod+,', descKey: 'settings.shortcuts.openSettings' },
        { combo: 'r', descKey: 'settings.shortcuts.refreshSelected' },
        { combo: 'shift+r', descKey: 'settings.shortcuts.forceRefresh' },
        { combo: '/', descKey: 'settings.shortcuts.focusSearch' },
      ],
    },
    {
      titleKey: 'settings.shortcuts.groups.reading',
      items: [
        { combo: 'j / ↓', descKey: 'settings.shortcuts.nextArticle' },
        { combo: 'k / ↑', descKey: 'settings.shortcuts.prevArticle' },
        { combo: 'space', descKey: 'settings.shortcuts.scrollDown' },
        { combo: 'shift+space', descKey: 'settings.shortcuts.scrollUp' },
        { combo: 'mod+shift+f', descKey: 'settings.shortcuts.toggleFocus' },
        { combo: 'Esc', descKey: 'settings.shortcuts.exitFocus' },
      ],
    },
    {
      titleKey: 'settings.shortcuts.groups.filter',
      items: [
        { combo: 'mod+1', descKey: 'settings.shortcuts.filterAll' },
        { combo: 'mod+2', descKey: 'settings.shortcuts.filterUnread' },
        { combo: 'mod+3', descKey: 'settings.shortcuts.filterStarred' },
      ],
    },
  ]

  return (
    <div>
      <h3 className={styles.sectionTitle}>{t('settings.shortcuts.title')}</h3>
      {groups.map((group) => (
        <div key={group.titleKey} className={styles.shortcutGroup}>
          <h4 className={styles.shortcutGroupTitle}>{t(group.titleKey)}</h4>
          {group.items.map((item) => (
            <div key={item.combo} className={styles.shortcutRow}>
              <kbd className={styles.kbd}>
                {formatCombo(item.combo, platform)}
              </kbd>
              <span className={styles.shortcutDesc}>{t(item.descKey)}</span>
            </div>
          ))}
        </div>
      ))}
    </div>
  )
}
