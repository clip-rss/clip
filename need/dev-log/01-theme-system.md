# 阶段 1：主题系统与全局样式

## 概述
实现亮色/暗色/Sepia 三档主题，全局 CSS 变量体系，主题切换逻辑。

## 步骤清单

- [x] 定义全局 CSS 变量 Token（`--bg-primary`, `--bg-secondary`, `--bg-tertiary`, `--text-primary`, `--text-secondary`, `--accent`, `--star`, `--border`）
- [x] 实现亮色主题变量值
- [x] 实现暗色主题变量值
- [x] 实现 Sepia 护眼主题变量值
- [x] 创建主题切换 Context/Store（zustand）
- [x] 实现主题切换逻辑：亮 / 暗 / Sepia / 跟随系统
- [x] 首次启动读取系统偏好（`matchMedia('prefers-color-scheme: dark')`）
- [x] 切换无闪烁：渲染前注入 class 到 `<html>`
- [x] 主题切换按钮组件（太阳/月亮/护眼/系统图标，四态轮换）
- [x] 全局字体、字号基准、圆角统一配置
- [x] WCAG AA 对比度验证（至少 4.5:1）

## 验收标准
- 三种主题切换正常无闪烁
- 跟随系统偏好正确
- 所有颜色组合满足对比度要求
