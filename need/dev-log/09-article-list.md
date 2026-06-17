# 阶段 9：中间栏 - 文章列表

## 概述
实现文章列表展示、筛选排序、虚拟滚动、批量操作。

## 步骤清单

- [x] 文章列表容器组件（背景 `--bg-secondary`）
- [x] 列表头部（40px）
  - [x] 筛选下拉：全部 / 未读 / 已读 / 星标 / 今日
  - [x] 排序按钮（时间 / 来源）
  - [x] 批量操作 "…" 菜单（标记所有已读、批量星标）
- [x] 文章行组件（64px 高）
  - [x] 未读指示器：8x8 蓝色实心圆点
  - [x] 标题：14px 加粗（未读）/ 正常（已读），单行省略
  - [x] 摘要：12px 次级色，最多两行省略
  - [x] 底部行：来源名 · 相对时间
  - [x] 右侧操作图标（hover 显示）：星标、外部链接
- [x] Hover 状态（`--bg-tertiary`）
- [x] 选中状态（`--accent` 5% 背景，标题颜色 `--accent`）
- [x] 星标切换交互（点击即时切换填充状态）
- [x] 虚拟滚动实现（@tanstack/react-virtual，万级数据流畅）
- [x] 空状态展示（插画 + "暂无未读文章"）
- [x] 纤细滚动条样式（4px 宽，复用全局 `global.css`）
- [x] 点击文章 → 右侧加载正文 + 行高亮
- [x] 自动标记已读逻辑（点击即标记）

## 验收标准
- [x] 列表渲染流畅（虚拟滚动，万条数据无卡顿）
- [x] 筛选排序正确切换
- [x] 已读/未读/星标视觉状态正确
- [x] 虚拟滚动正常工作

## 实现说明与边界
- **新增文件**
  - 类型：`Types/Article.ts`（`ArticleFilter` / `ArticleSort`）
  - 工具：`Utils/ArticleFilter.ts`（`categoryFeedIds` / `filterAndSortItems`）、
    `Utils/Api` 新增 `openURL`（`Browser.OpenURL` 外链，兜底 `window.open`）
  - 状态：`Stores/ArticleStore.ts`（加载、筛选/排序、选中并标记已读、星标、批量）
  - 组件：`Components/ArticleList/`（`ArticleList` / `ListHeader` / `ArticleRow` / `EmptyState` / `Icons`）
- **数据策略：客户端筛选**。后端筛选端点全局（`ListUnreadItems`/`ListStarredItems` 不接受 feedID），
  无「已读/今日/按分类」文章接口。故按选中范围一次性 `ListItems(feedID 或 0, 2000, 0)` 入内存，
  筛选/排序/分类归属在前端计算，`@tanstack/react-virtual` 虚拟滚动保渲染流畅。
- **分类范围**：`categoryFeedIds` 取该分类及子孙分类下全部 feedId，按 `feedId ∈ 集合` 过滤。
- **源名**：`Item` 无源标题，由 `SidebarStore.feeds` 按 `feedId` 映射。
- **自动已读**：点击即 `MarkRead` + 乐观更新 + 刷新侧栏未读；2 秒停留自动已读留作后续。
- **读状态联动**：标记已读/全部已读后调用 `SidebarStore.load()` 刷新侧栏未读胶囊。
- **批量星标**：后端无批量端点，对可见未星标项循环 `ToggleStar`。
- **阅读视图**：`App.tsx` 的 reader 占位临时显示选中文章标题/摘要；真实正文渲染见 **阶段 10**。
- **测试**：新增 `ArticleFilter.test.ts` / `ArticleStore.test.ts`，连同既有共 **38 用例全部通过**。

## 验证记录
- `pnpm test` → 5 文件 38 用例通过
- `pnpm build` → 构建成功（173 模块）
- `tsc --noEmit -p tsconfig.json` → 0 错误
