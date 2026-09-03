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
  /** 当前时间，默认 new Date()（便于测试注入）。 */
  now?: Date
}

/** 按筛选维度过滤、按时间方向排序，返回新数组。 */
export function filterAndSortItems(
  items: Item[],
  opts: FilterSortOptions,
): Item[] {
  const { filter, sort, allowedFeedIds = null, now = new Date() } = opts
  const startOfToday = new Date(
    now.getFullYear(),
    now.getMonth(),
    now.getDate(),
  ).getTime()

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

  return filtered.sort((a, b) =>
    sort === 'timeAsc'
      ? publishedMs(a) - publishedMs(b)
      : publishedMs(b) - publishedMs(a),
  )
}

/**
 * 在有序文章列表中，从 `currentId` 出发按方向寻找相邻的「候选」文章 id（专注模式 J/K 导航）。
 *
 * - `ordered`：范围内的完整有序列表（不应用读/未读等筛选），以保证 `currentId` 位置稳定——
 *   即便当前文章因刚被标记已读而退出可见集，仍能据其原位置找到下一篇。
 * - `candidateIds`：当前筛选下可见的 id 集合，作为落点的合法范围。
 * - `currentId` 为 `null` 时返回方向起点的首个候选（`dir>0` 取首个，`dir<0` 取末个）。
 *
 * 找不到返回 `null`。
 */
export function neighborItemId(
  ordered: Item[],
  candidateIds: Set<number>,
  currentId: number | null,
  dir: 1 | -1,
): number | null {
  if (currentId === null) {
    const seq = dir > 0 ? ordered : [...ordered].reverse()
    for (const it of seq) {
      if (candidateIds.has(it.id)) return it.id
    }
    return null
  }

  const idx = ordered.findIndex((it) => it.id === currentId)
  if (idx === -1) return null
  for (let i = idx + dir; i >= 0 && i < ordered.length; i += dir) {
    if (candidateIds.has(ordered[i].id)) return ordered[i].id
  }
  return null
}

/**
 * 选中文章查找：先查常规列表，再查搜索结果。
 * 搜索模式下选中的文章只存在于 searchResults（见 ArticleStore.patchItem），
 * 只查 items 会导致点击搜索结果后阅读栏一直显示空态。
 */
export function findSelectedItem(
  items: Item[],
  searchResults: Item[],
  selectedId: number | null,
): Item | null {
  if (selectedId === null) return null
  return (
    items.find((it) => it.id === selectedId) ??
    searchResults.find((it) => it.id === selectedId) ??
    null
  )
}
