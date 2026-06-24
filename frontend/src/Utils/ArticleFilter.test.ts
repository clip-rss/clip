import { describe, it, expect } from 'vitest'
import {
  categoryFeedIds,
  filterAndSortItems,
  neighborItemId,
} from './ArticleFilter'
import type { Category, FeedWithUnread, Item } from '../Types'

function cat(id: number, parentId: number | null = null): Category {
  return {
    id,
    name: `c${id}`,
    parentId,
    sortOrder: 0,
    createdAt: null,
    updatedAt: null,
  } as Category
}

function feed(id: number, categoryId: number | null): FeedWithUnread {
  return {
    id,
    title: `f${id}`,
    categoryId,
    unreadCount: 0,
  } as unknown as FeedWithUnread
}

interface ItemOpts {
  title?: string
  publishedAt?: string | null
  isRead?: boolean
  isStarred?: boolean
}

function item(id: number, feedId: number, opts: ItemOpts = {}): Item {
  return {
    id,
    feedId,
    title: opts.title ?? `t${id}`,
    author: '',
    publishedAt: opts.publishedAt ?? null,
    updatedAt: null,
    url: '',
    content: '',
    summary: '',
    enclosure: '',
    categories: '',
    isRead: opts.isRead ?? false,
    isStarred: opts.isStarred ?? false,
    readAt: null,
    note: '',
    createdAt: null,
  } as unknown as Item
}

describe('categoryFeedIds', () => {
  it('收集分类及其子孙分类下的 feedId', () => {
    const categories = [cat(1), cat(2, 1), cat(3, 2), cat(4)]
    const feeds = [
      feed(10, 1),
      feed(11, 2),
      feed(12, 3),
      feed(13, 4),
      feed(14, null),
    ]
    // 分类 1 → 含子 2、孙 3
    const ids = categoryFeedIds(categories, feeds, 1)
    expect([...ids].sort()).toEqual([10, 11, 12])
  })

  it('叶子分类只含直属源', () => {
    const categories = [cat(1), cat(2, 1)]
    const feeds = [feed(10, 1), feed(11, 2)]
    expect([...categoryFeedIds(categories, feeds, 2)]).toEqual([11])
  })
})

const NOW = new Date('2026-06-17T12:00:00Z')

describe('filterAndSortItems', () => {
  const items = [
    item(1, 100, {
      isRead: false,
      isStarred: true,
      publishedAt: '2026-06-17T08:00:00Z',
    }),
    item(2, 100, {
      isRead: true,
      isStarred: false,
      publishedAt: '2026-06-16T08:00:00Z',
    }),
    item(3, 200, {
      isRead: false,
      isStarred: false,
      publishedAt: '2026-06-10T08:00:00Z',
    }),
  ]

  it('unread 只留未读', () => {
    const r = filterAndSortItems(items, {
      filter: 'unread',
      sort: 'time',
      now: NOW,
    })
    expect(r.map((i) => i.id)).toEqual([1, 3])
  })

  it('read 只留已读', () => {
    const r = filterAndSortItems(items, {
      filter: 'read',
      sort: 'time',
      now: NOW,
    })
    expect(r.map((i) => i.id)).toEqual([2])
  })

  it('starred 只留星标', () => {
    const r = filterAndSortItems(items, {
      filter: 'starred',
      sort: 'time',
      now: NOW,
    })
    expect(r.map((i) => i.id)).toEqual([1])
  })

  it('today 只留今天发布', () => {
    const r = filterAndSortItems(items, {
      filter: 'today',
      sort: 'time',
      now: NOW,
    })
    expect(r.map((i) => i.id)).toEqual([1])
  })

  it('time 排序按发布时间倒序', () => {
    const r = filterAndSortItems(items, {
      filter: 'all',
      sort: 'time',
      now: NOW,
    })
    expect(r.map((i) => i.id)).toEqual([1, 2, 3])
  })

  it('allowedFeedIds 限定来源范围', () => {
    const r = filterAndSortItems(items, {
      filter: 'all',
      sort: 'time',
      allowedFeedIds: new Set([200]),
      now: NOW,
    })
    expect(r.map((i) => i.id)).toEqual([3])
  })

  it('source 排序按源名升序、其次时间倒序', () => {
    const titleOf = (id: number) => (id === 100 ? 'B源' : 'A源')
    const r = filterAndSortItems(items, {
      filter: 'all',
      sort: 'source',
      feedTitleOf: titleOf,
      now: NOW,
    })
    // A源(feed200)=id3 在前，B源(feed100)=id1,2（时间倒序）
    expect(r.map((i) => i.id)).toEqual([3, 1, 2])
  })
})

describe('neighborItemId', () => {
  // 有序（时间倒序）：[1, 2, 3, 4]
  const ordered = [
    item(1, 100, { publishedAt: '2026-06-17T08:00:00Z' }),
    item(2, 100, { publishedAt: '2026-06-16T08:00:00Z' }),
    item(3, 100, { publishedAt: '2026-06-15T08:00:00Z' }),
    item(4, 100, { publishedAt: '2026-06-14T08:00:00Z' }),
  ]
  const allVisible = new Set([1, 2, 3, 4])

  it('下一篇 / 上一篇（全部可见）', () => {
    expect(neighborItemId(ordered, allVisible, 2, 1)).toBe(3)
    expect(neighborItemId(ordered, allVisible, 2, -1)).toBe(1)
  })

  it('到达边界返回 null', () => {
    expect(neighborItemId(ordered, allVisible, 1, -1)).toBeNull()
    expect(neighborItemId(ordered, allVisible, 4, 1)).toBeNull()
  })

  it('跳过不可见项（候选集之外）', () => {
    const candidates = new Set([1, 4]) // 2、3 不可见
    expect(neighborItemId(ordered, candidates, 1, 1)).toBe(4)
    expect(neighborItemId(ordered, candidates, 4, -1)).toBe(1)
  })

  it('当前项已退出候选集时仍按原位置定位下一可见项', () => {
    // 当前 2 已读，候选集为剩余未读 [1, 3, 4]
    const candidates = new Set([1, 3, 4])
    expect(neighborItemId(ordered, candidates, 2, 1)).toBe(3)
    expect(neighborItemId(ordered, candidates, 2, -1)).toBe(1)
  })

  it('currentId 为 null 时取方向起点首个候选', () => {
    expect(neighborItemId(ordered, allVisible, null, 1)).toBe(1)
    expect(neighborItemId(ordered, allVisible, null, -1)).toBe(4)
  })

  it('currentId 不在有序列表中返回 null', () => {
    expect(neighborItemId(ordered, allVisible, 99, 1)).toBeNull()
  })
})
