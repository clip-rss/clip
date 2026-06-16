// Wails 事件名与负载类型（与 Go internal/scheduler 中的常量保持一致）。

/** 新文章到达事件名（对应 scheduler.ItemsUpdatedEvent）。 */
export const ItemsUpdatedEvent = 'items:updated'

/** 订阅源抓取失败事件名（对应 scheduler.FeedErrorEvent）。 */
export const FeedErrorEvent = 'feed:error'

/** items:updated 事件负载：某订阅源新增了文章。 */
export interface ItemsUpdatedPayload {
  feedId: number
  newItems: number
}

/** feed:error 事件负载：某订阅源抓取失败。 */
export interface FeedErrorPayload {
  feedId: number
  error: string
}
