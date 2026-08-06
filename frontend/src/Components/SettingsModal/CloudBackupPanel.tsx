import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useCloudBackupStore } from '../../Stores'
import { formatRelativeTime, toApiError } from '../../Utils'
import type { CloudBackupConfig, CloudBackupInfo } from '../../Types'
import { SegmentedControl, SettingRow, Toggle } from './Controls'
import styles from './SettingsModal.module.scss'

interface Feedback {
  ok: boolean
  msg: string
}

type BackupInterval = 'daily' | 'weekly'

function asTime(value: unknown): string | null {
  return typeof value === 'string' && value !== '' ? value : null
}

function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '0 MB'
  const mb = value / (1024 * 1024)
  if (mb < 1024) return `${mb.toFixed(mb >= 100 ? 0 : 1)} MB`
  return `${(mb / 1024).toFixed(1)} GB`
}

function formatScheduledTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

export function CloudBackupPanel(props: {
  available: boolean
  baseUrl: string
}): JSX.Element {
  const { available, baseUrl } = props
  const { t } = useTranslation()
  const config = useCloudBackupStore((s) => s.config)
  const status = useCloudBackupStore((s) => s.status)
  const backups = useCloudBackupStore((s) => s.backups)
  const remotePath = useCloudBackupStore((s) => s.remotePath)
  const loading = useCloudBackupStore((s) => s.loading)
  const loadingHistory = useCloudBackupStore((s) => s.loadingHistory)
  const saving = useCloudBackupStore((s) => s.saving)
  const backingUp = useCloudBackupStore((s) => s.backingUp)
  const restoringId = useCloudBackupStore((s) => s.restoringId)

  const [enabled, setEnabled] = useState(false)
  const [interval, setInterval] = useState<BackupInterval>('daily')
  const [retention, setRetention] = useState<3 | 5 | 10>(5)
  const [confirmRestore, setConfirmRestore] = useState<string | null>(null)
  const [feedback, setFeedback] = useState<Feedback | null>(null)

  useEffect(() => {
    void useCloudBackupStore.getState().load()
  }, [])

  useEffect(() => {
    setEnabled(config?.enabled ?? false)
    setInterval(config?.interval === 'weekly' ? 'weekly' : 'daily')
    setRetention(
      config?.retention === 3 || config?.retention === 10
        ? config.retention
        : 5,
    )
  }, [config])

  useEffect(() => {
    if (!available) return
    void useCloudBackupStore
      .getState()
      .refreshHistory()
      .catch(() => undefined)
  }, [available])

  function formValues(): CloudBackupConfig {
    return { enabled, interval, retention }
  }

  async function handleSave(): Promise<void> {
    setFeedback(null)
    try {
      await useCloudBackupStore.getState().save(formValues())
      setFeedback({ ok: true, msg: t('settings.sync.cloudBackup.saved') })
    } catch (err) {
      setFeedback({
        ok: false,
        msg: `${t('settings.sync.cloudBackup.saveError')}：${toApiError(err)}`,
      })
    }
  }

  async function handleBackup(): Promise<void> {
    setFeedback(null)
    try {
      const info = await useCloudBackupStore.getState().backupNow()
      setFeedback({
        ok: true,
        msg: t('settings.sync.cloudBackup.backupSuccess', {
          size: formatBytes(info.size),
        }),
      })
    } catch (err) {
      setFeedback({ ok: false, msg: toApiError(err) })
    }
  }

  async function handleRefresh(): Promise<void> {
    setFeedback(null)
    try {
      await useCloudBackupStore.getState().refreshHistory()
    } catch (err) {
      setFeedback({ ok: false, msg: toApiError(err) })
    }
  }

  async function handleRestore(info: CloudBackupInfo): Promise<void> {
    setConfirmRestore(null)
    setFeedback(null)
    try {
      const result = await useCloudBackupStore.getState().restore(info.id)
      setFeedback({
        ok: true,
        msg: t('settings.sync.cloudBackup.restoreSuccess', {
          path: result.rollbackPath,
        }),
      })
    } catch (err) {
      setFeedback({
        ok: false,
        msg: `${t('settings.sync.cloudBackup.restoreError')}：${toApiError(err)}`,
      })
    }
  }

  const busy = saving || backingUp || restoringId !== null
  const lastBackupAt = asTime(status?.lastBackupAt)
  const nextBackupAt = asTime(status?.nextBackupAt)
  const fullPath =
    baseUrl && remotePath
      ? `${baseUrl.replace(/\/+$/, '')}/${remotePath}`
      : remotePath

  return (
    <section>
      <h3 className={`${styles.sectionTitle} ${styles.sectionTitleSpaced}`}>
        {t('settings.sync.cloudBackup.title')}
      </h3>
      <p className={styles.sectionIntro}>
        {t('settings.sync.cloudBackup.intro')}
      </p>

      <SettingRow
        label={t('settings.sync.cloudBackup.enabled')}
        description={t('settings.sync.cloudBackup.enabledDesc')}
      >
        <Toggle
          checked={enabled}
          onChange={setEnabled}
          label={t('settings.sync.cloudBackup.enabled')}
        />
      </SettingRow>

      <SettingRow
        label={t('settings.sync.cloudBackup.interval')}
        description={t('settings.sync.cloudBackup.intervalDesc')}
      >
        <SegmentedControl
          value={interval}
          options={[
            { value: 'daily', label: t('settings.sync.cloudBackup.daily') },
            { value: 'weekly', label: t('settings.sync.cloudBackup.weekly') },
          ]}
          onChange={setInterval}
        />
      </SettingRow>

      <SettingRow
        label={t('settings.sync.cloudBackup.retention')}
        description={t('settings.sync.cloudBackup.retentionDesc')}
      >
        <SegmentedControl
          value={retention}
          options={([3, 5, 10] as const).map((value) => ({
            value,
            label: t('settings.sync.cloudBackup.retentionCount', {
              count: value,
            }),
          }))}
          onChange={setRetention}
        />
      </SettingRow>

      <p className={styles.syncNote}>
        {available
          ? t('settings.sync.cloudBackup.pathNote', { path: fullPath })
          : t('settings.sync.cloudBackup.unavailable')}
      </p>
      <p className={styles.syncNote}>
        {t('settings.sync.cloudBackup.securityNote')}
      </p>

      <div className={styles.btnRow}>
        <button
          type="button"
          className={styles.btn}
          onClick={handleSave}
          disabled={busy || (enabled && !available)}
        >
          {saving
            ? t('settings.sync.cloudBackup.saving')
            : t('settings.sync.cloudBackup.save')}
        </button>
        <button
          type="button"
          className={`${styles.btn} ${styles.btnPrimary}`}
          onClick={handleBackup}
          disabled={busy || !available}
        >
          {backingUp
            ? t('settings.sync.cloudBackup.backingUp')
            : t('settings.sync.cloudBackup.backupNow')}
        </button>
      </div>

      <div className={styles.cloudBackupStatus}>
        <SettingRow label={t('settings.sync.cloudBackup.lastBackup')}>
          <span className={styles.syncStatusValue}>
            {loading
              ? t('settings.data.loading')
              : lastBackupAt
                ? formatRelativeTime(lastBackupAt)
                : t('settings.sync.cloudBackup.never')}
          </span>
        </SettingRow>
        {enabled ? (
          <SettingRow label={t('settings.sync.cloudBackup.nextBackup')}>
            <span className={styles.syncStatusValue}>
              {nextBackupAt
                ? formatScheduledTime(nextBackupAt)
                : t('settings.sync.cloudBackup.pendingSchedule')}
            </span>
          </SettingRow>
        ) : null}
      </div>

      {status?.lastError ? (
        <p className={`${styles.feedback} ${styles.feedbackError}`}>
          {t('settings.sync.cloudBackup.lastError', {
            message: status.lastError,
          })}
        </p>
      ) : null}

      {feedback ? (
        <p
          className={`${styles.feedback} ${feedback.ok ? '' : styles.feedbackError}`}
        >
          {feedback.msg}
        </p>
      ) : null}

      <div className={styles.cloudBackupHistoryHeader}>
        <h4 className={styles.cloudBackupHistoryTitle}>
          {t('settings.sync.cloudBackup.history')}
        </h4>
        <button
          type="button"
          className={styles.btn}
          onClick={handleRefresh}
          disabled={busy || loadingHistory || !available}
        >
          {loadingHistory
            ? t('settings.sync.cloudBackup.refreshing')
            : t('settings.sync.cloudBackup.refresh')}
        </button>
      </div>

      {backups.length === 0 ? (
        <p className={styles.cloudBackupEmpty}>
          {loadingHistory
            ? t('settings.data.loading')
            : t('settings.sync.cloudBackup.historyEmpty')}
        </p>
      ) : (
        <div className={styles.cloudBackupList}>
          {backups.map((info) => {
            const createdAt = asTime(info.createdAt)
            const confirming = confirmRestore === info.id
            const restoring = restoringId === info.id
            return (
              <div className={styles.cloudBackupItem} key={info.id}>
                <div className={styles.cloudBackupItemText}>
                  <span className={styles.cloudBackupItemTitle}>
                    {createdAt
                      ? formatRelativeTime(createdAt)
                      : t('settings.sync.cloudBackup.unknownTime')}
                  </span>
                  <span className={styles.cloudBackupItemMeta}>
                    {t('settings.sync.cloudBackup.itemMeta', {
                      device:
                        info.deviceName ||
                        t('settings.sync.conflict.unknownDevice'),
                      size: formatBytes(info.size),
                    })}
                  </span>
                </div>
                {confirming ? (
                  <div className={styles.cloudBackupRestoreConfirm}>
                    <span className={styles.cloudBackupRestoreWarning}>
                      {t('settings.sync.cloudBackup.restoreWarning')}
                    </span>
                    <div className={styles.btnGroup}>
                      <button
                        type="button"
                        className={`${styles.btn} ${styles.btnDanger}`}
                        onClick={() => handleRestore(info)}
                        disabled={busy}
                      >
                        {t('settings.sync.cloudBackup.restoreConfirm')}
                      </button>
                      <button
                        type="button"
                        className={styles.btn}
                        onClick={() => setConfirmRestore(null)}
                      >
                        {t('confirm.cancel')}
                      </button>
                    </div>
                  </div>
                ) : (
                  <button
                    type="button"
                    className={styles.btn}
                    onClick={() => setConfirmRestore(info.id)}
                    disabled={busy}
                  >
                    {restoring
                      ? t('settings.sync.cloudBackup.restoring')
                      : t('settings.sync.cloudBackup.restore')}
                  </button>
                )}
              </div>
            )
          })}
        </div>
      )}
    </section>
  )
}
