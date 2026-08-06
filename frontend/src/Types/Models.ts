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
  FeedPreview,
  ConnectionTestResult,
} from '../../bindings/github.com/clip-rss/clip/api'

// 配置同步。Status / Result 在本项目里过于笼统，改名再导出。
export type {
  WebDAVView,
  WebDAVInput,
  ConflictInfo,
  Status as SyncStatus,
  Result as SyncResult,
} from '../../bindings/github.com/clip-rss/clip/internal/syncer'

export type {
  Config as CloudBackupConfig,
  Status as CloudBackupStatus,
  BackupInfo as CloudBackupInfo,
  RestoreResult as CloudRestoreResult,
} from '../../bindings/github.com/clip-rss/clip/internal/cloudbackup'

/** 一次同步的结果动作。与后端 syncer.Action 的四个取值一一对应。 */
export type SyncAction = 'noop' | 'pushed' | 'pulled' | 'conflict'
