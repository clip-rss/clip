// 一次性迁移：把 localStorage 里的旧偏好搬到后端设置。
//
// 主题与阅读排版原先由 zustand persist 存在 localStorage（clip-theme / clip-reader）。
// 收归后端后，老用户的既有选择必须搬过去，否则升级即被重置为出厂默认。
//
// ⚠️ 快照必须在模块加载时立即取。
// ThemeStore 订阅后端设置，load() 一返回就会把 clip-theme 改写成 v2 格式；
// 而迁移只能在 load() 之后跑（要先有后端基线）。那时再读就只剩 v2，读不到旧值了。
// 模块求值发生在任何异步 load 之前，故此处取到的必是迁移前的原始内容。

import { LEGACY_READER_KEY, THEME_CACHE_KEY } from './PrefsCache'
import { toReaderPrefs } from './ReaderStore'
import { useSettingsStore } from './SettingsStore'
import type { Settings, ThemePreference } from '../Types'

const THEME_PREFERENCES: readonly ThemePreference[] = [
  'light',
  'dark',
  'sepia',
  'system',
]

/** 待迁移的旧偏好；字段缺失表示该项无需迁移。 */
interface LegacySnapshot {
  theme?: ThemePreference
  reader?: Partial<Settings>
}

function readJSON(key: string): unknown {
  try {
    const raw = localStorage.getItem(key)
    return raw ? JSON.parse(raw) : null
  } catch {
    return null // 存储不可用或内容损坏，视为无旧数据
  }
}

/**
 * 取旧主题偏好。仅认 zustand persist 的形状（{state:{preference}}）；
 * 新版缓存是 {v,preference}，不会被误判为待迁移。
 */
function snapshotLegacyTheme(): ThemePreference | undefined {
  const parsed = readJSON(THEME_CACHE_KEY)
  if (!parsed || typeof parsed !== 'object') return undefined
  const state = (parsed as { state?: { preference?: unknown } }).state
  if (!state) return undefined
  const pref = state.preference
  return THEME_PREFERENCES.includes(pref as ThemePreference)
    ? (pref as ThemePreference)
    : undefined
}

/** 取旧阅读排版偏好，复用 toReaderPrefs 的取值校验。 */
function snapshotLegacyReader(): Partial<Settings> | undefined {
  const parsed = readJSON(LEGACY_READER_KEY)
  if (!parsed || typeof parsed !== 'object') return undefined
  const state = (parsed as { state?: Record<string, unknown> }).state
  if (!state) return undefined

  const prefs = toReaderPrefs({
    readerFontFamily: state.fontFamily as string,
    readerFontSize: state.fontSize as number,
    readerLineHeight: state.lineHeight as number,
    readerWidth: state.width as string,
    readerBackground: state.background as string,
  })
  return {
    readerFontFamily: prefs.fontFamily,
    readerFontSize: prefs.fontSize,
    readerLineHeight: prefs.lineHeight,
    readerWidth: prefs.width,
    readerBackground: prefs.background,
  }
}

// 模块加载即取快照（见文件头说明）。
const snapshot: LegacySnapshot = {
  theme: snapshotLegacyTheme(),
  reader: snapshotLegacyReader(),
}

/** 是否存在待迁移的旧偏好。 */
export function hasLegacyPrefs(): boolean {
  return snapshot.theme !== undefined || snapshot.reader !== undefined
}

/** 待写入后端的字段；无待迁移项时返回 null。 */
export function legacyPrefsPatch(): Partial<Settings> | null {
  if (!hasLegacyPrefs()) return null
  return {
    ...(snapshot.theme !== undefined ? { theme: snapshot.theme } : {}),
    ...(snapshot.reader ?? {}),
  }
}

/**
 * 清理旧 key。仅在后端写入确认成功后调用 —— 先删再写一旦失败就永久丢失用户设置。
 *
 * clip-theme 不删而是由 ThemeStore 改写为新版缓存格式（index.html 首帧要读它防闪烁），
 * 版本号变化即天然的幂等标记，不需要额外的迁移标记位。
 */
export function clearLegacyPrefs(): void {
  try {
    localStorage.removeItem(LEGACY_READER_KEY)
  } catch {
    // 清理失败无妨：后端已是数据源，旧 key 只会被忽略
  }
  snapshot.theme = undefined
  snapshot.reader = undefined
}

/**
 * 执行迁移。须在 SettingsStore.load() 之后调用：迁移是把本地旧值覆盖到后端，
 * 得先有后端基线才能合并写入。
 *
 * 旧值优先于后端默认值 —— 后端此前从未存过这两组配置，它那边只可能是出厂默认，
 * 拿默认值覆盖用户的既有选择就是把设置重置了。
 *
 * 无旧数据（全新安装、或已迁移过）时为无操作。
 */
export async function migrateLegacyPrefs(): Promise<void> {
  const patch = legacyPrefsPatch()
  if (!patch) return

  const store = useSettingsStore.getState()
  if (!store.settings) return // 后端设置未载入，本轮跳过，下次启动再试

  await store.update(patch)

  // update 失败会回滚并写 error；此时不能清旧 key，否则用户设置永久丢失。
  if (useSettingsStore.getState().error) return
  clearLegacyPrefs()
}
