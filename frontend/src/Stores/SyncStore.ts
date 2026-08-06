import { create } from 'zustand'
import { SyncService, toApiError } from '../Utils'
import type {
  ConnectionTestResult,
  SyncResult,
  SyncStatus,
  WebDAVInput,
  WebDAVView,
} from '../Types'
import { useSettingsStore } from './SettingsStore'

interface SyncState {
  /** 已保存的连接配置；未载入时为 null。永远不含密码。 */
  config: WebDAVView | null
  status: SyncStatus | null
  /** 同步文件相对用户所填地址的路径，由后端给出（见 SyncService.RemoteFilePath）。 */
  remotePath: string

  loading: boolean
  saving: boolean
  syncing: boolean
  /** 上一次操作的失败原因。成功的操作会清空它。 */
  error: string | null

  load: () => Promise<void>
  /** 保存配置。失败时抛出，由调用方决定怎么提示。 */
  save: (input: WebDAVInput) => Promise<void>
  /** 删除全部配置（含密码）。 */
  clear: () => Promise<void>
  /** 测试连接。结果以数据返回，不进 store —— 属于一次性的表单反馈。 */
  test: (input: WebDAVInput) => Promise<ConnectionTestResult>
  syncNow: () => Promise<SyncResult>
  /** 裁决冲突：keepLocal 为真用本机配置覆盖远端。 */
  resolve: (keepLocal: boolean) => Promise<SyncResult>
}

export const useSyncStore = create<SyncState>()((set, get) => ({
  config: null,
  status: null,
  remotePath: '',
  loading: false,
  saving: false,
  syncing: false,
  error: null,

  async load() {
    set({ loading: true, error: null })
    // allSettled 而非 all：凭据存储不可用时 GetWebDAVConfig 会失败，
    // 但状态仍读得到。用 all 的话设置页会整片空白，连「同步为何不可用」
    // 都显示不出来 —— 而那正是用户此刻最需要看到的信息。
    const [cfg, st, path] = await Promise.allSettled([
      SyncService.GetWebDAVConfig(),
      SyncService.GetSyncStatus(),
      SyncService.RemoteFilePath(),
    ])
    set({
      config: cfg.status === 'fulfilled' ? cfg.value : null,
      status: st.status === 'fulfilled' ? st.value : null,
      remotePath: path.status === 'fulfilled' ? path.value : '',
      loading: false,
      error:
        cfg.status === 'rejected'
          ? toApiError(cfg.reason)
          : st.status === 'rejected'
            ? toApiError(st.reason)
            : null,
    })
  },

  async save(input) {
    set({ saving: true, error: null })
    try {
      await SyncService.SaveWebDAVConfig(input)
    } catch (err) {
      set({ saving: false, error: toApiError(err) })
      throw err
    }
    set({ saving: false })
    // 重新载入：hasPassword 可能刚由 false 变 true，表单据此清空密码框。
    await get().load()
  },

  async clear() {
    set({ saving: true, error: null })
    try {
      await SyncService.ClearWebDAVConfig()
    } catch (err) {
      set({ saving: false, error: toApiError(err) })
      throw err
    }
    set({ saving: false })
    await get().load()
  },

  async test(input) {
    // 后端以数据形式回报失败（哪一步、什么建议），不 reject。
    // 真正 reject 的只有「凭据存储不可用」这类前置错误。
    return SyncService.TestWebDAVConnection(input)
  },

  async syncNow() {
    return runSync(set, () => SyncService.SyncNow())
  },

  async resolve(keepLocal) {
    return runSync(set, () => SyncService.ResolveConflict(keepLocal))
  },
}))

/** 同步与冲突裁决共用的执行壳：置位 syncing、应用拉取结果、刷新状态。 */
async function runSync(
  set: (partial: Partial<SyncState>) => void,
  call: () => Promise<SyncResult>,
): Promise<SyncResult> {
  set({ syncing: true, error: null })
  let res: SyncResult
  try {
    res = await call()
  } catch (err) {
    set({ syncing: false, error: toApiError(err) })
    // 失败原因也落在后端状态里（State.LastError），一并刷出来，
    // 免得关掉设置页再打开就只剩一片正常。
    await refreshStatus(set)
    throw err
  }

  // 拉取回来的设置立刻落到 SettingsStore：主题、排版、语言的订阅方都挂在
  // 它上面，不落就得等用户重启才能看到刚同步下来的配置。
  if (res.action === 'pulled' && res.settings) {
    useSettingsStore.getState().applyExternal(res.settings)
  }

  set({ syncing: false })
  await refreshStatus(set)
  return res
}

/** 只刷新状态，不动配置。同步后 hasPending / lastSyncAt / conflict 都会变。 */
async function refreshStatus(
  set: (partial: Partial<SyncState>) => void,
): Promise<void> {
  try {
    set({ status: await SyncService.GetSyncStatus() })
  } catch {
    // 状态读不到不该盖掉上面那条更有信息量的错误（同步本身的失败原因）。
  }
}
