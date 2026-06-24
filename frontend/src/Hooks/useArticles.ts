import { useMemo } from 'react'
import { useArticleStore, useSidebarStore } from '../Stores'
import { categoryFeedIds, filterAndSortItems, neighborItemId } from '../Utils'
import type { Item } from '../Types'

/** 当前选中范围下的「来源标题映射」与「分类限定集合」（多个 Hook 内部复用）。 */
function useScopeContext(): {
  feedTitleOf: (id: number) => string
  allowedFeedIds: Set<number> | null
} {
  const feeds = useSidebarStore((s) => s.feeds)
  const categories = useSidebarStore((s) => s.categories)
  const selection = useSidebarStore((s) => s.selection)

  const feedTitle = useMemo(() => {
    const map = new Map<number, string>()
    for (const f of feeds) map.set(f.id, f.title)
    return map
  }, [feeds])

  const allowedFeedIds = useMemo(() => {
    if (selection.kind !== 'category') return null
    return categoryFeedIds(categories, feeds, selection.id)
  }, [selection, categories, feeds])

  const feedTitleOf = useMemo(
    () => (id: number) => feedTitle.get(id) ?? '',
    [feedTitle],
  )

  return { feedTitleOf, allowedFeedIds }
}

/** 中间栏可见文章（筛选 + 排序），与文章列表展示完全一致。 */
export function useVisibleArticles(): Item[] {
  const items = useArticleStore((s) => s.items)
  const filter = useArticleStore((s) => s.filter)
  const sort = useArticleStore((s) => s.sort)
  const searchActive = useArticleStore((s) => s.searchActive)
  const searchResults = useArticleStore((s) => s.searchResults)
  const { feedTitleOf, allowedFeedIds } = useScopeContext()

  return useMemo(() => {
    // 搜索模式：全库结果，已按后端 rank/时间排序，不再套用筛选与分类限定。
    if (searchActive) return searchResults
    return filterAndSortItems(items, {
      filter,
      sort,
      allowedFeedIds,
      feedTitleOf,
    })
  }, [
    searchActive,
    searchResults,
    items,
    filter,
    sort,
    allowedFeedIds,
    feedTitleOf,
  ])
}

export interface ArticleNavigation {
  /** 上一篇 id（无则 null）。 */
  prevId: number | null
  /** 下一篇 id（无则 null）。 */
  nextId: number | null
  goPrev: () => void
  goNext: () => void
}

/**
 * 阅读导航：在当前可见文章序列中切换上一篇 / 下一篇（专注模式 J/K、↑/↓）。
 *
 * 定位基于范围内的完整有序列表，候选落点限定在当前筛选可见集，
 * 因此「读完即移出未读列表」不会打断连续阅读。
 */
export function useArticleNavigation(): ArticleNavigation {
  const items = useArticleStore((s) => s.items)
  const filter = useArticleStore((s) => s.filter)
  const sort = useArticleStore((s) => s.sort)
  const currentId = useArticleStore((s) => s.selectedItemId)
  const selectItem = useArticleStore((s) => s.selectItem)
  const { feedTitleOf, allowedFeedIds } = useScopeContext()

  const ordered = useMemo(
    () =>
      filterAndSortItems(items, {
        filter: 'all',
        sort,
        allowedFeedIds,
        feedTitleOf,
      }),
    [items, sort, allowedFeedIds, feedTitleOf],
  )

  const candidateIds = useMemo(() => {
    const visible = filterAndSortItems(items, {
      filter,
      sort,
      allowedFeedIds,
      feedTitleOf,
    })
    return new Set(visible.map((it) => it.id))
  }, [items, filter, sort, allowedFeedIds, feedTitleOf])

  const prevId = useMemo(
    () => neighborItemId(ordered, candidateIds, currentId, -1),
    [ordered, candidateIds, currentId],
  )
  const nextId = useMemo(
    () => neighborItemId(ordered, candidateIds, currentId, 1),
    [ordered, candidateIds, currentId],
  )

  return {
    prevId,
    nextId,
    goPrev: () => {
      if (prevId !== null) selectItem(prevId)
    },
    goNext: () => {
      if (nextId !== null) selectItem(nextId)
    },
  }
}
