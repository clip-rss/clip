import { describe, it, expect } from 'vitest'
import { modKey, shortcutHint } from './Shortcut'

describe('modKey', () => {
  it('mac 用 ⌘，其余用 Ctrl', () => {
    expect(modKey('mac')).toBe('⌘')
    expect(modKey('windows')).toBe('Ctrl')
    expect(modKey(null)).toBe('Ctrl')
  })
})

describe('shortcutHint', () => {
  it('mac 紧凑无分隔', () => {
    expect(shortcutHint('mac', ['N'])).toBe('(⌘N)')
    expect(shortcutHint('mac', ['Shift', 'I'])).toBe('(⌘ShiftI)')
  })

  it('非 mac 用加号分隔', () => {
    expect(shortcutHint('windows', ['N'])).toBe('(Ctrl+N)')
    expect(shortcutHint(null, ['Shift', 'E'])).toBe('(Ctrl+Shift+E)')
  })
})
