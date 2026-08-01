// OPML 订阅导入/导出工具：供 Sidebar 与设置面板共用，避免重复实现。

import { OPMLService } from './Api'
import type { ImportResult } from '../Types'

/** 读取 OPML 文件文本并导入，返回导入统计。调用方负责刷新侧栏。 */
export async function importOpmlFromFile(file: File): Promise<ImportResult> {
  const content = await file.text()
  return OPMLService.ImportOPML(content)
}

/** 导出全部订阅为 OPML：弹出系统保存对话框写盘，成功返回 true，取消返回 false。 */
export async function exportOpmlToFile(): Promise<boolean> {
  return OPMLService.ExportOPML()
}
