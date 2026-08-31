// 后端数据模型类型：由 `wails3 generate bindings` 生成，这里统一再导出，
// 业务代码从 `Types` 引入而非直接依赖深层 bindings 路径。

export type {
  Feed,
  FeedWithUnread,
  Item,
  ItemLight,
  Category,
  CategoryWithFeeds,
  Settings,
} from '../../bindings/github.com/clip-rss/clip/internal/store'

export type {
  RefreshOutcome,
  ImportResult,
  NewFeed,
  NewCategory,
  FeedPreview,
  ConnectionTestResult,
  WebDAVView,
  WebDAVInput,
} from '../../bindings/github.com/clip-rss/clip/api'

export type {
  Config as OPMLBackupConfig,
  Status as OPMLBackupStatus,
  BackupInfo as OPMLBackupInfo,
  ImportResult as OPMLImportResult,
} from '../../bindings/github.com/clip-rss/clip/internal/opmlbackup'
