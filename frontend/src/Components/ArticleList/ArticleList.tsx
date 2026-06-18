import { useEffect, useMemo, useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { useSidebarStore, useArticleStore } from '../../Stores'
import { useVisibleArticles } from '../../Hooks'
import { onItemsUpdated } from '../../Utils'
import ListHeader from './ListHeader'
import ArticleRow from './ArticleRow'
import EmptyState from './EmptyState'
import styles from './ArticleList.module.scss'

const ROW_HEIGHT = 64

function ArticleList(): JSX.Element {
  const selection = useSidebarStore((s) => s.selection)
  const feeds = useSidebarStore((s) => s.feeds)

  const loading = useArticleStore((s) => s.loading)
  const filter = useArticleStore((s) => s.filter)
  const sort = useArticleStore((s) => s.sort)
  const selectedItemId = useArticleStore((s) => s.selectedItemId)
  const load = useArticleStore((s) => s.load)
  const reload = useArticleStore((s) => s.reload)
  const setFilter = useArticleStore((s) => s.setFilter)
  const setSort = useArticleStore((s) => s.setSort)
  const selectItem = useArticleStore((s) => s.selectItem)
  const toggleStar = useArticleStore((s) => s.toggleStar)
  const markAllRead = useArticleStore((s) => s.markAllRead)
  const batchStar = useArticleStore((s) => s.batchStar)

  // 选中范围变化 → 重新加载
  useEffect(() => {
    load(selection)
  }, [selection, load])

  // 新文章事件 → 重新拉取当前范围
  useEffect(() => {
    const off = onItemsUpdated(() => reload())
    return off
  }, [reload])

  const feedTitle = useMemo(() => {
    const map = new Map<number, string>()
    for (const f of feeds) map.set(f.id, f.title)
    return map
  }, [feeds])

  const visibleItems = useVisibleArticles()

  const scrollRef = useRef<HTMLDivElement>(null)
  const virtualizer = useVirtualizer({
    count: visibleItems.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 8,
  })

  function handleMarkAllRead(): void {
    markAllRead(visibleItems.filter((it) => !it.isRead).map((it) => it.id))
  }

  function handleBatchStar(): void {
    batchStar(visibleItems.filter((it) => !it.isStarred).map((it) => it.id))
  }

  const showEmpty = !loading && visibleItems.length === 0

  return (
    <div className={styles.list}>
      <ListHeader
        filter={filter}
        sort={sort}
        onFilterChange={setFilter}
        onSortChange={setSort}
        onMarkAllRead={handleMarkAllRead}
        onBatchStar={handleBatchStar}
      />

      {showEmpty ? (
        <EmptyState filter={filter} />
      ) : (
        <div ref={scrollRef} className={styles.scroll} role="listbox" aria-label="文章列表">
          <div className={styles.virtualInner} style={{ height: virtualizer.getTotalSize() }}>
            {virtualizer.getVirtualItems().map((row) => {
              const item = visibleItems[row.index]
              return (
                <div
                  key={item.id}
                  className={styles.virtualRow}
                  style={{ height: row.size, transform: `translateY(${row.start}px)` }}
                >
                  <ArticleRow
                    item={item}
                    sourceName={feedTitle.get(item.feedId) ?? ''}
                    selected={item.id === selectedItemId}
                    onSelect={selectItem}
                    onToggleStar={toggleStar}
                  />
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}

export default ArticleList
