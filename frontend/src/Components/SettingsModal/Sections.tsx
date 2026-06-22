import { useEffect, useRef, useState } from 'react'
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
import { SegmentedControl, SettingRow, Toggle } from './Controls'
import styles from './SettingsModal.module.scss'

/* ============================ 通用 ============================ */

const UPDATE_INTERVAL_OPTIONS = [
  { value: 15, label: '15 分钟' },
  { value: 30, label: '30 分钟' },
  { value: 60, label: '1 小时' },
  { value: 0, label: '手动' },
]

const MAX_ITEMS_OPTIONS = [50, 100, 200, 500]

export function GeneralSection(): JSX.Element {
  const settings = useSettingsStore((s) => s.settings)
  const update = useSettingsStore((s) => s.update)

  return (
    <div>
      <h3 className={styles.sectionTitle}>通用</h3>
      <SettingRow
        label="全局更新间隔"
        description="新订阅源默认的自动刷新周期；选「手动」则不自动刷新"
      >
        <SegmentedControl
          value={settings?.defaultUpdateInterval ?? 30}
          options={UPDATE_INTERVAL_OPTIONS}
          onChange={(v) => update({ defaultUpdateInterval: v })}
        />
      </SettingRow>

      <SettingRow
        label="每源最大条目"
        description="新订阅源默认保留的最大文章数"
      >
        <select
          className={styles.select}
          value={settings?.defaultMaxItems ?? 100}
          onChange={(e) => update({ defaultMaxItems: Number(e.target.value) })}
        >
          {MAX_ITEMS_OPTIONS.map((n) => (
            <option key={n} value={n}>
              {n} 篇
            </option>
          ))}
        </select>
      </SettingRow>

      <SettingRow label="启动时最小化" description="重启应用后生效">
        <Toggle
          checked={settings?.launchMinimized ?? false}
          onChange={(v) => update({ launchMinimized: v })}
          label="启动时最小化"
        />
      </SettingRow>

      <SettingRow label="语言" description="界面文案完整翻译将在后续版本提供">
        <select
          className={styles.select}
          value={settings?.language ?? 'zh'}
          onChange={(e) => update({ language: e.target.value })}
        >
          <option value="zh">中文</option>
          <option value="en">English</option>
        </select>
      </SettingRow>
    </div>
  )
}

/* ============================ 阅读 ============================ */

const FONT_OPTIONS: { value: ReaderFontFamily; label: string }[] = [
  { value: 'sans', label: '无衬线' },
  { value: 'serif', label: '衬线' },
  { value: 'mono', label: '等宽' },
]

const SIZE_OPTIONS: { value: ReaderFontSize; label: string }[] = [
  { value: 14, label: '小' },
  { value: 16, label: '中' },
  { value: 18, label: '大' },
]

const LINE_OPTIONS: { value: ReaderLineHeight; label: string }[] = [
  { value: 1.5, label: '紧凑' },
  { value: 1.8, label: '适中' },
  { value: 2.0, label: '宽松' },
]

const WIDTH_OPTIONS: { value: ReaderWidth; label: string }[] = [
  { value: '640', label: '窄' },
  { value: '800', label: '宽' },
  { value: 'full', label: '全宽' },
]

const BG_OPTIONS: { value: ReaderBackground; label: string }[] = [
  { value: 'default', label: '跟随' },
  { value: 'light', label: '亮' },
  { value: 'sepia', label: '护眼' },
  { value: 'dark', label: '暗' },
]

const AUTO_MARK_OPTIONS = [
  { value: 0, label: '立即' },
  { value: 2000, label: '2 秒' },
  { value: 5000, label: '5 秒' },
  { value: -1, label: '关闭' },
]

export function ReadingSection(): JSX.Element {
  const reader = useReaderStore()
  const settings = useSettingsStore((s) => s.settings)
  const update = useSettingsStore((s) => s.update)

  return (
    <div>
      <h3 className={styles.sectionTitle}>阅读</h3>
      <SettingRow label="字体">
        <SegmentedControl
          value={reader.fontFamily}
          options={FONT_OPTIONS}
          onChange={reader.setFontFamily}
        />
      </SettingRow>
      <SettingRow label="字号">
        <SegmentedControl
          value={reader.fontSize}
          options={SIZE_OPTIONS}
          onChange={reader.setFontSize}
        />
      </SettingRow>
      <SettingRow label="行高">
        <SegmentedControl
          value={reader.lineHeight}
          options={LINE_OPTIONS}
          onChange={reader.setLineHeight}
        />
      </SettingRow>
      <SettingRow label="阅读宽度">
        <SegmentedControl
          value={reader.width}
          options={WIDTH_OPTIONS}
          onChange={reader.setWidth}
        />
      </SettingRow>
      <SettingRow
        label="阅读背景"
        description="阅读区独立背景色，可与全局主题不同"
      >
        <SegmentedControl
          value={reader.background}
          options={BG_OPTIONS}
          onChange={reader.setBackground}
        />
      </SettingRow>
      <SettingRow
        label="自动标记已读"
        description="打开文章后延迟多久标记为已读"
      >
        <SegmentedControl
          value={settings?.autoMarkReadDelay ?? 0}
          options={AUTO_MARK_OPTIONS}
          onChange={(v) => update({ autoMarkReadDelay: v })}
        />
      </SettingRow>
    </div>
  )
}

/* ============================ 主题 ============================ */

const THEME_OPTIONS: { value: ThemePreference; label: string }[] = [
  { value: 'light', label: '亮色' },
  { value: 'dark', label: '暗色' },
  { value: 'sepia', label: '护眼' },
  { value: 'system', label: '跟随系统' },
]

export function ThemeSection(): JSX.Element {
  const preference = useThemeStore((s) => s.preference)
  const setPreference = useThemeStore((s) => s.setPreference)

  return (
    <div>
      <h3 className={styles.sectionTitle}>主题</h3>
      <SettingRow label="外观" description="护眼为暖色（Sepia）配色">
        <SegmentedControl
          value={preference}
          options={THEME_OPTIONS}
          onChange={setPreference}
        />
      </SettingRow>
    </div>
  )
}

/* ============================ 通知 ============================ */

const NOTIF_OPTIONS = [
  { value: 'each', label: '每篇' },
  { value: 'summary', label: '摘要' },
  { value: 'off', label: '关闭' },
]

export function NotificationSection(): JSX.Element {
  const settings = useSettingsStore((s) => s.settings)
  const setNotificationMode = useSettingsStore((s) => s.setNotificationMode)

  return (
    <div>
      <h3 className={styles.sectionTitle}>通知</h3>
      <SettingRow
        label="新文章通知"
        description="每篇：每条新文章一条；摘要：每次刷新一条汇总"
      >
        <SegmentedControl
          value={settings?.notificationMode ?? 'each'}
          options={NOTIF_OPTIONS}
          onChange={(v) => setNotificationMode(v as 'each' | 'summary' | 'off')}
        />
      </SettingRow>
    </div>
  )
}

/* ============================ 数据管理 ============================ */

export function DataSection(): JSX.Element {
  const [dbPath, setDbPath] = useState('')
  const [confirmClear, setConfirmClear] = useState(false)
  const [feedback, setFeedback] = useState('')
  const [isError, setIsError] = useState(false)
  const [busy, setBusy] = useState(false)
  const importRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    SettingsService.DatabasePath()
      .then(setDbPath)
      .catch(() => setDbPath('（无法获取路径）'))
  }, [])

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
      notify(`已清理 ${removed} 篇已读且未收藏的文章。`)
    } catch (err) {
      notify(`清理失败：${toApiError(err)}`, true)
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
        `导入完成：新增 ${res.feeds} 个订阅源，跳过 ${res.skipped} 个重复，新建 ${res.categories} 个文件夹。`,
      )
    } catch (err) {
      notify(`OPML 导入失败：${toApiError(err)}`, true)
    } finally {
      setBusy(false)
    }
  }

  async function handleExportOpml(): Promise<void> {
    try {
      await exportOpmlToFile()
      notify('已导出 OPML 订阅文件。')
    } catch (err) {
      notify(`OPML 导出失败：${toApiError(err)}`, true)
    }
  }

  async function handleBackup(): Promise<void> {
    setBusy(true)
    try {
      const ok = await SettingsService.BackupDatabase()
      notify(ok ? '数据库已备份。' : '已取消备份。')
    } catch (err) {
      notify(`备份失败：${toApiError(err)}`, true)
    } finally {
      setBusy(false)
    }
  }

  async function handleRestore(): Promise<void> {
    setBusy(true)
    try {
      const ok = await SettingsService.RestoreDatabase()
      notify(ok ? '已导入备份，请重启应用以生效。' : '已取消恢复。')
    } catch (err) {
      notify(`恢复失败：${toApiError(err)}`, true)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <h3 className={styles.sectionTitle}>数据管理</h3>

      <SettingRow label="数据库位置" description="应用数据存储路径">
        <div className={styles.pathBox}>{dbPath || '加载中…'}</div>
      </SettingRow>

      <SettingRow
        label="清理缓存"
        description="删除已读且未收藏的文章并回收空间，保留未读与收藏"
      >
        {confirmClear ? (
          <div className={styles.btnGroup}>
            <button
              type="button"
              className={`${styles.btn} ${styles.btnDanger}`}
              onClick={handleClearCache}
              disabled={busy}
            >
              确认清理
            </button>
            <button
              type="button"
              className={styles.btn}
              onClick={() => setConfirmClear(false)}
            >
              取消
            </button>
          </div>
        ) : (
          <button
            type="button"
            className={styles.btn}
            onClick={() => setConfirmClear(true)}
            disabled={busy}
          >
            清理…
          </button>
        )}
      </SettingRow>

      <SettingRow
        label="OPML 订阅"
        description="导入或导出订阅源列表（不含文章）"
      >
        <div className={styles.btnGroup}>
          <button
            type="button"
            className={styles.btn}
            onClick={() => importRef.current?.click()}
            disabled={busy}
          >
            导入
          </button>
          <button
            type="button"
            className={styles.btn}
            onClick={handleExportOpml}
            disabled={busy}
          >
            导出
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
        label="数据库备份"
        description="备份或恢复完整数据库（含文章、笔记、已读状态）"
      >
        <div className={styles.btnGroup}>
          <button
            type="button"
            className={styles.btn}
            onClick={handleBackup}
            disabled={busy}
          >
            备份…
          </button>
          <button
            type="button"
            className={styles.btn}
            onClick={handleRestore}
            disabled={busy}
          >
            恢复…
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
