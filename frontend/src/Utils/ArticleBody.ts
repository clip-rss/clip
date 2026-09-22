// 「显示哪份正文」的唯一判定处（纯函数，便于测试）。
//
// 一篇文章可能有两份正文：RSS 给的 content（可能只是摘要）与按需从原文页面提取的
// fullContent。阅读视图、专注模式与工具栏三处都要用同一条规则，
// 分散写会让它们在「提取完成后」对同一篇文章给出不同判断。
//
// 用 Pick 而非整个 Item：这两个字段是唯一被读取的，测试里就不必造一个完整的 Item。

import type { Item } from '../Types'

type ArticleBodyFields = Pick<Item, 'content' | 'fullContent'>

/**
 * 实际要渲染的正文 HTML：提取过全文就用全文，否则回落到 RSS 正文。
 *
 * 两份都没有时返回空串，调用方据此展示空状态。
 */
export function articleBody(item: ArticleBodyFields): string {
  return item.fullContent || item.content
}

/** 是否有正文可渲染（空白正文视同没有）。 */
export function hasArticleBody(item: ArticleBodyFields): boolean {
  return articleBody(item).trim() !== ''
}
