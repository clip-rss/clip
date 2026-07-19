import { useEffect } from 'react'
import { useSettingsStore, useSidebarStore } from '../Stores'
import { DockService } from '../Utils'
import { usePlatform } from './usePlatform'

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
 * 在 macOS 上把未读文章总数同步到 Dock 图标 badge。
 *
 * - 未读数来源为 SidebarStore.feeds（单一数据源，随新文章/已读变化自动刷新）。
 * - 是否展示由设置项 showUnreadBadge 控制（默认开启）。
 * - 未读为 0 或开关关闭时移除 badge；否则显示数字。
 * - 仅 macOS 生效；其它平台该 Hook 为空操作。
 */
export function useDockBadge(): void {
  const platform = usePlatform()

  useEffect(() => {
    if (platform !== 'mac') return

    function sync(): void {
      const count = totalUnread(useSidebarStore.getState().feeds)
      const enabled = useSettingsStore.getState().settings?.showUnreadBadge ?? true
      const label = badgeLabel(count, enabled)
      const promise =
        label !== null ? DockService.SetBadge(label) : DockService.RemoveBadge()
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
