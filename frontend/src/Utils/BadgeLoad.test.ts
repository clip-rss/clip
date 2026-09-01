import { describe, it, expect } from 'vitest'
import { BADGE_WARN_THRESHOLD, badgeLoad, badgeWarnProgress } from './BadgeLoad'

describe('badgeLoad', () => {
  it('上限非正数（含 0 = 不限制）返回 null', () => {
    expect(badgeLoad(10, 0)).toBeNull()
    expect(badgeLoad(10, -5)).toBeNull()
  })

  it('未读按上限归一化', () => {
    expect(badgeLoad(250, 500)).toBe(0.5)
    expect(badgeLoad(400, 500)).toBe(0.8)
    expect(badgeLoad(500, 500)).toBe(1)
  })

  it('未读超过上限（如用户调低过上限）按 1 封顶', () => {
    expect(badgeLoad(600, 500)).toBe(1)
  })

  it('零未读负载为 0', () => {
    expect(badgeLoad(0, 500)).toBe(0)
  })
})

describe('badgeWarnProgress', () => {
  it('null 负载（无上限）返回 null', () => {
    expect(badgeWarnProgress(null)).toBeNull()
  })

  it('未达阈值保持常态，返回 null', () => {
    expect(badgeWarnProgress(0)).toBeNull()
    expect(badgeWarnProgress(0.79)).toBeNull()
  })

  it('阈值处为纯黄起点（进度 0）', () => {
    expect(badgeWarnProgress(BADGE_WARN_THRESHOLD)).toBe(0)
  })

  it('80%~100% 线性映射到 0~1', () => {
    expect(badgeWarnProgress(0.8)).toBe(0)
    expect(badgeWarnProgress(0.9)).toBeCloseTo(0.5)
    expect(badgeWarnProgress(1)).toBe(1)
  })

  it('负载超过 1（防御）按 1 封顶', () => {
    expect(badgeWarnProgress(1.2)).toBe(1)
  })
})
