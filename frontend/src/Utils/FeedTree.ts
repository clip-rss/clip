// 把扁平的「分类列表 + 订阅源列表」组装成左侧栏需要的多级树结构。
// 纯函数，无副作用，便于单元测试。

import type { Category, FeedWithUnread, FeedTree, FeedTreeNode } from '../Types'

/** 分类排序：先按 sortOrder 升序，相同则按名称本地化比较。 */
function compareCategory(a: Category, b: Category): number {
  if (a.sortOrder !== b.sortOrder) return a.sortOrder - b.sortOrder
  return a.name.localeCompare(b.name)
}

/** 订阅源排序：按标题本地化比较（无持久化 sortOrder 字段）。 */
function compareFeed(a: FeedWithUnread, b: FeedWithUnread): number {
  return a.title.localeCompare(b.title)
}

/**
 * 连续失败达到该次数的订阅源视为「已失效」。
 * 后端按 2ⁿ 指数退避重试（封顶 24h），默认 30 min 间隔下约 3.5 h 连续失败触达阈值；
 * 任意一次成功即清零计数出组，因此误伤（服务端临时故障）会自愈。
 */
export const DEAD_FEED_THRESHOLD = 3

/** 是否归入「已失效」分组。暂停的源不再被抓取、计数冻结，不参与判死。 */
export function isDeadFeed(f: FeedWithUnread): boolean {
  return f.status !== 'paused' && f.errorCount >= DEAD_FEED_THRESHOLD
}

/**
 * 构建左侧栏树。
 * - 分类按 parentId 形成多级嵌套；parentId 为 null 或指向不存在的父级时视为根。
 * - 连续失败的源归入 dead 并从分类/未分类中移出（恢复后自动回到原位）。
 * - 其余订阅源 categoryId 为 null/0 归入 uncategorized。
 * - 每个分类节点的 unreadCount 递归累加自身直属源与全部子孙分类的未读数。
 */
export function buildFeedTree(
  categories: Category[],
  feeds: FeedWithUnread[],
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

  // 订阅源按分类分组（死源移出，null/0 视为未分类）
  const feedsOf = new Map<number, FeedWithUnread[]>()
  const uncategorized: FeedWithUnread[] = []
  const dead: FeedWithUnread[] = []
  for (const f of feeds) {
    if (isDeadFeed(f)) {
      dead.push(f)
      continue
    }
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
    const ownFeeds = (feedsOf.get(category.id) ?? []).slice().sort(compareFeed)
    const childCats = (childrenOf.get(category.id) ?? [])
      .slice()
      .sort(compareCategory)
      .filter((c) => !visited.has(c.id)) // 防御性：避免环导致的无限递归
    const children = childCats.map(buildNode)

    let unreadCount = 0
    for (const f of ownFeeds) unreadCount += f.unreadCount
    for (const child of children) unreadCount += child.unreadCount

    return { category, children, feeds: ownFeeds, unreadCount }
  }

  const roots = (childrenOf.get(0) ?? [])
    .slice()
    .sort(compareCategory)
    .map(buildNode)

  let totalUnread = 0
  for (const f of feeds) totalUnread += f.unreadCount

  return {
    roots,
    uncategorized: uncategorized.sort(compareFeed),
    dead: dead.sort(compareFeed),
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
