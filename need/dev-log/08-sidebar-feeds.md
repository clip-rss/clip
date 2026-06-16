# 阶段 8：左侧栏 - 源与文件夹树

## 概述
实现左侧栏的 Feed 源列表、文件夹树形结构、交互操作。

## 步骤清单

- [x] 左侧栏容器组件（背景 `--bg-primary`，内边距）
- [x] 头部："源" 标题 + "＋" 图标按钮（下拉：新建文件夹 / 添加订阅）
- [x] 文件夹树组件
  - [x] 文件夹项（32px 高，展开箭头，文件夹图标，名称，未读计数胶囊）
  - [x] 展开/折叠动画（箭头旋转 90°）
  - [x] RSS 源项（32px 高，favicon 16x16，名称，未读计数胶囊）
- [x] 未读计数胶囊样式（`--accent` 10% 背景，数字居中，0 时隐藏）
- [x] 选中状态（`--accent` 10% 背景，左侧 3px 指示条）
- [x] Hover 状态（`--bg-secondary` 背景）
- [x] 右键菜单（编辑、暂停更新、删除）
- [x] 拖拽排序（源列表项可拖拽**归类**：拖入文件夹 / 拖出到未分类）
- [x] 底部状态栏（"上次更新：X 分钟前"）
- [x] 导入/导出 OPML 图标按钮
- [x] 数据绑定：从 Store 加载 Feed 列表与分类
- [x] 点击源项 → 触发中间栏文章列表更新

## 验收标准
- [x] 树形结构正确展示，支持多级嵌套
- [x] 展开折叠流畅，状态保持（`expanded` 集合持久化）
- [x] 未读计数实时更新（订阅 `items:updated` / `feed:error` 事件刷新）
- [x] 右键菜单功能正常（feed：编辑/暂停⇄恢复/删除；folder：重命名/删除）
- [x] 拖拽可用（拖拽归类）

## 实现说明与边界
- **新增文件**
  - 类型：`Types/Sidebar.ts`（`Selection` / `FeedTreeNode` / `FeedTree`）
  - 工具：`Utils/FeedTree.ts`（`buildFeedTree`）、`Utils/Time.ts`（`formatRelativeTime` / `latestUpdated`）
  - 状态：`Stores/SidebarStore.ts`（数据加载、选中、展开、增删改、拖拽归类）
  - 组件：`Components/Sidebar/`（`Sidebar` / `FolderItem` / `FeedItem` / `UnreadBadge` /
    `ConfirmDialog` / `RenameInput` / `Icons` / `layout`）
- **测试**：搭建 Vitest（`vitest.config.ts` + `pnpm test`）。
  覆盖 `FeedTree`、`Time`、`SidebarStore` 共 21 个用例，全部通过。
- **拖拽范围**：后端 `Feed` 未暴露 `sortOrder`，故仅实现「拖拽归类」（`MoveToCategory`，持久化）。
  源在同一分类内的自由重排需后端新增 `Feed.sortOrder` 字段后再做。
- **添加订阅**：头部「＋」的「添加订阅」项预留 `onAddFeed` 回调（未接入时禁用）；
  两阶段检测弹窗归 **阶段 12（订阅管理）**。
- **OPML**：导入用隐藏 `<input type="file">` 读文本 → `ImportOPML`；导出用 Blob 下载 `ExportOPML` 结果。
- **中间栏联动**：`App.tsx` 的 list 占位临时读取 `SidebarStore.selection` 显示选中项；
  真实文章列表见 **阶段 09**。

## 验证记录
- `pnpm test` → 3 文件 21 用例通过
- `pnpm build` → 构建成功（160 模块）
- `tsc --noEmit -p tsconfig.json` → 0 错误
