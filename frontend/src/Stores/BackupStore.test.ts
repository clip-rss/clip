import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest'

vi.mock('../Utils', () => ({
  WebDAVConfigService: {
    GetWebDAVConfig: vi.fn(),
    SaveWebDAVConfig: vi.fn(),
    TestWebDAVConnection: vi.fn(),
    ClearWebDAVConfig: vi.fn(),
  },
  OPMLBackupService: {
    GetOPMLBackupConfig: vi.fn(),
    SaveOPMLBackupConfig: vi.fn(),
    GetOPMLBackupStatus: vi.fn(),
    ListOPMLBackups: vi.fn(),
    BackupOPMLToCloud: vi.fn(),
    RestoreOPMLFromCloud: vi.fn(),
    OPMLBackupRemotePath: vi.fn(),
  },
  toApiError: (e: unknown) => String(e),
}))

import { WebDAVConfigService, OPMLBackupService } from '../Utils'
import { useBackupStore } from './BackupStore'

const GetWebDAVConfig = WebDAVConfigService.GetWebDAVConfig as Mock
const SaveWebDAVConfig = WebDAVConfigService.SaveWebDAVConfig as Mock
const TestWebDAVConnection = WebDAVConfigService.TestWebDAVConnection as Mock
const ClearWebDAVConfig = WebDAVConfigService.ClearWebDAVConfig as Mock

const GetOPMLBackupConfig = OPMLBackupService.GetOPMLBackupConfig as Mock
const SaveOPMLBackupConfig = OPMLBackupService.SaveOPMLBackupConfig as Mock
const GetOPMLBackupStatus = OPMLBackupService.GetOPMLBackupStatus as Mock
const ListOPMLBackups = OPMLBackupService.ListOPMLBackups as Mock
const BackupOPMLToCloud = OPMLBackupService.BackupOPMLToCloud as Mock
const RestoreOPMLFromCloud = OPMLBackupService.RestoreOPMLFromCloud as Mock
const OPMLBackupRemotePath = OPMLBackupService.OPMLBackupRemotePath as Mock

beforeEach(() => {
  vi.clearAllMocks()
  useBackupStore.setState({
    webdavConfig: null,
    webdavLoading: false,
    webdavSaving: false,
    webdavTesting: false,
    opmlConfig: null,
    opmlStatus: null,
    opmlBackups: [],
    opmlLoading: false,
    opmlSaving: false,
    opmlBacking: false,
    opmlRestoring: false,
    remotePath: '',
  })
})

describe('BackupStore - WebDAV', () => {
  it('loadWebDAVConfig 从后端获取配置', async () => {
    const config = {
      url: 'https://dav.example.com',
      username: 'user',
      hasPassword: true,
    }
    GetWebDAVConfig.mockResolvedValue(config)

    await useBackupStore.getState().loadWebDAVConfig()

    expect(useBackupStore.getState().webdavConfig).toEqual(config)
    expect(useBackupStore.getState().webdavLoading).toBe(false)
  })

  it('saveWebDAVConfig 保存并重新加载', async () => {
    const input = {
      url: 'https://dav.example.com',
      username: 'user',
      password: 'pass',
    }
    const saved = {
      url: input.url,
      username: input.username,
      hasPassword: true,
    }
    SaveWebDAVConfig.mockResolvedValue(undefined)
    GetWebDAVConfig.mockResolvedValue(saved)

    await useBackupStore.getState().saveWebDAVConfig(input)

    expect(SaveWebDAVConfig).toHaveBeenCalledWith(input)
    expect(useBackupStore.getState().webdavConfig).toEqual(saved)
  })

  it('testWebDAVConnection 返回测试结果', async () => {
    const input = {
      url: 'https://dav.example.com',
      username: 'user',
      password: 'pass',
    }
    const result = { ok: true, message: '', hint: '', step: '' }
    TestWebDAVConnection.mockResolvedValue(result)

    const res = await useBackupStore.getState().testWebDAVConnection(input)

    expect(res).toEqual(result)
    expect(TestWebDAVConnection).toHaveBeenCalledWith(input)
  })

  it('clearWebDAVConfig 清除配置', async () => {
    useBackupStore.setState({
      webdavConfig: {
        url: 'https://dav.example.com',
        username: 'user',
        hasPassword: true,
      },
    })
    ClearWebDAVConfig.mockResolvedValue(undefined)

    await useBackupStore.getState().clearWebDAVConfig()

    expect(useBackupStore.getState().webdavConfig).toBeNull()
    expect(ClearWebDAVConfig).toHaveBeenCalled()
  })
})

describe('BackupStore - OPML Backup', () => {
  it('loadOPMLConfig 从后端获取配置', async () => {
    const config = { retention: 7 }
    GetOPMLBackupConfig.mockResolvedValue(config)

    await useBackupStore.getState().loadOPMLConfig()

    expect(useBackupStore.getState().opmlConfig).toEqual(config)
  })

  it('saveOPMLConfig 保存并重新加载', async () => {
    const config = { retention: 10 }
    SaveOPMLBackupConfig.mockResolvedValue(undefined)
    GetOPMLBackupConfig.mockResolvedValue(config)

    await useBackupStore.getState().saveOPMLConfig(config)

    expect(SaveOPMLBackupConfig).toHaveBeenCalledWith(config)
    expect(useBackupStore.getState().opmlConfig).toEqual(config)
  })

  it('loadOPMLStatus 获取状态', async () => {
    const status = { lastBackupAt: '2024-01-01T00:00:00Z', lastError: '' }
    GetOPMLBackupStatus.mockResolvedValue(status)

    await useBackupStore.getState().loadOPMLStatus()

    expect(useBackupStore.getState().opmlStatus).toEqual(status)
  })

  it('listOPMLBackups 获取备份列表', async () => {
    const backups = [
      {
        id: '1',
        createdAt: '2024-01-01T00:00:00Z',
        deviceName: 'Mac',
        size: 1024,
      },
      {
        id: '2',
        createdAt: '2024-01-02T00:00:00Z',
        deviceName: 'Mac',
        size: 2048,
      },
    ]
    ListOPMLBackups.mockResolvedValue(backups)

    await useBackupStore.getState().listOPMLBackups()

    expect(useBackupStore.getState().opmlBackups).toEqual(backups)
  })

  it('backupOPML 执行备份并刷新状态', async () => {
    const info = {
      id: '3',
      createdAt: '2024-01-03T00:00:00Z',
      deviceName: 'Mac',
      size: 3072,
    }
    const status = { lastBackupAt: '2024-01-03T00:00:00Z', lastError: '' }
    BackupOPMLToCloud.mockResolvedValue(info)
    GetOPMLBackupStatus.mockResolvedValue(status)
    ListOPMLBackups.mockResolvedValue([info])

    const result = await useBackupStore.getState().backupOPML()

    expect(result).toEqual(info)
    expect(useBackupStore.getState().opmlStatus).toEqual(status)
    expect(useBackupStore.getState().opmlBackups).toEqual([info])
  })

  it('restoreOPML 执行恢复', async () => {
    const result = { categories: 5, feeds: 20, skipped: 0 }
    RestoreOPMLFromCloud.mockResolvedValue(result)

    const res = await useBackupStore.getState().restoreOPML('backup-id')

    expect(res).toEqual(result)
    expect(RestoreOPMLFromCloud).toHaveBeenCalledWith('backup-id')
  })

  it('loadRemotePath 获取远端路径', async () => {
    OPMLBackupRemotePath.mockResolvedValue('clip/opml-backups/')

    await useBackupStore.getState().loadRemotePath()

    expect(useBackupStore.getState().remotePath).toBe('clip/opml-backups/')
  })
})

describe('BackupStore - load', () => {
  it('load 并行加载所有数据', async () => {
    const webdavConfig = {
      url: 'https://dav.example.com',
      username: 'user',
      hasPassword: true,
    }
    const opmlConfig = { retention: 7 }
    const opmlStatus = { lastBackupAt: '2024-01-01T00:00:00Z', lastError: '' }
    const remotePath = 'clip/opml-backups/'

    GetWebDAVConfig.mockResolvedValue(webdavConfig)
    GetOPMLBackupConfig.mockResolvedValue(opmlConfig)
    GetOPMLBackupStatus.mockResolvedValue(opmlStatus)
    OPMLBackupRemotePath.mockResolvedValue(remotePath)

    await useBackupStore.getState().load()

    expect(useBackupStore.getState().webdavConfig).toEqual(webdavConfig)
    expect(useBackupStore.getState().opmlConfig).toEqual(opmlConfig)
    expect(useBackupStore.getState().opmlStatus).toEqual(opmlStatus)
    expect(useBackupStore.getState().remotePath).toBe(remotePath)
  })
})
