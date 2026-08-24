import { useToastStore, type ToastType } from '../Stores'

/**
 * 在任意位置（组件、store、事件回调）显示一条 toast 通知。
 * 自动 4000ms 消失；传 0 可禁止自动消失。
 *
 * @example
 *   showToast('代理连接失败：超时', 'error')
 *   showToast('设置已保存', 'success')
 *   showToast('正在同步…', 'info', 0) // 不自动消失
 */
export function showToast(
  message: string,
  type?: ToastType,
  duration?: number,
): void {
  useToastStore.getState().addToast(message, type, duration)
}
