import { useEffect, useState } from 'react'
import { SystemService } from '../../bindings/changeme/api'

export type Platform = 'mac' | 'windows'

/**
 * 通过 Go 后端（runtime.GOOS）获取当前运行平台。
 *
 * 返回 `null` 表示尚未解析完成（首帧），解析后为 `'mac'` 或 `'windows'`。
 */
export function usePlatform(): Platform | null {
  const [platform, setPlatform] = useState<Platform | null>(null)

  useEffect(() => {
    let active = true
    SystemService.Platform()
      .then((value) => {
        if (active) {
          setPlatform(value as Platform)
        }
      })
      .catch(() => {
        // 调用被取消或失败时忽略，保持 null
      })
    return () => {
      active = false
    }
  }, [])

  return platform
}
