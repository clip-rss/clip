import { describe, it, expect, beforeEach } from 'vitest'
import {
  THEME_CACHE_KEY,
  THEME_CACHE_VERSION,
  readThemeCache,
  writeThemeCache,
} from './PrefsCache'

beforeEach(() => {
  localStorage.clear()
})

describe('PrefsCache', () => {
  it('无缓存时返回 null', () => {
    expect(readThemeCache()).toBeNull()
  })

  it('写入后可读回', () => {
    writeThemeCache('sepia')
    expect(readThemeCache()).toBe('sepia')
  })

  it('写入的是当前版本格式', () => {
    writeThemeCache('dark')
    const raw = JSON.parse(localStorage.getItem(THEME_CACHE_KEY)!)
    expect(raw).toEqual({ v: THEME_CACHE_VERSION, preference: 'dark' })
  })

  // 老用户升级后的第一帧，迁移还没跑，此时仍要能涂对主题。
  it('兼容 zustand persist 旧格式', () => {
    localStorage.setItem(
      THEME_CACHE_KEY,
      JSON.stringify({ state: { preference: 'dark' }, version: 0 }),
    )
    expect(readThemeCache()).toBe('dark')
  })

  it('非法偏好值返回 null', () => {
    localStorage.setItem(
      THEME_CACHE_KEY,
      JSON.stringify({ v: 2, preference: 'neon' }),
    )
    expect(readThemeCache()).toBeNull()
  })

  it('损坏内容返回 null 而非抛错', () => {
    localStorage.setItem(THEME_CACHE_KEY, '{ not json')
    expect(readThemeCache()).toBeNull()
  })

  it('四档偏好都能往返', () => {
    for (const pref of ['light', 'dark', 'sepia', 'system'] as const) {
      writeThemeCache(pref)
      expect(readThemeCache()).toBe(pref)
    }
  })
})
