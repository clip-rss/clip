import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest'

vi.mock('../Utils', () => ({
  FontService: {
    FetchFontCatalog: vi.fn(),
    ListInstalledFonts: vi.fn(),
  },
}))

import { FontService } from '../Utils'
import {
  useFontStore,
  installedFontIds,
  allowedFontFamilies,
} from './FontStore'

const FetchFontCatalog = FontService.FetchFontCatalog as Mock
const ListInstalledFonts = FontService.ListInstalledFonts as Mock

const catalog = { version: 1, release: 'fonts-v1', fonts: [] }

beforeEach(() => {
  vi.resetAllMocks()
  useFontStore.setState({
    catalog: null,
    catalogFailed: false,
    installed: [],
    ready: false,
    refreshing: false,
  })
})

describe('FontStore.refresh', () => {
  it('并行拉取目录与已装清单，完成后 ready', async () => {
    FetchFontCatalog.mockResolvedValue(catalog)
    ListInstalledFonts.mockResolvedValue([
      { id: 'lxgw-wenkai', files: [], totalSize: 0 },
    ])

    await useFontStore.getState().refresh()

    const s = useFontStore.getState()
    expect(s.catalog).toEqual(catalog)
    expect(s.installed).toHaveLength(1)
    expect(s.ready).toBe(true)
    expect(s.refreshing).toBe(false)
    expect(installedFontIds()).toEqual(['lxgw-wenkai'])
  })

  it('目录失败置 catalogFailed，已装清单照常', async () => {
    FetchFontCatalog.mockRejectedValue(new Error('boom'))
    ListInstalledFonts.mockResolvedValue([{ id: 'a', files: [], totalSize: 0 }])

    await useFontStore.getState().refresh()

    const s = useFontStore.getState()
    expect(s.catalog).toBeNull()
    expect(s.catalogFailed).toBe(true)
    expect(s.installed).toHaveLength(1)
    expect(s.ready).toBe(true)
  })

  it('已装清单失败不致命：catalog 照常，installed 保持原样', async () => {
    FetchFontCatalog.mockResolvedValue(catalog)
    ListInstalledFonts.mockRejectedValue(new Error('boom'))

    await useFontStore.getState().refresh()

    const s = useFontStore.getState()
    expect(s.catalog).toEqual(catalog)
    expect(s.installed).toEqual([])
    expect(s.ready).toBe(true)
  })

  it('刷新中重入直接返回，不并发', async () => {
    let release!: (v: unknown) => void
    FetchFontCatalog.mockReturnValue(
      new Promise((resolve) => {
        release = resolve
      }),
    )
    ListInstalledFonts.mockResolvedValue([])

    const first = useFontStore.getState().refresh()
    await useFontStore.getState().refresh() // 应被 refreshing 挡住
    expect(FetchFontCatalog).toHaveBeenCalledTimes(1)

    release(catalog)
    await first
    expect(useFontStore.getState().catalog).toEqual(catalog)
  })
})

describe('allowedFontFamilies', () => {
  it('预设三档 ∪ 已装字体 id', () => {
    expect(
      allowedFontFamilies([{ id: 'lxgw-wenkai', files: [], totalSize: 0 }]),
    ).toEqual(['sans', 'serif', 'mono', 'lxgw-wenkai'])
  })
})
