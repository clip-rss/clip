import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest'

// 下方的漂移守卫测试要读后端生成的 Action 枚举真值，而 import 生成的 models.js
// 会连带拉起 @wailsio/runtime —— 它装了个定时器，在测试环境拆掉 jsdom 之后才触发，
// 于是抛一个与本测试无关的 "window is not defined"。
// 生成的 models 只用到 Create.Nullable / Create.Array，替掉即可。
vi.mock('@wailsio/runtime', () => ({
  Create: {
    Nullable: (fn: unknown) => fn,
    Array: (fn: unknown) => fn,
  },
}))

vi.mock('../Utils', () => ({
  SyncService: {
    GetWebDAVConfig: vi.fn(),
    GetSyncStatus: vi.fn(),
    RemoteFilePath: vi.fn(),
    SaveWebDAVConfig: vi.fn(),
    ClearWebDAVConfig: vi.fn(),
    TestWebDAVConnection: vi.fn(),
    SyncNow: vi.fn(),
    ResolveConflict: vi.fn(),
  },
  SettingsService: {
    GetSettings: vi.fn(),
    UpdateSettings: vi.fn(),
  },
  toApiError: (e: unknown) => String(e),
}))

import { SettingsService, SyncService } from '../Utils'
import { useSyncStore } from './SyncStore'
import { useSettingsStore } from './SettingsStore'
import type { Settings, SyncAction, WebDAVView } from '../Types'
// 后端生成的枚举，用于守住前端手写的 SyncAction 联合类型不漂移。
import { Action } from '../../bindings/github.com/clip-rss/clip/internal/syncer'

const GetWebDAVConfig = SyncService.GetWebDAVConfig as unknown as Mock
const GetSyncStatus = SyncService.GetSyncStatus as unknown as Mock
const RemoteFilePath = SyncService.RemoteFilePath as unknown as Mock
const SaveWebDAVConfig = SyncService.SaveWebDAVConfig as unknown as Mock
const ClearWebDAVConfig = SyncService.ClearWebDAVConfig as unknown as Mock
const TestWebDAVConnection = SyncService.TestWebDAVConnection as unknown as Mock
const SyncNow = SyncService.SyncNow as unknown as Mock
const ResolveConflict = SyncService.ResolveConflict as unknown as Mock
const UpdateSettings = SettingsService.UpdateSettings as unknown as Mock

const view: WebDAVView = {
  enabled: true,
  url: 'https://dav.example.com/dav/',
  username: 'alice',
  hasPassword: true,
}

const emptyStatus = {
  lastSyncAt: null,
  lastError: '',
  hasPending: false,
  conflict: null,
  deviceName: 'mac-mini',
}

const defaults: Settings = {
  theme: 'system',
  language: 'zh',
  defaultUpdateInterval: 30,
  defaultMaxItems: 100,
  notificationMode: 'each',
  showUnreadBadge: true,
  autoMarkReadDelay: 0,
  launchMinimized: false,
  windowWidth: 1200,
  windowHeight: 800,
  proxyHost: '',
  proxyPort: 0,
  reduceMotion: false,
  showFocusIndicator: true,
  readerFontFamily: 'sans',
  readerFontSize: 16,
  readerLineHeight: 1.8,
  readerWidth: '640',
  readerBackground: 'default',
}

beforeEach(() => {
  vi.clearAllMocks()
  useSyncStore.setState({
    config: null,
    status: null,
    remotePath: '',
    loading: false,
    saving: false,
    syncing: false,
    error: null,
  })
  useSettingsStore.setState({ settings: null, loading: false, error: null })

  GetWebDAVConfig.mockResolvedValue(view)
  GetSyncStatus.mockResolvedValue(emptyStatus)
  RemoteFilePath.mockResolvedValue('clip/settings.json')
})

describe('SyncStore.load', () => {
  it('一次载入配置、状态与远端路径', async () => {
    await useSyncStore.getState().load()
    const s = useSyncStore.getState()
    expect(s.config?.username).toBe('alice')
    expect(s.status?.deviceName).toBe('mac-mini')
    expect(s.remotePath).toBe('clip/settings.json')
    expect(s.loading).toBe(false)
    expect(s.error).toBeNull()
  })

  // 凭据存储不可用（密钥文件所在路径被占等）时 GetWebDAVConfig 会失败，
  // 但状态仍读得到。用 Promise.all 的话设置页会整片空白 ——
  // 连「同步为何不可用」都显示不出来，而那正是此刻唯一有用的信息。
  it('配置读取失败仍保留状态与错误原因', async () => {
    GetWebDAVConfig.mockRejectedValue(new Error('凭据存储不可用'))
    await useSyncStore.getState().load()
    const s = useSyncStore.getState()
    expect(s.config).toBeNull()
    expect(s.status?.deviceName).toBe('mac-mini')
    expect(s.error).toMatch(/凭据存储不可用/)
    expect(s.loading).toBe(false)
  })
})

describe('SyncStore.save', () => {
  it('保存后重新载入，hasPassword 随之更新', async () => {
    GetWebDAVConfig.mockResolvedValueOnce({ ...view, hasPassword: false })
    await useSyncStore.getState().load()
    expect(useSyncStore.getState().config?.hasPassword).toBe(false)

    SaveWebDAVConfig.mockResolvedValue(undefined)
    GetWebDAVConfig.mockResolvedValue({ ...view, hasPassword: true })

    await useSyncStore.getState().save({
      enabled: true,
      url: 'https://dav.example.com/dav/',
      username: 'alice',
      password: 'hunter2',
    })

    expect(SaveWebDAVConfig).toHaveBeenCalledWith(
      expect.objectContaining({ password: 'hunter2' }),
    )
    // 重新载入过：密码框据此清空并改提示为「已保存，留空则不修改」。
    expect(useSyncStore.getState().config?.hasPassword).toBe(true)
    expect(useSyncStore.getState().saving).toBe(false)
  })

  // 保存失败必须抛出，不能只落在 store 里。
  // 只落 store 的话组件的 catch 不会执行，于是照常显示「已保存」——
  // 而后端其实拒了这份配置（明文 http、密码没填等），用户被明确地误导。
  it('保存失败既记录也抛出', async () => {
    SaveWebDAVConfig.mockRejectedValue(new Error('必须使用 https'))
    await expect(
      useSyncStore.getState().save({
        enabled: true,
        url: 'http://dav.example.com/dav/',
        username: 'alice',
        password: '',
      }),
    ).rejects.toThrow(/https/)
    expect(useSyncStore.getState().error).toMatch(/https/)
    expect(useSyncStore.getState().saving).toBe(false)
  })
})

describe('SyncStore.clear', () => {
  it('清除后重新载入', async () => {
    ClearWebDAVConfig.mockResolvedValue(undefined)
    GetWebDAVConfig.mockResolvedValue({
      enabled: false,
      url: '',
      username: '',
      hasPassword: false,
    })
    await useSyncStore.getState().clear()
    expect(ClearWebDAVConfig).toHaveBeenCalled()
    expect(useSyncStore.getState().config?.url).toBe('')
  })
})

describe('SyncStore.test', () => {
  // 测试连接的结果是一次性的表单反馈，不进 store。
  it('原样透传表单配置与结果，不落 store', async () => {
    const result = { ok: false, step: 'connect', message: '认证失败', hint: '用应用密码' }
    TestWebDAVConnection.mockResolvedValue(result)

    const input = {
      enabled: true,
      url: 'https://dav.example.com/dav/',
      username: 'alice',
      password: 'pw',
    }
    await expect(useSyncStore.getState().test(input)).resolves.toEqual(result)
    expect(TestWebDAVConnection).toHaveBeenCalledWith(input)
    expect(useSyncStore.getState().error).toBeNull()
  })
})

describe('SyncStore.syncNow', () => {
  // 关键一条：拉取回来的设置必须立刻落到 SettingsStore。
  // 主题、排版、语言的订阅方都挂在它上面，不落就得等重启才看得到刚同步下来的配置
  // —— 而用户刚点的按钮明明返回了「已应用远端配置」。
  it('拉取结果立刻落到 SettingsStore', async () => {
    const pulled = { ...defaults, theme: 'sepia', readerFontSize: 18 }
    SyncNow.mockResolvedValue({
      action: 'pulled',
      conflict: null,
      settings: pulled,
    })

    const res = await useSyncStore.getState().syncNow()
    expect(res.action).toBe('pulled')
    expect(useSettingsStore.getState().settings?.theme).toBe('sepia')
    expect(useSettingsStore.getState().settings?.readerFontSize).toBe(18)
  })

  // applyExternal 而非 update：后端在同步时已经写过库了。
  // 走 update 会把刚拉下来的配置原样写回后端，触发一次变更回调、
  // 进而安排一次无谓的推送（后端的 suppress 只覆盖它自己那条写入路径）。
  it('应用拉取结果不回写后端', async () => {
    // ⚠️ 必须先塞一份 settings。留 null 的话 SettingsStore.update 会在
    // `if (!prev) return` 处直接返回，于是「改成走 update」这种回写变异
    // 也照样不会调到 UpdateSettings —— 这条守卫会假绿。
    useSettingsStore.setState({ settings: defaults })
    SyncNow.mockResolvedValue({
      action: 'pulled',
      conflict: null,
      settings: { ...defaults, theme: 'dark' },
    })
    await useSyncStore.getState().syncNow()
    expect(useSettingsStore.getState().settings?.theme).toBe('dark')
    expect(UpdateSettings).not.toHaveBeenCalled()
  })

  it('noop 不碰 SettingsStore', async () => {
    useSettingsStore.setState({ settings: defaults })
    SyncNow.mockResolvedValue({ action: 'noop', conflict: null, settings: null })
    await useSyncStore.getState().syncNow()
    expect(useSettingsStore.getState().settings?.theme).toBe('system')
    expect(UpdateSettings).not.toHaveBeenCalled()
  })

  it('同步后刷新状态', async () => {
    SyncNow.mockResolvedValue({ action: 'pushed', conflict: null, settings: null })
    GetSyncStatus.mockResolvedValue({
      ...emptyStatus,
      hasPending: false,
      lastSyncAt: '2026-08-06T10:00:00Z',
    })
    await useSyncStore.getState().syncNow()
    expect(useSyncStore.getState().status?.lastSyncAt).toBe(
      '2026-08-06T10:00:00Z',
    )
    expect(useSyncStore.getState().syncing).toBe(false)
  })

  // 失败时也要刷状态：后端把原因写进了 State.LastError，
  // 不刷的话关掉设置页再打开就只剩一片正常，用户不知道自动推送一直在失败。
  it('同步失败时抛出并刷新状态', async () => {
    SyncNow.mockRejectedValue(new Error('连不上服务器'))
    GetSyncStatus.mockResolvedValue({
      ...emptyStatus,
      lastError: '连不上服务器',
    })
    await expect(useSyncStore.getState().syncNow()).rejects.toThrow(/连不上/)
    expect(useSyncStore.getState().error).toMatch(/连不上/)
    expect(useSyncStore.getState().status?.lastError).toBe('连不上服务器')
    expect(useSyncStore.getState().syncing).toBe(false)
  })

  it('冲突结果原样返回，不自动裁决', async () => {
    const conflict = {
      remoteDeviceName: 'thinkpad',
      remoteUpdatedAt: '2026-08-06T09:00:00Z',
      remoteRevision: 4,
      detectedAt: '2026-08-06T10:00:00Z',
    }
    SyncNow.mockResolvedValue({ action: 'conflict', conflict, settings: null })
    const res = await useSyncStore.getState().syncNow()
    expect(res.action).toBe('conflict')
    expect(res.conflict?.remoteDeviceName).toBe('thinkpad')
    expect(ResolveConflict).not.toHaveBeenCalled()
  })
})

describe('SyncStore.resolve', () => {
  it.each([
    ['保留本机', true],
    ['使用远端', false],
  ])('%s 时把 keepLocal 透传给后端', async (_label, keepLocal) => {
    ResolveConflict.mockResolvedValue({
      action: keepLocal ? 'pushed' : 'pulled',
      conflict: null,
      settings: keepLocal ? null : defaults,
    })
    await useSyncStore.getState().resolve(keepLocal as boolean)
    expect(ResolveConflict).toHaveBeenCalledWith(keepLocal)
  })

  it('选择远端时应用拉回的设置', async () => {
    ResolveConflict.mockResolvedValue({
      action: 'pulled',
      conflict: null,
      settings: { ...defaults, language: 'en' },
    })
    await useSyncStore.getState().resolve(false)
    expect(useSettingsStore.getState().settings?.language).toBe('en')
  })
})

// SyncAction 是前端手写的联合类型（后端生成的 Action 是宽 string，
// 拿不到字面量类型）。后端新增一个动作而这里没跟上，组件里
// t(`settings.sync.result.${action}`) 会静默取不到文案，只显示原始 key。
describe('SyncAction 与后端枚举', () => {
  it('覆盖 syncer.Action 的全部取值', () => {
    const ALL: SyncAction[] = ['noop', 'pushed', 'pulled', 'conflict']
    const backend = Object.entries(Action)
      .filter(([k]) => k !== '$zero')
      .map(([, v]) => v)
    expect([...backend].sort()).toEqual([...ALL].sort())
  })
})
