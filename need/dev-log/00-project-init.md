# 阶段 0：项目初始化与基础架构

## 概述
搭建项目基础架构，确保开发环境完备、基本依赖就绪。

## 步骤清单

- [x] Wails v3 项目脚手架创建（Go + React + TypeScript + Vite）
- [x] Go module 初始化（go.mod）
- [x] 前端 pnpm 依赖安装（React, Vite, TypeScript）
- [x] Taskfile 构建配置
- [x] 安装前端核心依赖：Tailwind CSS, Radix UI, @tanstack/react-virtual, zustand, sass
- [x] 配置 Tailwind CSS（darkMode: 'class'）+ CSS 变量色彩体系
- [x] 配置 prettier 格式化规则
- [x] 建立前端目录结构：`src/Components/`, `src/Pages/`, `src/Utils/`, `src/Types/`, `src/Hooks/`, `src/Stores/`, `src/Styles/`
- [x] 建立 Go 后端模块目录：`internal/store/`, `internal/fetcher/`, `internal/scheduler/`, `internal/opml/`, `internal/reader/`, `api/`
- [x] 配置 SCSS module 方案（`xxx.module.scss`）
- [x] 配置窗口最小尺寸 800×600

## 验收标准
- 项目可正常 `task dev` 启动
- Tailwind + CSS 变量亮暗切换可正常工作
- 前后端目录结构完整
