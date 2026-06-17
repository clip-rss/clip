// 文章列表的客户端筛选与排序（纯函数，便于测试）。
// 后端无「已读/今日/按分类」文章接口，故这些维度在前端计算。

import type {
  ArticleFilter,
  ArticleSort,
  Category,
  FeedWithUnread,
  Item,
} from '../Types'

function publishedMs(item: Item): number {
  const t = new Date(item.publishedAt as unknown as string).getTime()
  return Number.isNaN(t) ? 0 : t
}

/** 收集某分类及其全部子孙分类下的 feedId 集合（用于按分类过滤文章）。 */
export function categoryFeedIds(
  categories: Category[],
  feeds: FeedWithUnread[],
  categoryId: number,
): Set<number> {
  const childrenOf = new Map<number, number[]>()
  for (const c of categories) {
    if (c.parentId === null) continue
    const arr = childrenOf.get(c.parentId)
    if (arr) arr.push(c.id)
    else childrenOf.set(c.parentId, [c.id])
  }

  const catIds = new Set<number>()
  const stack: number[] = [categoryId]
  while (stack.length > 0) {
    const id = stack.pop() as number
    if (catIds.has(id)) continue
    catIds.add(id)
    for (const child of childrenOf.get(id) ?? []) stack.push(child)
  }

  const feedIds = new Set<number>()
  for (const f of feeds) {
    if (f.categoryId !== null && catIds.has(f.categoryId)) feedIds.add(f.id)
  }
  return feedIds
}

export interface FilterSortOptions {
  filter: ArticleFilter
  sort: ArticleSort
  /** 限定的 feedId 集合（按分类选中时传入）；null/undefined 表示不按源限定。 */
  allowedFeedIds?: Set<number> | null
  /** feedId → 源名，排序 'source' 时使用。 */
  feedTitleOf?: (feedId: number) => string
  /** 当前时间，默认 new Date()（便于测试注入）。 */
  now?: Date
}

/** 按筛选维度过滤、按排序维度排序，返回新数组。 */
export function filterAndSortItems(items: Item[], opts: FilterSortOptions): Item[] {
  const { filter, sort, allowedFeedIds = null, feedTitleOf, now = new Date() } = opts
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()

  const filtered = items.filter((it) => {
    if (allowedFeedIds && !allowedFeedIds.has(it.feedId)) return false
    switch (filter) {
      case 'unread':
        return !it.isRead
      case 'read':
        return it.isRead
      case 'starred':
        return it.isStarred
      case 'today':
        return publishedMs(it) >= startOfToday
      case 'all':
      default:
        return true
    }
  })

  const titleOf = feedTitleOf ?? (() => '')
  return filtered.sort((a, b) => {
    if (sort === 'source') {
      const cmp = titleOf(a.feedId).localeCompare(titleOf(b.feedId))
      if (cmp !== 0) return cmp
    }
    return publishedMs(b) - publishedMs(a) // 时间倒序（亦作 source 的次级排序）
  })
}
