import { useEffect } from 'react'
import { useSettingsStore, useSidebarStore } from '../Stores'
import { DockService } from '../Utils'
import { usePlatform, type Platform } from './usePlatform'

/** 统计所有订阅源的未读文章总数。 */
export function totalUnread(feeds: { unreadCount: number }[]): number {
  let total = 0
  for (const f of feeds) total += f.unreadCount
  return total
}

/**
 * 根据「是否展示角标」开关与未读总数决定 Dock badge 的目标标签。
 * - 开关关闭：返回 null（应移除 badge）。
 * - 开关开启且未读为 0：返回 null。
 * - 开关开启且未读 >0：返回该数字的字符串形式。
 */
export function badgeLabel(count: number, enabled: boolean): string | null {
  if (!enabled || count <= 0) return null
  return String(count)
}

/**
 * 平台无关的 badge 动作。macOS 显示未读数字；Windows 任务栏叠加图标只有 ~16px，
 * 放不下数字，故仅显示一个红点（见 useDockBadge 注释）。
 */
export type BadgeAction =
  | { kind: 'remove' }
  | { kind: 'number'; label: string }
  | { kind: 'dot' }

/**
 * 依据平台、未读数与开关，算出应对 Dock/任务栏做的 badge 操作。
 * - 开关关闭或未读为 0：移除。
 * - Windows：显示红点（不显示数字）。
 * - macOS：显示未读数字。
 */
export function badgeAction(
  platform: Platform,
  count: number,
  enabled: boolean,
): BadgeAction {
  const label = badgeLabel(count, enabled)
  if (label === null) return { kind: 'remove' }
  if (platform === 'windows') return { kind: 'dot' }
  return { kind: 'number', label }
}

/**
 * Windows 任务栏红点的自定义 badge 选项。
 *
 * Wails 默认在红圈中心画一个白点（createBadgeIcon），把「文字色」也设为红即可
 * 抹掉白点，得到一个纯实心红点。字体相关字段在无文字（label=''）时不生效。
 */
const WINDOWS_RED_DOT = {
  TextColour: { R: 255, G: 0, B: 0, A: 255 },
  BackgroundColour: { R: 255, G: 0, B: 0, A: 255 },
  FontName: '',
  FontSize: 0,
  SmallFontSize: 0,
}

/**
 * 把未读文章总数同步到应用图标 badge。
 *
 * - 未读数来源为 SidebarStore.feeds（单一数据源，随新文章/已读变化自动刷新）。
 * - 是否展示由设置项 showUnreadBadge 控制（默认开启）。
 * - 未读为 0 或开关关闭时移除 badge。
 * - macOS：Dock 图标显示未读数字。
 * - Windows：任务栏图标叠加一个红点（叠加图标仅 ~16px，放不下数字）。
 * - 其它情况该 Hook 为空操作。
 */
export function useDockBadge(): void {
  const platform = usePlatform()

  useEffect(() => {
    if (platform !== 'mac' && platform !== 'windows') return

    function sync(): void {
      const count = totalUnread(useSidebarStore.getState().feeds)
      const enabled = useSettingsStore.getState().settings?.showUnreadBadge ?? true
      const action = badgeAction(platform as Platform, count, enabled)
      let promise: ReturnType<typeof DockService.RemoveBadge>
      switch (action.kind) {
        case 'remove':
          promise = DockService.RemoveBadge()
          break
        case 'dot':
          promise = DockService.SetCustomBadge('', WINDOWS_RED_DOT)
          break
        case 'number':
          promise = DockService.SetBadge(action.label)
          break
      }
      // 忽略 badge 更新失败（不影响主流程）。
      promise.catch(() => {})
    }

    // 立即用当前状态刷新一次。
    sync()

    // 未读数变化 → 重算 badge。
    const offSidebar = useSidebarStore.subscribe((state, prev) => {
      if (totalUnread(state.feeds) !== totalUnread(prev.feeds)) sync()
    })
    // 开关变化 → 重算 badge。
    const offSettings = useSettingsStore.subscribe((state, prev) => {
      if (state.settings?.showUnreadBadge !== prev.settings?.showUnreadBadge) sync()
    })

    return () => {
      offSidebar()
      offSettings()
    }
  }, [platform])
}
