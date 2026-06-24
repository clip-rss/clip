import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface LayoutState {
  sidebarWidth: number
  listWidth: number
  /** 专注阅读模式：隐藏三栏 chrome，仅显示全屏阅读区。不持久化。 */
  focusMode: boolean
  /** 笔记面板（阅读区底部抽屉）是否展开。会话态，不持久化。 */
  notePanelOpen: boolean
  setSidebarWidth: (width: number) => void
  setListWidth: (width: number) => void
  resizeSidebar: (delta: number) => void
  resizeList: (delta: number) => void
  enterFocus: () => void
  exitFocus: () => void
  toggleFocus: () => void
  toggleNotePanel: () => void
  closeNotePanel: () => void
}

const SIDEBAR_MIN = 180
const SIDEBAR_MAX = 400
const SIDEBAR_DEFAULT = 260

const LIST_MIN = 280
const LIST_MAX = 560
const LIST_DEFAULT = 380

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max)
}

export const useLayoutStore = create<LayoutState>()(
  persist(
    (set) => ({
      sidebarWidth: SIDEBAR_DEFAULT,
      listWidth: LIST_DEFAULT,
      focusMode: false,
      notePanelOpen: false,
      setSidebarWidth(width: number) {
        set({ sidebarWidth: clamp(width, SIDEBAR_MIN, SIDEBAR_MAX) })
      },
      setListWidth(width: number) {
        set({ listWidth: clamp(width, LIST_MIN, LIST_MAX) })
      },
      resizeSidebar(delta: number) {
        set((s) => ({
          sidebarWidth: clamp(s.sidebarWidth + delta, SIDEBAR_MIN, SIDEBAR_MAX),
        }))
      },
      resizeList(delta: number) {
        set((s) => ({
          listWidth: clamp(s.listWidth + delta, LIST_MIN, LIST_MAX),
        }))
      },
      enterFocus() {
        set({ focusMode: true })
      },
      exitFocus() {
        set({ focusMode: false })
      },
      toggleFocus() {
        set((s) => ({ focusMode: !s.focusMode }))
      },
      toggleNotePanel() {
        set((s) => ({ notePanelOpen: !s.notePanelOpen }))
      },
      closeNotePanel() {
        set({ notePanelOpen: false })
      },
    }),
    {
      name: 'clip-layout',
      // 仅持久化栏宽；focusMode 为会话态，不应跨启动恢复。
      partialize: (s) => ({
        sidebarWidth: s.sidebarWidth,
        listWidth: s.listWidth,
      }),
    },
  ),
)

export { SIDEBAR_MIN, SIDEBAR_MAX, LIST_MIN, LIST_MAX }
