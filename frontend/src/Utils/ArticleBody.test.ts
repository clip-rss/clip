import { describe, expect, it } from 'vitest'
import {
  articleBody,
  hasArticleBody,
  hasRssContent,
  fullTextButtonMode,
  FULL_TEXT_TITLE_KEY,
  type FullTextButtonMode,
} from './ArticleBody'

const BOTH = { content: '<p>摘要</p>', fullContent: '<p>全文</p>' }
const ONLY_FULL = { content: '', fullContent: '<p>全文</p>' }
const ONLY_RSS = { content: '<p>摘要</p>', fullContent: '' }
const NEITHER = { content: '', fullContent: '' }

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

describe('articleBody 的摘要模式（手动切回 RSS 正文）', () => {
  it('showSummary 时忽略全文，用 RSS 正文', () => {
    expect(articleBody(BOTH, true)).toBe('<p>摘要</p>')
  })

  it('showSummary 时没有 RSS 正文就是空（调用方据此禁掉切换）', () => {
    expect(articleBody(ONLY_FULL, true)).toBe('')
    expect(hasArticleBody(ONLY_FULL, true)).toBe(false)
  })

  it('默认（不传）仍是全文优先，与加开关之前一致', () => {
    expect(articleBody(BOTH)).toBe('<p>全文</p>')
  })
})

describe('hasRssContent', () => {
  it('纯空白的 RSS 正文不算有摘要可切', () => {
    expect(hasRssContent(ONLY_RSS)).toBe(true)
    expect(hasRssContent(ONLY_FULL)).toBe(false)
    expect(hasRssContent({ content: '  \n', fullContent: '' })).toBe(false)
  })
})

describe('fullTextButtonMode', () => {
  it('提取中优先于其他一切', () => {
    expect(fullTextButtonMode(BOTH, true, false)).toBe('fetching')
    expect(fullTextButtonMode(ONLY_RSS, true, false)).toBe('fetching')
  })

  it('没有全文时是抓取按钮', () => {
    expect(fullTextButtonMode(ONLY_RSS, false, false)).toBe('fetch')
    expect(fullTextButtonMode(NEITHER, false, false)).toBe('fetch')
  })

  it('有全文且有摘要时是可切换的开关', () => {
    expect(fullTextButtonMode(BOTH, false, false)).toBe('full')
    expect(fullTextButtonMode(BOTH, false, true)).toBe('summary')
  })

  it('有全文但 RSS 没给正文时是完成态（没有摘要可切）', () => {
    // 这条最容易漏：不判 hasRssContent 就会给出一个切过去是空状态的开关。
    expect(fullTextButtonMode(ONLY_FULL, false, false)).toBe('done')
    expect(fullTextButtonMode(ONLY_FULL, false, true)).toBe('done')
  })

  it('每种形态都有文案 key，且 full 提示的是「切回摘要」', () => {
    const modes: FullTextButtonMode[] = [
      'fetch',
      'fetching',
      'done',
      'full',
      'summary',
    ]
    for (const mode of modes) {
      expect(FULL_TEXT_TITLE_KEY[mode]).toBeTruthy()
    }
    expect(FULL_TEXT_TITLE_KEY.full).toBe('reader.fullText.showSummary')
    expect(FULL_TEXT_TITLE_KEY.summary).toBe('reader.fullText.showFull')
  })
})
