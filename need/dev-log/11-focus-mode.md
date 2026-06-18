# 阶段 11：专注阅读模式

## 概述
实现全屏单栏专注阅读，包括浮动控制条、过渡动画、键盘导航。

## 步骤清单

- [x] 专注模式状态管理（Store 中添加 focusMode 状态）
- [x] 触发方式：工具栏按钮 / 快捷键 `Ctrl+Shift+F`（Cmd+Shift+F）
- [x] 过渡动画（300ms ease，三栏淡化 → 全屏放大淡入）
- [x] 全屏阅读区（隐藏顶部工具栏、侧栏、列表栏）
- [x] 浮动控制条
  - [x] 鼠标移到顶部 40px 区域时显示
  - [x] 高度 44px，半透明背景 + `backdrop-blur: 12px`
  - [x] 左部：退出按钮（← + "退出专注"）
  - [x] 中部：文章标题（18px 加粗居中）
  - [x] 右部：星标、标记已读、外部链接、笔记开关
  - [x] 3 秒无操作自动隐藏
- [x] 正文排版（最大宽 680px 居中）
- [x] 独立阅读背景切换（纯白/纯黑/米黄 `#F5ECD7`）
- [x] 键盘导航
  - [x] `J/K` 或上下箭头切换上/下一篇文章
  - [x] 切换时 200ms 交叉淡化动画
- [x] 退出方式：Esc / 按钮 / 再次点击切换图标
- [x] 退出后恢复三栏布局，阅读位置保留

## 验收标准
- [x] 进入/退出专注模式过渡流畅
- [x] 浮动控制条显隐正确
- [x] 键盘切换文章正常
- [x] 退出后阅读位置保持

## 实现说明与边界
- **新增文件**
  - 组件：`Components/FocusMode/`（`FocusMode` 覆盖层 / `FocusControlBar` 浮动控制条 /
    `FocusMode.module.scss`）
  - Hooks：`Hooks/useArticles.ts`（`useVisibleArticles` / `useArticleNavigation`）、
    `Hooks/useFocusHotkey.ts`（全局 `Ctrl/Cmd+Shift+F`）
  - 工具：`Utils/ArticleFilter.ts` 新增纯函数 `neighborItemId`（有序列表中按方向取相邻可见项）
  - 复用：抽出 `Components/ReadingView/ReaderArticle.tsx`（标题+元信息+正文+结尾提示），
    供阅读视图与专注模式共用；`ReadingView/index.ts` 追加导出 `ReaderArticle` / `Lightbox`；
    `Icons.tsx` 新增 `BackIcon`
- **状态**：`focusMode` 放入 `LayoutStore`（含 `enterFocus/exitFocus/toggleFocus`），
  `persist.partialize` 仅持久化栏宽，专注态为会话态不跨启动恢复。
- **触发与退出**：工具栏布局按钮 `toggleFocus`（未选中文章时禁用）；全局快捷键进入需已选中文章、
  退出始终可用；覆盖层内 `Esc` 退出、`J/↓` 下一篇、`K/↑` 上一篇（输入框内不劫持，灯箱开启时 Esc 先关灯箱）。
- **过渡生命周期**：`FocusMode` 自管 `mounted/active`——关闭后保留 300ms 播放退出动画再卸载；
  `opacity + scale(0.96→1)` 实现"放大淡入"，覆盖层不透明即遮住三栏（近似"原界面淡化"）。
- **导航定位**：`useArticleNavigation` 以"范围内完整有序列表"定位当前文章，落点限定在"当前筛选可见集"，
  因此读完即移出未读列表不会打断连续阅读（`neighborItemId` 纯函数，含单测）。
- **控制条显隐**：进入即显示并 3 秒后自动隐藏；鼠标进入顶部 40px 热区显示，悬停控制条本身保持显示，
  切换文章标题闪现。
- **正文与背景**：固定最大宽 680px 居中；字体/字号/行高沿用 `ReaderStore`；阅读背景复用
  `readerBackgroundClass`（`light/sepia/dark` → 纯白/护眼 `#F5ECD7`/纯黑），由阅读设置驱动、专注模式如实呈现。
- **阅读位置保留**：三栏始终挂载于覆盖层之下，退出后 DOM 滚动位置与所选文章自然保留。
- **复用一致性**：`ArticleList` 改用 `useVisibleArticles`，与专注导航共享同一筛选/排序，避免顺序分叉。
- **笔记按钮**：占位禁用，归 **阶段 14**。
- **无障碍**：覆盖层 `role="dialog" aria-modal`，按钮 `aria-label/aria-pressed`，
  并尊重 `prefers-reduced-motion`（关闭过渡与动画）。

## 验证记录
- `pnpm test` → 9 文件 64 用例通过（新增 `LayoutStore` 与 `neighborItemId` 单测）
- `pnpm build` → 构建成功（192 模块）
- 说明：本仓库未安装独立 `typescript`，类型校验依赖 Vite/esbuild 转译与编辑器；
  Prettier 配置（`singleAttributePerLine`）全仓未启用，代码沿用既有手写风格。
