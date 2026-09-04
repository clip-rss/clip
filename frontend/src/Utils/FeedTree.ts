// 把扁平的「分类列表 + 订阅源列表」组装成左侧栏需要的多级树结构。
// 纯函数，无副作用，便于单元测试。

import type {
  Category,
  FeedWithUnread,
  FeedTree,
  FeedTreeNode,
  FeedSort,
} from '../Types'

/** 分类排序：先按 sortOrder 升序，相同则按名称本地化比较。 */
function compareCategory(a: Category, b: Category): number {
  if (a.sortOrder !== b.sortOrder) return a.sortOrder - b.sortOrder
  return a.name.localeCompare(b.name)
}

/** 订阅源排序：按标题本地化比较（默认排序）。 */
function compareFeed(a: FeedWithUnread, b: FeedWithUnread): number {
  return a.title.localeCompare(b.title)
}

/**
 * 根据指定排序方式返回对应的订阅源比较器。
 * - 'default'：按标题字母序
 * - 'created'：按订阅时间降序（最新在前）
 * - 'unread'：按未读数降序（最多在前）
 */
export function compareFeedBy(
  sortBy: FeedSort,
): (a: FeedWithUnread, b: FeedWithUnread) => number {
  switch (sortBy) {
    case 'created':
      return (a, b) => {
        const ta = a.createdAt ? new Date(a.createdAt).getTime() : 0
        const tb = b.createdAt ? new Date(b.createdAt).getTime() : 0
        return tb - ta // 降序，最新在前
      }
    case 'unread':
      return (a, b) => b.unreadCount - a.unreadCount // 降序，最多在前
    default:
      return compareFeed
  }
}

/**
 * 构建左侧栏树。
 * - 分类按 parentId 形成多级嵌套；parentId 为 null 或指向不存在的父级时视为根。
 * - 订阅源 categoryId 为 null/0 归入 uncategorized。
 * - 每个分类节点的 unreadCount 递归累加自身直属源与全部子孙分类的未读数。
 * - sortBy 控制订阅源在分类内和未分类区的排序方式，默认按标题字母序。
 */
export function buildFeedTree(
  categories: Category[],
  feeds: FeedWithUnread[],
  sortBy: FeedSort = 'default',
): FeedTree {
  // 分类按父级分组
  const childrenOf = new Map<number, Category[]>()
  const validIds = new Set<number>()
  for (const c of categories) validIds.add(c.id)
  for (const c of categories) {
    const parent =
      c.parentId !== null && validIds.has(c.parentId) ? c.parentId : 0
    const list = childrenOf.get(parent)
    if (list) list.push(c)
    else childrenOf.set(parent, [c])
  }

  // 订阅源按分类分组（null/0 视为未分类）
  const feedsOf = new Map<number, FeedWithUnread[]>()
  const uncategorized: FeedWithUnread[] = []
  for (const f of feeds) {
    const cid = f.categoryId
    if (cid === null || cid === 0 || !validIds.has(cid)) {
      uncategorized.push(f)
      continue
    }
    const list = feedsOf.get(cid)
    if (list) list.push(f)
    else feedsOf.set(cid, [f])
  }

  const visited = new Set<number>()

  function buildNode(category: Category): FeedTreeNode {
    visited.add(category.id)
    const ownFeeds = (feedsOf.get(category.id) ?? [])
      .slice()
      .sort(compareFeedBy(sortBy))
    const childCats = (childrenOf.get(category.id) ?? [])
      .slice()
      .sort(compareCategory)
      .filter((c) => !visited.has(c.id)) // 防御性：避免环导致的无限递归
    const children = childCats.map(buildNode)

    let unreadCount = 0
    for (const f of ownFeeds) unreadCount += f.unreadCount
    for (const child of children) unreadCount += child.unreadCount

    // badge 负载口径：只统计设了保留上限的源（maxItems>0），不限制的源没有
    // 分母，不计入；否则一个不限量的源就能把文件夹负载永远顶满。
    let capacity = 0
    let cappedUnread = 0
    for (const f of ownFeeds) {
      if (f.maxItems > 0) {
        capacity += f.maxItems
        cappedUnread += f.unreadCount
      }
    }
    for (const child of children) {
      capacity += child.capacity
      cappedUnread += child.cappedUnread
    }

    return {
      category,
      children,
      feeds: ownFeeds,
      unreadCount,
      capacity,
      cappedUnread,
    }
  }

  const roots = (childrenOf.get(0) ?? [])
    .slice()
    .sort(compareCategory)
    .map(buildNode)

  let totalUnread = 0
  for (const f of feeds) totalUnread += f.unreadCount

  return {
    roots,
    uncategorized: uncategorized.sort(compareFeedBy(sortBy)),
    totalUnread,
  }
}

/** 归属文件夹下拉用的分类项：按树前序展开，depth 用于缩进。 */
export interface CategoryOption {
  id: number
  name: string
  depth: number
}

/**
 * 将分类列表按父子层级前序展开为带缩进深度的扁平选项，供归属文件夹下拉使用。
 * 根分类（parentId 为 null 或指向不存在父级）depth 为 0，子分类依次递增。
 */
export function flattenCategories(categories: Category[]): CategoryOption[] {
  const validIds = new Set<number>()
  for (const c of categories) validIds.add(c.id)

  const childrenOf = new Map<number, Category[]>()
  for (const c of categories) {
    const parent =
      c.parentId !== null && validIds.has(c.parentId) ? c.parentId : 0
    const list = childrenOf.get(parent)
    if (list) list.push(c)
    else childrenOf.set(parent, [c])
  }

  const out: CategoryOption[] = []
  const visited = new Set<number>()
  function walk(parentId: number, depth: number): void {
    const children = (childrenOf.get(parentId) ?? [])
      .slice()
      .sort(compareCategory)
    for (const c of children) {
      if (visited.has(c.id)) continue // 防御性：避免环
      visited.add(c.id)
      out.push({ id: c.id, name: c.name, depth })
      walk(c.id, depth + 1)
    }
  }
  walk(0, 0)
  return out
}

/**
 * 判定订阅源是否处于异常状态（侧栏 ⚠ 标记同口径）。
 * status==='error' 是历史遗留值，现行代码只会写 active/paused；
 * 实际异常信号是 errorCount>0 且 lastError 非空（RecordFeedFailure 写入，成功后清零）。
 */
export function isFeedErrored(feed: FeedWithUnread): boolean {
  return feed.status === 'error' || (feed.errorCount > 0 && !!feed.lastError)
}

/** 全部异常订阅源的 id 列表（「批量删除异常订阅源」的筛选口径）。 */
export function erroredFeedIds(feeds: FeedWithUnread[]): number[] {
  return feeds.filter(isFeedErrored).map((f) => f.id)
}
