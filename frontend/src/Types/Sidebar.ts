// 左侧栏相关类型定义。

import type { Category, FeedWithUnread } from './Models'

/** 左侧栏当前选中项。 */
export type Selection =
  | { kind: 'all' }
  | { kind: 'feed'; id: number }
  | { kind: 'category'; id: number }

/** 文件夹树节点：一个分类及其子分类、直属订阅源与递归未读计数。 */
export interface FeedTreeNode {
  category: Category
  children: FeedTreeNode[]
  feeds: FeedWithUnread[]
  /** 自身直属源未读 + 所有子分类未读，递归累加。 */
  unreadCount: number
  /**
   * badge 负载配色的容量口径：直属源与子孙分类中**设了保留上限**（maxItems>0）
   * 的源的 maxItems 之和，递归累加。不限制（maxItems=0）的源不计入。
   */
  capacity: number
  /** 上述计入容量的那些源的未读之和，与 capacity 构成文件夹 badge 的负载分子分母。 */
  cappedUnread: number
}

/** 构建后的左侧栏树结构。 */
export interface FeedTree {
  /** 根级分类节点（parentId 为 null）。 */
  roots: FeedTreeNode[]
  /** 未归类的订阅源（categoryId 为 null/0）。 */
  uncategorized: FeedWithUnread[]
  /** 全部订阅源的未读总数。 */
  totalUnread: number
}
