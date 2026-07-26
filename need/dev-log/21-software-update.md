# 阶段 21：软件更新（自动更新）

## 概述
基于 Wails v3 内置 `updater` 包实现应用自动更新：菜单「Check for Updates…」触发检查，
自建「Software Update」子窗口展示检查/下载/安装进度。Provider 为 GitHub Releases。

## 步骤清单

- [x] 菜单栏「Check for Updates…」入口 — 从 `application.DefaultApplicationMenu()` 出发（保留默认菜单及快捷键），macOS 放 App 菜单 About 之后，其他平台放 Help 菜单（`main.go`）
- [x] GitHub Provider 接入 — `github.New{Repository, ChecksumAsset:"SHA256SUMS"}`，`Updater.Init` 注入
- [x] 自建 Software Update 子窗口 — `softwareUpdateWindow`，每轮 `rebuild` 新建（详见 memory: project-updater-byowindow）
- [x] 交互式更新流程「先展示、后决定」— `updateController`：点检查只 `Check`（不下载），窗口展示 release notes + 更新/稍后/跳过/关闭；仅点「更新」才 `DownloadAndInstall`。`Config.Window = updater.WindowNone`，UI 自驱
- [x] Software Update 窗口 i18n — **单一数据源**：Go 启动时取 `frontend/src/I18n/locales/{en,zh}.json` 的 `updater` 段，建窗时连同当前语言注入 window.html 的占位符（`__CLIP_I18N_DICT__`/`__CLIP_I18N_LANG__`）。语言现读 `store.Settings.Language`；因窗口每轮 rebuild，下次打开即用最新语言（弹窗开着时切语言不实时生效，可接受）。有回归测试 `TestUpdaterI18nInjection`
- [ ] **临时目录孤儿更新包兜底清理**（见下方「待做」）
- [ ] 更新失败错误消息本地化（错误文本由 Updater 产生，多为英文；窗口仅翻译了无消息时的兜底文案）
- [ ] 「跳过此版本」的持久化（当前 `SkipVersion` 只在内存，重启即失效 → 可存 store.Settings）

## 待做：临时目录孤儿更新包兜底清理

**问题**（读 `wails/v3@v3.0.0-alpha.98/pkg/updater` 源码得出的静态结论，未实机观测）：

- 更新包下载到系统临时目录 `os.MkdirTemp("", "wails-update-*")`
  - macOS `$TMPDIR`、Windows `%TEMP%`
  - 流程：`.artifact`（边下边校验）→ rename 成真实文件名 → 若为 .zip/.tar.gz 就地解压到 `.payload-*` 子目录
- **关闭更新弹窗不会删除已下载的包**：Cancel / Remind / Skip / 点 X 都只走 `closeWindow()`→`session.close()`（只取消事件监听、关窗口句柄），完全不碰 staging 目录。
- staging 目录只在这些时机被删：下次**真正开始下载**时删上一份（`discardStaging` @ `updater.go:283`）／下载校验解压失败／`Restart()` 换装后由 helper 子进程删／OS 自行清理。
- **孤儿场景**：`u.stagingDir` 只存在内存。用户「下载完更新 → 关弹窗 → 不重启 → 退出 App」后，这条记录丢失；下次启动再检查，`discardStaging()` 因 `stagingDir==""` 删不掉旧目录 → 残留一份完整更新包（macOS 常为几十~上百 MB）。每重复一次就多攒一份。
- **注意注释与实现不符**：`discardStaging` 注释声称「Called before a new download begins **and on Check**」，但实际 `Check()` 并未调用它——只有走到 `DownloadAndInstall` 才清。别指望「重新检查一下」清旧包。

**待做方案**（择一或组合）：

- [ ] App 启动时扫描 `os.TempDir()` 下的 `wails-update-*` 目录，按修改时间清理超过阈值（如 >24h 或保留最新 1 份）的孤儿目录
- [ ] 或：在 `OnShutdown` 里，若存在已下载但未 Restart 的 staging（`app.Updater.DownloadedPath() != ""`），主动 `RemoveAll`
- [ ] 清理需谨慎并发：可能有另一个 Clip 实例正在下载，勿删他人正在写入的目录（结合目录时间戳 / 文件锁）

**相关 API**：`app.Updater.DownloadedPath()` 返回最后一次已暂存更新的路径（未暂存则 ""）。

## 实现说明（已完成部分）

### i18n（单一数据源 = locale JSON）
更新窗口是独立 HTML 页（无 React/i18next）。改文案**只改** `frontend/src/I18n/locales/{en,zh}.json`
的 `updater` 段即可，不必碰 HTML。机制：
- Go 用 `//go:embed` 收两份 locale JSON（`updaterLocaleEN/ZH`），`updaterI18nDict()` 取其中
  `updater` 段拼成 `{en:{...},zh:{...}}`。
- `buildSoftwareUpdateHTML(dict, lang)` 把字典与当前语言替换掉 window.html 的两个占位符
  `__CLIP_I18N_DICT__` / `__CLIP_I18N_LANG__`（各只能出现一次，Go 用 Replace(...,1)）。
- 语言在**建窗时**由 `langFn`（现读 `store.Settings.Language`）注入；窗口每轮 rebuild，故下次
  打开即最新语言。首帧即正确、无闪烁、无竞态，也无需再发 `clip:updater:lang` 事件。
- 回归测试 `TestUpdaterI18nInjection`：断言占位符被替换、字典合法 JSON、en/zh key 一致。
- ⚠️ 升级 Wails 同步 window.html 模板时，注意别丢了那两个占位符。

### main.go
- `softwareUpdateHTML`：`//go:embed build/updater/window.html`（复制自 Wails 内置模板，其模板 HTML 未导出；已本地化）
- `softwareUpdateWindow`：自管理窗口，`ensure()` 用 `app.Window.NewWithOptions`（`AllowSimpleEventEmit:true`）新建，`rebuild()` 每轮关旧建新（保证干净 JS 状态）；`OnWindowEvent(WindowClosing)` 清空引用
- `updateController`：编排流程。启动时 `wire()` 一次性注册全部监听（用户动作 + 状态事件 + window:ready 重放）；`check()` = rebuild 窗口 + 只 `Check`
- `Updater.Init(updater.Config{ ..., Window: updater.WindowNone })` — Updater 无头，UI 自驱
- 菜单 `checkForUpdates` → `updCtrl.check()`

### 流程与事件契约
- 状态：Updater `app.Event.Emit` 广播 `wails:updater:*`（check-started/update-available/no-update/downloading/verifying/installing/update-ready/error），窗口 JS 监听渲染
- 动作：窗口按钮 `AllowSimpleEventEmit` 回发 `wails:updater:user:*`，`updateController` 用 `app.Event.On` 收 → 调 `Check`/`DownloadAndInstall`/`Restart`/`SkipVersion`
- `EventMeta`（当前版本）需自己在 window:ready 时补发（`Check` 不发它）
- 窗口尺寸自己按状态 full/compact（WindowNone 下 Updater 不自动 SetSize）

### 未验证项
- 完整交互（真实弹窗、展示 notes、走完下载/安装/Restart）需 `wails3 build` 打包后实机测试
- 窗口 ready 重放与 Check 的竞态、尺寸自适应，均为源码静态分析结论，未实机观测
- 临时目录/清理结论同为静态分析
