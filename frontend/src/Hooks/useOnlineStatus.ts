import { useEffect, useState } from 'react'

/**
 * 监听浏览器在线/离线状态（基于 navigator.onLine + online/offline 事件）。
 *
 * 返回 true 表示在线，false 表示离线。
 *
 * 注意：navigator.onLine 只能检测浏览器与本地网络的连接状态，
 * 无法判断是否真正能访问互联网（如局域网断网但本地 WiFi 连接）。
 */
export function useOnlineStatus(): boolean {
  const [online, setOnline] = useState(() => navigator.onLine)

  useEffect(() => {
    function handleOnline(): void {
      setOnline(true)
    }

    function handleOffline(): void {
      setOnline(false)
    }

    window.addEventListener('online', handleOnline)
    window.addEventListener('offline', handleOffline)

    return () => {
      window.removeEventListener('online', handleOnline)
      window.removeEventListener('offline', handleOffline)
    }
  }, [])

  return online
}
