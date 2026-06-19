# 阶段 12：订阅管理功能

## 概述
实现添加订阅的多种方式、订阅编辑、OPML 导入导出。

## 步骤清单

- [x] 添加订阅模态框组件
  - [x] 输入 RSS/Atom URL
  - [x] URL 检测与验证（加载中状态）
  - [x] 展示 Feed 信息预览（标题、描述、文章数）
  - [x] 选择归属文件夹
  - [x] 确认添加
- [x] 网站 URL 自动发现 RSS（输入普通 URL → 解析 `<link rel="alternate">`）
- [x] OPML 导入功能
  - [x] 文件选择对话框
  - [x] 解析 OPML 结构
  - [x] 批量导入 Feed（含文件夹层级）
  - [x] 导入进度展示（导入完成后弹窗汇总新增/跳过/分类数）
  - [x] 重复源检测与跳过
- [x] OPML 导出功能
  - [x] 导出当前所有 Feed 与文件夹结构
  - [x] 文件保存对话框
  - [ ] 可选附带阅读状态（自定义扩展）—— 经确认本阶段暂不实现，仅导出标准 OPML
- [x] 编辑订阅源
  - [x] 修改标题
  - [x] 修改分类/文件夹
  - [x] 设置单源更新间隔
  - [x] 设置保留文章数量上限
- [x] 暂停/恢复源更新
- [x] 删除订阅源（二次确认）

## 验收标准
- [x] 手动 URL 和网站自动发现均可正确添加订阅
- [x] OPML 导入导出格式正确，兼容主流阅读器
- [x] 编辑、暂停、删除功能正常

## 实现说明
- 后端：新增 `FeedService.PreviewFeed`（检测/自动发现但不入库）与 `Fetcher.Discover`（抓网页+发现 Feed 链接）；`FeedService.AddFeed` 增加 `categoryID` 参数以支持归属文件夹。
- 前端：新增 `Components/AddFeedModal`（两阶段状态机）与 `Components/EditFeedModal`（标题/文件夹/间隔/上限）；`Utils.flattenCategories` 供归属文件夹下拉复用；右键菜单「编辑」拆为「重命名」（内联）+「编辑…」（弹窗）。
- 入口：Toolbar「＋ 订阅」与 Sidebar「添加订阅」均接入 AddFeedModal。
- 测试：`api`/`fetcher` 新增 `TestPreviewFeed`、`TestAddFeedIntoCategory`、`TestDiscover`；前端新增 `flattenCategories` 单测。`go test ./...`、`pnpm test`、`pnpm typecheck`、`pnpm build` 均通过。
