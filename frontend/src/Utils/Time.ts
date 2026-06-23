import i18next from 'i18next'
import type { FeedWithUnread } from '../Types'

const MINUTE = 60 * 1000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

export function formatRelativeTime(value: Date | string | null | undefined, now: Date = new Date()): string {
  const t = i18next.t.bind(i18next)

  if (value === null || value === undefined || value === '') return t('time.never')
  const date = value instanceof Date ? value : new Date(value)
  const ms = date.getTime()
  if (Number.isNaN(ms)) return t('time.never')

  const diff = now.getTime() - ms
  if (diff < 0) return t('time.justNow')
  if (diff < MINUTE) return t('time.justNow')
  if (diff < HOUR) return t('time.minutesAgo', { count: Math.floor(diff / MINUTE) })
  if (diff < DAY) return t('time.hoursAgo', { count: Math.floor(diff / HOUR) })
  if (diff < 30 * DAY) {
    const days = Math.floor(diff / DAY)
    return days === 1 ? t('time.daysAgo', { count: days }) : t('time.daysAgo', { count: days })
  }

  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

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
