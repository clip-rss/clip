import { create } from 'zustand'
import { ItemService, showToast, toApiError } from '../Utils'
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

/** 将 ItemLight 转换为 Item（content / fullContent 为空字符串）。 */
function lightToItem(light: ItemLight): Item {
  return { ...light, content: '', fullContent: '' }
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
  /**
   * 重新拉取当前范围（新文章事件等），不清空选中与已加载正文；
   * 选中文章若已不在新列表中（被清理等）才恢复为未选中。
   */
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

  /**
   * 抓取原文页面并提取正文（「RSS 只给摘要」的源用）。
   *
   * 与 loadFullContent 是两条不同的路径，别混：那个读的是**本地库**里 RSS 没随
   * 列表接口带出来的 content，零网络请求；这个真的会去请求文章原站，结果落在
   * fullContent 上，不覆盖 content。
   */
  fetchFullContent: (id: number) => Promise<void>
  /** 正在提取全文的文章 ID。 */
  fullTextLoadingId: number | null

  /**
   * 当前选中的文章是否正在显示 RSS 摘要（而不是提取出的全文）。
   *
   * 只对当前这一篇有效，切换文章即复位：阅读视图与专注模式同时只渲染一篇，
   * 所以不需要按文章 ID 存一份映射。默认 false —— 有全文就显示全文，
   * 与加这个开关之前的行为完全一致。
   *
   * 不持久化是刻意的：这是「这篇文章现在看哪一份」，属于会话状态而非用户偏好，
   * 后端 Settings 里没有它的位置。
   */
  showSummary: boolean
  /** 在摘要与全文之间切换（仅两份正文都在时由工具栏按钮调用）。 */
  toggleBodyMode: () => void

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
  // reload（事件驱动刷新）时保留选中文章与已加载正文，不打断正在进行的阅读。
  async function fetchAndResolve(
    selection: Selection,
    preserveSelection = false,
  ): Promise<void> {
    set({ loading: true, error: null })
    try {
      const feedId = scopeFeedId(selection)
      const currentFilter = get().filter

      // 全局视图下「星标/未读/有笔记」筛选走后端专用端点，绕过 LOAD_LIMIT 限制。
      // 否则超出最新 LOAD_LIMIT 篇的旧文章不进内存，这些筛选会静默漏掉目标文章。
      let lights: ItemLight[]
      if (currentFilter === 'starred' && feedId <= 0) {
        lights = (await ItemService.ListStarredItemsLight(LOAD_LIMIT, 0)) ?? []
      } else if (currentFilter === 'unread' && feedId <= 0) {
        lights = (await ItemService.ListUnreadItemsLight(LOAD_LIMIT, 0)) ?? []
      } else if (currentFilter === 'hasNote' && feedId <= 0) {
        lights = (await ItemService.ListNotedItemsLight(LOAD_LIMIT, 0)) ?? []
      } else {
        lights = (await ItemService.ListItemsLight(feedId, LOAD_LIMIT, 0)) ?? []
      }
      const prev = get()
      // 回填已加载/已提取过的正文，避免阅读中的文章被清成空正文或退回 RSS 摘要。
      const loadedContent = new Map<number, string>()
      const loadedFullContent = new Map<number, string>()
      if (preserveSelection) {
        for (const it of prev.items) {
          if (it.content) loadedContent.set(it.id, it.content)
          if (it.fullContent) loadedFullContent.set(it.id, it.fullContent)
        }
      }
      const items = (lights ?? []).map((light) => {
        const base = lightToItem(light)
        const content = loadedContent.get(base.id) ?? base.content
        const fullContent = loadedFullContent.get(base.id) ?? base.fullContent
        return { ...base, content, fullContent }
      })
      const pending = prev.pendingSelectId
      // 选中恢复优先级：通知定位 > 原选中（仍存在于新列表时）> 清空。
      let selectedId: number | null = null
      if (pending !== null && items.some((it) => it.id === pending)) {
        selectedId = pending
      } else if (
        preserveSelection &&
        prev.selectedItemId !== null &&
        items.some((it) => it.id === prev.selectedItemId)
      ) {
        selectedId = prev.selectedItemId
      }
      set({
        items,
        loading: false,
        selectedItemId: selectedId,
        pendingSelectId: null,
        // 摘要态只属于它被切换时的那一篇。这里也要复位，因为通知定位
        // （pendingSelectId）这条路不走 selectItem，否则会把上一篇的显示模式带过来；
        // reload 保持同一篇时则要留着——正文刷新不该把正在读的摘要顶回全文。
        showSummary:
          selectedId === prev.selectedItemId ? prev.showSummary : false,
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
    sort: 'timeDesc',
    selectedItemId: null,
    currentSelection: { kind: 'all' },
    searchQuery: '',
    searchResults: [],
    searching: false,
    searchActive: false,
    pendingSelectId: null,
    loadingContentId: null,
    fullTextLoadingId: null,
    showSummary: false,

    async load(selection) {
      set({
        currentSelection: selection,
        selectedItemId: null,
        pendingSelectId: null,
      })
      await fetchAndResolve(selection)
    },

    async reload() {
      await fetchAndResolve(get().currentSelection, true)
    },

    setFilter(filter) {
      const prev = get().filter
      set({ filter })

      if (get().searchActive) return

      // 'read' 需要 reload 以获取最新 readAt 排序。
      // 'starred' / 'hasNote' 在全局视图走后端专用端点（绕过 LOAD_LIMIT），
      // 因此进入要取回超限的旧文章，离开要重新拉全量——进出都得 reload。
      const usesDedicatedEndpoint = (f: ArticleFilter): boolean =>
        f === 'starred' || f === 'hasNote'
      if (
        (filter === 'read' && prev !== 'read') ||
        usesDedicatedEndpoint(filter) !== usesDedicatedEndpoint(prev)
      ) {
        void get().reload()
      }
    },

    setSort(sort) {
      set({ sort })
    },

    selectItem(id) {
      // 切换文章先取消上一篇仍未触发的延迟标记。
      window.clearTimeout(autoMarkTimer)
      // 上一篇的「正在看摘要」的切换只属于那一篇，切换即清除。
      set({ selectedItemId: id, showSummary: false })
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
      const cur =
        get().items.find((it) => it.id === id) ??
        get().searchResults.find((it) => it.id === id)
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
      const apply = (list: Item[]): Item[] =>
        list.map((it) =>
          idSet.has(it.id) ? ({ ...it, isRead: true } as Item) : it,
        )
      set({
        items: apply(get().items),
        searchResults: apply(get().searchResults),
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

    async fetchFullContent(id) {
      // 防重复：同一篇已在提取中就不要再发一次（后端会再抓一遍原文）。
      if (get().fullTextLoadingId === id) return
      const existing =
        get().items.find((it) => it.id === id) ??
        get().searchResults.find((it) => it.id === id)
      // 已有提取结果，无需再联网——后端也会直接返回库里的那份。
      if (existing?.fullContent) return

      set({ fullTextLoadingId: id })
      try {
        const html = await ItemService.FetchFullContent(id)
        if (html) {
          patchItem(id, { fullContent: html })
        }
      } catch (err) {
        // 提取失败不阻断阅读：正文照常显示 RSS 给的内容，只弹一条 toast。
        // 后端已把错误本地化过（见 internal/i18n），这里直接展示即可。
        // 用 toast 而不是内联提示：阅读区是正文的地盘，失败属于「刚才那个操作」，
        // 且切换文章时不该留下一条属于上一篇的提示。
        showToast(toApiError(err), 'error')
      } finally {
        // 只在仍是同一篇文章时清除 loading（避免快速切换时错误清除）。
        if (get().fullTextLoadingId === id) {
          set({ fullTextLoadingId: null })
        }
      }
    },

    toggleBodyMode() {
      set((s) => ({ showSummary: !s.showSummary }))
    },

    async batchStar(ids) {
      if (ids.length === 0) return
      const idSet = new Set(ids)
      const apply = (list: Item[]): Item[] =>
        list.map((it) =>
          idSet.has(it.id) ? ({ ...it, isStarred: true } as Item) : it,
        )
      set({
        items: apply(get().items),
        searchResults: apply(get().searchResults),
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
