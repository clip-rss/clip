// 自定义 Hooks 统一导出
export { usePlatform } from './usePlatform'
export type { Platform } from './usePlatform'
export { useVisibleArticles, useArticleNavigation } from './useArticles'
export type { ArticleNavigation } from './useArticles'
export {
  useHotkeys,
  comboFromEvent,
  isEditableTarget,
  isModalOpen,
} from './useHotkeys'
export type { Hotkey } from './useHotkeys'
export { useAppHotkeys } from './useAppHotkeys'
export { useNotificationNavigation } from './useNotificationNavigation'
export { useDockBadge } from './useDockBadge'
