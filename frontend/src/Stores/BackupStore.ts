import { create } from 'zustand'
import { WebDAVConfigService, OPMLBackupService } from '../Utils'

import type {
  WebDAVView,
  WebDAVInput,
  ConnectionTestResult,
  OPMLBackupConfig,
  OPMLBackupStatus,
  OPMLBackupInfo,
  OPMLImportResult,
} from '../Types'

interface BackupState {
  // WebDAV 配置
  webdavConfig: WebDAVView | null
  webdavLoading: boolean
  webdavSaving: boolean
  webdavTesting: boolean

  // OPML 备份配置
  opmlConfig: OPMLBackupConfig | null
  opmlStatus: OPMLBackupStatus | null
  opmlBackups: OPMLBackupInfo[]
  opmlLoading: boolean
  opmlSaving: boolean
  opmlBacking: boolean
  opmlRestoring: boolean
  opmlDeleting: string | null

  remotePath: string

  // WebDAV 方法
  loadWebDAVConfig: () => Promise<void>
  saveWebDAVConfig: (input: WebDAVInput) => Promise<void>
  testWebDAVConnection: (input: WebDAVInput) => Promise<ConnectionTestResult>
  clearWebDAVConfig: () => Promise<void>

  // OPML 备份方法
  loadOPMLConfig: () => Promise<void>
  saveOPMLConfig: (config: OPMLBackupConfig) => Promise<void>
  loadOPMLStatus: () => Promise<void>
  listOPMLBackups: () => Promise<void>
  backupOPML: () => Promise<OPMLBackupInfo>
  restoreOPML: (id: string) => Promise<OPMLImportResult>
  deleteOPMLBackup: (id: string) => Promise<void>
  loadRemotePath: () => Promise<void>
  load: () => Promise<void>
}

export const useBackupStore = create<BackupState>((set, get) => ({
  // 初始状态
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
  opmlDeleting: null,

  remotePath: '',

  // WebDAV 配置方法
  loadWebDAVConfig: async () => {
    set({ webdavLoading: true })
    try {
      const config = await WebDAVConfigService.GetWebDAVConfig()
      set({ webdavConfig: config })
    } catch (error) {
      console.error('Failed to load WebDAV config:', error)
    } finally {
      set({ webdavLoading: false })
    }
  },

  saveWebDAVConfig: async (input: WebDAVInput) => {
    set({ webdavSaving: true })
    try {
      await WebDAVConfigService.SaveWebDAVConfig(input)
      await get().loadWebDAVConfig()
    } finally {
      set({ webdavSaving: false })
    }
  },

  testWebDAVConnection: async (input: WebDAVInput) => {
    set({ webdavTesting: true })
    try {
      return await WebDAVConfigService.TestWebDAVConnection(input)
    } finally {
      set({ webdavTesting: false })
    }
  },

  clearWebDAVConfig: async () => {
    set({ webdavSaving: true })
    try {
      await WebDAVConfigService.ClearWebDAVConfig()
      set({ webdavConfig: null })
    } finally {
      set({ webdavSaving: false })
    }
  },

  // OPML 备份方法
  loadOPMLConfig: async () => {
    set({ opmlLoading: true })
    try {
      const config = await OPMLBackupService.GetOPMLBackupConfig()
      set({ opmlConfig: config })
    } catch (error) {
      console.error('Failed to load OPML backup config:', error)
    } finally {
      set({ opmlLoading: false })
    }
  },

  saveOPMLConfig: async (config: OPMLBackupConfig) => {
    set({ opmlSaving: true })
    try {
      await OPMLBackupService.SaveOPMLBackupConfig(config)
      await get().loadOPMLConfig()
    } finally {
      set({ opmlSaving: false })
    }
  },

  loadOPMLStatus: async () => {
    try {
      const status = await OPMLBackupService.GetOPMLBackupStatus()
      set({ opmlStatus: status })
    } catch (error) {
      console.error('Failed to load OPML backup status:', error)
    }
  },

  listOPMLBackups: async () => {
    set({ opmlLoading: true })
    try {
      const backups = await OPMLBackupService.ListOPMLBackups()
      set({ opmlBackups: backups || [] })
    } catch (error) {
      console.error('Failed to list OPML backups:', error)
      throw error
    } finally {
      set({ opmlLoading: false })
    }
  },

  backupOPML: async () => {
    set({ opmlBacking: true })
    try {
      const info = await OPMLBackupService.BackupOPMLToCloud()
      await get().loadOPMLStatus()
      await get().listOPMLBackups()
      return info
    } finally {
      set({ opmlBacking: false })
    }
  },

  restoreOPML: async (id: string) => {
    set({ opmlRestoring: true })
    try {
      const result = await OPMLBackupService.RestoreOPMLFromCloud(id)
      return result
    } finally {
      set({ opmlRestoring: false })
    }
  },

  deleteOPMLBackup: async (id: string) => {
    set({ opmlDeleting: id })
    try {
      await OPMLBackupService.DeleteOPMLBackup(id)
      set((state) => ({
        opmlBackups: state.opmlBackups.filter((backup) => backup.id !== id),
      }))
    } finally {
      set({ opmlDeleting: null })
    }
  },

  loadRemotePath: async () => {
    try {
      const path = await OPMLBackupService.OPMLBackupRemotePath()
      set({ remotePath: path })
    } catch (error) {
      console.error('Failed to load remote path:', error)
    }
  },

  // 加载所有数据
  load: async () => {
    await Promise.all([
      get().loadWebDAVConfig(),
      get().loadOPMLConfig(),
      get().loadOPMLStatus(),
      get().loadRemotePath(),
    ])
  },
}))
