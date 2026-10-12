// 已下载字体的资产供给与 @font-face 生成（纯函数，便于测试）。
//
// 字体文件由后端中间件（internal/fontproxy）从本地目录读盘回包，
// URL 形态 /__clip/font/<id>/<file>。这里只负责生成前端要用的
// 家族名 / 下载地址 / @font-face 规则，不触碰网络。

import type { FontCatalog, InstalledFont } from '../Types'

/**
 * 字体资产路由前缀。⚠️ 与后端 internal/fontproxy.PathPrefix 是一份契约，
 * 两侧必须同时改。
 */
export const FONT_ASSET_PREFIX = '/__clip/font'

/** 断言字面量非法字符，防清单内容破坏 CSS（id/文件名都来自远端清单）。 */
const SAFE_SEGMENT = /^[A-Za-z0-9][A-Za-z0-9._-]*$/

/** 字体 id / 文件名是否只含安全字符（进 CSS / URL 前的一道白名单）。 */
export function isSafeFontSegment(name: string): boolean {
  return SAFE_SEGMENT.test(name)
}

/** 合成家族名：clip-font-<id>。与后端 @font-face 注入共用命名，不得与本地同名字体撞。 */
export function fontFaceFamily(id: string): string {
  return `clip-font-${id}`
}

/** 字体文件在 WebView 里的下载地址（文件落盘名是清单 paths 的裸文件名）。 */
export function fontAssetURL(id: string, file: string): string {
  return `${FONT_ASSET_PREFIX}/${id}/${file}`
}

/** 扩展名 → CSS format() 关键字。 */
export function fontFormat(name: string): string {
  const ext = name.toLowerCase().split('.').pop() ?? ''
  switch (ext) {
    case 'woff2':
      return 'woff2'
    case 'woff':
      return 'woff'
    case 'otf':
      return 'opentype'
    case 'ttf':
      return 'truetype'
    default:
      return ''
  }
}

/** 每生成一条 @font-face 都该校验 id / 文件名，不符直接丢弃该条（防 CSS 注入）。 */
function safe(id: string, file: string): boolean {
  return isSafeFontSegment(id) && isSafeFontSegment(file)
}

/**
 * 清单 files[].file 带仓库内目录前缀（lxgw-wenkai/xxx.ttf），而**落盘文件名是裸名
 * **（后端 assetName 归一化）。URL 的 file 段必须用 basename，与磁盘一致。
 */
function basename(p: string): string {
  const i = p.lastIndexOf('/')
  return i >= 0 ? p.slice(i + 1) : p
}

/**
 * 生成已下载字体的 @font-face 规则。
 *
 * 数据源是**目录清单**（weight / format / 文件名都在清单里），已装集合只做
 * 「装了哪款注入哪款」的过滤：清单缺（离线）或未下载的字体族不产生规则。
 */
export function buildFontFaceCss(
  catalog: FontCatalog | null,
  installed: InstalledFont[],
): string {
  if (!catalog) return ''
  const installedIds = new Set(installed.map((f) => f.id))

  const faces: string[] = []
  for (const font of catalog.fonts) {
    if (!installedIds.has(font.id)) continue
    for (const file of font.files) {
      const name = basename(file.file)
      if (!safe(font.id, name)) continue
      const format = fontFormat(name)
      if (!format) continue
      faces.push(
        `@font-face{font-family:"${fontFaceFamily(font.id)}";` +
          `src:url("${fontAssetURL(font.id, name)}") format("${format}");` +
          `font-weight:${file.weight};font-display:swap}`,
      )
    }
  }
  return faces.join('\n')
}
