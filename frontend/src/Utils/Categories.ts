// 文章标签（items.categories）的解析。
// 后端以 JSON 数组字符串存储 RSS/Atom 的 <category>（见 scheduler.encodeCategories），
// 空则为空串。内容源自订阅源，属不可信输入，故解析全程不抛异常。

/**
 * 解析文章标签字符串为标签数组。
 *
 * 空串、非法 JSON、非数组结构一律返回空数组；数组内非字符串项与空白项被丢弃。
 * 保留源顺序，并按大小写不敏感去重（保留首次出现的写法，避免 `Tech`/`tech` 重复成两枚）。
 */
export function parseCategories(raw: string): string[] {
  if (!raw) return []

  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return []
  }
  if (!Array.isArray(parsed)) return []

  const seen = new Set<string>()
  const tags: string[] = []
  for (const entry of parsed) {
    if (typeof entry !== 'string') continue
    const tag = entry.trim()
    if (!tag) continue
    const key = tag.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    tags.push(tag)
  }
  return tags
}
