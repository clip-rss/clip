import type { Platform } from '../Hooks/usePlatform'

/** 平台对应的主修饰键显示符：macOS 用 ⌘，其余用 Ctrl。 */
export function modKey(platform: Platform | null): string {
  return platform === 'mac' ? '⌘' : 'Ctrl'
}

/**
 * 拼装快捷键提示文案，如 `(⌘N)` / `(Ctrl+Shift+I)`。
 *
 * @param platform 当前平台（null 时按非 mac 处理）。
 * @param keys 主键之外的按键序列，如 `['Shift', 'I']` 或 `['N']`。
 */
export function shortcutHint(platform: Platform | null, keys: string[]): string {
  const mod = modKey(platform)
  const sep = platform === 'mac' ? '' : '+'
  return `(${[mod, ...keys].join(sep)})`
}
