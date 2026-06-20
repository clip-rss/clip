import { useEffect } from 'react'
import { useArticleStore, useSidebarStore } from '../Stores'
import { ItemService, onNotificationOpen } from '../Utils'
import type { NotificationOpenPayload } from '../Types'

/**
 * 订阅点击通知事件：查找文章所属源，切换侧栏选中，等待列表加载后自动定位。
 */
export function useNotificationNavigation(): void {
  useEffect(() => {
    const off = onNotificationOpen(async (payload: NotificationOpenPayload) => {
      // 查找文章确定其所属订阅源。
      const item = await ItemService.GetItem(payload.articleId)
      if (!item) return
      // 预设定位 ID；load 完成后自动消费。
      useArticleStore.getState().scheduleSelect(item.id)
      // 切换到该文章的订阅源列表，触发 ArticleList 拉取。
      useSidebarStore.getState().select({ kind: 'feed', id: item.feedId })
    })
    return off
  }, [])
}
