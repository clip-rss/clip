import { describe, expect, it } from 'vitest'
import { clampToRange, resolveDraft } from './InputNumber'

describe('clampToRange', () => {
  it('两端无界时原样返回', () => {
    expect(clampToRange(7, null, null)).toBe(7)
    expect(clampToRange(7)).toBe(7)
  })

  it('越界时收窄到边界', () => {
    expect(clampToRange(0, 1, 30)).toBe(1)
    expect(clampToRange(99, 1, 30)).toBe(30)
    expect(clampToRange(15, 1, 30)).toBe(15)
  })

  it('只在给定的一侧有界', () => {
    expect(clampToRange(-5, 0, null)).toBe(0)
    expect(clampToRange(999, null, 30)).toBe(30)
  })
})

describe('resolveDraft', () => {
  it('合法数字原样返回', () => {
    expect(resolveDraft('20', 16, 1, 30)).toBe(20)
    expect(resolveDraft('0', 16, null, null)).toBe(0)
  })

  it('空串回落 fallback', () => {
    expect(resolveDraft('', 16, 1, 30)).toBe(16)
  })

  it('非数字回落 fallback，而不是被解析成部分数字', () => {
    expect(resolveDraft('abc', 16, 1, 30)).toBe(16)
    expect(resolveDraft('-1', 16, 1, 30)).toBe(16)
    expect(resolveDraft('1.5', 16, 1, 30)).toBe(16)
    expect(resolveDraft('2e3', 16, null, null)).toBe(16)
  })

  it('越界收窄到边界而非回落 fallback', () => {
    expect(resolveDraft('0', 16, 1, 30)).toBe(1)
    expect(resolveDraft('999', 16, 1, 30)).toBe(30)
  })
})
