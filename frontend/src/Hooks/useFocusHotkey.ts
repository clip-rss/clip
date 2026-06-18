import { useEffect } from 'react'
import { useArticleStore, useLayoutStore } from '../Stores'

/** 是否为可编辑元素（避免在输入框内劫持快捷键）。 */
function isEditable(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null
  if (!el) return false
  const tag = el.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || el.isContentEditable
}

/**
 * 全局快捷键 `Ctrl+Shift+F`（macOS `Cmd+Shift+F`）切换专注阅读模式。
 *
 * 进入需已选中文章；退出始终可用。在 App 顶层挂载一次。
 */
export function useFocusHotkey(): void {
  useEffect(() => {
    function onKey(e: KeyboardEvent): void {
      if (!(e.ctrlKey || e.metaKey) || !e.shiftKey || e.key.toLowerCase() !== 'f') return
      if (isEditable(e.target)) return
      e.preventDefault()
      const inFocus = useLayoutStore.getState().focusMode
      const hasItem = useArticleStore.getState().selectedItemId !== null
      if (inFocus || hasItem) useLayoutStore.getState().toggleFocus()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
}
