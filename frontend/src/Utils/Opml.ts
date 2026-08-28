// OPML 订阅导入/导出工具：供 Sidebar 与设置面板共用，避免重复实现。

import { OPMLService } from './Api'
import type { ImportResult } from '../Types'

/** 读取 OPML 文件文本并导入，返回导入统计。调用方负责刷新侧栏。 */
export async function importOpmlFromFile(file: File): Promise<ImportResult> {
  const content = await file.text()
  return OPMLService.ImportOPML(content)
}

/**
 * 从远程地址导入 OPML，返回导入统计。调用方负责刷新侧栏。
 *
 * 拉取放在后端：WebView 是本地 origin，前端直接 fetch 远程地址会被 CORS 拦掉，
 * 且后端能复用抓取用的超时、重试与代理配置。
 */
export async function importOpmlFromURL(url: string): Promise<ImportResult> {
  return OPMLService.ImportOPMLFromURL(url)
}

/** 导出全部订阅为 OPML：弹出系统保存对话框写盘，成功返回 true，取消返回 false。 */
export async function exportOpmlToFile(): Promise<boolean> {
  return OPMLService.ExportOPML()
}
