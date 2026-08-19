import { describe, it, expect, beforeEach } from 'vitest'
import { useSearchHistoryStore, SEARCH_HISTORY_MAX } from './SearchHistoryStore'

function push(...queries: string[]): void {
  for (const q of queries) useSearchHistoryStore.getState().push(q)
}

const history = (): string[] => useSearchHistoryStore.getState().history

beforeEach(() => {
  useSearchHistoryStore.setState({ history: [] })
})

describe('SearchHistoryStore', () => {
  it('最近搜索置顶', () => {
    push('a', 'b')
    expect(history()).toEqual(['b', 'a'])
  })

  it('重复项置顶而非追加', () => {
    push('a', 'b', 'a')
    expect(history()).toEqual(['a', 'b'])
  })

  it('超出上限时挤掉最旧的', () => {
    push('1', '2', '3', '4', '5', '6')
    expect(history()).toHaveLength(SEARCH_HISTORY_MAX)
    expect(history()).toEqual(['6', '5', '4', '3', '2'])
  })

  it('丢弃自身前缀：防抖中途停顿产生的中间态不占位', () => {
    push('科技', '科技周刊')
    expect(history()).toEqual(['科技周刊'])
  })

  it('反向前缀是独立查询，正常保留', () => {
    push('科技周刊', '科技')
    expect(history()).toEqual(['科技', '科技周刊'])
  })

  it('空白查询不入历史', () => {
    push('', '   ')
    expect(history()).toEqual([])
  })

  it('入历史前去除首尾空白', () => {
    push('  hi  ')
    expect(history()).toEqual(['hi'])
  })

  it('clear 清空', () => {
    push('a', 'b')
    useSearchHistoryStore.getState().clear()
    expect(history()).toEqual([])
  })
})
