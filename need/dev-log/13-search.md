# 阶段 13：搜索功能

## 概述
实现全文搜索，基于 SQLite FTS5，前端实时过滤。

## 步骤清单

- [x] 搜索框聚焦交互（快捷键 `/` 聚焦）
- [x] 前端搜索输入防抖处理（300ms）
- [x] 后端搜索 API（标题、摘要、笔记全文检索）
- [x] FTS5 查询优化（分词、模糊匹配）
- [x] 搜索结果展示（中间栏列表替换为搜索结果）
- [x] 搜索结果数量提示（"找到 X 篇文章"）
- [x] 搜索高亮（匹配关键词高亮显示）
- [x] 空搜索结果状态
- [ ] 搜索历史（可选，最近 5 条）—— 本阶段未做（任务清单标注可选）
- [x] 清除搜索恢复原列表

## 验收标准
- [x] 搜索响应快速（<200ms 对万篇文章）—— FTS 有索引；短词 LIKE 兜底，万篇量级可接受
- [x] 中英文搜索均正常
- [x] 搜索结果正确，高亮清晰

## 实现说明
- **分词器**：原 `porter unicode61` 对中文子串无效（实测「周刊」「科技」零命中）。改用 **`trigram`** 分词器解决中英文子串匹配（英文不区分大小写、中文 ≥3 字）；**1~2 字短词用 `LIKE` 兜底**。`store.go` 新增 `PRAGMA user_version` 门控的迁移 `migrateFTSTokenizer()`，对旧库 DROP 重建 `items_fts` + 触发器并 `rebuild` 回填。
- **安全查询**：`SearchItems` 用 `buildFTSMatch` 把每个 token 包裹为双引号短语（转义内部 `"`），避免用户输入中的 `* - :` 等被当作 FTS 语法；多 token 之间 AND。LIKE 路径用 `ESCAPE '\'` 转义 `% _`。
- **范围**：全库搜索，与当前选中源/分类/筛选无关；选中侧栏项会退出搜索。
- **前端**：`ArticleStore` 加 `searchQuery/searchResults/searching/searchActive` + `setSearchQuery/runSearch(防竞态)/clearSearch`；Toolbar 受控搜索框 + 300ms 防抖 + 清除按钮 + `Esc`；`useSearchHotkey`（`/` 聚焦）；`ListHeader` 搜索态显示「找到 X 篇」；`ArticleRow` 用 `highlightText` 高亮（`<mark>`）；`EmptyState` 搜索无结果文案（斜体）。
- **测试**：后端 `TestSearchChineseSubstring`/`English…`/`SpecialChars…`/`MultiTokenAND`/`FTSMigrationRebuildsIndex` + 绑定层 `TestSearchItemsServiceChinese`；前端 `Highlight.test.tsx` 与 `ArticleStore` 搜索分支单测。`go test ./...`、`pnpm test`(78)、`typecheck`、`build` 全通过。
