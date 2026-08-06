import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useSyncStore } from '../../Stores'
import { formatRelativeTime, toApiError } from '../../Utils'
import type { ConflictInfo, WebDAVInput } from '../../Types'
import { SettingRow, Toggle } from './Controls'
import { CloudBackupPanel } from './CloudBackupPanel'
import styles from './SettingsModal.module.scss'

/** 一次操作后的反馈。ok 决定用普通色还是危险色。 */
interface Feedback {
  ok: boolean
  msg: string
  /** 可操作建议，与 msg 分行展示（后端 ConnectionTestResult.hint）。 */
  hint?: string
}

/** 后端 time.Time 过 IPC 后是 ISO 字符串；绑定里的类型是 any。 */
function asTime(value: unknown): string | null {
  return typeof value === 'string' && value !== '' ? value : null
}

export function SyncSection(): JSX.Element {
  const { t } = useTranslation()
  const config = useSyncStore((s) => s.config)
  const status = useSyncStore((s) => s.status)
  const remotePath = useSyncStore((s) => s.remotePath)
  const loading = useSyncStore((s) => s.loading)
  const saving = useSyncStore((s) => s.saving)
  const syncing = useSyncStore((s) => s.syncing)

  // 表单是本地状态，保存时才提交 —— 与 ProxySection 一致。
  // 开关也算表单的一部分：先开开关再填地址是很常见的顺序，若开关即刻生效，
  // 用户会在填完地址之前先撞上一条「地址必须以 https:// 开头」。
  const [enabled, setEnabled] = useState(false)
  const [url, setUrl] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [testing, setTesting] = useState(false)
  const [confirmRemove, setConfirmRemove] = useState(false)
  const [feedback, setFeedback] = useState<Feedback | null>(null)

  useEffect(() => {
    void useSyncStore.getState().load()
  }, [])

  // 后端配置变化后重置表单。密码框始终留空：后端从不回传密码，
  // 而空串对它的含义正是「保持原密码不变」。
  useEffect(() => {
    setEnabled(config?.enabled ?? false)
    setUrl(config?.url ?? '')
    setUsername(config?.username ?? '')
    setPassword('')
  }, [config])

  function formValues(): WebDAVInput {
    return { enabled, url: url.trim(), username: username.trim(), password }
  }

  async function handleTest(): Promise<void> {
    setTesting(true)
    setFeedback(null)
    try {
      const res = await useSyncStore.getState().test(formValues())
      if (res.ok) {
        setFeedback({ ok: true, msg: t('settings.sync.testSuccess') })
        return
      }
      // step 取 connect / mkcol / write，三个失败原因完全不同的阶段。
      // 认不出的步骤（后端日后新增）回落到笼统的「连接失败」。
      const stepName = t(`settings.sync.steps.${res.step}`, {
        defaultValue: '',
      })
      setFeedback({
        ok: false,
        msg: `${
          stepName
            ? t('settings.sync.stepFailed', { step: stepName })
            : t('settings.sync.testFailed')
        }：${res.message}`,
        hint: res.hint,
      })
    } catch (err) {
      // 走到这里只有「凭据存储不可用」这类前置错误。
      setFeedback({ ok: false, msg: toApiError(err) })
    } finally {
      setTesting(false)
    }
  }

  async function handleSave(): Promise<void> {
    setFeedback(null)
    try {
      await useSyncStore.getState().save(formValues())
      setFeedback({ ok: true, msg: t('settings.sync.saved') })
    } catch (err) {
      setFeedback({
        ok: false,
        msg: `${t('settings.sync.saveError')}：${toApiError(err)}`,
      })
    }
  }

  async function handleSyncNow(): Promise<void> {
    setFeedback(null)
    try {
      const res = await useSyncStore.getState().syncNow()
      // 冲突不是错误，也不在这里报 —— 下方冲突区会渲染出来让用户裁决。
      if (res.action !== 'conflict') {
        setFeedback({ ok: true, msg: t(`settings.sync.result.${res.action}`) })
      }
    } catch (err) {
      setFeedback({ ok: false, msg: toApiError(err) })
    }
  }

  async function handleResolve(keepLocal: boolean): Promise<void> {
    setFeedback(null)
    try {
      await useSyncStore.getState().resolve(keepLocal)
      setFeedback({ ok: true, msg: t('settings.sync.conflict.resolved') })
    } catch (err) {
      setFeedback({ ok: false, msg: toApiError(err) })
    }
  }

  async function handleRemove(): Promise<void> {
    setConfirmRemove(false)
    setFeedback(null)
    try {
      await useSyncStore.getState().clear()
      setFeedback({ ok: true, msg: t('settings.sync.removed') })
    } catch (err) {
      setFeedback({
        ok: false,
        msg: `${t('settings.sync.removeError')}：${toApiError(err)}`,
      })
    }
  }

  const busy = saving || syncing || testing
  const conflict = status?.conflict ?? null
  const configured = Boolean(config?.url) || Boolean(config?.hasPassword)
  const lastSyncAt = asTime(status?.lastSyncAt)

  return (
    <div>
      <h3 className={styles.sectionTitle}>{t('settings.sync.title')}</h3>
      <p className={styles.sectionIntro}>{t('settings.sync.intro')}</p>

      <SettingRow
        label={t('settings.sync.enabled')}
        description={t('settings.sync.enabledDesc')}
      >
        <Toggle
          checked={enabled}
          onChange={setEnabled}
          label={t('settings.sync.enabled')}
        />
      </SettingRow>

      <SettingRow
        label={t('settings.sync.url')}
        description={t('settings.sync.urlDesc')}
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

      <SettingRow label={t('settings.sync.username')}>
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
        label={t('settings.sync.password')}
        description={t('settings.sync.passwordDesc')}
      >
        <input
          className={styles.input}
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder={
            config?.hasPassword
              ? t('settings.sync.passwordKeep')
              : t('settings.sync.passwordPlaceholder')
          }
          autoComplete="new-password"
        />
      </SettingRow>

      {url.trim() && remotePath ? (
        <p className={styles.syncNote}>
          {t('settings.sync.pathNote', {
            path: `${url.trim().replace(/\/+$/, '')}/${remotePath}`,
          })}
        </p>
      ) : null}
      <p className={styles.syncNote}>{t('settings.sync.securityNote')}</p>
      <p className={styles.syncNote}>{t('settings.sync.syncedItems')}</p>

      <div className={styles.btnRow}>
        <button
          type="button"
          className={styles.btn}
          onClick={handleTest}
          disabled={busy}
        >
          {testing ? t('settings.sync.testing') : t('settings.sync.test')}
        </button>
        <button
          type="button"
          className={`${styles.btn} ${styles.btnPrimary}`}
          onClick={handleSave}
          disabled={busy}
        >
          {saving ? t('settings.sync.saving') : t('settings.sync.save')}
        </button>
      </div>

      {feedback ? (
        <p
          className={`${styles.feedback} ${feedback.ok ? '' : styles.feedbackError}`}
        >
          {feedback.msg}
          {feedback.hint ? (
            <span className={styles.feedbackHint}>{feedback.hint}</span>
          ) : null}
        </p>
      ) : null}

      <h3 className={`${styles.sectionTitle} ${styles.sectionTitleSpaced}`}>
        {t('settings.sync.statusTitle')}
      </h3>

      <SettingRow
        label={t('settings.sync.lastSync')}
        description={
          status?.deviceName
            ? t('settings.sync.thisDevice', { name: status.deviceName })
            : undefined
        }
      >
        <span className={styles.syncStatusValue}>
          {loading
            ? t('settings.data.loading')
            : lastSyncAt
              ? formatRelativeTime(lastSyncAt)
              : t('settings.sync.never')}
        </span>
      </SettingRow>

      {/* 未配置时不谈「有改动待推送」：没有远端可推，说了只会让人困惑。 */}
      {configured ? (
        <SettingRow
          label={
            status?.hasPending
              ? t('settings.sync.pending')
              : t('settings.sync.upToDate')
          }
        >
          <button
            type="button"
            className={styles.btn}
            onClick={handleSyncNow}
            disabled={busy || !config?.enabled}
          >
            {syncing
              ? t('settings.sync.syncingNow')
              : t('settings.sync.syncNow')}
          </button>
        </SettingRow>
      ) : null}

      {/* 冲突时不显示上次错误：冲突区已经把当前该做的事讲清楚了。 */}
      {status?.lastError && !conflict ? (
        <p className={`${styles.feedback} ${styles.feedbackError}`}>
          {t('settings.sync.lastError', { message: status.lastError })}
        </p>
      ) : null}

      {conflict ? (
        <ConflictPanel
          conflict={conflict}
          localDevice={status?.deviceName ?? ''}
          busy={busy}
          resolving={syncing}
          onResolve={handleResolve}
        />
      ) : null}

      {configured ? (
        <SettingRow
          label={t('settings.sync.remove')}
          description={t('settings.sync.removeDesc')}
        >
          {confirmRemove ? (
            <div className={styles.btnGroup}>
              <button
                type="button"
                className={`${styles.btn} ${styles.btnDanger}`}
                onClick={handleRemove}
                disabled={busy}
              >
                {t('settings.sync.removeConfirm')}
              </button>
              <button
                type="button"
                className={styles.btn}
                onClick={() => setConfirmRemove(false)}
              >
                {t('confirm.cancel')}
              </button>
            </div>
          ) : (
            <button
              type="button"
              className={styles.btn}
              onClick={() => setConfirmRemove(true)}
              disabled={busy}
            >
              {t('settings.sync.remove')}…
            </button>
          )}
        </SettingRow>
      ) : null}

      <CloudBackupPanel
        available={Boolean(config?.url && config?.hasPassword)}
        baseUrl={config?.url ?? ''}
      />
    </div>
  )
}

/** 冲突裁决：两侧摘要 + 二选一。不自动决定，也不做字段级合并。 */
function ConflictPanel(props: {
  conflict: ConflictInfo
  localDevice: string
  busy: boolean
  resolving: boolean
  onResolve: (keepLocal: boolean) => void
}): JSX.Element {
  const { t } = useTranslation()
  const { conflict, localDevice, busy, resolving, onResolve } = props
  const remoteUpdatedAt = asTime(conflict.remoteUpdatedAt)

  return (
    <div className={styles.conflictBox}>
      <h4 className={styles.conflictTitle}>
        {t('settings.sync.conflict.title')}
      </h4>
      <p className={styles.conflictDesc}>{t('settings.sync.conflict.desc')}</p>

      <div className={styles.conflictSides}>
        <div className={styles.conflictSide}>
          <span className={styles.conflictSideLabel}>
            {t('settings.sync.conflict.localSide')}
          </span>
          <span className={styles.conflictSideMeta}>
            {localDevice || t('settings.sync.conflict.unknownDevice')}
          </span>
          <span className={styles.conflictSideMeta}>
            {t('settings.sync.conflict.localCurrent')}
          </span>
        </div>
        <div className={styles.conflictSide}>
          <span className={styles.conflictSideLabel}>
            {t('settings.sync.conflict.remoteSide')}
          </span>
          <span className={styles.conflictSideMeta}>
            {conflict.remoteDeviceName ||
              t('settings.sync.conflict.unknownDevice')}
          </span>
          <span className={styles.conflictSideMeta}>
            {remoteUpdatedAt
              ? t('settings.sync.conflict.updatedAt', {
                  time: formatRelativeTime(remoteUpdatedAt),
                })
              : ''}
          </span>
        </div>
      </div>

      <div className={styles.btnRow}>
        <button
          type="button"
          className={styles.btn}
          onClick={() => onResolve(true)}
          disabled={busy}
        >
          {resolving
            ? t('settings.sync.conflict.resolving')
            : t('settings.sync.conflict.keepLocal')}
        </button>
        <button
          type="button"
          className={styles.btn}
          onClick={() => onResolve(false)}
          disabled={busy}
        >
          {t('settings.sync.conflict.keepRemote')}
        </button>
      </div>
    </div>
  )
}
