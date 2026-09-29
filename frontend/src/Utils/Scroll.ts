// 阅读区滚动工具：锚点跳转前快照阅读进度、按比例恢复，共用 reduced-motion 判定。
//
// 存「进度比例（0..1）」而非像素偏移：同一会话同一容器内两者等价，但跨容器
// （阅读视图↔专注模式行宽不同）、全文/摘要切换、图片异步加载改高度时，
// 比例能落在正确的大致位置并随 clamp 保住边界，像素死偏移会漂到别的段落。

/** 系统「减弱动态」偏好或设置里的「关闭动画」生效时，滚动不做平滑动画。
 *  CSS 的 `scroll-behavior: auto` 管不到 scrollTo/scrollIntoView 显式传入的 behavior，只能在这判。 */
export function prefersReducedMotion(): boolean {
  if (document.documentElement.classList.contains('reduce-motion')) return true
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/** 把任意阅读进度归一化到 [0,1]；非有限值（NaN/Infinity）用 fallback 兜底。 */
export function clampRatio(ratio: number, fallback = 0): number {
  if (!Number.isFinite(ratio)) return fallback
  return Math.min(Math.max(ratio, 0), 1)
}

/** 滚动容器当前的阅读进度（0..1）；容器不可滚动时返回 null（无意义，不该快照）。 */
export function scrollFraction(el: HTMLElement): number | null {
  const max = el.scrollHeight - el.clientHeight
  if (max <= 0) return null
  return clampRatio(el.scrollTop / max)
}

/** 按快照的进度恢复滚动位置；clamp 兜底内容高度变化。 */
export function restoreScrollFraction(el: HTMLElement, ratio: number): void {
  const max = el.scrollHeight - el.clientHeight
  if (max <= 0) return
  el.scrollTo({
    top: clampRatio(ratio) * max,
    behavior: prefersReducedMotion() ? 'auto' : 'smooth',
  })
}
