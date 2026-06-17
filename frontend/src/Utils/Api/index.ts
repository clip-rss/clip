// 后端 API 统一封装。
//
// 唯一集中绑定路径的地方：业务代码从 `Utils`（或 `Utils/Api`）引入服务，
// 不直接依赖 `bindings/github.com/...` 深层路径；重新生成绑定后只需改这里。
//
// 各方法返回 Wails 的 CancellablePromise：后端返回非 nil error 时 Promise 被 reject，
// 前端用 try/catch 即可捕获，错误信息可经 toApiError 归一化为字符串。

export {
  FeedService,
  ItemService,
  CategoryService,
  SettingsService,
  OPMLService,
  SystemService,
} from '../../../bindings/github.com/clip-rss/clip/api'

export { onItemsUpdated, onFeedError } from './Events'

import { Browser } from '@wailsio/runtime'

/** 将后端调用 reject 的错误归一化为可读字符串。 */
export function toApiError(err: unknown): string {
  if (err instanceof Error) return err.message
  if (typeof err === 'string') return err
  return String(err)
}

/** 在系统默认浏览器中打开外部链接（Wails 运行时，兜底 window.open）。 */
export function openURL(url: string): void {
  if (!url) return
  try {
    Browser.OpenURL(url)
  } catch {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}
