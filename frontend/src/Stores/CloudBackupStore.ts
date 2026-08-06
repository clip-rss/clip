import { create } from 'zustand'
import { CloudBackupService, toApiError } from '../Utils'
import type {
  CloudBackupConfig,
  CloudBackupInfo,
  CloudBackupStatus,
  CloudRestoreResult,
} from '../Types'

interface CloudBackupState {
  config: CloudBackupConfig | null
  status: CloudBackupStatus | null
  backups: CloudBackupInfo[]
  remotePath: string

  loading: boolean
  loadingHistory: boolean
  saving: boolean
  backingUp: boolean
  restoringId: string | null
  error: string | null

  load: () => Promise<void>
  refreshHistory: () => Promise<void>
  save: (config: CloudBackupConfig) => Promise<void>
  backupNow: () => Promise<CloudBackupInfo>
  restore: (id: string) => Promise<CloudRestoreResult>
}

export const useCloudBackupStore = create<CloudBackupState>()((set, get) => ({
  config: null,
  status: null,
  backups: [],
  remotePath: '',
  loading: false,
  loadingHistory: false,
  saving: false,
  backingUp: false,
  restoringId: null,
  error: null,

  async load() {
    set({ loading: true, error: null })
    const [config, status, path] = await Promise.allSettled([
      CloudBackupService.GetCloudBackupConfig(),
      CloudBackupService.GetCloudBackupStatus(),
      CloudBackupService.CloudBackupRemotePath(),
    ])
    set({
      config: config.status === 'fulfilled' ? config.value : null,
      status: status.status === 'fulfilled' ? status.value : null,
      remotePath: path.status === 'fulfilled' ? path.value : '',
      loading: false,
      error:
        config.status === 'rejected'
          ? toApiError(config.reason)
          : status.status === 'rejected'
            ? toApiError(status.reason)
            : null,
    })
  },

  async refreshHistory() {
    set({ loadingHistory: true, error: null })
    try {
      const backups = await CloudBackupService.ListCloudBackups()
      set({ backups, loadingHistory: false })
    } catch (err) {
      set({ loadingHistory: false, error: toApiError(err) })
      throw err
    }
  },

  async save(config) {
    set({ saving: true, error: null })
    try {
      await CloudBackupService.SaveCloudBackupConfig(config)
    } catch (err) {
      set({ saving: false, error: toApiError(err) })
      throw err
    }
    set({ saving: false })
    await get().load()
  },

  async backupNow() {
    set({ backingUp: true, error: null })
    let info: CloudBackupInfo
    try {
      info = await CloudBackupService.BackupDatabaseToCloud()
    } catch (err) {
      set({ backingUp: false, error: toApiError(err) })
      await refreshStatus(set)
      throw err
    }
    set({ backingUp: false })
    await Promise.all([refreshStatus(set), get().refreshHistory()])
    return info
  },

  async restore(id) {
    set({ restoringId: id, error: null })
    let result: CloudRestoreResult
    try {
      result = await CloudBackupService.RestoreDatabaseFromCloud(id)
    } catch (err) {
      set({ restoringId: null, error: toApiError(err) })
      await refreshStatus(set)
      throw err
    }
    set({ restoringId: null })
    await refreshStatus(set)
    return result
  },
}))

async function refreshStatus(
  set: (partial: Partial<CloudBackupState>) => void,
): Promise<void> {
  try {
    set({ status: await CloudBackupService.GetCloudBackupStatus() })
  } catch {
    // 保留原始操作错误；状态刷新失败不应覆盖更具体的原因。
  }
}
