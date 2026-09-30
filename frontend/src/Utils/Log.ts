// 前端运行时日志桥：把关键操作经 SystemService.Log 落进与后端同一个日志文件，
// 供出问题时拿到原始材料（见后端 internal/logging）。
//
// 日志是尽力而为的诊断手段，绝不应因桥接失败反过来打断用户操作，因此对同步抛错与
// Promise reject 都做吞没处理（浏览器预览、应用退出过程中绑定可能不可用）。
import { SystemService } from './Api'

export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

function emit(level: LogLevel, scope: string, message: string): void {
  try {
    const p = SystemService.Log(level, scope, message) as
      | Promise<void>
      | undefined
    if (p && typeof p.catch === 'function') p.catch(() => {})
  } catch {
    /* 绑定不可用时静默：日志尽力而为 */
  }
}

export function logDebug(scope: string, message: string): void {
  emit('debug', scope, message)
}

export function logInfo(scope: string, message: string): void {
  emit('info', scope, message)
}

export function logWarn(scope: string, message: string): void {
  emit('warn', scope, message)
}

export function logError(scope: string, message: string): void {
  emit('error', scope, message)
}
