import { describe, it, expect, beforeEach } from 'vitest'
import { useLayoutStore, SIDEBAR_MIN, SIDEBAR_MAX, LIST_MIN, LIST_MAX } from './LayoutStore'

beforeEach(() => {
  useLayoutStore.setState({ sidebarWidth: 260, listWidth: 380, focusMode: false })
})

describe('LayoutStore 栏宽', () => {
  it('setSidebarWidth 限制在 [MIN, MAX]', () => {
    const { setSidebarWidth } = useLayoutStore.getState()
    setSidebarWidth(0)
    expect(useLayoutStore.getState().sidebarWidth).toBe(SIDEBAR_MIN)
    setSidebarWidth(9999)
    expect(useLayoutStore.getState().sidebarWidth).toBe(SIDEBAR_MAX)
  })

  it('setListWidth 限制在 [MIN, MAX]', () => {
    const { setListWidth } = useLayoutStore.getState()
    setListWidth(0)
    expect(useLayoutStore.getState().listWidth).toBe(LIST_MIN)
    setListWidth(9999)
    expect(useLayoutStore.getState().listWidth).toBe(LIST_MAX)
  })

  it('resizeSidebar 累加 delta 且限制在 [MIN, MAX]', () => {
    const { resizeSidebar } = useLayoutStore.getState()
    resizeSidebar(10)
    expect(useLayoutStore.getState().sidebarWidth).toBe(270)
    resizeSidebar(-5)
    expect(useLayoutStore.getState().sidebarWidth).toBe(265)
    resizeSidebar(-9999)
    expect(useLayoutStore.getState().sidebarWidth).toBe(SIDEBAR_MIN)
  })

  it('resizeList 累加 delta 且限制在 [MIN, MAX]', () => {
    const { resizeList } = useLayoutStore.getState()
    resizeList(20)
    expect(useLayoutStore.getState().listWidth).toBe(400)
    resizeList(-10)
    expect(useLayoutStore.getState().listWidth).toBe(390)
    resizeList(9999)
    expect(useLayoutStore.getState().listWidth).toBe(LIST_MAX)
  })
})

describe('LayoutStore 专注模式', () => {
  it('默认关闭', () => {
    expect(useLayoutStore.getState().focusMode).toBe(false)
  })

  it('enter/exit 切换状态', () => {
    const s = useLayoutStore.getState()
    s.enterFocus()
    expect(useLayoutStore.getState().focusMode).toBe(true)
    s.exitFocus()
    expect(useLayoutStore.getState().focusMode).toBe(false)
  })

  it('toggleFocus 取反', () => {
    const { toggleFocus } = useLayoutStore.getState()
    toggleFocus()
    expect(useLayoutStore.getState().focusMode).toBe(true)
    toggleFocus()
    expect(useLayoutStore.getState().focusMode).toBe(false)
  })
})
