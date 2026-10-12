// 共享的字体状态：目录清单 + 本机已装字体族。
//
// 三处共用同一份：@font-face 注入（阶段 C）、阅读器字体选择器分组（阶段 D）、
// 字体库面板的「已下载」状态。面板打开时调 refresh() 保证最新；启动时也调一次，
// 让下载过的字体在阅读器里直接可用。

import { create } from 'zustand'
import { FontService } from '../Utils'
import { READER_FONT_PRESETS } from '../Types'
import type { FontCatalog, InstalledFont } from '../Types'

interface FontState {
  /** 远程目录清单。拉取失败为 null（字体库面板据此显示失败态）。 */
  catalog: FontCatalog | null
  catalogFailed: boolean
  /** 本机已装字体族清单，由 ListInstalledFonts 扫目录得出，离线可用。 */
  installed: InstalledFont[]
  /** 初次拉取是否已完成（无论成败）。用于阶段 C 时序：清单到齐前白名单是待定的。 */
  ready: boolean
  /** 拉取进行中。 */
  refreshing: boolean
  refresh: () => Promise<void>
}

export const useFontStore = create<FontState>()((set, get) => ({
  catalog: null,
  catalogFailed: false,
  installed: [],
  ready: false,
  refreshing: false,

  async refresh() {
    if (get().refreshing) return
    set({ refreshing: true, catalogFailed: false })
    // 目录与已装清单并行拉取、互不阻塞：各自成败独立。
    // 目录失败不影响「已下载」展示；已装清单失败最坏是标不出已下载。
    const catalogP = FontService.FetchFontCatalog()
      .then((cat) => set({ catalog: cat }))
      .catch(() => set({ catalog: null, catalogFailed: true }))
    const installedP = FontService.ListInstalledFonts()
      .then((list) => set({ installed: list }))
      .catch(() => {})
    await Promise.allSettled([catalogP, installedP])
    set({ refreshing: false, ready: true })
  },
}))

/** 已装字体 id 集合的辅助读取（供 ReaderStore 动态白名单等使用）。 */
export function installedFontIds(): string[] {
  return useFontStore.getState().installed.map((f) => f.id)
}

/**
 * 当前合法的字体取值集合：预设三档 ∪ 已装字体 id。
 * 动态白名单的底座，ReaderStore 收窄时使用；`ready` 之前它是不完整的，
 * 调用方须按阶段 C 的时序规则（到齐后再收窄一次）处理。
 */
export function allowedFontFamilies(installed: InstalledFont[]): string[] {
  return [...READER_FONT_PRESETS, ...installed.map((f) => f.id)]
}
