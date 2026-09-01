// 未读 badge 相对「每源保留上限」的负载计算。
// 纯函数，供侧栏 UnreadBadge 的黄→红渐变配色使用，便于单元测试。

/** 未读达到保留上限的该比例后，badge 开始进入警告配色（黄色起点）。 */
export const BADGE_WARN_THRESHOLD = 0.8

/**
 * 未读数相对保留上限的负载，返回 0~1（超过按 1 封顶）。
 * maxItems 非正数（含 0 = 不限制）返回 null，表示不参与警告配色。
 */
export function badgeLoad(unread: number, maxItems: number): number | null {
  if (maxItems <= 0) return null
  return Math.min(1, Math.max(0, unread / maxItems))
}

/**
 * 负载映射为黄→红渐变进度：阈值处为 0（纯黄），100% 处为 1（纯红）。
 * 未达阈值或无上限（load 为 null）返回 null，badge 保持常态 accent 配色。
 */
export function badgeWarnProgress(load: number | null): number | null {
  if (load === null || load < BADGE_WARN_THRESHOLD) return null
  return Math.min(1, (load - BADGE_WARN_THRESHOLD) / (1 - BADGE_WARN_THRESHOLD))
}
