import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest'

// 把后端服务整体替换为可控的 mock，避免加载真实 Wails 绑定。
vi.mock('../Utils', () => ({
  CategoryService: {
    ListCategories: vi.fn(),
    AddCategory: vi.fn(),
    UpdateCategory: vi.fn(),
    DeleteCategory: vi.fn(),
    MoveToCategory: vi.fn(),
  },
  FeedService: {
    ListFeedsWithUnread: vi.fn(),
    UpdateFeed: vi.fn(),
    DeleteFeed: vi.fn(),
    PauseFeed: vi.fn(),
    ResumeFeed: vi.fn(),
    RefreshFeed: vi.fn(),
    RefreshAll: vi.fn(),
    ForceRefreshAll: vi.fn(),
  },
  toApiError: (e: unknown) => String(e),
}))

import { CategoryService, FeedService } from '../Utils'
import { useSidebarStore } from './SidebarStore'

const ListCategories = CategoryService.ListCategories as Mock
const ListFeeds = FeedService.ListFeedsWithUnread as Mock
const AddCategory = CategoryService.AddCategory as Mock
const MoveToCategory = CategoryService.MoveToCategory as Mock
const UpdateFeed = FeedService.UpdateFeed as Mock
const DeleteFeed = FeedService.DeleteFeed as Mock
const RefreshFeed = FeedService.RefreshFeed as Mock
const RefreshAll = FeedService.RefreshAll as Mock
const ForceRefreshAll = FeedService.ForceRefreshAll as Mock

function reset(): void {
  useSidebarStore.setState({
    categories: [],
    feeds: [],
    selection: { kind: 'all' },
    expanded: new Set<number>(),
    loading: false,
    error: null,
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  ListCategories.mockResolvedValue([])
  ListFeeds.mockResolvedValue([])
  AddCategory.mockResolvedValue(null)
  MoveToCategory.mockResolvedValue(undefined)
  UpdateFeed.mockResolvedValue(undefined)
  DeleteFeed.mockResolvedValue(undefined)
  RefreshFeed.mockResolvedValue(undefined)
  RefreshAll.mockResolvedValue([])
  ForceRefreshAll.mockResolvedValue([])
  reset()
})

describe('SidebarStore', () => {
  it('load 并发拉取并写入状态', async () => {
    ListCategories.mockResolvedValue([{ id: 1, name: '科技' }])
    ListFeeds.mockResolvedValue([
      { id: 10, title: 'A', categoryId: 1, unreadCount: 3 },
    ])

    await useSidebarStore.getState().load()

    const s = useSidebarStore.getState()
    expect(s.categories).toHaveLength(1)
    expect(s.feeds).toHaveLength(1)
    expect(s.loading).toBe(false)
    expect(s.error).toBeNull()
  })

  it('load 失败时记录 error', async () => {
    ListCategories.mockRejectedValue(new Error('boom'))
    await useSidebarStore.getState().load()
    const s = useSidebarStore.getState()
    expect(s.error).toContain('boom')
    expect(s.loading).toBe(false)
  })

  it('select 设置选中项', () => {
    useSidebarStore.getState().select({ kind: 'feed', id: 7 })
    expect(useSidebarStore.getState().selection).toEqual({
      kind: 'feed',
      id: 7,
    })
  })

  it('toggleExpand 切换展开集合', () => {
    const { toggleExpand, isExpanded } = useSidebarStore.getState()
    expect(isExpanded(3)).toBe(false)
    toggleExpand(3)
    expect(useSidebarStore.getState().isExpanded(3)).toBe(true)
    useSidebarStore.getState().toggleExpand(3)
    expect(useSidebarStore.getState().isExpanded(3)).toBe(false)
  })

  it('addCategory 以 parentID=0 新建并重新加载；空名忽略', async () => {
    await useSidebarStore.getState().addCategory('  ')
    expect(AddCategory).not.toHaveBeenCalled()

    await useSidebarStore.getState().addCategory('  新分类 ')
    expect(AddCategory).toHaveBeenCalledWith('新分类', 0)
    expect(ListCategories).toHaveBeenCalled() // 触发了 load
  })

  it('moveFeed 调用 MoveToCategory 后重新加载', async () => {
    await useSidebarStore.getState().moveFeed(10, 2)
    expect(MoveToCategory).toHaveBeenCalledWith(10, 2)
    expect(ListFeeds).toHaveBeenCalled()
  })

  it('renameFeed 基于已加载源提交完整对象', async () => {
    useSidebarStore.setState({
      feeds: [
        { id: 10, title: '旧', categoryId: null, unreadCount: 0 } as never,
      ],
    })
    await useSidebarStore.getState().renameFeed(10, '新标题')
    expect(UpdateFeed).toHaveBeenCalledTimes(1)
    expect((UpdateFeed.mock.calls[0][0] as { title: string }).title).toBe(
      '新标题',
    )
  })

  it('删除当前选中的源后选中项回退到「全部」', async () => {
    useSidebarStore.setState({ selection: { kind: 'feed', id: 10 } })
    await useSidebarStore.getState().deleteFeed(10)
    expect(DeleteFeed).toHaveBeenCalledWith(10)
    expect(useSidebarStore.getState().selection).toEqual({ kind: 'all' })
  })

  it('refreshSelected 选中单源时刷新该源', async () => {
    useSidebarStore.setState({ selection: { kind: 'feed', id: 7 } })
    await useSidebarStore.getState().refreshSelected()
    expect(RefreshFeed).toHaveBeenCalledWith(7)
    expect(RefreshAll).not.toHaveBeenCalled()
    expect(ListFeeds).toHaveBeenCalled() // 触发 load 刷新源元信息
  })

  it('refreshSelected 非单源选中时刷新全部', async () => {
    useSidebarStore.setState({ selection: { kind: 'category', id: 2 } })
    await useSidebarStore.getState().refreshSelected()
    expect(RefreshAll).toHaveBeenCalled()
    expect(RefreshFeed).not.toHaveBeenCalled()
  })

  it('refreshSelected 失败时记录 error', async () => {
    RefreshAll.mockRejectedValueOnce(new Error('net'))
    useSidebarStore.setState({ selection: { kind: 'all' } })
    await useSidebarStore.getState().refreshSelected()
    expect(useSidebarStore.getState().error).toContain('net')
  })

  it('forceRefreshAll 调用 ForceRefreshAll 后重新加载', async () => {
    await useSidebarStore.getState().forceRefreshAll()
    expect(ForceRefreshAll).toHaveBeenCalled()
    expect(ListFeeds).toHaveBeenCalled()
  })
})
