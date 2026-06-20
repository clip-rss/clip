import { Events } from '@wailsio/runtime'

import {
  ItemsUpdatedEvent,
  FeedErrorEvent,
  NotificationOpenEvent,
  type ItemsUpdatedPayload,
  type FeedErrorPayload,
  type NotificationOpenPayload,
} from '../../Types/Events'

/**
 * 订阅“新文章到达”事件。
 * @returns 取消订阅函数，组件卸载时调用。
 */
export function onItemsUpdated(handler: (payload: ItemsUpdatedPayload) => void): () => void {
  return Events.On(ItemsUpdatedEvent, (ev) => handler(ev.data as ItemsUpdatedPayload))
}

/**
 * 订阅“订阅源抓取失败”事件。
 * @returns 取消订阅函数，组件卸载时调用。
 */
export function onFeedError(handler: (payload: FeedErrorPayload) => void): () => void {
  return Events.On(FeedErrorEvent, (ev) => handler(ev.data as FeedErrorPayload))
}

/** 订阅「点击通知」事件，返回取消订阅函数。 */
export function onNotificationOpen(handler: (payload: NotificationOpenPayload) => void): () => void {
  return Events.On(NotificationOpenEvent, (ev) => handler(ev.data as NotificationOpenPayload))
}
