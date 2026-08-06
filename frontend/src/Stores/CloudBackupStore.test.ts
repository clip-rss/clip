import { beforeEach, describe, expect, it, vi, type Mock } from 'vitest'

vi.mock('../Utils', () => ({
  CloudBackupService: {
    GetCloudBackupConfig: vi.fn(),
    GetCloudBackupStatus: vi.fn(),
    CloudBackupRemotePath: vi.fn(),
    ListCloudBackups: vi.fn(),
    SaveCloudBackupConfig: vi.fn(),
    BackupDatabaseToCloud: vi.fn(),
    RestoreDatabaseFromCloud: vi.fn(),
  },
  toApiError: (e: unknown) => String(e),
}))

import { CloudBackupService } from '../Utils'
import { useCloudBackupStore } from './CloudBackupStore'

const GetConfig = CloudBackupService.GetCloudBackupConfig as Mock
const GetStatus = CloudBackupService.GetCloudBackupStatus as Mock
const GetPath = CloudBackupService.CloudBackupRemotePath as Mock
const ListBackups = CloudBackupService.ListCloudBackups as Mock
const SaveConfig = CloudBackupService.SaveCloudBackupConfig as Mock
const BackupNow = CloudBackupService.BackupDatabaseToCloud as Mock
const Restore = CloudBackupService.RestoreDatabaseFromCloud as Mock

const config = { enabled: true, interval: 'daily', retention: 5 }
const status = {
  lastBackupAt: null,
  lastAttemptAt: null,
  lastBackup: null,
  lastRestoreAt: null,
  lastError: '',
  nextBackupAt: '2026-08-08T10:00:00Z',
  rollbackPath: '',
}
const backup = {
  id: 'backup-1',
  file: 'clip/backups/backup-1.db',
  deviceName: 'mac-mini',
  createdAt: '2026-08-07T10:00:00Z',
  size: 1024,
  sha256: 'a'.repeat(64),
  databaseVersion: 1,
}

beforeEach(() => {
  vi.clearAllMocks()
  useCloudBackupStore.setState({
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
  })
  GetConfig.mockResolvedValue(config)
  GetStatus.mockResolvedValue(status)
  GetPath.mockResolvedValue('clip/backups/')
  ListBackups.mockResolvedValue([backup])
})

describe('CloudBackupStore.load', () => {
  it('载入本机配置、状态与远端路径', async () => {
    await useCloudBackupStore.getState().load()
    const state = useCloudBackupStore.getState()
    expect(state.config).toEqual(config)
    expect(state.status?.nextBackupAt).toBe('2026-08-08T10:00:00Z')
    expect(state.remotePath).toBe('clip/backups/')
    expect(state.loading).toBe(false)
  })

  it('配置读取失败仍保留状态', async () => {
    GetConfig.mockRejectedValue(new Error('config broken'))
    await useCloudBackupStore.getState().load()
    expect(useCloudBackupStore.getState().config).toBeNull()
    expect(useCloudBackupStore.getState().status).toEqual(status)
    expect(useCloudBackupStore.getState().error).toMatch(/config broken/)
  })
})

describe('CloudBackupStore actions', () => {
  it('刷新远端版本列表', async () => {
    await useCloudBackupStore.getState().refreshHistory()
    expect(useCloudBackupStore.getState().backups).toEqual([backup])
    expect(useCloudBackupStore.getState().loadingHistory).toBe(false)
  })

  it('保存后重新载入配置', async () => {
    SaveConfig.mockResolvedValue(undefined)
    await useCloudBackupStore.getState().save(config)
    expect(SaveConfig).toHaveBeenCalledWith(config)
    expect(GetConfig).toHaveBeenCalled()
    expect(useCloudBackupStore.getState().saving).toBe(false)
  })

  it('立即备份后刷新状态和版本列表', async () => {
    BackupNow.mockResolvedValue(backup)
    const result = await useCloudBackupStore.getState().backupNow()
    expect(result).toEqual(backup)
    expect(GetStatus).toHaveBeenCalled()
    expect(ListBackups).toHaveBeenCalled()
    expect(useCloudBackupStore.getState().backups).toEqual([backup])
    expect(useCloudBackupStore.getState().backingUp).toBe(false)
  })

  it('恢复指定版本并刷新状态', async () => {
    Restore.mockResolvedValue({
      restartRequired: true,
      rollbackPath: '/tmp/clip-before-cloud-restore.db',
    })
    const result = await useCloudBackupStore.getState().restore('backup-1')
    expect(Restore).toHaveBeenCalledWith('backup-1')
    expect(result.restartRequired).toBe(true)
    expect(useCloudBackupStore.getState().restoringId).toBeNull()
    expect(GetStatus).toHaveBeenCalled()
  })

  it('备份失败既记录又抛出', async () => {
    BackupNow.mockRejectedValue(new Error('quota exceeded'))
    await expect(useCloudBackupStore.getState().backupNow()).rejects.toThrow(
      /quota exceeded/,
    )
    expect(useCloudBackupStore.getState().error).toMatch(/quota exceeded/)
    expect(useCloudBackupStore.getState().backingUp).toBe(false)
  })
})
