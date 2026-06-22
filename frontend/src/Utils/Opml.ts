// OPML 订阅导入/导出工具：供 Sidebar 与设置面板共用，避免重复实现。

import { OPMLService } from './Api'
import type { ImportResult } from '../Types'

/** 读取 OPML 文件文本并导入，返回导入统计。调用方负责刷新侧栏。 */
export async function importOpmlFromFile(file: File): Promise<ImportResult> {
  const content = await file.text()
  return OPMLService.ImportOPML(content)
}

/** 导出全部订阅为 OPML 并触发浏览器下载（文件名 clip-feeds.opml）。 */
export async function exportOpmlToFile(): Promise<void> {
  const content = await OPMLService.ExportOPML()
  const blob = new Blob([content], { type: 'text/xml;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'clip-feeds.opml'
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
