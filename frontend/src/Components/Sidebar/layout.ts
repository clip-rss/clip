// 左侧栏行缩进与拖拽常量（组件内部共享）。

const ROW_BASE_PADDING = 8
const INDENT_STEP = 16

/** 树深度对应的行左内边距（px）。根级 depth=0。 */
export function rowPaddingLeft(depth: number): number {
  return ROW_BASE_PADDING + Math.max(depth, 0) * INDENT_STEP
}

/** 拖拽订阅源时 dataTransfer 使用的自定义类型，承载 feedId。 */
export const FEED_DRAG_TYPE = 'application/x-clip-feed-id'
