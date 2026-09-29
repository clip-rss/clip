// 正文链接的落点判定：把 RSS 正文里的相对链接解析成绝对地址，并挡掉打不开的地址。
//
// 为什么必须在点击前挡：`openURL` 最终走 Wails 的 Browser.OpenURL，Go 侧
// ValidateAndSanitizeURL 会拒绝**空 scheme** 的地址 —— `url.Parse("#toc")` 的 Scheme
// 正是空串，于是「目录」这类页内锚点被判废，报 `invalid URL: scheme not allowed`。
// 而且拒绝是 **Promise reject**，`openURL` 里那个 try/catch 接不住，会冒到 window 的
// unhandledrejection，被 CrashBoundary 接手成整页崩溃 —— issue #7。
//
// 相对地址的解析基准同样是 articleUrl；媒体地址的版本在 Utils/Media.ts（那边还会改写成
// 代理地址），两者互不相干，只是共用同一个基准。

/** 点一个正文链接该落到哪里。 */
export type LinkTarget =
  /** 交给系统浏览器打开：已解析成绝对的 http(s) 地址。 */
  | { kind: 'external'; url: string }
  /** 本文档内的锚点：页内滚动到该 id，不开浏览器。 */
  | { kind: 'anchor'; id: string }
  /** 打不开：空 href、非 http(s) 协议，或基准缺失导致解析失败。 */
  | { kind: 'invalid' }

/**
 * 解析正文里的一个 href，判断该开浏览器、页内滚动、还是丢弃。
 *
 * **只认 http(s)**：`javascript:` / `data:` / `file:` / `ftp:` 被 Wails 直接拒绝；
 * `mailto:` 虽然放行，但同样过不了「含 shell 元字符」那一关（`?subject=…&body=…` 里的
 * 空格就是元字符），与其让它偶发崩溃，不如统一不开 —— 阅读视图的链接本来就是文章链接。
 *
 * 返回的 `external.url` 因此保证是**带 host 的绝对 http(s) 地址**，正是 Wails 唯一
 * 无条件接受的形态。剩下的风险只有 URL 里的 `;` `$` `|`（URL 规范允许、Wails 判为
 * shell 元字符），由 `openURL` 的兜底接管。
 */
export function resolveLink(raw: string, articleUrl: string): LinkTarget {
  const href = raw.trim()
  if (!href) return { kind: 'invalid' }

  // 纯锚点：`#toc` 没有 scheme，补基准也补不出别的地址，单独认掉。
  // 同文档的完整地址（`https://site.com/post/1#toc`）在 RSS 里也常见，见下面 isSameDocument。
  if (href.startsWith('#')) return anchorTarget(href.slice(1))

  const base = articleUrl.trim()
  // 先按绝对地址解析、失败再套基准：`new URL` 会**先**校验 base，基准非法时连合法的
  // 绝对地址一起抛掉（Media.ts 的 absoluteMediaUrl 先认绝对地址，同一个道理）。
  const parsed = parseUrl(href) ?? (base ? parseUrl(href, base) : null)
  if (!parsed) return { kind: 'invalid' }

  // `new URL` 把 `mailto:` 之类也算「绝对地址」，协议必须自己挡。
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return { kind: 'invalid' }
  }
  if (!parsed.host) return { kind: 'invalid' }

  // 指回本文档自身、只差一个 fragment 的地址：仍然是页内锚点，不该弹浏览器。
  if (parsed.hash && isSameDocument(parsed, base)) {
    return anchorTarget(parsed.hash.slice(1))
  }
  return { kind: 'external', url: parsed.href }
}

/** 链接与文章是否指向同一篇文档（忽略 fragment）。 */
function isSameDocument(link: URL, articleUrl: string): boolean {
  const base = parseUrl(articleUrl)
  if (!base) return false
  return (
    link.origin === base.origin &&
    link.pathname === base.pathname &&
    link.search === base.search
  )
}

/** 把 `#` 后面的 fragment 变成锚点落点；空 fragment 判废。 */
function anchorTarget(fragment: string): LinkTarget {
  const id = decodeFragment(fragment)
  return id ? { kind: 'anchor', id } : { kind: 'invalid' }
}

/**
 * fragment 在链接里是百分号编码的（`#a%20b`），DOM 里的 id 是解码后的原文，查询前
 * 必须解码 —— 不解码就永远匹配不上带空格/中文的 id。残缺的 `%` 解不出来时按原文查，
 * 查不到自然什么都不做。
 */
function decodeFragment(fragment: string): string {
  try {
    return decodeURIComponent(fragment).trim()
  } catch {
    return fragment.trim()
  }
}

/** 解析 URL，失败返回 null（`new URL` 对相对地址、非法基准都抛）。 */
function parseUrl(url: string, base?: string): URL | null {
  try {
    return base ? new URL(url, base) : new URL(url)
  } catch {
    return null
  }
}
