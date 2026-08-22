// 轻量 Markdown → HTML 转换，用于渲染远程拉取的 CHANGELOG.md。
//
// 逐行解析而非按空行分块：CHANGELOG 里标题与紧随其后的列表之间没有空行，
// 按块分割会把「标题 + 列表」当成同一种块，只能二选一（历史上整块退化成了纯文本段落）。
//
// 支持的子集：ATX 标题（# ~ ######）、无序/有序列表、围栏代码块、引用、
// 分隔线、段落，以及行内的加粗、斜体、删除线、行内代码、链接。
// 不支持嵌套列表（缩进项按同级处理）、表格、图片、引用式链接。
//
// 输出仍是不可信内容，调用方必须再经 sanitizeHtml 才能交给 dangerouslySetInnerHTML。

const HEADING = /^(#{1,6})\s+(.*)$/
const UL_ITEM = /^\s*[-*+]\s+(.*)$/
const OL_ITEM = /^\s*\d+[.)]\s+(.*)$/
const QUOTE = /^>\s?(.*)$/
const RULE = /^\s*(?:-{3,}|\*{3,}|_{3,})\s*$/
const FENCE = /^\s*```(\w*)\s*$/

/** 把 Markdown 文本转成 HTML 片段。 */
export function markdownToHtml(md: string): string {
  if (!md) return ''

  // 统一换行并去掉 NUL：CRLF 会残留在 `.*` 捕获的标题文本里，
  // NUL 会与行内代码占位符冲突。
  const lines = md
    .replace(/\r\n?/g, '\n')
    .replace(/\u0000/g, '')
    .split('\n')

  const out: string[] = []
  let list: { tag: 'ul' | 'ol'; items: string[] } | null = null
  let quote: string[] | null = null
  let para: string[] | null = null
  let fence: string[] | null = null

  /** 结束当前累积的列表/引用/段落块。 */
  const flush = (): void => {
    if (list) {
      out.push(`<${list.tag}>${list.items.join('')}</${list.tag}>`)
      list = null
    }
    if (quote) {
      out.push(`<blockquote>${inline(quote.join(' '))}</blockquote>`)
      quote = null
    }
    if (para) {
      out.push(`<p>${inline(para.join(' '))}</p>`)
      para = null
    }
  }

  for (const line of lines) {
    // 围栏代码块内部不做任何 Markdown 解析
    if (fence) {
      if (FENCE.test(line)) {
        out.push(`<pre><code>${escapeHtml(fence.join('\n'))}</code></pre>`)
        fence = null
      } else {
        fence.push(line)
      }
      continue
    }

    const fenceStart = line.match(FENCE)
    if (fenceStart) {
      flush()
      fence = []
      continue
    }

    // 空行结束当前块
    if (!line.trim()) {
      flush()
      continue
    }

    const heading = line.match(HEADING)
    if (heading) {
      flush()
      const level = heading[1].length
      out.push(`<h${level}>${inline(heading[2].trim())}</h${level}>`)
      continue
    }

    if (RULE.test(line)) {
      flush()
      out.push('<hr />')
      continue
    }

    const ul = line.match(UL_ITEM)
    if (ul) {
      if (para) flush()
      if (quote) flush()
      if (list?.tag !== 'ul') {
        flush()
        list = { tag: 'ul', items: [] }
      }
      list.items.push(`<li>${inline(ul[1].trim())}</li>`)
      continue
    }

    const ol = line.match(OL_ITEM)
    if (ol) {
      if (para) flush()
      if (quote) flush()
      if (list?.tag !== 'ol') {
        flush()
        list = { tag: 'ol', items: [] }
      }
      list.items.push(`<li>${inline(ol[1].trim())}</li>`)
      continue
    }

    const q = line.match(QUOTE)
    if (q) {
      if (list) flush()
      if (para) flush()
      quote ??= []
      quote.push(q[1].trim())
      continue
    }

    // 列表项/引用的后续续行并入上一项，避免被当成新段落拆开
    if (list && list.items.length > 0) {
      const last = list.items.length - 1
      list.items[last] = list.items[last].replace(
        /<\/li>$/,
        ` ${inline(line.trim())}</li>`,
      )
      continue
    }
    if (quote) {
      quote.push(line.trim())
      continue
    }

    para ??= []
    para.push(line.trim())
  }

  // 文件结尾未闭合的围栏代码块也要输出，否则内容整段丢失
  if (fence) {
    out.push(`<pre><code>${escapeHtml(fence.join('\n'))}</code></pre>`)
  }
  flush()

  return out.join('\n')
}

/** 行内格式：加粗、斜体、删除线、行内代码、链接。先转义再替换。 */
function inline(text: string): string {
  const codes: string[] = []
  // 行内代码先摘出占位，避免其中的 * _ [ 被当成格式标记
  let out = escapeHtml(text).replace(/`([^`]+)`/g, (_m, code: string) => {
    codes.push(code)
    return `\u0000${codes.length - 1}\u0000`
  })

  out = out
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/__(.+?)__/g, '<strong>$1</strong>')
    .replace(/~~(.+?)~~/g, '<del>$1</del>')
    .replace(/(^|[^*\w])\*([^*\s][^*]*?)\*(?!\w)/g, '$1<em>$2</em>')
    .replace(/(^|[^_\w])_([^_\s][^_]*?)_(?!\w)/g, '$1<em>$2</em>')
    .replace(
      /\[([^\]]*)\]\(([^)\s]+)\)/g,
      '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>',
    )

  return out.replace(
    /\u0000(\d+)\u0000/g,
    (_m, i: string) => `<code>${codes[Number(i)]}</code>`,
  )
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}
