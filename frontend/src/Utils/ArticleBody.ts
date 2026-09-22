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
 * 实际要渲染的正文 HTML。
 *
 * showSummary 为真时强制用 RSS 正文（手动切回摘要，不看 fullContent）；否则提取过
 * 全文就用全文，再回落 RSS 正文。两份都没有时返回空串，调用方据此展示空状态。
 */
export function articleBody(
  item: ArticleBodyFields,
  showSummary = false,
): string {
  if (showSummary) return item.content
  return item.fullContent || item.content
}

/** 是否有正文可渲染（空白正文视同没有）。 */
export function hasArticleBody(
  item: ArticleBodyFields,
  showSummary = false,
): boolean {
  return articleBody(item, showSummary).trim() !== ''
}

/** RSS 正文是否非空——只有它为真时才有「切回摘要」这条路可走。 */
export function hasRssContent(item: ArticleBodyFields): boolean {
  return item.content.trim() !== ''
}

/**
 * 「获取全文」按钮当前的形态。五态而非三态，因为提取完成后按钮从
 * 「一次性抓取」变成了「摘要 ⇄ 全文」开关。
 *
 * - `fetch`     未提取，点击去抓原文
 * - `fetching`  提取中，禁用
 * - `done`      已有全文，但 RSS 根本没给正文，没有摘要可切（禁用，保持原完成态）
 * - `full`      正在看全文，点击切回摘要
 * - `summary`   正在看摘要，点击切回全文
 */
export type FullTextButtonMode =
  | 'fetch'
  | 'fetching'
  | 'done'
  | 'full'
  | 'summary'

export function fullTextButtonMode(
  item: ArticleBodyFields,
  fetching: boolean,
  showSummary: boolean,
): FullTextButtonMode {
  if (fetching) return 'fetching'
  if (!item.fullContent) return 'fetch'
  if (!hasRssContent(item)) return 'done'
  return showSummary ? 'summary' : 'full'
}

/**
 * 每种形态对应的提示文案 key。
 *
 * 放在这里而不是各工具栏里：文案描述的是「点下去会发生什么」，两处工具栏必须
 * 给出同一句，分散写迟早分叉。`full` 提示的是切回摘要，所以是 `showSummary`
 * 而不是 `showFull` —— 别按名字望文生义。
 */
export const FULL_TEXT_TITLE_KEY: Record<FullTextButtonMode, string> = {
  fetch: 'reader.fullText.fetch',
  fetching: 'reader.fullText.fetching',
  done: 'reader.fullText.done',
  full: 'reader.fullText.showSummary',
  summary: 'reader.fullText.showFull',
}
