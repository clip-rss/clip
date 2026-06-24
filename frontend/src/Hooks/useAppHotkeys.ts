import { useMemo } from 'react'
import { useArticleStore, useLayoutStore, useSidebarStore } from '../Stores'
import { type Hotkey, useHotkeys } from './useHotkeys'

interface AppHotkeyActions {
  /** 打开「添加订阅」弹窗（App 持有弹窗状态）。 */
  onAddFeed: () => void
  /** 打开设置面板（App 持有弹窗状态）。 */
  onOpenSettings: () => void
}

/** 焦点是否落在交互元素上——此时应保留空格/回车的默认行为（如激活按钮）。 */
function isInteractive(el: Element | null): boolean {
  if (!el) return false
  const tag = el.tagName
  if (
    tag === 'BUTTON' ||
    tag === 'A' ||
    tag === 'INPUT' ||
    tag === 'TEXTAREA' ||
    tag === 'SELECT'
  ) {
    return true
  }
  if (el.getAttribute('role') === 'button') return true
  return (el as HTMLElement).isContentEditable === true
}

/** 翻动当前可见阅读区（dir=1 向下，dir=-1 向上）。专注模式优先其覆盖层。 */
function pageReader(dir: 1 | -1): void {
  const inFocus = useLayoutStore.getState().focusMode
  const selector = inFocus
    ? '[data-reader-scroll="focus"]'
    : '[data-reader-scroll="main"]'
  const el = document.querySelector(selector) as HTMLElement | null
  if (!el) return
  const step = Math.max(el.clientHeight - 60, 100)
  el.scrollBy({ top: dir * step, behavior: 'smooth' })
}

/**
 * 注册应用全部全局快捷键。在 App 顶层挂载一次。
 *
 * 覆盖：添加订阅、刷新、阅读区翻页、筛选切换、聚焦搜索、专注模式。
 * `Esc`（关闭模态/退出专注）与专注模式下的 `J/K` 分别由 Radix 与 FocusMode 自行处理。
 */
export function useAppHotkeys(actions: AppHotkeyActions): void {
  const { onAddFeed, onOpenSettings } = actions

  const bindings = useMemo<Hotkey[]>(() => {
    return [
      {
        combo: 'mod+n',
        handler: (e) => {
          e.preventDefault()
          onAddFeed()
        },
      },
      {
        combo: 'mod+,',
        handler: (e) => {
          e.preventDefault()
          onOpenSettings()
        },
      },
      {
        combo: 'mod+shift+f',
        handler: (e) => {
          e.preventDefault()
          const inFocus = useLayoutStore.getState().focusMode
          const hasItem = useArticleStore.getState().selectedItemId !== null
          if (inFocus || hasItem) useLayoutStore.getState().toggleFocus()
        },
      },
      {
        combo: 'mod+1',
        handler: (e) => {
          e.preventDefault()
          useArticleStore.getState().setFilter('all')
        },
      },
      {
        combo: 'mod+2',
        handler: (e) => {
          e.preventDefault()
          useArticleStore.getState().setFilter('unread')
        },
      },
      {
        combo: 'mod+3',
        handler: (e) => {
          e.preventDefault()
          useArticleStore.getState().setFilter('starred')
        },
      },
      {
        combo: 'r',
        handler: (e) => {
          e.preventDefault()
          void useSidebarStore.getState().refreshSelected()
        },
      },
      {
        combo: 'shift+r',
        handler: (e) => {
          e.preventDefault()
          void useSidebarStore.getState().forceRefreshAll()
        },
      },
      {
        combo: '/',
        handler: (e) => {
          e.preventDefault()
          const input = document.getElementById(
            'toolbar-search',
          ) as HTMLInputElement | null
          input?.focus()
        },
      },
      {
        combo: 'space',
        handler: (e) => {
          if (isInteractive(document.activeElement)) return // 保留按钮等的空格默认行为
          e.preventDefault()
          pageReader(1)
        },
      },
      {
        combo: 'shift+space',
        handler: (e) => {
          if (isInteractive(document.activeElement)) return
          e.preventDefault()
          pageReader(-1)
        },
      },
    ]
  }, [onAddFeed, onOpenSettings])

  useHotkeys(bindings)
}
