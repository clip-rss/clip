# 阶段 10：右侧栏 - 阅读视图

## 概述
实现文章正文阅读区域，包括内容渲染、排版、工具栏、滚动位置记忆。

## 步骤清单

- [x] 阅读视图容器组件（背景 `--bg-primary`）
- [x] 阅读工具栏（浮动顶部，40px 高，下阴影）
  - [x] 左部：文章标题（18px 加粗，溢出省略）
  - [x] 右部操作：标记已读/未读、星标、笔记开关、浏览器打开、更多菜单
- [x] 正文区域
  - [x] 最大宽度居中，上下 24px 内边距
  - [x] 文章大标题（24px 加粗）
  - [x] 元数据行（作者、发布时间、来源，14px 次级色）
  - [x] 分隔线
  - [x] HTML 正文渲染（DOMPurify 清洗，统一排版）
- [x] 正文排版
  - [x] 字体切换（无衬线/衬线/等宽）
  - [x] 字号设置（14/16/18px）
  - [x] 行高设置（1.5/1.8/2.0）
  - [x] 最大阅读宽度（640/800/全宽）
- [x] 图片处理
  - [x] 懒加载 + 灰色骨架屏占位
  - [x] 最大宽度 100%，圆角 6px
  - [x] 点击放大预览（灯箱效果）
- [x] 代码块（背景 `--bg-tertiary`，等宽字体，横向滚动）
- [x] 链接行为：系统浏览器打开
- [x] 纤细滚动条（4px，复用全局）
- [x] 底部 "已是全部内容" 提示
- [x] 滚动位置记忆（切换文章时记住进度）
- [x] 阅读区独立背景色选项（跟随主题/亮色/护眼/暗色）

## 验收标准
- [x] HTML 正文正确渲染，无 XSS 风险（DOMPurify + script/on*/style 清除，含 Sanitize 单测）
- [x] 排版设置实时生效（持久化 ReaderStore）
- [x] 图片懒加载、放大预览正常
- [x] 代码块统一样式（按约定不做 token 级语法高亮）
- [x] 滚动位置记忆正确

## 实现说明与边界
- **新增文件**
  - 类型：`Types/Reader.ts`（字体/字号/行高/宽度/背景 + `ReaderPrefs`）
  - 工具：`Utils/Sanitize.ts`（`sanitizeHtml`，DOMPurify + img 懒加载/外链 rel）、
    `Utils/ReaderStyle.ts`（`readerContentStyle` / `readerBackgroundClass`）
  - 状态：`Stores/ReaderStore.ts`（排版偏好，zustand+persist，名 `clip-reader`）
  - 组件：`Components/ReadingView/`（`ReadingView` / `ReaderToolbar` / `ReaderContent` /
    `ReaderSettingsMenu` / `Lightbox` / `Icons`）
  - 全局：`global.css` 新增 `.light` 主题类（供独立阅读背景在暗色应用下重置 token）
- **正文安全**：引入 `dompurify`；`Item.content` 经 `sanitizeHtml` 清洗后 `dangerouslySetInnerHTML`。
- **排版设置存前端**：后端 `Settings` 无阅读排版字段，故用持久化 `ReaderStore`（沿用 ThemeStore/LayoutStore 模式）。
- **独立阅读背景**：映射到全局 `.light/.sepia/.dark` 类施加于滚动容器，在子树内重置 CSS 变量 token，
  标题/正文/边框一并适配；`default` 继承应用主题。
- **链接/图片**：委托点击——`<a>` 经 `openURL` 用系统浏览器打开；`<img>` 点击灯箱放大（Esc/点遮罩关闭）。
- **工具栏复用**：标记已读⇄未读/星标复用 `ArticleStore`，联动列表与侧栏未读。
- **笔记按钮**：占位禁用，归 **阶段 14**。
- **代码高亮**：按约定仅样式化代码块，不引语法高亮库。
- **测试**：新增 `Sanitize` / `ReaderStyle` / `ReaderStore` 单测，连同既有共 **53 用例全部通过**。

## 验证记录
- `pnpm test` → 8 文件 53 用例通过
- `pnpm build` → 构建成功（185 模块）
- `tsc --noEmit -p tsconfig.json` → 0 错误
