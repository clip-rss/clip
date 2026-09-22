import { describe, expect, it } from 'vitest'
import { articleBody, hasArticleBody } from './ArticleBody'

describe('articleBody', () => {
  it('没有提取结果时用 RSS 正文', () => {
    expect(articleBody({ content: '<p>摘要</p>', fullContent: '' })).toBe(
      '<p>摘要</p>',
    )
  })

  it('提取到全文后优先用全文', () => {
    expect(
      articleBody({ content: '<p>摘要</p>', fullContent: '<p>全文</p>' }),
    ).toBe('<p>全文</p>')
  })

  it('提取结果为空串时回落，不会渲染成空正文', () => {
    // 后端不写空串，但类型上允许——回落比显示空白安全。
    expect(articleBody({ content: '<p>摘要</p>', fullContent: '' })).toBe(
      '<p>摘要</p>',
    )
  })

  it('两份都空时返回空串', () => {
    expect(articleBody({ content: '', fullContent: '' })).toBe('')
  })
})

describe('hasArticleBody', () => {
  it('有 RSS 正文即为 true', () => {
    expect(hasArticleBody({ content: '<p>x</p>', fullContent: '' })).toBe(true)
  })

  it('只有提取结果也为 true', () => {
    expect(hasArticleBody({ content: '', fullContent: '<p>全文</p>' })).toBe(
      true,
    )
  })

  it('纯空白视同没有正文', () => {
    expect(hasArticleBody({ content: '   \n\t ', fullContent: '' })).toBe(false)
    expect(hasArticleBody({ content: '', fullContent: '  ' })).toBe(false)
  })

  it('两份都空为 false', () => {
    expect(hasArticleBody({ content: '', fullContent: '' })).toBe(false)
  })
})
