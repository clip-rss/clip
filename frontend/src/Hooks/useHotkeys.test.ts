import { describe, it, expect, afterEach } from 'vitest'
import { comboFromEvent, isEditableTarget, isModalOpen } from './useHotkeys'

function key(init: KeyboardEventInit): KeyboardEvent {
  return new KeyboardEvent('keydown', init)
}

describe('comboFromEvent', () => {
  it('单键归一化为小写', () => {
    expect(comboFromEvent(key({ key: 'r' }))).toBe('r')
    expect(comboFromEvent(key({ key: 'R', shiftKey: true }))).toBe('shift+r')
  })

  it('Ctrl 与 Cmd 都归一化为 mod', () => {
    expect(comboFromEvent(key({ key: 'n', ctrlKey: true }))).toBe('mod+n')
    expect(comboFromEvent(key({ key: 'n', metaKey: true }))).toBe('mod+n')
  })

  it('修饰键顺序固定为 mod+alt+shift+key', () => {
    expect(
      comboFromEvent(key({ key: 'i', ctrlKey: true, shiftKey: true })),
    ).toBe('mod+shift+i')
    expect(
      comboFromEvent(
        key({ key: 'i', metaKey: true, altKey: true, shiftKey: true }),
      ),
    ).toBe('mod+alt+shift+i')
  })

  it('空格映射为 space', () => {
    expect(comboFromEvent(key({ key: ' ' }))).toBe('space')
    expect(comboFromEvent(key({ key: ' ', shiftKey: true }))).toBe(
      'shift+space',
    )
  })

  it('数字与符号键', () => {
    expect(comboFromEvent(key({ key: '1', ctrlKey: true }))).toBe('mod+1')
    expect(comboFromEvent(key({ key: '/' }))).toBe('/')
  })

  it('仅按下修饰键本身返回 null', () => {
    expect(comboFromEvent(key({ key: 'Shift', shiftKey: true }))).toBeNull()
    expect(comboFromEvent(key({ key: 'Control', ctrlKey: true }))).toBeNull()
    expect(comboFromEvent(key({ key: 'Meta', metaKey: true }))).toBeNull()
  })
})

describe('isEditableTarget', () => {
  it('输入框/文本域视为可编辑', () => {
    expect(isEditableTarget(document.createElement('input'))).toBe(true)
    expect(isEditableTarget(document.createElement('textarea'))).toBe(true)
  })

  it('普通元素与 null 不是可编辑', () => {
    expect(isEditableTarget(document.createElement('div'))).toBe(false)
    expect(isEditableTarget(null)).toBe(false)
  })

  it('contenteditable 视为可编辑', () => {
    const el = document.createElement('div')
    el.contentEditable = 'true'
    // jsdom 不据 contentEditable 属性推导 isContentEditable，手动桩。
    Object.defineProperty(el, 'isContentEditable', { value: true })
    expect(isEditableTarget(el)).toBe(true)
  })
})

describe('isModalOpen', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('无对话框时为 false', () => {
    expect(isModalOpen()).toBe(false)
  })

  it('Radix 打开态对话框为 true', () => {
    document.body.innerHTML = '<div role="dialog" data-state="open"></div>'
    expect(isModalOpen()).toBe(true)
  })

  it('无 data-state 的 role=dialog（如专注覆盖层）不算模态', () => {
    document.body.innerHTML = '<div role="dialog" aria-modal="true"></div>'
    expect(isModalOpen()).toBe(false)
  })
})
