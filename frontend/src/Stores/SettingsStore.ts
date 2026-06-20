import { create } from 'zustand'
import { SettingsService, toApiError } from '../Utils'
import type { Settings } from '../Types'

type NotificationMode = 'each' | 'summary' | 'off'

interface SettingsState {
  settings: Settings | null
  loading: boolean
  error: string | null

  load: () => Promise<void>
  /** 更新通知模式（乐观写入，失败回滚）。 */
  setNotificationMode: (mode: NotificationMode) => Promise<void>
}

export const useSettingsStore = create<SettingsState>()((set, get) => ({
  settings: null,
  loading: false,
  error: null,

  async load() {
    set({ loading: true, error: null })
    try {
      const s = await SettingsService.GetSettings()
      set({ settings: s, loading: false })
    } catch (err) {
      set({ error: toApiError(err), loading: false })
    }
  },

  async setNotificationMode(mode) {
    const prev = get().settings
    if (!prev || prev.notificationMode === mode) return
    const next = { ...prev, notificationMode: mode }
    set({ settings: next })
    try {
      await SettingsService.UpdateSettings(next)
    } catch (err) {
      set({ settings: prev, error: toApiError(err) })
    }
  },
}))
