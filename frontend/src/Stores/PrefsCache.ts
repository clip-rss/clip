// 主题偏好的 localStorage 首帧缓存。
//
// 后端设置是数据源，但后端读取是异步的：等它返回再涂主题会闪一下白底。
// index.html 里有一段内联脚本在 React 之前同步读本缓存并涂 class，
// 本模块是该缓存格式的唯一拥有者（key、版本号、读写）。
//
// ⚠️ 改动格式必须同步改 frontend/index.html 的内联脚本，两处要能读同一份数据。

import type { ThemePreference } from '../Types'

/** 缓存 key。沿用旧 key，靠内部版本号区分新旧格式。 */
export const THEME_CACHE_KEY = 'clip-theme'

/**
 * 当前缓存格式版本。
 *
 * v2 = {v:2, preference}，本模块写入。
 * 无 v 字段而有 state.preference = zustand persist 的旧格式，属待迁移数据
 * （见 LegacyPrefs）。版本号变化本身即迁移的幂等标记，不需要额外标记位。
 */
export const THEME_CACHE_VERSION = 2

/** 旧阅读排版偏好的 localStorage key。收归后端后仅用于一次性迁移。 */
export const LEGACY_READER_KEY = 'clip-reader'

const PREFERENCES: readonly ThemePreference[] = [
  'light',
  'dark',
  'sepia',
  'system',
]

function isPreference(value: unknown): value is ThemePreference {
  return PREFERENCES.includes(value as ThemePreference)
}

/**
 * 读首帧缓存。兼容 v2 与 zustand persist 旧格式 —— 老用户升级后的第一帧
 * 仍要能涂对主题，此时迁移尚未执行。
 */
export function readThemeCache(): ThemePreference | null {
  try {
    const raw = localStorage.getItem(THEME_CACHE_KEY)
    if (!raw) return null
    const parsed: unknown = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return null

    // v2
    const v2 = (parsed as { preference?: unknown }).preference
    if (isPreference(v2)) return v2

    // 旧格式
    const legacy = (parsed as { state?: { preference?: unknown } }).state
      ?.preference
    if (isPreference(legacy)) return legacy

    return null
  } catch {
    return null // 存储不可用或内容损坏，按无缓存处理
  }
}

/** 写首帧缓存。失败不抛：缓存只影响首帧观感，后端才是数据源。 */
export function writeThemeCache(preference: ThemePreference): void {
  try {
    localStorage.setItem(
      THEME_CACHE_KEY,
      JSON.stringify({ v: THEME_CACHE_VERSION, preference }),
    )
  } catch {
    // 忽略：隐私模式或配额耗尽时最多是启动闪一下
  }
}
