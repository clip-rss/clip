import { create } from 'zustand'
import { ItemService, toApiError } from '../Utils'
import { useSidebarStore } from './SidebarStore'
import { useSettingsStore } from './SettingsStore'
import { useSearchHistoryStore } from './SearchHistoryStore'
import type {
  ArticleFilter,
  ArticleSort,
  Item,
  ItemLight,
  Selection,
} from '../Types'

/** 单次拉取上限：客户端筛选/排序 + 虚拟滚动，足够覆盖常规留存量。 */
const LOAD_LIMIT = 2000

/** 将 ItemLight 转换为 Item（content 为空字符串）。 */
function lightToItem(light: ItemLight): Item {
  return { ...light, content: '' }
}

/** 自动标记已读的待定计时器（延迟模式下生效，切换文章时清除）。 */
let autoMarkTimer: number | undefined

interface ArticleState {
  items: Item[]
  loading: boolean
  error: string | null
  filter: ArticleFilter
  sort: ArticleSort
  selectedItemId: number | null
  /** 当前列表的选中范围，供事件驱动的 reload 复用。 */
  currentSelection: Selection

  /** 搜索输入原文（受控搜索框）。 */
  searchQuery: string
  /** 全库搜索结果（独立于 items，不套用筛选/分类）。 */
  searchResults: Item[]
  /** 搜索请求进行中。 */
  searching: boolean
  /** 是否处于搜索模式（中间栏展示搜索结果）。 */
  searchActive: boolean

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
  /** 保存文章笔记：乐观更新本地 note 字段并写入后端。 */
  saveNote: (id: number, note: string) => Promise<void>

  /**
   * 按需加载文章正文：首次选中某篇文章时，若其 content 为空，
   * 则从后端拉取完整 Item 并 patch 到列表（content 字段填充）。
   */
  loadFullContent: (id: number) => Promise<void>
  /** content 正在加载中的文章 ID。 */
  loadingContentId: number | null

  /** 仅更新搜索框文本（不触发请求；防抖在调用方）。 */
  setSearchQuery: (q: string) => void
  /** 按当前 searchQuery 执行全库搜索；空查询等价于清除。 */
  runSearch: () => Promise<void>
  /** 退出搜索模式，恢复原列表。 */
  clearSearch: () => void

  /** 待定位文章 ID，由通知点击设置，load 完成时消费。 */
  pendingSelectId: number | null
  /** 通知点击后置位待定位 ID，下次 load 完成时自动选中。 */
  scheduleSelect: (id: number) => void
}

function scopeFeedId(selection: Selection): number {
  return selection.kind === 'feed' ? selection.id : 0
}

/** 刷新侧栏未读计数（读状态变化后调用）。 */
function refreshSidebar(): void {
  void useSidebarStore.getState().load()
}

export const useArticleStore = create<ArticleState>()((set, get) => {
  /** 局部更新某文章字段（同步 items 与 searchResults，保证两种列表显示一致）。 */
  function patchItem(id: number, patch: Partial<Item>): void {
    const apply = (list: Item[]): Item[] =>
      list.map((it) => (it.id === id ? ({ ...it, ...patch } as Item) : it))
    set({
      items: apply(get().items),
      searchResults: apply(get().searchResults),
    })
  }

  /** 乐观标记已读并写入后端（选中文章自动标记复用）。 */
  function markReadOptimistic(id: number): void {
    patchItem(id, { isRead: true })
    ItemService.MarkRead(id)
      .then(refreshSidebar)
      .catch((err) => set({ error: toApiError(err) }))
  }

  // 封装 load：完成后消费 pendingSelectId，若有匹配则自动选中。
  async function fetchAndResolve(selection: Selection): Promise<void> {
    set({ loading: true, error: null })
    try {
      const lights = await ItemService.ListItemsLight(
        scopeFeedId(selection),
        LOAD_LIMIT,
        0,
      )
      const items = (lights ?? []).map(lightToItem)
      const pending = get().pendingSelectId
      const selectedId =
        pending !== null && items.some((it) => it.id === pending)
          ? pending
          : null
      set({
        items,
        loading: false,
        selectedItemId: selectedId,
        pendingSelectId: null,
      })
    } catch (err) {
      set({ error: toApiError(err), loading: false })
    }
  }

  return {
    items: [],
    loading: false,
    error: null,
    filter: 'all',
    sort: 'time',
    selectedItemId: null,
    currentSelection: { kind: 'all' },
    searchQuery: '',
    searchResults: [],
    searching: false,
    searchActive: false,
    pendingSelectId: null,
    loadingContentId: null,

    async load(selection) {
      set({
        currentSelection: selection,
        selectedItemId: null,
        pendingSelectId: null,
      })
      await fetchAndResolve(selection)
    },

    async reload() {
      await fetchAndResolve(get().currentSelection)
    },

    setFilter(filter) {
      set({ filter })
    },

    setSort(sort) {
      set({ sort })
    },

    selectItem(id) {
      // 切换文章先取消上一篇仍未触发的延迟标记。
      window.clearTimeout(autoMarkTimer)
      set({ selectedItemId: id })
      const { items, searchResults } = get()
      const item =
        items.find((it) => it.id === id) ??
        searchResults.find((it) => it.id === id)
      if (!item) return

      // 按需加载正文：列表拉取的轻量版本 content 为空，点击时才拉取完整内容。
      if (!item.content) {
        void get().loadFullContent(id)
      }

      if (item.isRead) return

      const delay = useSettingsStore.getState().settings?.autoMarkReadDelay ?? 0
      if (delay < 0) return // 关闭自动标记已读
      if (delay === 0) {
        markReadOptimistic(id)
        return
      }
      // 延迟标记：到点后若该文仍选中且未读才标记。
      autoMarkTimer = window.setTimeout(() => {
        const cur = get()
        const target =
          cur.items.find((it) => it.id === id) ??
          cur.searchResults.find((it) => it.id === id)
        if (cur.selectedItemId === id && target && !target.isRead) {
          markReadOptimistic(id)
        }
      }, delay)
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
        items: get().items.map((it) =>
          idSet.has(it.id) ? ({ ...it, isRead: true } as Item) : it,
        ),
      })
      try {
        await ItemService.BatchMarkRead(ids)
        refreshSidebar()
      } catch (err) {
        set({ error: toApiError(err) })
        await get().reload()
      }
    },

    async saveNote(id, note) {
      const cur =
        get().items.find((it) => it.id === id) ??
        get().searchResults.find((it) => it.id === id)
      if (!cur || cur.note === note) return
      const prev = cur.note
      patchItem(id, { note })
      try {
        await ItemService.AddNote(id, note)
      } catch (err) {
        patchItem(id, { note: prev }) // 回滚
        set({ error: toApiError(err) })
      }
    },

    async loadFullContent(id) {
      // 防止并发重复加载同一篇文章。
      if (get().loadingContentId === id) return
      const existing =
        get().items.find((it) => it.id === id) ??
        get().searchResults.find((it) => it.id === id)
      // 若已有 content，无需再请求。
      if (existing?.content) return
      set({ loadingContentId: id })
      try {
        const full = await ItemService.GetItem(id)
        if (full) {
          patchItem(id, { content: full.content })
        }
      } catch (err) {
        // content 加载失败不阻断阅读流程，仅记录错误。
        set({ error: toApiError(err) })
      } finally {
        // 只在仍是同一篇文章时清除 loading 状态（避免快速切换时错误清除）。
        if (get().loadingContentId === id) {
          set({ loadingContentId: null })
        }
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

    setSearchQuery(q) {
      set({ searchQuery: q })
    },

    async runSearch() {
      // 读取最新输入（而非闭包捕获），避免防抖回调与 clearSearch 竞态。
      const trimmed = get().searchQuery.trim()
      if (!trimmed) {
        get().clearSearch()
        return
      }
      set({
        searching: true,
        searchActive: true,
        selectedItemId: null,
        error: null,
      })
      try {
        const results = await ItemService.SearchItems(trimmed, LOAD_LIMIT, 0)
        // 防竞态：输入在请求期间变化（含被清除）则丢弃本次结果。
        if (get().searchQuery.trim() !== trimmed) return
        set({ searchResults: results ?? [], searching: false })
        // 仅在确实命中时入历史：过了防竞态检查，且不记无结果的错字。
        if (results?.length) useSearchHistoryStore.getState().push(trimmed)
      } catch (err) {
        set({ error: toApiError(err), searching: false, searchResults: [] })
      }
    },

    clearSearch() {
      set({
        searchQuery: '',
        searchResults: [],
        searching: false,
        searchActive: false,
      })
    },

    scheduleSelect(id) {
      set({ pendingSelectId: id })
    },
  }
})
