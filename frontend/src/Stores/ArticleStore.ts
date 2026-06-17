import { create } from 'zustand'
import { ItemService, toApiError } from '../Utils'
import { useSidebarStore } from './SidebarStore'
import type { ArticleFilter, ArticleSort, Item, Selection } from '../Types'

/** 单次拉取上限：客户端筛选/排序 + 虚拟滚动，足够覆盖常规留存量。 */
const LOAD_LIMIT = 2000

interface ArticleState {
  items: Item[]
  loading: boolean
  error: string | null
  filter: ArticleFilter
  sort: ArticleSort
  selectedItemId: number | null
  /** 当前列表的选中范围，供事件驱动的 reload 复用。 */
  currentSelection: Selection

  /** 按选中范围加载文章（清空已选文章）。 */
  load: (selection: Selection) => Promise<void>
  /** 重新拉取当前范围（保留已选文章），用于新文章事件。 */
  reload: () => Promise<void>
  setFilter: (filter: ArticleFilter) => void
  setSort: (sort: ArticleSort) => void
  selectItem: (id: number) => void
  toggleStar: (id: number) => Promise<void>
  markRead: (id: number) => Promise<void>
  markUnread: (id: number) => Promise<void>
  markAllRead: (ids: number[]) => Promise<void>
  batchStar: (ids: number[]) => Promise<void>
}

function scopeFeedId(selection: Selection): number {
  return selection.kind === 'feed' ? selection.id : 0
}

/** 刷新侧栏未读计数（读状态变化后调用）。 */
function refreshSidebar(): void {
  void useSidebarStore.getState().load()
}

export const useArticleStore = create<ArticleState>()((set, get) => {
  async function fetchScope(selection: Selection): Promise<void> {
    set({ loading: true, error: null })
    try {
      const items = await ItemService.ListItems(scopeFeedId(selection), LOAD_LIMIT, 0)
      set({ items: items ?? [], loading: false })
    } catch (err) {
      set({ error: toApiError(err), loading: false })
    }
  }

  /** 局部更新某文章字段。 */
  function patchItem(id: number, patch: Partial<Item>): void {
    set({
      items: get().items.map((it) => (it.id === id ? ({ ...it, ...patch } as Item) : it)),
    })
  }

  return {
    items: [],
    loading: false,
    error: null,
    filter: 'unread',
    sort: 'time',
    selectedItemId: null,
    currentSelection: { kind: 'all' },

    async load(selection) {
      set({ currentSelection: selection, selectedItemId: null })
      await fetchScope(selection)
    },

    async reload() {
      await fetchScope(get().currentSelection)
    },

    setFilter(filter) {
      set({ filter })
    },

    setSort(sort) {
      set({ sort })
    },

    selectItem(id) {
      set({ selectedItemId: id })
      const item = get().items.find((it) => it.id === id)
      if (!item || item.isRead) return
      patchItem(id, { isRead: true }) // 点击即标记已读（乐观）
      ItemService.MarkRead(id)
        .then(refreshSidebar)
        .catch((err) => set({ error: toApiError(err) }))
    },

    async toggleStar(id) {
      const cur = get().items.find((it) => it.id === id)
      if (!cur) return
      const next = !cur.isStarred
      patchItem(id, { isStarred: next })
      try {
        await ItemService.ToggleStar(id)
      } catch (err) {
        patchItem(id, { isStarred: !next }) // 回滚
        set({ error: toApiError(err) })
      }
    },

    async markRead(id) {
      patchItem(id, { isRead: true })
      try {
        await ItemService.MarkRead(id)
        refreshSidebar()
      } catch (err) {
        patchItem(id, { isRead: false })
        set({ error: toApiError(err) })
      }
    },

    async markUnread(id) {
      patchItem(id, { isRead: false })
      try {
        await ItemService.MarkUnread(id)
        refreshSidebar()
      } catch (err) {
        patchItem(id, { isRead: true })
        set({ error: toApiError(err) })
      }
    },

    async markAllRead(ids) {
      if (ids.length === 0) return
      const idSet = new Set(ids)
      set({
        items: get().items.map((it) => (idSet.has(it.id) ? ({ ...it, isRead: true } as Item) : it)),
      })
      try {
        await ItemService.BatchMarkRead(ids)
        refreshSidebar()
      } catch (err) {
        set({ error: toApiError(err) })
        await get().reload()
      }
    },

    async batchStar(ids) {
      if (ids.length === 0) return
      const idSet = new Set(ids)
      set({
        items: get().items.map((it) =>
          idSet.has(it.id) ? ({ ...it, isStarred: true } as Item) : it,
        ),
      })
      try {
        // 无批量星标端点，逐条切换（仅对传入的未星标项）。
        await Promise.all(ids.map((id) => ItemService.ToggleStar(id)))
      } catch (err) {
        set({ error: toApiError(err) })
        await get().reload()
      }
    },
  }
})
