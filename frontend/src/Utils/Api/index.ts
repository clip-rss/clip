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
  WebDAVConfigService,
  OPMLBackupService,
  SystemService,
  FontService,
} from '../../../bindings/github.com/clip-rss/clip/api'

export { DockService } from '../../../bindings/github.com/wailsapp/wails/v3/pkg/services/dock'

export {
  onItemsUpdated,
  onFeedError,
  onFeedRefreshing,
  onNotificationOpen,
  onOPMLImportProgress,
  onDatabaseRestoreProgress,
} from './Events'

import { Browser } from '@wailsio/runtime'

/**
 * 将后端调用 reject 的错误归一化为可读字符串。
 *
 * 绑定调用失败时 Wails 运行时把整个响应体原样塞进 `Error.message`，也就是后端
 * `CallError` 的 JSON：`{"message":…,"cause":…,"kind":"RuntimeError"}`。
 * 其中 `message` 是 Go 那侧的错误文案，`cause`（默认序列化多为 `{}`）和 `kind`
 * 对用户毫无意义，因此这里剥出 `message`；不是这种 JSON 时原样返回。
 */
export function toApiError(err: unknown): string {
  const raw =
    err instanceof Error
      ? err.message
      : typeof err === 'string'
        ? err
        : String(err)
  try {
    const message: unknown = JSON.parse(raw)?.message
    if (typeof message === 'string' && message) return message
  } catch {
    /* 不是 JSON，按原样返回 */
  }
  return raw
}

/** 在系统默认浏览器中打开外部链接（Wails 运行时，兜底 window.open）。 */
export function openURL(url: string): void {
  if (!url) return
  try {
    // 后端会拒绝它打不开的地址（空 scheme、javascript:、含 shell 元字符等），而拒绝是
    // **Promise reject** —— 下面的 try/catch 接不住，不挂 catch 就会冒成未处理拒绝，
    // 被 CrashBoundary 接手成整页崩溃（issue #7）。
    // 这里只负责把它咽下去，不做二次跳转：后端明确拒绝的地址，换 window.open 也一样打不开。
    void Browser.OpenURL(url).catch(() => {})
  } catch {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}
