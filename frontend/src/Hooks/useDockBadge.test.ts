import { describe, it, expect, vi } from 'vitest'

// useDockBadge 通过 barrel 引入 Stores/Utils（含 Wails 绑定与主题副作用），
// 单测仅覆盖纯逻辑，故在导入阶段将其 mock 掉。
vi.mock('../Stores', () => ({
  useSidebarStore: { getState: vi.fn(), subscribe: vi.fn() },
  useSettingsStore: { getState: vi.fn(), subscribe: vi.fn() },
}))
vi.mock('../Utils', () => ({
  DockService: {
    SetBadge: vi.fn(),
    RemoveBadge: vi.fn(),
    SetCustomBadge: vi.fn(),
  },
}))

import { totalUnread, badgeLabel, badgeAction } from './useDockBadge'

describe('totalUnread', () => {
  it('空列表返回 0', () => {
    expect(totalUnread([])).toBe(0)
  })

  it('累加所有订阅源的未读数', () => {
    expect(
      totalUnread([{ unreadCount: 3 }, { unreadCount: 0 }, { unreadCount: 5 }]),
    ).toBe(8)
  })
})

describe('badgeLabel', () => {
  it('开关关闭时始终返回 null（移除 badge）', () => {
    expect(badgeLabel(0, false)).toBeNull()
    expect(badgeLabel(42, false)).toBeNull()
  })

  it('开关开启且未读为 0 时返回 null', () => {
    expect(badgeLabel(0, true)).toBeNull()
  })

  it('开关开启且未读大于 0 时返回数字字符串', () => {
    expect(badgeLabel(1, true)).toBe('1')
    expect(badgeLabel(42, true)).toBe('42')
  })
})

describe('badgeAction', () => {
  it('开关关闭时始终移除（任意平台）', () => {
    expect(badgeAction('mac', 42, false)).toEqual({ kind: 'remove' })
    expect(badgeAction('windows', 42, false)).toEqual({ kind: 'remove' })
  })

  it('未读为 0 时移除（任意平台）', () => {
    expect(badgeAction('mac', 0, true)).toEqual({ kind: 'remove' })
    expect(badgeAction('windows', 0, true)).toEqual({ kind: 'remove' })
  })

  it('macOS 有未读时显示数字', () => {
    expect(badgeAction('mac', 1, true)).toEqual({ kind: 'number', label: '1' })
    expect(badgeAction('mac', 128, true)).toEqual({
      kind: 'number',
      label: '128',
    })
  })

  it('Windows 有未读时只显示红点（不带数字）', () => {
    expect(badgeAction('windows', 1, true)).toEqual({ kind: 'dot' })
    expect(badgeAction('windows', 128, true)).toEqual({ kind: 'dot' })
  })
})
