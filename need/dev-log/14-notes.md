# 阶段 14：笔记功能

## 概述
实现文章笔记面板，支持 Markdown 纯文本编辑与搜索。

## 步骤清单

- [x] 笔记面板组件（阅读视图底部抽屉，工具栏按钮开关；阅读视图与专注模式共用）
- [x] 笔记编辑器（纯文本 textarea，等宽字体，支持 Markdown 语法）
- [x] 笔记自动保存（防抖 500ms；切换文章/关闭/卸载时立即冲刷）
- [x] 笔记关联文章（通过 item id，ArticleStore.saveNote → ItemService.AddNote）
- [x] 笔记在搜索中可被检索（后端 FTS5 已索引 note 列，trigram + LIKE 兜底）
- [x] 笔记导出（note 为 Item 模型字段，随文章元数据序列化/持久化）
- [x] 阅读工具栏笔记开关按钮状态（有笔记时图标高亮，面板展开时激活态）

## 验收标准
- [x] 笔记可正常编辑保存
- [x] 笔记与文章正确关联
- [x] 搜索可命中笔记内容

## 实现说明
- 后端在阶段 04/13 已就绪：`items.note` 列、`Store.UpdateItemNote`、`ItemService.AddNote`（含生成绑定）、FTS5 索引 note。本阶段为纯前端接入。
- 面板开关为会话态，放在 `LayoutStore.notePanelOpen`（不持久化），阅读视图与专注模式共用。
- 笔记草稿为 `NotePanel` 本地态，乐观写入 `items`/`searchResults`（保证高亮与搜索一致），后端失败回滚。
