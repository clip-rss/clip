import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { CategoryService, FeedService, toApiError } from '../Utils'
import type { Category, FeedWithUnread, Selection } from '../Types'

interface SidebarState {
  categories: Category[]
  feeds: FeedWithUnread[]
  selection: Selection
  /** 已展开的分类 id 集合（持久化）。 */
  expanded: Set<number>
  /** 批量勾选（复选框多选）的订阅源 id 集合，与单选 selection 解耦。 */
  multiSelectIds: Set<number>
  /** 批量选择模式：开启后 feed 行显示复选框，顶部出现「删除(n)/取消」。 */
  batchMode: boolean
  loading: boolean
  error: string | null

  /** 并发拉取分类与订阅源（含未读计数）。 */
  load: () => Promise<void>
  select: (selection: Selection) => void
  toggleExpand: (categoryId: number) => void
  isExpanded: (categoryId: number) => boolean

  toggleMultiSelect: (id: number) => void
  /** 进入批量选择模式（重置勾选）。 */
  enterBatchMode: () => void
  /** 退出批量选择模式（清空勾选）。 */
  exitBatchMode: () => void

  addCategory: (name: string) => Promise<void>
  renameCategory: (id: number, name: string) => Promise<void>
  deleteCategory: (id: number) => Promise<void>

  renameFeed: (id: number, title: string) => Promise<void>
  deleteFeed: (id: number) => Promise<void>
  /** 批量删除订阅源；不引入后端批量接口，循环调单条 DeleteFeed。 */
  deleteFeeds: (ids: number[]) => Promise<void>
  pauseFeed: (id: number) => Promise<void>
  resumeFeed: (id: number) => Promise<void>
  refreshFeed: (id: number) => Promise<void>
  /** 把订阅源移入分类；categoryId 为 0 表示移出到「未分类」。 */
  moveFeed: (feedId: number, categoryId: number) => Promise<void>

  /** 刷新当前选中源（条件 GET）；非单源选中时刷新全部。 */
  refreshSelected: () => Promise<void>
  /** 强制全量刷新（忽略条件 GET）。 */
  forceRefreshAll: () => Promise<void>
}

export const useSidebarStore = create<SidebarState>()(
  persist(
    (set, get) => ({
      categories: [],
      feeds: [],
      selection: { kind: 'all' },
      expanded: new Set<number>(),
      multiSelectIds: new Set<number>(),
      batchMode: false,
      loading: false,
      error: null,

      async load() {
        set({ loading: true, error: null })
        try {
          const [categories, feeds] = await Promise.all([
            CategoryService.ListCategories(),
            FeedService.ListFeedsWithUnread(),
          ])
          set({
            categories: categories ?? [],
            feeds: feeds ?? [],
            loading: false,
          })
        } catch (err) {
          set({ error: toApiError(err), loading: false })
        }
      },

      select(selection) {
        set({ selection })
      },

      toggleExpand(categoryId) {
        const next = new Set(get().expanded)
        if (next.has(categoryId)) next.delete(categoryId)
        else next.add(categoryId)
        set({ expanded: next })
      },

      isExpanded(categoryId) {
        return get().expanded.has(categoryId)
      },

      toggleMultiSelect(id) {
        const next = new Set(get().multiSelectIds)
        if (next.has(id)) next.delete(id)
        else next.add(id)
        set({ multiSelectIds: next })
      },

      enterBatchMode() {
        set({ batchMode: true, multiSelectIds: new Set<number>() })
      },

      exitBatchMode() {
        set({ batchMode: false, multiSelectIds: new Set<number>() })
      },

      async addCategory(name) {
        const trimmed = name.trim()
        if (!trimmed) return
        try {
          await CategoryService.AddCategory(trimmed, 0)
          await get().load()
        } catch (err) {
          set({ error: toApiError(err) })
        }
      },

      async renameCategory(id, name) {
        const trimmed = name.trim()
        if (!trimmed) return
        const category = get().categories.find((c) => c.id === id)
        if (!category) return
        try {
          await CategoryService.UpdateCategory({ ...category, name: trimmed })
          await get().load()
        } catch (err) {
          set({ error: toApiError(err) })
        }
      },

      async deleteCategory(id) {
        try {
          await CategoryService.DeleteCategory(id)
          resetSelectionIfMatches(get, set, 'category', id)
          await get().load()
        } catch (err) {
          set({ error: toApiError(err) })
        }
      },

      async renameFeed(id, title) {
        const trimmed = title.trim()
        if (!trimmed) return
        const feed = get().feeds.find((f) => f.id === id)
        if (!feed) return
        try {
          // UpdateFeed 接收完整 Feed；FeedWithUnread 是其超集（多 unreadCount），后端按字段取用。
          await FeedService.UpdateFeed({ ...feed, title: trimmed })
          await get().load()
        } catch (err) {
          set({ error: toApiError(err) })
        }
      },

      async deleteFeed(id) {
        try {
          await FeedService.DeleteFeed(id)
          resetSelectionIfMatches(get, set, 'feed', id)
          await get().load()
        } catch (err) {
          set({ error: toApiError(err) })
        }
      },

      async deleteFeeds(ids) {
        try {
          await Promise.all(ids.map((id) => FeedService.DeleteFeed(id)))
          resetSelectionIfMatchesAny(get, set, ids)
          await get().load()
          set({ batchMode: false, multiSelectIds: new Set<number>() })
        } catch (err) {
          set({ error: toApiError(err) })
        }
      },

      async pauseFeed(id) {
        try {
          await FeedService.PauseFeed(id)
          await get().load()
        } catch (err) {
          set({ error: toApiError(err) })
        }
      },

      async resumeFeed(id) {
        try {
          await FeedService.ResumeFeed(id)
          await get().load()
        } catch (err) {
          set({ error: toApiError(err) })
        }
      },

      async refreshFeed(id) {
        try {
          await FeedService.RefreshFeed(id)
          await get().load()
        } catch (err) {
          set({ error: toApiError(err) })
        }
      },

      async moveFeed(feedId, categoryId) {
        try {
          await CategoryService.MoveToCategory(feedId, categoryId)
          await get().load()
        } catch (err) {
          set({ error: toApiError(err) })
        }
      },

      async refreshSelected() {
        try {
          await FeedService.RefreshAll()
          // 新文章经后端 items:updated 事件驱动列表刷新；此处刷新源元信息（上次更新等）。
          await get().load()
        } catch (err) {
          set({ error: toApiError(err) })
        }
      },

      async forceRefreshAll() {
        try {
          await FeedService.ForceRefreshAll()
          await get().load()
        } catch (err) {
          set({ error: toApiError(err) })
        }
      },
    }),
    {
      name: 'clip-sidebar',
      // 仅持久化展开集合（Set 转数组）。
      partialize: (state) => ({ expanded: Array.from(state.expanded) }),
      merge: (persisted, current) => {
        const p = persisted as { expanded?: number[] } | undefined
        return { ...current, expanded: new Set(p?.expanded ?? []) }
      },
    },
  ),
)

/** 删除后若当前选中项正是被删对象，则回退到「全部文章」。 */
function resetSelectionIfMatches(
  get: () => SidebarState,
  set: (partial: Partial<SidebarState>) => void,
  kind: 'feed' | 'category',
  id: number,
): void {
  const { selection } = get()
  if (selection.kind === kind && selection.id === id) {
    set({ selection: { kind: 'all' } })
  }
}

/** 批量版：若当前选中的订阅源属于被删集合，回退到「全部文章」。 */
function resetSelectionIfMatchesAny(
  get: () => SidebarState,
  set: (partial: Partial<SidebarState>) => void,
  ids: number[],
): void {
  const { selection } = get()
  if (selection.kind === 'feed' && ids.includes(selection.id)) {
    set({ selection: { kind: 'all' } })
  }
}
