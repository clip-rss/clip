// 正文媒体地址改写：把远程 http(s) 图片/视频换成后端代理地址。
//
// 为什么要代理：正文图片是 app 代抓的，国内 CDN 普遍按 Referer 做防盗链 —— 某类站点对**空 Referer** 直接 403。
// 前端无论怎么设 referrerpolicy 都只能救其中一类站点（「拦空 Referer」与「拦跨域 Referer」两类规则互斥），只有让服务端带上**文章源站的** Referer 去抓，才能同时满足两类。详见 internal/mediaproxy。
//
// ⚠️ MEDIA_PROXY_PATH 与 internal/mediaproxy/mediaproxy.go 的 PathPrefix 是一份契约，
// 两侧必须同时改。这里用**相对路径**而不是写死协议：Windows 上页面源是
// http://wails.localhost，macOS/Linux 上才是 wails://localhost，相对路径两端都能解析
// 到 Wails 的资产服务器。

export const MEDIA_PROXY_PATH = '/__clip/media'

/** 是否是绝对的远程 http(s) 地址（data: / blob: / 相对地址都不算）。 */
export function isAbsoluteRemoteMedia(url: string): boolean {
  return /^https?:\/\//i.test(url.trim())
}

/**
 * 把可能是相对地址的媒体 URL 解析成绝对地址；解析不出来返回空串。
 *
 * RSS 正文里的相对地址在入库时没有被解析（只有 item.link / enclosure 解析过），
 * 以 articleUrl 为基准补上这一步，相对图片才终于能显示。
 */
export function absoluteMediaUrl(url: string, articleUrl: string): string {
  const raw = url.trim()
  if (!raw) return ''
  if (isAbsoluteRemoteMedia(raw)) return raw
  if (!articleUrl) return ''
  try {
    return new URL(raw, articleUrl).href
  } catch {
    return ''
  }
}

/**
 * 媒体地址 → 后端代理地址。
 *
 * articleUrl 为空（如更新日志弹窗没有文章上下文）、地址不是远程 http(s)、或解析不出
 * 源站时，一律原样返回 —— 不代理只是回到改动前的行为，不会更糟。
 *
 * **幂等**：已经是代理地址的输入原样返回。视频点击时拿到的是 `<video>` 的 currentSrc
 * （已被清洗成代理地址），灯箱会再调用一次本函数 —— 不挡住就会套成代理的代理。
 */
export function mediaProxyUrl(url: string, articleUrl: string): string {
  if (!articleUrl) return url
  if (isProxiedMediaUrl(url)) return url
  const absolute = absoluteMediaUrl(url, articleUrl)
  // 只代理远程 http(s)：`new URL` 会把 data: / blob: 也解析成「绝对地址」，
  // 直接放过会把内联图片也塞进代理（白白多一跳，还可能撑爆 URL 长度）。
  if (!absolute || !isAbsoluteRemoteMedia(absolute)) return url
  const origin = originOf(articleUrl)
  if (!origin) return absolute
  return `${MEDIA_PROXY_PATH}?u=${encodeURIComponent(absolute)}&r=${encodeURIComponent(origin)}`
}

/** 是否已经是本代理的地址（相对或绝对形式都算）。 */
function isProxiedMediaUrl(url: string): boolean {
  const raw = url.trim()
  if (raw.startsWith(MEDIA_PROXY_PATH)) return true
  try {
    return new URL(raw).pathname === MEDIA_PROXY_PATH
  } catch {
    return false
  }
}

/**
 * 改写 srcset 的值：它是「url 描述符, url 描述符」的候选列表，逐个候选改写后拼回。
 *
 * 不处理的话浏览器会从 srcset 里挑一个**没代理的**候选去加载，防盗链照样 403 ——
 * 单改 src 是白改。
 */
export function rewriteSrcsetValue(value: string, articleUrl: string): string {
  if (!articleUrl) return value
  return value
    .split(',')
    .map((candidate) => {
      const parts = candidate.trim().split(/\s+/)
      const url = parts[0]
      if (!url) return ''
      return [mediaProxyUrl(url, articleUrl), ...parts.slice(1)].join(' ')
    })
    .filter(Boolean)
    .join(', ')
}

/** 取 URL 的源站 origin（`https://site.com`）；无法解析或非 http(s) 时返回空串。 */
export function originOf(url: string): string {
  try {
    const parsed = new URL(url)
    // data: / file: 这类非特殊协议的 origin 是字符串 "null"，必须挡掉。
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return ''
    return parsed.origin
  } catch {
    return ''
  }
}
