import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface LayoutState {
  sidebarWidth: number
  listWidth: number
  setSidebarWidth: (width: number) => void
  setListWidth: (width: number) => void
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
      setSidebarWidth(width: number) {
        set({ sidebarWidth: clamp(width, SIDEBAR_MIN, SIDEBAR_MAX) })
      },
      setListWidth(width: number) {
        set({ listWidth: clamp(width, LIST_MIN, LIST_MAX) })
      },
    }),
    { name: 'clip-layout' },
  ),
)

export { SIDEBAR_MIN, SIDEBAR_MAX, LIST_MIN, LIST_MAX }
