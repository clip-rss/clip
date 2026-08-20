import { afterEach, describe, expect, it, vi } from 'vitest'
import { initialLanguage } from './index'

describe('initialLanguage', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('detects Traditional Chinese before the generic zh branch', () => {
    vi.spyOn(navigator, 'languages', 'get').mockReturnValue([
      'zh-Hant-TW',
      'en-US',
    ])
    expect(initialLanguage()).toBe('zh-TW')
  })

  it('recognizes locale encoding suffixes and preserves Simplified Chinese', () => {
    vi.spyOn(navigator, 'languages', 'get')
      .mockReturnValueOnce(['zh_TW.UTF-8'])
      .mockReturnValueOnce(['zh-CN'])
    expect(initialLanguage()).toBe('zh-TW')
    expect(initialLanguage()).toBe('zh')
  })

  it('falls back to English for non-Chinese locales', () => {
    vi.spyOn(navigator, 'languages', 'get').mockReturnValue(['ja-JP', 'en-US'])
    expect(initialLanguage()).toBe('en')
  })
})
