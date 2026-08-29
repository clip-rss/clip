import { describe, it, expect } from 'vitest'
import { buildFeedTree, flattenCategories } from './FeedTree'
import type { Category, FeedWithUnread } from '../Types'

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
    createdAt: null,
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
