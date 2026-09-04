import { describe, it, expect } from 'vitest'
import {
  buildFeedTree,
  flattenCategories,
  compareFeedBy,
  isFeedErrored,
  erroredFeedIds,
} from './FeedTree'
import type { Category, FeedWithUnread, FeedSort } from '../Types'

function cat(
  id: number,
  name: string,
  parentId: number | null = null,
  sortOrder = 0,
): Category {
  return {
    id,
    name,
    parentId,
    sortOrder,
    createdAt: null,
    updatedAt: null,
  } as Category
}

function feed(
  id: number,
  title: string,
  categoryId: number | null,
  unreadCount: number,
  createdAt?: string | null,
): FeedWithUnread {
  return {
    id,
    url: '',
    title,
    description: '',
    link: '',
    icon: '',
    categoryId,
    updateInterval: 0,
    maxItems: 0,
    lastUpdated: null,
    errorCount: 0,
    lastError: null,
    status: 'active',
    createdAt: createdAt ?? null,
    updatedAt: null,
    unreadCount,
  } as FeedWithUnread
}

describe('buildFeedTree', () => {
  it('空输入返回空结构', () => {
    const tree = buildFeedTree([], [])
    expect(tree.roots).toEqual([])
    expect(tree.uncategorized).toEqual([])
    expect(tree.totalUnread).toBe(0)
  })

  it('categoryId 为 null 或 0 的源归入未分类', () => {
    const feeds = [feed(1, 'A', null, 2), feed(2, 'B', 0, 3)]
    const tree = buildFeedTree([], feeds)
    expect(tree.uncategorized).toHaveLength(2)
    expect(tree.roots).toEqual([])
    expect(tree.totalUnread).toBe(5)
  })

  it('指向不存在父级的分类被当作根节点', () => {
    const categories = [cat(1, '孤儿', 999)]
    const tree = buildFeedTree(categories, [])
    expect(tree.roots).toHaveLength(1)
    expect(tree.roots[0].category.id).toBe(1)
  })

  it('构建多级嵌套并递归累加未读计数', () => {
    const categories = [cat(1, '科技'), cat(2, '前端', 1)]
    const feeds = [
      feed(10, '直属科技源', 1, 4),
      feed(11, 'React 周刊', 2, 5),
      feed(12, '未分类源', null, 1),
    ]
    const tree = buildFeedTree(categories, feeds)

    expect(tree.roots).toHaveLength(1)
    const tech = tree.roots[0]
    expect(tech.category.id).toBe(1)
    expect(tech.feeds.map((f) => f.id)).toEqual([10])
    expect(tech.children).toHaveLength(1)

    const frontend = tech.children[0]
    expect(frontend.category.id).toBe(2)
    expect(frontend.unreadCount).toBe(5)

    // 科技 = 直属 4 + 子分类前端 5 = 9
    expect(tech.unreadCount).toBe(9)
    // 未分类不计入分类，但计入总数
    expect(tree.uncategorized).toHaveLength(1)
    expect(tree.totalUnread).toBe(10)
  })

  it('分类按 sortOrder 升序，其次按名称', () => {
    const categories = [
      cat(1, 'Beta', null, 2),
      cat(2, 'Alpha', null, 1),
      cat(3, 'Gamma', null, 1),
    ]
    const tree = buildFeedTree(categories, [])
    // sortOrder: Alpha(1) Gamma(1) Beta(2) → 同序按名称 Alpha<Gamma
    expect(tree.roots.map((n) => n.category.name)).toEqual([
      'Alpha',
      'Gamma',
      'Beta',
    ])
  })

  it('分类内的源按标题排序', () => {
    const categories = [cat(1, '分类')]
    const feeds = [feed(1, '香蕉', 1, 0), feed(2, '苹果', 1, 0)]
    const tree = buildFeedTree(categories, feeds)
    const titles = tree.roots[0].feeds.map((f) => f.title)
    expect(titles).toEqual(['苹果', '香蕉'].sort((a, b) => a.localeCompare(b)))
  })

  it('capacity/cappedUnread 只聚合设了保留上限的源', () => {
    const categories = [cat(1, '根'), cat(2, '子', 1)]
    const feeds = [
      { ...feed(10, '限量源', 1, 90), maxItems: 100 },
      { ...feed(11, '不限量子源', 2, 300), maxItems: 0 },
      { ...feed(12, '子分类限量源', 2, 40), maxItems: 50 },
    ]
    const tree = buildFeedTree(categories, feeds)

    const child = tree.roots[0].children[0]
    // 不限量源没有分母，分子分母都不计入
    expect(child.capacity).toBe(50)
    expect(child.cappedUnread).toBe(40)

    const root = tree.roots[0]
    // 根 = 直属(100/90) + 子分类(50/40)
    expect(root.capacity).toBe(150)
    expect(root.cappedUnread).toBe(130)
  })

  it('全部源都不限量时 capacity 与 cappedUnread 为 0', () => {
    const categories = [cat(1, '根')]
    const feeds = [feed(10, 'A', 1, 5), feed(11, 'B', 1, 6)]
    const tree = buildFeedTree(categories, feeds)
    expect(tree.roots[0].capacity).toBe(0)
    expect(tree.roots[0].cappedUnread).toBe(0)
  })
})

it('sortBy=created 时分类内源按订阅时间降序', () => {
  const categories = [cat(1, '分类')]
  const feeds = [
    feed(1, '旧源', 1, 0, '2024-01-01T00:00:00Z'),
    feed(2, '新源', 1, 0, '2025-06-15T00:00:00Z'),
  ]
  const tree = buildFeedTree(categories, feeds, 'created')
  const ids = tree.roots[0].feeds.map((f) => f.id)
  expect(ids).toEqual([2, 1])
})

it('sortBy=created 时未分类源按订阅时间降序', () => {
  const feeds = [
    feed(1, '旧源', null, 0, '2024-01-01T00:00:00Z'),
    feed(2, '新源', null, 0, '2025-06-15T00:00:00Z'),
  ]
  const tree = buildFeedTree([], feeds, 'created')
  expect(tree.uncategorized.map((f) => f.id)).toEqual([2, 1])
})

it('sortBy=unread 时分类内源按未读数降序', () => {
  const categories = [cat(1, '分类')]
  const feeds = [
    feed(1, '少', 1, 3),
    feed(2, '多', 1, 42),
    feed(3, '中', 1, 15),
  ]
  const tree = buildFeedTree(categories, feeds, 'unread')
  const ids = tree.roots[0].feeds.map((f) => f.id)
  expect(ids).toEqual([2, 3, 1])
})

it('sortBy=unread 时未分类源按未读数降序', () => {
  const feeds = [
    feed(1, '少', null, 3),
    feed(2, '多', null, 42),
    feed(3, '中', null, 15),
  ]
  const tree = buildFeedTree([], feeds, 'unread')
  expect(tree.uncategorized.map((f) => f.id)).toEqual([2, 3, 1])
})

it('compareFeedBy("default") 按标题字母序', () => {
  const feeds = [feed(1, '香蕉', null, 0), feed(2, '苹果', null, 0)]
  const sorted = [...feeds].sort(compareFeedBy('default'))
  expect(sorted[0].title).toBe('苹果')
  expect(sorted[1].title).toBe('香蕉')
})

it('compareFeedBy("created") 按订阅时间降序，无 createdAt 时排末尾', () => {
  const feeds = [
    feed(1, '无时间', null, 0, null),
    feed(2, '旧源', null, 0, '2024-01-01T00:00:00Z'),
    feed(3, '新源', null, 0, '2025-06-15T00:00:00Z'),
  ]
  const sorted = [...feeds].sort(compareFeedBy('created'))
  expect(sorted.map((f) => f.title)).toEqual(['新源', '旧源', '无时间'])
})

it('compareFeedBy("unread") 按未读数降序', () => {
  const feeds = [
    feed(1, '少', null, 3),
    feed(2, '多', null, 42),
    feed(3, '中', null, 15),
  ]
  const sorted = [...feeds].sort(compareFeedBy('unread'))
  expect(sorted.map((f) => f.title)).toEqual(['多', '中', '少'])
})

describe('flattenCategories', () => {
  it('空输入返回空数组', () => {
    expect(flattenCategories([])).toEqual([])
  })

  it('按前序展开并标注层级深度', () => {
    // 子分类用 sortOrder 决定顺序，避免依赖区域排序规则。
    const categories = [
      cat(2, 'B 子', 1, 2),
      cat(1, '根', null, 0),
      cat(3, 'A 子', 1, 1),
    ]
    const flat = flattenCategories(categories)
    expect(flat).toEqual([
      { id: 1, name: '根', depth: 0 },
      { id: 3, name: 'A 子', depth: 1 },
      { id: 2, name: 'B 子', depth: 1 },
    ])
  })

  it('指向不存在父级的分类视为根', () => {
    const flat = flattenCategories([cat(1, '孤儿', 999)])
    expect(flat).toEqual([{ id: 1, name: '孤儿', depth: 0 }])
  })
})

describe('isFeedErrored / erroredFeedIds', () => {
  it('errorCount>0 且 lastError 非空视为异常', () => {
    const f = {
      ...feed(1, 'A', null, 0),
      errorCount: 3,
      lastError: 'connection refused',
    }
    expect(isFeedErrored(f)).toBe(true)
  })

  it('status=error 为历史遗留口径，同样视为异常', () => {
    const f = { ...feed(1, 'A', null, 0), status: 'error' }
    expect(isFeedErrored(f)).toBe(true)
  })

  it('errorCount>0 但 lastError 为空不视为异常', () => {
    const f = { ...feed(1, 'A', null, 0), errorCount: 2, lastError: null }
    expect(isFeedErrored(f)).toBe(false)
  })

  it('成功刷新后错误清零的源不视为异常', () => {
    const f = { ...feed(1, 'A', null, 0), errorCount: 0, lastError: '' }
    expect(isFeedErrored(f)).toBe(false)
  })

  it('暂停中的异常源仍视为异常', () => {
    const f = {
      ...feed(1, 'A', null, 0),
      status: 'paused',
      errorCount: 1,
      lastError: 'timeout',
    }
    expect(isFeedErrored(f)).toBe(true)
  })

  it('erroredFeedIds 只保留异常源的 id', () => {
    const feeds = [
      feed(1, '正常', null, 0),
      { ...feed(2, '异常', null, 0), errorCount: 1, lastError: '404' },
      { ...feed(3, '遗留', null, 0), status: 'error' as const },
      {
        ...feed(4, '暂停异常', null, 0),
        status: 'paused' as const,
        errorCount: 5,
        lastError: 'x',
      },
    ]
    expect(erroredFeedIds(feeds)).toEqual([2, 3, 4])
    expect(erroredFeedIds([])).toEqual([])
  })
})
