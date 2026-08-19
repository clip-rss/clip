import { create } from 'zustand'
import { persist } from 'zustand/middleware'

const MAX = 5

interface SearchHistoryState {
  history: string[]
  push: (query: string) => void
  clear: () => void
}

/**
 * 搜索历史：最近 MAX 条，LRU 语义（重复项置顶而非追加）。存 localStorage
 */
export const useSearchHistoryStore = create<SearchHistoryState>()(
  persist(
    (set) => ({
      history: [],

      push(query) {
        const q = query.trim()
        if (!q) return
        set((s) => {
          const rest = s.history.filter((h) => h !== q && !q.startsWith(h))
          return { history: [q, ...rest].slice(0, MAX) }
        })
      },

      clear() {
        set({ history: [] })
      },
    }),
    { name: 'clip-search-history' },
  ),
)

export { MAX as SEARCH_HISTORY_MAX }
