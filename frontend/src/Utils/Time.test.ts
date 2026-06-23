import { describe, it, expect, beforeAll } from 'vitest'
import '../I18n'
import { formatRelativeTime, latestUpdated } from './Time'
import type { FeedWithUnread } from '../Types'

const NOW = new Date('2026-06-16T12:00:00Z')

function at(iso: string): Date {
  return new Date(iso)
}

describe('formatRelativeTime', () => {
  beforeAll(async () => {
    const i18next = await import('i18next')
    await i18next.default.changeLanguage('zh')
  })

  it('空值返回对应文案', () => {
    expect(formatRelativeTime(null, NOW)).toBe('从未')
    expect(formatRelativeTime(undefined, NOW)).toBe('从未')
    expect(formatRelativeTime('', NOW)).toBe('从未')
  })

  it('非法时间返回对应文案', () => {
    expect(formatRelativeTime('not-a-date', NOW)).toBe('从未')
  })

  it('未来或刚刚返回「刚刚」', () => {
    expect(formatRelativeTime(at('2026-06-16T12:00:30Z'), NOW)).toBe('刚刚')
    expect(formatRelativeTime(at('2026-06-16T11:59:30Z'), NOW)).toBe('刚刚')
  })

  it('分钟 / 小时 / 天', () => {
    expect(formatRelativeTime(at('2026-06-16T11:55:00Z'), NOW)).toBe('5 分钟前')
    expect(formatRelativeTime(at('2026-06-16T09:00:00Z'), NOW)).toBe('3 小时前')
    expect(formatRelativeTime(at('2026-06-14T12:00:00Z'), NOW)).toBe('2 天前')
  })

  it('超过 30 天显示具体日期', () => {
    expect(formatRelativeTime(at('2026-01-01T08:00:00Z'), NOW)).toBe('2026-01-01')
  })
})

function feedWith(lastUpdated: string | null): FeedWithUnread {
  return { id: 1, lastUpdated, unreadCount: 0 } as unknown as FeedWithUnread
}

describe('latestUpdated', () => {
  it('全空返回 null', () => {
    expect(latestUpdated([feedWith(null), feedWith(null)])).toBeNull()
    expect(latestUpdated([])).toBeNull()
  })

  it('取最近一次时间，忽略空与非法值', () => {
    const feeds = [
      feedWith('2026-06-01T00:00:00Z'),
      feedWith(null),
      feedWith('2026-06-10T00:00:00Z'),
      feedWith('invalid'),
    ]
    const result = latestUpdated(feeds)
    expect(result?.toISOString()).toBe('2026-06-10T00:00:00.000Z')
  })
})
