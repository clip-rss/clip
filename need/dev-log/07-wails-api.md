# 阶段 7：Wails API 绑定层

## 概述
将 Go 后端功能通过 Wails Bind 暴露给前端调用。

## 步骤清单

- [x] 创建 `api` 包结构
- [x] Feed 管理 API（AddFeed, UpdateFeed, DeleteFeed, ListFeeds, GetFeed）
- [x] 文章查询 API（ListItems, GetItem, SearchItems）
- [x] 文章操作 API（MarkRead, MarkUnread, ToggleStar, BatchMarkRead, AddNote）
- [x] Category 管理 API（AddCategory, UpdateCategory, DeleteCategory, ListCategories, MoveToCategory）
- [x] 刷新 API（RefreshFeed, RefreshAll, ForceRefresh）
- [x] 设置 API（GetSettings, UpdateSettings）
- [x] OPML 导入导出 API（ImportOPML, ExportOPML）
- [x] 统一错误处理与返回格式
- [x] 前端 TypeScript 类型定义（与 Go 结构体对应）
- [x] 前端 API 工具函数封装（统一导出在 `utils/index.ts`）
- [x] Wails Events 事件注册（`items:updated`, `feed:error` 等）

## 验收标准
- 前端可通过类型安全的方式调用所有后端方法
- 错误处理统一，前端可正确捕获
- Events 事件正常推送与接收
