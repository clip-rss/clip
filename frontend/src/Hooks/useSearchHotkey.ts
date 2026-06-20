import { useEffect } from 'react'

/** 是否为可编辑元素（避免在输入框内劫持快捷键）。 */
function isEditable(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null
  if (!el) return false
  const tag = el.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || el.isContentEditable
}

/**
 * 全局快捷键 `/` 聚焦顶部搜索框（非输入态时）。在 App 顶层挂载一次。
 *
 * @param focus 聚焦搜索框的回调（由 Toolbar 通过 ref 提供）。
 */
export function useSearchHotkey(focus: () => void): void {
  useEffect(() => {
    function onKey(e: KeyboardEvent): void {
      if (e.key !== '/' || e.ctrlKey || e.metaKey || e.altKey) return
      if (isEditable(e.target)) return
      e.preventDefault()
      focus()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [focus])
}
