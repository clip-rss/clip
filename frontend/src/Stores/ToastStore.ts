import { create } from 'zustand'

export type ToastType = 'success' | 'error' | 'info'

export interface ToastItem {
  id: number
  message: string
  type: ToastType
  /** 自动消失时长（ms），0 表示不自动消失。 */
  duration: number
  /** 当前剩余时长（ms）。初始 = duration，暂停时重新计算。 */
  remaining: number
  /** 创建（或上次恢复）时间戳，用于计算已流逝的时间。 */
  createdAt: number
  /** 正在执行退出动画。 */
  exiting: boolean
}

const DEFAULT_DURATION = 4000

let nextId = 1

/** 模块级 timer map，不进入 zustand 状态，以便 pause/resume 精细控制。 */
const dismissTimers = new Map<number, ReturnType<typeof setTimeout>>()

interface ToastState {
  queue: ToastItem[]
  /** 鼠标正悬浮在 toast 容器上方，暂停自动消失倒计时。 */
  hovering: boolean

  addToast: (message: string, type?: ToastType, duration?: number) => void
  /** 触发退出动画。 */
  dismiss: (id: number) => void
  /** 从队列中移除（动画结束后调用）。 */
  remove: (id: number) => void

  /** 暂停全部自动消失计时器（鼠标进入容器）。 */
  pauseAll: () => void
  /** 恢复全部自动消失计时器（鼠标离开容器）。 */
  resumeAll: () => void
}

export const useToastStore = create<ToastState>((set, get) => ({
  queue: [],
  hovering: false,

  addToast: (message, type = 'info', duration) => {
    const id = nextId++
    const actualDuration = duration ?? DEFAULT_DURATION
    const now = Date.now()

    set((s) => ({
      queue: [
        ...s.queue,
        {
          id,
          message,
          type,
          duration: actualDuration,
          remaining: actualDuration,
          createdAt: now,
          exiting: false,
        },
      ],
    }))

    if (actualDuration > 0) {
      const timerId = setTimeout(() => {
        get().dismiss(id)
      }, actualDuration)
      dismissTimers.set(id, timerId)
    }
  },

  dismiss: (id) => {
    // 清理 timer
    const timer = dismissTimers.get(id)
    if (timer) {
      clearTimeout(timer)
      dismissTimers.delete(id)
    }

    set((s) => ({
      queue: s.queue.map((t) => (t.id === id ? { ...t, exiting: true } : t)),
    }))
    // 等退出动画播完再移除
    setTimeout(() => {
      get().remove(id)
    }, 250)
  },

  remove: (id) => {
    dismissTimers.delete(id)
    set((s) => ({ queue: s.queue.filter((t) => t.id !== id) }))
  },

  pauseAll: () => {
    const now = Date.now()

    // 清除全部 timer
    dismissTimers.forEach((timer) => clearTimeout(timer))
    dismissTimers.clear()

    set((s) => ({
      hovering: true,
      queue: s.queue.map((t) => {
        if (t.exiting || t.duration <= 0) return t
        const elapsed = now - t.createdAt
        return { ...t, remaining: Math.max(0, t.duration - elapsed) }
      }),
    }))
  },

  resumeAll: () => {
    set((s) => {
      // 为每个还有剩余的 toast 重新设定 timer
      for (const t of s.queue) {
        if (t.exiting || t.remaining <= 0) continue
        const timerId = setTimeout(() => {
          get().dismiss(t.id)
        }, t.remaining)
        dismissTimers.set(t.id, timerId)
      }

      return {
        hovering: false,
        queue: s.queue.map((t) => ({
          ...t,
          // 重置 createdAt 为「现在」，后续 pause 从此刻算流逝
          createdAt: t.exiting || t.duration <= 0 ? t.createdAt : Date.now(),
        })),
      }
    })
  },
}))
