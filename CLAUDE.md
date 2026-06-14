# Clip - RSS 阅读器项目

## 项目概述

跨平台（macOS / Windows）RSS 阅读桌面应用，基于 Wails v3（Go + React + TypeScript）。

## 技术栈

- **桌面框架**：Wails v3 (Go 1.25 + WebView)
- **前端**：React 18, TypeScript, Vite 8, pnpm
- **样式**：Tailwind CSS + CSS 变量 + SCSS Modules (`xxx.module.scss`)
- **状态管理**：Zustand
- **虚拟滚动**：@tanstack/react-virtual
- **数据库**：SQLite (modernc.org/sqlite, 纯 Go)
- **构建**：Taskfile

## 文档索引

| 文件 | 说明 |
|:-----|:-----|
| [need/doc/clip.md](need/doc/clip.md) | 产品需求文档 |
| [need/doc/design.md](need/doc/design.md) | UI 设计稿规范 |
| [need/doc/rules.md](need/doc/rules.md) | 开发编码规范 |
| [need/dev-log/README.md](need/dev-log/README.md) | **开发步骤总览与进度** |

## 开发步骤日志

所有开发步骤日志位于 `need/dev-log/` 目录，按模块细分为 21 个文件（阶段 00-20）。每个文件包含该模块的具体任务清单，用 `[x]`/`[ ]` 标记完成状态。

**进度查看**：`need/dev-log/README.md` 中有总览表格。

**建议开发顺序**：
1. 基础框架（00-03）：项目初始化 → 主题 → 布局 → 工具栏
2. 后端核心（04-07）：数据库 → RSS抓取 → 调度器 → API绑定
3. 前端 UI（08-11）：侧栏 → 文章列表 → 阅读视图 → 专注模式
4. 功能完善（12-15）：订阅管理 → 搜索 → 笔记 → 快捷键
5. 体验增强（16-20）：通知 → 设置 → 国际化 → 性能 → 无障碍

## 编码规范（摘要）

- React 组件使用 `function` 声明，不用箭头函数
- 明确参数与返回值类型，公共类型放 `src/types/`
- 样式使用 `xxx.module.scss`
- 目录和文件名使用**大驼峰**
- `utils/` 和公共 `components/` 通过 `index.ts` 统一导出再使用
- 代码简洁、低耦合、模块化
- 完成模块后必须有测试用例

## Go 后端模块结构

```
internal/
  store/       # 数据库操作
  fetcher/     # HTTP 抓取、RSS/Atom 解析
  scheduler/   # 定时调度
  opml/        # OPML 导入导出
  reader/      # Readability 正文提取
api/           # Wails 绑定方法，暴露给前端
```

## Wails 代码生成（wails3 generate）

修改 `api/` 暴露给前端的绑定方法或 Go 模型后，运行 `wails3 generate bindings` 重新生成前端 TS 绑定与模型，**不要手写** `frontend/bindings/` 下的文件。常用子命令：

| 命令 | 说明 |
|:-----|:-----|
| `wails3 generate bindings` | 生成前端 TS 绑定 + 模型（改动 `api/` 后必用） |
| `wails3 generate constants` | 从 Go 生成 JS 常量 |
| `wails3 generate runtime` | 生成预构建运行时 |
| `wails3 generate icons` | 生成应用图标 |
| `wails3 generate syso` | 生成 Windows `.syso` 资源文件 |

> 凡涉及 Go ↔ 前端绑定/模型/常量同步的场景，直接调用对应 `wails3 generate` 子命令，无需询问。

## 前端目录结构

```
frontend/src/
  Components/  # 公共组件
  Pages/       # 页面组件
  Utils/       # 工具函数
  Types/       # TypeScript 类型定义
  Hooks/       # 自定义 Hooks
  Stores/      # Zustand 状态管理
  Styles/      # 全局样式、主题变量
```

## 工作流程

1. 开发前查看 `need/dev-log/README.md` 确认当前进度
2. 选择下一个待开发模块，打开对应日志文件
3. 按步骤清单逐项完成，完成后标记 `[x]`
4. 模块全部完成后，更新 `README.md` 中状态为 ✅
5. 每个模块完成后需通过测试用例
