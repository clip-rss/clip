import type { ReactNode } from 'react'

/** 转义正则元字符，使关键词按字面量匹配。 */
function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/**
 * 将 query 中的各关键词（按空白切分）在文本中高亮为 <mark>。
 * 大小写不敏感；无匹配时原样返回文本。供搜索结果的标题/摘要复用。
 *
 * @param text 原始文本
 * @param query 搜索输入原文
 * @param markClass <mark> 的 className
 */
export function highlightText(
  text: string,
  query: string,
  markClass: string,
): ReactNode {
  const tokens = query.trim().split(/\s+/).filter(Boolean)
  if (text === '' || tokens.length === 0) return text

  const pattern = tokens.map(escapeRegExp).join('|')
  const re = new RegExp(`(${pattern})`, 'gi')
  const parts = text.split(re)
  if (parts.length === 1) return text

  // split 带捕获组：奇数下标为匹配片段。
  return parts.map((part, i) =>
    i % 2 === 1 ? (
      <mark key={i} className={markClass}>
        {part}
      </mark>
    ) : (
      part
    ),
  )
}
