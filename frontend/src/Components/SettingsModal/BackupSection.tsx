import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useBackupStore } from '../../Stores/BackupStore'
import { formatRelativeTime, toApiError } from '../../Utils'
import { SettingRow } from './Controls'
import styles from './SettingsModal.module.scss'

import type {
  WebDAVInput,
  OPMLBackupConfig,
  OPMLBackupInfo,
  OPMLImportResult,
} from '../../Types'

/** 一次操作后的反馈。ok 决定用普通色还是危险色。 */
interface Feedback {
  ok: boolean
  msg: string
  hint?: string
}

interface PendingBackupAction {
  id: string
  kind: 'restore' | 'delete'
}

/** 后端 time.Time 过 IPC 后是 ISO 字符串 */
function asTime(value: unknown): string | null {
  return typeof value === 'string' && value !== '' ? value : null
}

/** 格式化文件大小 */
function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function BackupSection(): JSX.Element {
  const { t } = useTranslation()

  // WebDAV 状态
  const webdavConfig = useBackupStore((s) => s.webdavConfig)
  const webdavLoading = useBackupStore((s) => s.webdavLoading)
  const webdavSaving = useBackupStore((s) => s.webdavSaving)
  const webdavTesting = useBackupStore((s) => s.webdavTesting)

  // OPML 状态
  const opmlConfig = useBackupStore((s) => s.opmlConfig)
  const opmlStatus = useBackupStore((s) => s.opmlStatus)
  const opmlBackups = useBackupStore((s) => s.opmlBackups)
  const opmlLoading = useBackupStore((s) => s.opmlLoading)
  const opmlSaving = useBackupStore((s) => s.opmlSaving)
  const opmlBacking = useBackupStore((s) => s.opmlBacking)
  const opmlRestoring = useBackupStore((s) => s.opmlRestoring)
  const opmlDeleting = useBackupStore((s) => s.opmlDeleting)
  const remotePath = useBackupStore((s) => s.remotePath)

  // WebDAV 表单状态
  const [url, setUrl] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [webdavFeedback, setWebdavFeedback] = useState<Feedback | null>(null)
  const [confirmClear, setConfirmClear] = useState(false)

  // OPML 配置状态。自动备份已移除，配置只剩「保留版本数」。
  const [retention, setRetention] = useState(7)
  const [opmlFeedback, setOpmlFeedback] = useState<Feedback | null>(null)
  const [pendingBackupAction, setPendingBackupAction] =
    useState<PendingBackupAction | null>(null)

  useEffect(() => {
    void useBackupStore.getState().load()
  }, [])

  // 同步 WebDAV 配置到表单
  useEffect(() => {
    setUrl(webdavConfig?.url ?? '')
    setUsername(webdavConfig?.username ?? '')
    setPassword('')
  }, [webdavConfig])

  // 同步 OPML 配置到表单
  useEffect(() => {
    if (opmlConfig) {
      setRetention(opmlConfig.retention)
    }
  }, [opmlConfig])

  function webdavFormValues(): WebDAVInput {
    return { url: url.trim(), username: username.trim(), password }
  }

  async function handleTestWebDAV(): Promise<void> {
    setWebdavFeedback(null)
    try {
      const res = await useBackupStore
        .getState()
        .testWebDAVConnection(webdavFormValues())
      if (res.ok) {
        setWebdavFeedback({
          ok: true,
          msg: t('settings.backup.webdav.testSuccess'),
        })
        return
      }
      const stepName = t(`settings.backup.webdav.steps.${res.step}`, {
        defaultValue: '',
      })
      setWebdavFeedback({
        ok: false,
        msg: `${
          stepName
            ? t('settings.backup.webdav.stepFailed', { step: stepName })
            : t('settings.backup.webdav.testFailed')
        }：${res.message}`,
        hint: res.hint,
      })
    } catch (err) {
      setWebdavFeedback({ ok: false, msg: toApiError(err) })
    }
  }

  async function handleSaveWebDAV(): Promise<void> {
    setWebdavFeedback(null)
    try {
      await useBackupStore.getState().saveWebDAVConfig(webdavFormValues())
      setWebdavFeedback({ ok: true, msg: t('settings.backup.webdav.saved') })
    } catch (err) {
      setWebdavFeedback({
        ok: false,
        msg: `${t('settings.backup.webdav.saveError')}：${toApiError(err)}`,
      })
    }
  }

  async function handleClearWebDAV(): Promise<void> {
    setConfirmClear(false)
    setWebdavFeedback(null)
    try {
      await useBackupStore.getState().clearWebDAVConfig()
      setWebdavFeedback({ ok: true, msg: t('settings.backup.webdav.cleared') })
    } catch (err) {
      setWebdavFeedback({
        ok: false,
        msg: `${t('settings.backup.webdav.clearError')}：${toApiError(err)}`,
      })
    }
  }

  async function handleSaveOPMLConfig(): Promise<void> {
    setOpmlFeedback(null)
    try {
      const config: OPMLBackupConfig = { retention }
      await useBackupStore.getState().saveOPMLConfig(config)
      setOpmlFeedback({ ok: true, msg: t('settings.backup.opml.saved') })
    } catch (err) {
      setOpmlFeedback({
        ok: false,
        msg: `${t('settings.backup.opml.saveError')}：${toApiError(err)}`,
      })
    }
  }

  async function handleBackupNow(): Promise<void> {
    setOpmlFeedback(null)
    try {
      const info = await useBackupStore.getState().backupOPML()
      setOpmlFeedback({
        ok: true,
        msg: t('settings.backup.opml.backupSuccess', {
          size: formatSize(info.size),
        }),
      })
    } catch (err) {
      setOpmlFeedback({ ok: false, msg: toApiError(err) })
    }
  }

  async function handleRefreshBackups(): Promise<void> {
    setOpmlFeedback(null)
    try {
      await useBackupStore.getState().listOPMLBackups()
    } catch (err) {
      setOpmlFeedback({ ok: false, msg: toApiError(err) })
    }
  }

  async function handleRestore(id: string): Promise<void> {
    setPendingBackupAction(null)
    setOpmlFeedback(null)
    try {
      const result = await useBackupStore.getState().restoreOPML(id)
      setOpmlFeedback({
        ok: true,
        msg: t('settings.backup.opml.restoreSuccess', {
          feeds: result.Feeds,
          categories: result.Categories,
        }),
      })
    } catch (err) {
      setOpmlFeedback({
        ok: false,
        msg: `${t('settings.backup.opml.restoreError')}：${toApiError(err)}`,
      })
    }
  }

  async function handleDeleteBackup(id: string): Promise<void> {
    setOpmlFeedback(null)
    try {
      await useBackupStore.getState().deleteOPMLBackup(id)
      setOpmlFeedback({
        ok: true,
        msg: t('settings.backup.opml.deleteSuccess'),
      })
    } catch (err) {
      setOpmlFeedback({
        ok: false,
        msg: `${t('settings.backup.opml.deleteError')}：${toApiError(err)}`,
      })
    } finally {
      setPendingBackupAction(null)
    }
  }

  const busy =
    webdavSaving ||
    webdavTesting ||
    opmlSaving ||
    opmlBacking ||
    opmlRestoring ||
    opmlDeleting !== null
  const configured =
    Boolean(webdavConfig?.url) || Boolean(webdavConfig?.hasPassword)
  const lastBackupAt = asTime(opmlStatus?.lastBackupAt)

  return (
    <div>
      {/* WebDAV 配置 */}
      <h3 className={styles.sectionTitle}>
        {t('settings.backup.webdav.title')}
      </h3>
      <p className={styles.sectionIntro}>{t('settings.backup.intro')}</p>

      <SettingRow
        label={t('settings.backup.webdav.url')}
        description={t('settings.backup.webdav.urlDesc')}
      >
        <input
          className={styles.input}
          type="url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="https://dav.example.com/dav/"
          spellCheck={false}
          autoComplete="off"
        />
      </SettingRow>

      <SettingRow label={t('settings.backup.webdav.username')}>
        <input
          className={styles.input}
          type="text"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          spellCheck={false}
          autoComplete="off"
        />
      </SettingRow>

      <SettingRow
        label={t('settings.backup.webdav.password')}
        description={t('settings.backup.webdav.passwordDesc')}
      >
        <input
          className={styles.input}
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder={
            webdavConfig?.hasPassword
              ? t('settings.backup.webdav.passwordKeep')
              : t('settings.backup.webdav.passwordPlaceholder')
          }
          autoComplete="new-password"
        />
      </SettingRow>

      {url.trim() && remotePath ? (
        <p className={styles.syncNote}>
          {t('settings.backup.opml.pathNote', {
            path: `${url.trim().replace(/\/+$/, '')}/${remotePath}`,
          })}
        </p>
      ) : null}

      <div className={styles.btnRow}>
        <button
          type="button"
          className={styles.btn}
          onClick={handleTestWebDAV}
          disabled={busy}
        >
          {webdavTesting
            ? t('settings.backup.webdav.testing')
            : t('settings.backup.webdav.test')}
        </button>
        <button
          type="button"
          className={`${styles.btn} ${styles.btnPrimary}`}
          onClick={handleSaveWebDAV}
          disabled={busy}
        >
          {webdavSaving
            ? t('settings.backup.webdav.saving')
            : t('settings.backup.webdav.save')}
        </button>
      </div>

      {webdavFeedback ? (
        <p
          className={`${styles.feedback} ${webdavFeedback.ok ? '' : styles.feedbackError}`}
        >
          {webdavFeedback.msg}
          {webdavFeedback.hint ? (
            <span className={styles.feedbackHint}>{webdavFeedback.hint}</span>
          ) : null}
        </p>
      ) : null}

      {configured ? (
        <SettingRow
          label={t('settings.backup.webdav.clear')}
          description={t('settings.backup.webdav.clearDesc')}
        >
          {confirmClear ? (
            <div className={styles.btnGroup}>
              <button
                type="button"
                className={`${styles.btn} ${styles.btnDanger}`}
                onClick={handleClearWebDAV}
                disabled={busy}
              >
                {t('settings.backup.webdav.clearConfirm')}
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
              {t('settings.backup.webdav.clear')}…
            </button>
          )}
        </SettingRow>
      ) : null}

      {/* OPML 云备份 */}
      <h3 className={`${styles.sectionTitle} ${styles.sectionTitleSpaced}`}>
        {t('settings.backup.opml.title')}
      </h3>
      <p className={styles.sectionIntro}>{t('settings.backup.opml.intro')}</p>

      {!configured ? (
        <p className={`${styles.feedback} ${styles.feedbackError}`}>
          {t('settings.backup.opml.unavailable')}
        </p>
      ) : (
        <>
          <SettingRow
            label={t('settings.backup.opml.retention')}
            description={t('settings.backup.opml.retentionDesc')}
          >
            <input
              className={styles.input}
              type="number"
              value={retention}
              onChange={(e) => setRetention(parseInt(e.target.value, 10) || 7)}
              min="1"
              max="30"
            />
          </SettingRow>

          <div className={styles.btnRow}>
            <button
              type="button"
              className={`${styles.btn} ${styles.btnPrimary}`}
              onClick={handleSaveOPMLConfig}
              disabled={busy}
            >
              {opmlSaving
                ? t('settings.backup.opml.saving')
                : t('settings.backup.opml.save')}
            </button>
          </div>

          <SettingRow label={t('settings.backup.opml.lastBackup')}>
            <span className={styles.syncStatusValue}>
              {webdavLoading
                ? t('settings.data.loading')
                : lastBackupAt
                  ? formatRelativeTime(lastBackupAt)
                  : t('settings.backup.opml.never')}
            </span>
          </SettingRow>

          {opmlStatus?.lastError ? (
            <p className={`${styles.feedback} ${styles.feedbackError}`}>
              {t('settings.backup.opml.lastError', {
                message: opmlStatus.lastError,
              })}
            </p>
          ) : null}

          <SettingRow label={t('settings.backup.opml.backupNow')}>
            <button
              type="button"
              className={styles.btn}
              onClick={handleBackupNow}
              disabled={busy}
            >
              {opmlBacking
                ? t('settings.backup.opml.backingUp')
                : t('settings.backup.opml.backupNow')}
            </button>
          </SettingRow>

          {opmlFeedback ? (
            <p
              className={`${styles.feedback} ${opmlFeedback.ok ? '' : styles.feedbackError}`}
            >
              {opmlFeedback.msg}
            </p>
          ) : null}

          {/* 备份历史 */}
          <h4 className={styles.sectionSubtitle}>
            {t('settings.backup.opml.history')}
          </h4>
          <div className={styles.btnRow}>
            <button
              type="button"
              className={styles.btn}
              onClick={handleRefreshBackups}
              disabled={busy}
            >
              {opmlLoading
                ? t('settings.backup.opml.refreshing')
                : t('settings.backup.opml.refresh')}
            </button>
          </div>

          {opmlBackups.length === 0 ? (
            <p className={styles.syncNote}>
              {t('settings.backup.opml.historyEmpty')}
            </p>
          ) : (
            <div className={styles.backupList}>
              {opmlBackups.map((backup) => (
                <BackupItem
                  key={backup.id}
                  backup={backup}
                  busy={busy}
                  pendingAction={pendingBackupAction}
                  deleting={opmlDeleting === backup.id}
                  onRestore={handleRestore}
                  onDelete={handleDeleteBackup}
                  onConfirm={setPendingBackupAction}
                />
              ))}
            </div>
          )}
        </>
      )}
    </div>
  )
}

/** 单个备份项 */
function BackupItem(props: {
  backup: OPMLBackupInfo
  busy: boolean
  pendingAction: PendingBackupAction | null
  deleting: boolean
  onRestore: (id: string) => void
  onDelete: (id: string) => void
  onConfirm: (action: PendingBackupAction | null) => void
}): JSX.Element {
  const { t } = useTranslation()
  const {
    backup,
    busy,
    pendingAction,
    deleting,
    onRestore,
    onDelete,
    onConfirm,
  } = props
  const createdAt = asTime(backup.createdAt)
  const activeAction =
    pendingAction?.id === backup.id ? pendingAction.kind : null

  return (
    <div className={styles.backupItem}>
      <div className={styles.backupInfo}>
        <div className={styles.backupTime}>
          {createdAt
            ? formatRelativeTime(createdAt)
            : t('settings.backup.opml.unknownTime')}
        </div>
        <div className={styles.backupMeta}>
          {backup.deviceName} · {formatSize(backup.size)}
        </div>
      </div>
      <div className={styles.backupActions}>
        {activeAction ? (
          <>
            <button
              type="button"
              className={`${styles.btn} ${styles.btnDanger}`}
              onClick={() =>
                activeAction === 'restore'
                  ? onRestore(backup.id)
                  : onDelete(backup.id)
              }
              disabled={busy}
              title={t(
                activeAction === 'restore'
                  ? 'settings.backup.opml.restoreWarning'
                  : 'settings.backup.opml.deleteWarning',
              )}
            >
              {deleting
                ? t('settings.backup.opml.deleting')
                : t(
                    activeAction === 'restore'
                      ? 'settings.backup.opml.restoreConfirm'
                      : 'settings.backup.opml.deleteConfirm',
                  )}
            </button>
            <button
              type="button"
              className={styles.btn}
              onClick={() => onConfirm(null)}
              disabled={busy}
            >
              {t('confirm.cancel')}
            </button>
          </>
        ) : (
          <>
            <button
              type="button"
              className={styles.btn}
              onClick={() => onConfirm({ id: backup.id, kind: 'restore' })}
              disabled={busy}
            >
              {t('settings.backup.opml.restore')}
            </button>
            <button
              type="button"
              className={`${styles.btn} ${styles.btnDanger}`}
              onClick={() => onConfirm({ id: backup.id, kind: 'delete' })}
              disabled={busy}
            >
              {t('settings.backup.opml.delete')}
            </button>
          </>
        )}
      </div>
    </div>
  )
}
