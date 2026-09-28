import { describe, it, expect, vi, beforeEach } from 'vitest'

const { openURLMock } = vi.hoisted(() => ({ openURLMock: vi.fn() }))

// 只换掉 Browser，其余（newRuntimeCaller 等）用真的，免得把生成绑定的 import 链打断。
vi.mock('@wailsio/runtime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@wailsio/runtime')>()),
  Browser: { OpenURL: openURLMock },
}))

import { openURL } from './index'

const REFUSED = 'Invalid browser call: invalid URL: scheme not allowed'

/**
 * 监听 node 的未处理拒绝。
 *
 * jsdom **不会**为 promise 触发 window 的 `unhandledrejection`（实测：同一个裸 rejection
 * 只有 node 侧计数为 1），所以只能挂 node 的。项目没有 @types/node，这里手写最小签名。
 */
function watchUnhandledRejections(): {
  seen: unknown[]
  stop: () => void
} {
  const seen: unknown[] = []
  const node = (
    globalThis as unknown as {
      process: {
        on: (event: string, cb: (reason: unknown) => void) => void
        off: (event: string, cb: (reason: unknown) => void) => void
      }
    }
  ).process
  const onUnhandled = (reason: unknown): void => {
    seen.push(reason)
  }
  node.on('unhandledRejection', onUnhandled)
  return {
    seen,
    stop: () => node.off('unhandledRejection', onUnhandled),
  }
}

describe('openURL', () => {
  beforeEach(() => {
    openURLMock.mockReset()
  })

  it('把地址原样交给 Wails 运行时', () => {
    openURLMock.mockResolvedValue(undefined)
    openURL('https://x.com/a')

    expect(openURLMock).toHaveBeenCalledWith('https://x.com/a')
  })

  it('空地址不调用运行时', () => {
    openURL('')

    expect(openURLMock).not.toHaveBeenCalled()
  })

  it('运行时同步抛错时兜底 window.open', () => {
    const opened = vi.spyOn(window, 'open').mockReturnValue(null)
    openURLMock.mockImplementation(() => {
      throw new Error('no runtime')
    })

    openURL('https://x.com/a')

    expect(opened).toHaveBeenCalled()
    opened.mockRestore()
  })

  it('给运行时返回的 promise 挂了 catch（返回值可能不是 Promise，所以按鸭子类型断言）', () => {
    let catchAttached = false
    openURLMock.mockReturnValue({
      catch: () => {
        catchAttached = true
        return Promise.resolve()
      },
    })

    openURL('https://x.com/a')

    expect(catchAttached).toBe(true)
  })

  // issue #7 的崩溃路径：后端拒绝地址时返回的是 **Promise reject**，早先的 try/catch 只接
  // 同步抛错，接不住它 —— 于是冒到未处理拒绝，被 CrashBoundary 接手成整页崩溃。
  it('后端拒绝地址时不产生未处理拒绝', async () => {
    const watch = watchUnhandledRejections()
    try {
      openURLMock.mockReturnValue(Promise.reject(new Error(REFUSED)))

      openURL('#toc-1')
      // 未处理拒绝是在微任务队列排空后才上报的，得让出几拍
      await new Promise((resolve) => setTimeout(resolve, 0))
    } finally {
      watch.stop()
    }

    expect(openURLMock).toHaveBeenCalledWith('#toc-1')
    expect(watch.seen).toEqual([])
  })
})
