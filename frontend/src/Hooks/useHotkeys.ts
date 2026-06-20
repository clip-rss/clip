import { useEffect, useRef } from 'react'

/** 单条快捷键绑定。 */
export interface Hotkey {
  /**
   * 归一化组合键，与 {@link comboFromEvent} 输出一致。
   * 形如 `mod+n`、`mod+shift+i`、`shift+r`、`space`、`shift+space`、`r`、`/`。
   * `mod` 同时匹配 Ctrl 与 Cmd，实现跨平台。
   */
  combo: string
  handler: (e: KeyboardEvent) => void
  /** 是否允许在输入框/可编辑元素内触发（默认 false）。 */
  allowInInput?: boolean
}

const MODIFIER_KEYS = new Set(['Shift', 'Control', 'Alt', 'Meta'])

/**
 * 将键盘事件归一化为组合键字符串（修饰键顺序固定：mod→alt→shift→key）。
 *
 * - `mod` = Ctrl 或 Cmd（跨平台）；
 * - 空格映射为 `space`；其余键取 `e.key` 小写；
 * - 仅按下修饰键本身时返回 `null`（无意义组合）。
 */
export function comboFromEvent(e: KeyboardEvent): string | null {
  if (MODIFIER_KEYS.has(e.key)) return null
  const parts: string[] = []
  if (e.ctrlKey || e.metaKey) parts.push('mod')
  if (e.altKey) parts.push('alt')
  if (e.shiftKey) parts.push('shift')
  let key = e.key.toLowerCase()
  if (key === ' ' || key === 'spacebar') key = 'space'
  parts.push(key)
  return parts.join('+')
}

/** 目标是否为可编辑元素（输入框/文本域/contenteditable）。 */
export function isEditableTarget(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null
  if (!el) return false
  const tag = el.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || el.isContentEditable === true
}

/**
 * 是否有 Radix 模态对话框处于打开态。
 *
 * 只匹配带 `data-state="open"` 的 `[role="dialog"]`——专注模式覆盖层虽也是
 * `role="dialog"` 但无 `data-state`，因此不会被误判为模态，其内快捷键照常可用。
 */
export function isModalOpen(): boolean {
  return document.querySelector('[role="dialog"][data-state="open"]') !== null
}

/**
 * 全局快捷键框架：在 window 上挂载单一 keydown 监听，按归一化组合键分发。
 *
 * 统一守卫——可编辑元素内、模态打开时默认不触发；首个匹配的绑定生效（防冲突）。
 * 绑定数组用 ref 持有，监听只安装一次，避免重复注册/注销。
 */
export function useHotkeys(bindings: Hotkey[]): void {
  const ref = useRef(bindings)
  ref.current = bindings

  useEffect(() => {
    function onKey(e: KeyboardEvent): void {
      if (e.isComposing) return
      // 模态打开时全局快捷键让位（其 Esc/焦点陷阱由 Radix 处理）。
      if (isModalOpen()) return
      const combo = comboFromEvent(e)
      if (!combo) return
      for (const binding of ref.current) {
        if (binding.combo !== combo) continue
        if (!binding.allowInInput && isEditableTarget(e.target)) return
        binding.handler(e)
        return
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
}
