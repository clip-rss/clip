// 已下载字体的 @font-face 注入。
//
// 「装了哪款注入哪款」：只对已装字体族（∩ 目录清单）生成 @font-face，绝不全局挂。
// 规则由 Utils/fonts.ts 的 buildFontFaceCss 生成（纯函数），这里只负责接状态。

import { useFontStore } from '../Stores'
import { buildFontFaceCss } from '../Utils/fonts'

/** 返回当前应注入的 @font-face CSS 文本（无已装字体或缺清单时为空串）。 */
export function useFontFaceCss(): string {
  const catalog = useFontStore((s) => s.catalog)
  const installed = useFontStore((s) => s.installed)
  return buildFontFaceCss(catalog, installed)
}
