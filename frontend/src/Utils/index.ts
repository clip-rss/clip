// 工具函数统一导出

// 后端 API 服务与事件订阅
export {
  FeedService,
  ItemService,
  CategoryService,
  SettingsService,
  OPMLService,
  WebDAVConfigService,
  OPMLBackupService,
  SystemService,
  DockService,
  onItemsUpdated,
  onFeedError,
  onNotificationOpen,
  toApiError,
  openURL,
} from './Api'

// OPML 订阅导入/导出
export { importOpmlFromFile, exportOpmlToFile } from './Opml'

// 树构建与时间格式化
export { buildFeedTree, flattenCategories } from './FeedTree'
export type { CategoryOption } from './FeedTree'
export { formatRelativeTime, latestUpdated } from './Time'

// 文章筛选排序
export {
  categoryFeedIds,
  filterAndSortItems,
  neighborItemId,
} from './ArticleFilter'
export type { FilterSortOptions } from './ArticleFilter'

// 文章标签解析
export { parseCategories } from './Categories'

// 搜索关键词高亮
export { highlightText } from './Highlight'

// 快捷键提示文案
export { modKey, shortcutHint } from './Shortcut'

// 阅读视图：HTML 清洗与排版样式
export { sanitizeHtml } from './Sanitize'
export { readerContentStyle, readerBackgroundClass } from './ReaderStyle'
export type { ReaderContentStyle } from './ReaderStyle'
