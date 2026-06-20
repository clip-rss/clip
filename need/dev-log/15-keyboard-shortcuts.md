# 阶段 15：键盘快捷键系统

## 概述
实现全局键盘快捷键，提升操作效率。

## 步骤清单

- [x] 快捷键管理框架（统一注册/注销，防冲突）——`Hooks/useHotkeys.ts` 单一监听 + 归一化组合键 + 守卫；`useAppHotkeys` 声明式绑定，App 顶层挂载一次
- [x] `Ctrl+N` / `Cmd+N`：添加订阅
- [x] `Ctrl+Shift+I`：导入 OPML（经自定义事件触发 Sidebar 既有流程）
- [x] `Ctrl+Shift+E`：导出 OPML
- [x] `Space`：阅读区向下翻页（焦点在按钮/链接时保留默认行为）
- [x] `Shift+Space`：阅读区向上翻页
- [x] `r`：刷新当前源或全部源（`SidebarStore.refreshSelected`）
- [x] `Shift+R`：强制全量刷新（`forceRefreshAll`）
- [x] `/`：聚焦搜索框
- [x] `Esc`：关闭模态框（Radix 自处理）/ 退出专注模式（FocusMode 自处理）
- [x] `Ctrl+1/2/3`：切换文章筛选（全部/未读/星标）— 见说明
- [x] `Ctrl+Shift+F`（Cmd+Shift+F）：切换专注模式
- [x] `J/K`：专注模式下切换上/下一篇文章（FocusMode 自处理）
- [x] 快捷键提示（tooltip 中显示对应快捷键，按平台显示 ⌘/Ctrl）

## 验收标准
- [x] 所有快捷键可正常触发
- [x] 不与系统/浏览器快捷键冲突（修饰键归一化，首个匹配生效）
- [x] 模态框打开时不误触全局快捷键（`isModalOpen` 检测 Radix `data-state="open"`）

## 实现说明
- **框架**：`useHotkeys(bindings)` 在 window 挂单一 keydown 监听，用 `comboFromEvent` 把事件归一化为 `mod+shift+key` 形式（`mod` 跨平台匹配 Ctrl/Cmd），首个匹配的绑定生效以防冲突；统一守卫可编辑元素与模态态。
- **Ctrl+1/2/3**：原文「切换布局模式」在当前代码无对应概念（仅三栏 + 专注两态，列表/紧凑/卡片视图未实现），按约定改绑文章筛选（1=全部 2=未读 3=星标，复用 `ArticleStore.setFilter`）。视图密度功能落地后可再调整。
- **模态判定**：专注模式覆盖层虽为 `role="dialog"` 但无 `data-state`，不会被当作模态，确保专注模式内快捷键照常可用。
- 旧的 `useFocusHotkey` / `useSearchHotkey` 已并入统一框架并删除。
