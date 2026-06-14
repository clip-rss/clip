# 阶段 00 剩余项实施计划

## 目标
完成项目基础架构搭建：前端依赖安装、样式体系配置、目录结构、Go 后端模块骨架、窗口最小尺寸配置。

## 实施步骤

### 1. 前端依赖安装

在 `frontend/` 下安装：

**运行时依赖**：
- `tailwindcss` + `@tailwindcss/vite` — Tailwind CSS v4（Vite 插件方式集成）
- `@radix-ui/react-dialog`, `@radix-ui/react-dropdown-menu`, `@radix-ui/react-tooltip`, `@radix-ui/react-context-menu` — 无头组件库
- `@tanstack/react-virtual` — 虚拟滚动
- `zustand` — 状态管理
- `clsx` — class 合并工具

**开发依赖**：
- `sass` — SCSS 支持
- `prettier` — 格式化
- `typescript` — TS 编译器（确保存在）

### 2. Tailwind CSS 配置

使用 Tailwind v4 的 CSS-first 方式，在 `src/Styles/global.css` 中通过 `@import "tailwindcss"` 引入，并定义 `@theme` 层中的 CSS 变量色彩系统。无需 `tailwind.config.js` 配置文件（v4 新模式）。

在 `vite.config.js` 中添加 `@tailwindcss/vite` 插件。

### 3. CSS 变量主题体系

在全局样式中定义 `:root`（亮色）和 `.dark`（暗色）的 CSS 变量，包含设计稿中所有色彩 token。

### 4. SCSS Module 配置

Vite 原生支持 `.module.scss`，只需安装 `sass` 即可。组件通过 `import styles from './Xxx.module.scss'` 使用。

### 5. Prettier 配置

在 `frontend/` 下创建 `.prettierrc`。

### 6. 前端目录结构

```
frontend/src/
├── Components/     # 公共组件（index.ts 统一导出）
├── Pages/          # 页面组件
├── Utils/          # 工具函数（index.ts 统一导出）
├── Types/          # TypeScript 类型定义
├── Hooks/          # 自定义 Hooks
├── Stores/         # Zustand 状态管理
├── Styles/         # 全局样式、主题变量
│   └── global.css  # Tailwind 入口 + CSS 变量
└── App.tsx         # 根组件
```

### 7. Go 后端模块目录

```
internal/
├── store/       # 数据库操作
├── fetcher/     # RSS 抓取、解析
├── scheduler/   # 定时调度
├── opml/        # OPML 导入导出
├── reader/      # Readability 正文提取
api/             # Wails 绑定方法
```

每个包创建一个基础 `.go` 文件以确保包可编译。

### 8. 窗口最小尺寸

在 `main.go` 中为窗口添加 `MinWidth: 800, MinHeight: 600` 配置。

### 9. 清理模板代码

- 移除 `greetservice.go`（示例代码）
- 更新 `main.go` 移除 GreetService 和时间事件 goroutine
- 重写 `App.tsx` 为基础空壳

### 10. 修复 index.html

将 `<script src="/src/main.jsx">` 修正为 `main.tsx`。
