import { create } from 'zustand'
import { SettingsService, toApiError } from '../Utils'
import type { Settings } from '../Types'

type NotificationMode = 'each' | 'summary' | 'off'

interface SettingsState {
  settings: Settings | null
  loading: boolean
  error: string | null

  load: () => Promise<void>
  /** 通用更新：乐观合并写入，失败回滚。 */
  update: (partial: Partial<Settings>) => Promise<void>
  /** 更新通知模式（基于 update 的薄封装）。 */
  setNotificationMode: (mode: NotificationMode) => Promise<void>
  /**
   * 接受一份后端已经落库的完整设置，只更新本地状态，**不回写后端**。
   *
   * 用于配置同步拉取之后：那份设置正是后端刚写进库的，再走 update 会重新
   * 调一次 UpdateSettings，触发后端的设置变更回调、安排一次推送 ——
   * 把刚拉下来的配置又推回远端。后端只对引擎自己的写入路径做了抑制
   * （api.settingsWriter），拦不住前端发起的这一次。
   */
  applyExternal: (settings: Settings) => void
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

  async update(partial) {
    const prev = get().settings
    if (!prev) return
    const next = { ...prev, ...partial } as Settings
    set({ settings: next, error: null })
    try {
      await SettingsService.UpdateSettings(next)
    } catch (err) {
      set({ settings: prev, error: toApiError(err) })
    }
  },

  async setNotificationMode(mode) {
    const prev = get().settings
    if (!prev || prev.notificationMode === mode) return
    await get().update({ notificationMode: mode })
  },

  applyExternal(settings) {
    set({ settings, error: null })
  },
}))
