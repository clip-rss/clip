import { describe, it, expect } from 'vitest'
import { toApiError } from './index'

describe('toApiError', () => {
  it('剥出 Wails CallError JSON 里的 message', () => {
    const err = new Error(
      JSON.stringify({
        message: '代理连接失败：超时',
        cause: {},
        kind: 'RuntimeError',
      }),
    )
    expect(toApiError(err)).toBe('代理连接失败：超时')
  })

  it('普通错误与字符串原样返回', () => {
    expect(toApiError(new Error('网络不可用'))).toBe('网络不可用')
    expect(toApiError('plain string')).toBe('plain string')
    expect(toApiError(42)).toBe('42')
  })

  it('JSON 但没有可用 message 时原样返回', () => {
    expect(toApiError(new Error('{"kind":"RuntimeError"}'))).toBe(
      '{"kind":"RuntimeError"}',
    )
    expect(toApiError(new Error('null'))).toBe('null')
    expect(toApiError(new Error('{"message":""}'))).toBe('{"message":""}')
  })
})
