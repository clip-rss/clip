import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest'
import type { Item } from '../Types'

vi.mock('../Utils', () => ({
  ItemService: {
    ListItems: vi.fn(),
    MarkRead: vi.fn(),
    MarkUnread: vi.fn(),
    ToggleStar: vi.fn(),
    BatchMarkRead: vi.fn(),
    SearchItems: vi.fn(),
    AddNote: vi.fn(),
  },
  // SidebarStore.load 依赖（refreshSidebar 会触发）
  CategoryService: { ListCategories: vi.fn() },
  FeedService: { ListFeedsWithUnread: vi.fn() },
  SettingsService: {
    GetSettings: vi.fn(),
    UpdateSettings: vi.fn(),
  },
  toApiError: (e: unknown) => String(e),
}))

import { ItemService } from '../Utils'
import { useArticleStore } from './ArticleStore'
import { useSettingsStore } from './SettingsStore'

const ListItems = ItemService.ListItems as Mock
const MarkRead = ItemService.MarkRead as Mock
const ToggleStar = ItemService.ToggleStar as Mock
const BatchMarkRead = ItemService.BatchMarkRead as Mock
const SearchItems = ItemService.SearchItems as Mock
const AddNote = ItemService.AddNote as Mock

function item(id: number, opts: Partial<Item> = {}): Item {
  return { id, feedId: 1, isRead: false, isStarred: false, ...opts } as Item
}

function reset(): void {
  useArticleStore.setState({
    items: [],
    loading: false,
    error: null,
    filter: 'unread',
    sort: 'time',
    selectedItemId: null,
    currentSelection: { kind: 'all' },
    searchQuery: '',
    searchResults: [],
    searching: false,
    searchActive: false,
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  ListItems.mockResolvedValue([])
  MarkRead.mockResolvedValue(undefined)
  ToggleStar.mockResolvedValue(undefined)
  BatchMarkRead.mockResolvedValue(undefined)
  SearchItems.mockResolvedValue([])
  AddNote.mockResolvedValue(undefined)
  reset()
})

describe('ArticleStore', () => {
  it('load(feed) 按源拉取并清空已选', async () => {
    useArticleStore.setState({ selectedItemId: 9 })
    ListItems.mockResolvedValue([item(1)])
    await useArticleStore.getState().load({ kind: 'feed', id: 5 })
    expect(ListItems).toHaveBeenCalledWith(5, 2000, 0)
    expect(useArticleStore.getState().items).toHaveLength(1)
    expect(useArticleStore.getState().selectedItemId).toBeNull()
  })

  it('load(all) 用 feedID=0 拉取全部', async () => {
    await useArticleStore.getState().load({ kind: 'all' })
    expect(ListItems).toHaveBeenCalledWith(0, 2000, 0)
  })

  it('setFilter / setSort 更新状态', () => {
    useArticleStore.getState().setFilter('starred')
    useArticleStore.getState().setSort('source')
    expect(useArticleStore.getState().filter).toBe('starred')
    expect(useArticleStore.getState().sort).toBe('source')
  })

  it('selectItem 设置选中并对未读项乐观标记已读', () => {
    useArticleStore.setState({ items: [item(10, { isRead: false })] })
    useArticleStore.getState().selectItem(10)
    const s = useArticleStore.getState()
    expect(s.selectedItemId).toBe(10)
    expect(s.items[0].isRead).toBe(true)
    expect(MarkRead).toHaveBeenCalledWith(10)
  })

  it('selectItem 对已读项不再调用 MarkRead', () => {
    useArticleStore.setState({ items: [item(10, { isRead: true })] })
    useArticleStore.getState().selectItem(10)
    expect(MarkRead).not.toHaveBeenCalled()
  })

  it('toggleStar 乐观翻转并调用后端', async () => {
    useArticleStore.setState({ items: [item(10, { isStarred: false })] })
    await useArticleStore.getState().toggleStar(10)
    expect(ToggleStar).toHaveBeenCalledWith(10)
    expect(useArticleStore.getState().items[0].isStarred).toBe(true)
  })

  it('markAllRead 批量标记并调用 BatchMarkRead', async () => {
    useArticleStore.setState({
      items: [
        item(1, { isRead: false }),
        item(2, { isRead: false }),
        item(3, { isRead: true }),
      ],
    })
    await useArticleStore.getState().markAllRead([1, 2])
    expect(BatchMarkRead).toHaveBeenCalledWith([1, 2])
    const items = useArticleStore.getState().items
    expect(items.every((i) => i.isRead)).toBe(true)
  })

  it('batchStar 对每个 id 调用 ToggleStar', async () => {
    useArticleStore.setState({ items: [item(1), item(2)] })
    await useArticleStore.getState().batchStar([1, 2])
    expect(ToggleStar).toHaveBeenCalledTimes(2)
    expect(useArticleStore.getState().items.every((i) => i.isStarred)).toBe(
      true,
    )
  })

  it('saveNote 乐观更新 note 并调用 AddNote', async () => {
    useArticleStore.setState({ items: [item(10, { note: '' })] })
    await useArticleStore.getState().saveNote(10, '我的笔记')
    expect(AddNote).toHaveBeenCalledWith(10, '我的笔记')
    expect(useArticleStore.getState().items[0].note).toBe('我的笔记')
  })

  it('saveNote note 未变化时跳过后端调用', async () => {
    useArticleStore.setState({ items: [item(10, { note: '原文' })] })
    await useArticleStore.getState().saveNote(10, '原文')
    expect(AddNote).not.toHaveBeenCalled()
  })

  it('saveNote 同步 searchResults 中的同 id 文章', async () => {
    useArticleStore.setState({
      items: [],
      searchResults: [item(20, { note: '' })],
    })
    await useArticleStore.getState().saveNote(20, '笔记内容')
    expect(AddNote).toHaveBeenCalledWith(20, '笔记内容')
    expect(useArticleStore.getState().searchResults[0].note).toBe('笔记内容')
  })

  it('saveNote 后端失败时回滚 note', async () => {
    AddNote.mockRejectedValueOnce('boom')
    useArticleStore.setState({ items: [item(10, { note: '旧' })] })
    await useArticleStore.getState().saveNote(10, '新')
    const s = useArticleStore.getState()
    expect(s.items[0].note).toBe('旧') // 回滚
    expect(s.error).toBe('boom')
  })

  it('runSearch 按当前 query 全库搜索并进入搜索态', async () => {
    SearchItems.mockResolvedValue([item(7), item(8)])
    useArticleStore.getState().setSearchQuery('周刊')
    await useArticleStore.getState().runSearch()
    const s = useArticleStore.getState()
    expect(SearchItems).toHaveBeenCalledWith('周刊', 2000, 0)
    expect(s.searchActive).toBe(true)
    expect(s.searchResults).toHaveLength(2)
    expect(s.searching).toBe(false)
  })

  it('runSearch 空 query 等价清除，不发请求', async () => {
    useArticleStore.getState().setSearchQuery('   ')
    await useArticleStore.getState().runSearch()
    expect(SearchItems).not.toHaveBeenCalled()
    expect(useArticleStore.getState().searchActive).toBe(false)
  })

  it('clearSearch 退出搜索态并清空结果', async () => {
    SearchItems.mockResolvedValue([item(7)])
    useArticleStore.getState().setSearchQuery('go')
    await useArticleStore.getState().runSearch()
    useArticleStore.getState().clearSearch()
    const s = useArticleStore.getState()
    expect(s.searchActive).toBe(false)
    expect(s.searchQuery).toBe('')
    expect(s.searchResults).toEqual([])
  })

  it('runSearch 期间 query 变化则丢弃过期结果', async () => {
    // 请求 resolve 前把 query 改掉，结果不应落地。
    SearchItems.mockImplementation(async () => {
      useArticleStore.setState({ searchQuery: '别的词' })
      return [item(1)]
    })
    useArticleStore.getState().setSearchQuery('周刊')
    await useArticleStore.getState().runSearch()
    expect(useArticleStore.getState().searchResults).toEqual([])
  })

  it('选中搜索结果即便不在 items 中也能标记已读', () => {
    useArticleStore.setState({
      searchActive: true,
      searchResults: [item(20, { isRead: false })],
      items: [],
    })
    useArticleStore.getState().selectItem(20)
    const s = useArticleStore.getState()
    expect(s.selectedItemId).toBe(20)
    expect(s.searchResults[0].isRead).toBe(true)
    expect(MarkRead).toHaveBeenCalledWith(20)
  })

  // ─── 自动标记已读延迟 ───

  describe('autoMarkReadDelay', () => {
    beforeEach(() => {
      vi.useRealTimers()
      vi.useFakeTimers()
      // 默认为立即标记（delay = 0）
      useSettingsStore.setState({
        settings: {
          theme: 'system',
          language: 'zh',
          defaultUpdateInterval: 30,
          defaultMaxItems: 100,
          notificationMode: 'each',
          autoMarkReadDelay: 0,
          launchMinimized: false,
          windowWidth: 1200,
          windowHeight: 800,
          proxyHost: '',
          proxyPort: 0,
        },
      })
    })

    it('delay=0 时点击即立即乐观标记已读', () => {
      useArticleStore.setState({ items: [item(1, { isRead: false })] })
      useArticleStore.getState().selectItem(1)
      expect(useArticleStore.getState().items[0].isRead).toBe(true)
      expect(MarkRead).toHaveBeenCalledWith(1)
    })

    it('delay=2000 时点击后 2s 才标记已读', () => {
      useSettingsStore.setState({
        settings: {
          ...useSettingsStore.getState().settings!,
          autoMarkReadDelay: 2000,
        },
      })
      useArticleStore.setState({ items: [item(1, { isRead: false })] })
      useArticleStore.getState().selectItem(1)

      // 定时器到期前不应标记
      expect(useArticleStore.getState().items[0].isRead).toBe(false)
      expect(MarkRead).not.toHaveBeenCalled()

      vi.advanceTimersByTime(2000)
      expect(useArticleStore.getState().items[0].isRead).toBe(true)
      expect(MarkRead).toHaveBeenCalledWith(1)
    })

    it('delay>0 时切换文章则取消前一延迟，前一篇保持未读', () => {
      useSettingsStore.setState({
        settings: {
          ...useSettingsStore.getState().settings!,
          autoMarkReadDelay: 5000,
        },
      })
      useArticleStore.setState({
        items: [item(1, { isRead: false }), item(2, { isRead: false })],
      })
      useArticleStore.getState().selectItem(1)
      vi.advanceTimersByTime(1000) // 才过 1s
      useArticleStore.getState().selectItem(2) // 切换到第二篇

      // 前一篇不应被标记（计时器已清除）
      expect(useArticleStore.getState().items[0].isRead).toBe(false)

      // 第二篇到点后标记
      vi.advanceTimersByTime(5000)
      expect(useArticleStore.getState().items[1].isRead).toBe(true)
      expect(MarkRead).toHaveBeenCalledWith(2)
      expect(MarkRead).not.toHaveBeenCalledWith(1)
    })

    it('delay<0 时点击不自动标记已读', () => {
      useSettingsStore.setState({
        settings: {
          ...useSettingsStore.getState().settings!,
          autoMarkReadDelay: -1,
        },
      })
      useArticleStore.setState({ items: [item(1, { isRead: false })] })
      useArticleStore.getState().selectItem(1)
      expect(useArticleStore.getState().items[0].isRead).toBe(false)
      expect(MarkRead).not.toHaveBeenCalled()
    })
  })
})
