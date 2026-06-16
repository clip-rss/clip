// 时间格式化工具。后端 time.Time 序列化为 ISO 字符串，可直接 new Date 解析。

import type { FeedWithUnread } from '../Types'

const MINUTE = 60 * 1000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

/**
 * 相对时间文案（中文）：刚刚 / X 分钟前 / X 小时前 / X 天前 / 具体日期。
 * @param value 时间，可为 Date、ISO 字符串或 null。
 * @param now 当前时间，默认 new Date()（便于测试注入）。
 */
export function formatRelativeTime(value: Date | string | null | undefined, now: Date = new Date()): string {
  if (value === null || value === undefined || value === '') return '从未'
  const date = value instanceof Date ? value : new Date(value)
  const ms = date.getTime()
  if (Number.isNaN(ms)) return '从未'

  const diff = now.getTime() - ms
  if (diff < 0) return '刚刚'
  if (diff < MINUTE) return '刚刚'
  if (diff < HOUR) return `${Math.floor(diff / MINUTE)} 分钟前`
  if (diff < DAY) return `${Math.floor(diff / HOUR)} 小时前`
  if (diff < 30 * DAY) return `${Math.floor(diff / DAY)} 天前`

  // 超过 30 天显示具体日期
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

/** 取一批订阅源中最近一次成功更新的时间，全部为空时返回 null。 */
export function latestUpdated(feeds: FeedWithUnread[]): Date | null {
  let latest: number | null = null
  for (const f of feeds) {
    if (!f.lastUpdated) continue
    const ms = new Date(f.lastUpdated as unknown as string).getTime()
    if (Number.isNaN(ms)) continue
    if (latest === null || ms > latest) latest = ms
  }
  return latest === null ? null : new Date(latest)
}
