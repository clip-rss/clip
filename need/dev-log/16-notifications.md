# 阶段 16：通知系统

## 概述
实现新文章到达的系统原生通知。

## 步骤清单

- [x] Go 侧系统通知集成 — Wails v3 内置 `notifications` 服务（mac/win/linux），无额外依赖
- [x] 通知类型配置（每篇新文章 / 仅摘要 / 关闭）— `store.Settings.NotificationMode` 字段，常量 `NotifyEach/NotifySummary/NotifyOff`
- [x] 通知内容格式（标题 = 源名，正文 = 文章标题；摘要模式合并）
- [x] 点击通知行为：调起窗口 + 定位到该文章 — `OnNotificationResponse` → `UnMinimise/Restore/Focus` + `notification:open` 事件 → `useNotificationNavigation` 自动查找源并切换列表选中
- [x] 通知设置 UI — 工具栏 🔔 图标 + Radix 下拉单选框（每篇/摘要/关闭），`SettingsStore` 持久化
- [x] macOS 通知权限请求处理 — `ApplicationStarted` 时自动调用 `RequestNotificationAuthorization()`
- [x] Windows 通知 Toast 适配 — 由 Wails notifications 服务封装的 Windows Toast 实现，透明适配

## 验收标准
- [x] 新文章到达时正确弹出系统通知（通过 `scheduler.Notifier` 钩子，首次抓取不通知，避免订阅刷屏）
- [x] 点击通知可正确跳转（前端定位链：GetItem → scheduleSelect → load 消费 → 选中）
- [x] 用户可配置通知行为（工具栏下拉持久化到 Settings）

## 实现说明

### 架构分层
```
scheduler.refreshFeed
  → notifier.Notify(feed, newItems)     // 仅当 LastUpdated ≠ nil（非首次）
    → notify.Plan(mode, feedTitle, items) // 纯函数：each/summary/off 决策
      → notify.Sender.Send(msg)          // 接口
        → wailsNotifSender.Send          // 实现：调 notifications.SendNotification
```

### 后端
- **store.Settings**: `NotificationsEnabled bool` → `NotificationMode string` (each/summary/off)，默认 each，向后兼容（旧 JSON 缺字段时降级默认值）
- **scheduler.Notifier 接口**: `Notify(ctx, feed, items)` — 单方法，`nopNotifier` 默认，`WithNotifier` 注入
- **scheduler.refreshFeed**: 收集新建条目 `[]NewItem`，仅在 `feed.LastUpdated != nil` 时调用 notifier（首次订阅抓取不通知，避免上百条刷屏）
- **internal/notify 包**: `Plan()` 纯函数（可单测），规则：
  - `off` → 空
  - `summary` → 一条摘要（标题含篇数，正文展示前 3 篇标题，超出加省略）
  - `each` → 每篇一条，但 >5 篇自动合并为摘要（节流）
- **main.go**: 注册 `notifications.New()` 为 application.Service；`wailsNotifSender` 实现 `notify.Sender` 接口；`OnNotificationResponse` 回调调用 `UnMinimise/Restore/Focus` + `app.Event.Emit("notification:open")` 传递 articleId

### 前端
- **SettingsStore**: 封装 `SettingsService.GetSettings/UpdateSettings`，乐观写入+失败回滚
- **Toolbar**: 新增 🔔 图标按钮（Radix `DropdownMenu.CheckboxItem` 三选一），关闭时图标显示斜线
- **ArticleStore**: 新增 `pendingSelectId` + `scheduleSelect`；`load` 内部改写为 `fetchAndResolve`，完成时自动消费待定位 ID
- **通知点击定位链**: `onNotificationOpen` → `ItemService.GetItem(articleId)` → `scheduleSelect(id)` → `select({kind:'feed'})` → `load` → `fetchAndResolve` 自动命中 → ReadingView 展示文章

### 未验证项
- 原生通知弹窗仅在实际桌面窗口（`wails3 build` 后）可见，CI/开发模式无法测试
- macOS 通知权限弹窗需要签名/打包后的应用才能正确弹出
