# 阶段 19：性能优化与离线支持 - 实施计划

## 一、现状分析

### 1.1 当前架构概览

**后端数据层（Go）**：
- `internal/store/items.go`：所有查询函数已支持 `LIMIT`/`OFFSET` 参数
  - `ListAllItems(limit, offset)`、`ListUnreadItems(limit, offset)` 等
  - 查询返回**完整字段**（包括 `content`、`summary`），未做字段选择优化
  - `SearchItems` 使用 FTS5 trigram + LIKE 兜底
- `items` 表 schema：
  - `content TEXT`：存储 RSS feed 原始正文 HTML
  - `summary TEXT`：摘要
  - 已有索引：`published_at DESC`、`is_read`、`is_starred`、`feed_id`
  - FTS5 虚拟表 `items_fts(title, summary, note)` with trigram
- **没有单独的"提取全文"字段**，`reader.go` 仅为 2 行 stub

**前端数据流（React + Zustand）**：
- `ArticleStore.ts`：
  - 单次 load 固定拉取 `LOAD_LIMIT = 2000` 条到内存
  - **全量替换** `items` 数组（`set({ items: items ?? [] })`）
  - 筛选/排序在**客户端**完成（`ArticleFilter.ts`），每次 render 都重算（useMemo）
  - 标记已读/星标：乐观更新 + patch 单个 item（不重载列表）
- `ArticleList.tsx` + `@tanstack/react-virtual`：
  - 固定行高 `ROW_HEIGHT = 88`（`estimateSize`）
  - `overscan: 8`
  - 未使用 `measureElement`（非动态高度）
  - `ArticleRow` 未 memo 化
- `ReadingView`：
  - 文章 `content` 已在 `items` 数组中，直接渲染（无二次请求）
  - 图片：`<img>` 直接指向远程 URL（`loading="lazy"` via `Sanitize.ts` hook）
  - 无图片代理/本地缓存

**调度器（Scheduler）**：
- `Start(ctx)` / `Stop()`：全局启停
- `PauseFeed(id)` / `ResumeFeed(id)`：单源级暂停/恢复
- **无全局"离线模式"暂停机制**

**离线检测**：
- 前后端均**无**现有实现（grep `navigator.onLine`、网络探测均未找到）

**图片缓存**：
- Favicon：仅存储 URL 字符串到 `feeds.icon`（未 base64 内嵌）
- 文章图片：无任何缓存/代理

**PRD 性能指标**（`need/doc/clip.md:251-253`）：
- 订阅源 <100 时启动 <2s
- 内存占用 <200MB（万篇文章）
- （列表滚动 60fps 未明确，但虚拟滚动需流畅）

**PRD 离线要求**（`:130-133`）：
- 文章正文和图片缓存到本地 SQLite
- UI 显示离线状态，暂停更新定时器

---

## 二、性能瓶颈诊断

### 2.1 启动性能
- **风险点**：`ArticleStore.load()` 启动时拉取 2000 条 × 全字段（含 `content`），单条 content 可能数十 KB
  - 假设平均 20KB/条 × 2000 = 40MB 序列化传输
  - 数据库 SELECT 无字段筛选（`SELECT id, feed_id, title, ..., content, summary, ...`）
- **影响**：启动首屏延迟，内存峰值

### 2.2 列表渲染性能
- **客户端全量筛选排序**：每次 filter/sort 变化或 selection 变化都完整遍历 2000 条
- **ArticleRow 未 memo**：`selectedItemId`/`filter` 等全局状态变化时，所有可见行重新渲染
- **固定行高非问题**：虚拟滚动配置合理，overscan 适中

### 2.3 内存占用
- **当前策略**：2000 条 × (20KB content + 元数据) ≈ 40-50MB per load
- **潜在问题**：多次切换 feed/category 后，旧数组虽被替换但 GC 前累积
- **万篇缓存**：前端不可能持有万篇全文在内存；PRD 指标指的是**数据库**

### 2.4 离线支持缺失
- 无本地图片缓存 → 离线时图片全部加载失败
- 无离线状态检测 → 调度器仍在尝试请求，产生大量超时错误
- 无 UI 提示

---

## 三、优化策略设计

### 3.1 启动性能优化

#### 方案 A：列表查询字段选择优化（推荐）
- **问题**：列表视图只需 `title`、`summary`（前 200 字）、`published_at`、`is_read`、`is_starred`、`feed_id`，无需 `content`
- **方案**：
  1. 新增 `internal/store/items.go` 函数：`ListItemsLight(feedID, limit, offset)` 返回 `ItemLight` 结构（不含 `content`）
  2. API 新增 `ItemService.ListItemsLight(feedID, limit, offset)`
  3. 前端 `ArticleStore.load()` 调用 `ListItemsLight`，阅读视图点击时再调 `GetItem(id)` 获取完整 content
  4. 生成 bindings：`wails3 generate bindings`
- **收益**：启动时数据传输减少 90%+，内存占用减半

#### 方案 B：减小 LOAD_LIMIT（备选）
- 将 2000 降至 500，结合真正的分页/无限滚动
- **问题**：客户端筛选/排序会受限，需配合后端筛选逻辑重构
- **结论**：暂不采用（改动大，边际收益不明显）

#### 方案 C：启动时延迟加载（补充）
- 应用启动先加载 sidebar 数据（轻量），文章列表等用户首次点击时再拉取
- **实施**：`App.tsx` mount 时只调 `sidebar.load()`，`ArticleList` 依赖 `selection` 变化触发 load（已有逻辑）
- **收益**：空窗口启动时间减少至 <1s

### 3.2 列表渲染性能优化

#### 优化 1：React.memo 包裹 ArticleRow
```tsx
const ArticleRow = React.memo(function ArticleRow(props: ArticleRowProps) { ... })
```
- 配合 `key={item.id}`，避免无关状态变化时重渲染所有行

#### 优化 2：稳定化 props（避免内联对象/函数）
- 检查 `ArticleList.tsx` 传给 row 的 callbacks 是否 useCallback 包裹
- feedTitle map 已 useMemo，符合预期

#### 优化 3：Virtualizer 配置优化
- 当前 `estimateSize: () => ROW_HEIGHT` 固定，overscan=8 合理
- **可选**：未来若需动态高度（卡片视图），切换到 `measureElement` + `ResizeObserver`

### 3.3 内存控制

#### 方案：限制前端内存列表 + 缓存清理提醒
- **前端**：已通过 LOAD_LIMIT=2000 + 分次 load 替换控制，符合 PRD
- **后端**：已有 `store.PruneReadItems()`（删除已读且非星标文章）
- **UI**：设置面板已有缓存统计（`GetCacheStats`）与清理按钮
- **补充**：新增"离线缓存占用"统计（图片缓存大小），一并展示

### 3.4 离线支持实现

#### 3.4.1 离线状态检测

**前端（React）**：
- 新增 `Hooks/useOnlineStatus.ts`：
```ts
export function useOnlineStatus(): boolean {
  const [online, setOnline] = useState(() => navigator.onLine)
  useEffect(() => {
    const handleOnline = () => setOnline(true)
    const handleOffline = () => setOnline(false)
    window.addEventListener('online', handleOnline)
    window.addEventListener('offline', handleOffline)
    return () => {
      window.removeEventListener('online', handleOnline)
      window.removeEventListener('offline', handleOffline)
    }
  }, [])
  return online
}
```

**后端（Go）**：
- 新增 `api/system.go: IsOnline() bool`（探测 DNS 8.8.8.8:53 或 HEAD google.com）
- Scheduler 监听离线 → 暂停轮询（保留 ticker 但 skip tick）

#### 3.4.2 离线 UI 提示

- 新增 `Components/OfflineBanner/OfflineBanner.tsx`：
  - 顶部黄色提示条："📡 离线模式 - 定时更新已暂停"
  - 使用 `useOnlineStatus()` 控制显示
- i18n 新增 keys：`offline.banner`, `offline.syncPaused`
- 插入 `Layout.tsx` 顶部（Toolbar 之前）

#### 3.4.3 图片本地缓存（可选，分阶段）

**阶段 A（本期）**：正文 content 已在 DB（离线可读文字），图片离线失败是可接受的降级
**阶段 B（后期）**：
- 方案 1：后端代理 + 磁盘缓存（`internal/cache/image.go`，LRU eviction）
- 方案 2：抓取时下载图片存 base64 到 `items.images_cache TEXT`（增大 DB 体积）
- **本期不实施**，在计划中标注为"可选"

#### 3.4.4 调度器离线暂停

- `Scheduler` 新增字段 `offlineMode bool` + `SetOfflineMode(bool)`
- `tickSafe` 内检查：`if s.offlineMode { return }`
- Wails 应用层：定时调用 `api/system.IsOnline()`，状态变化时调 `scheduler.SetOfflineMode(!online)` + 发送事件给前端

---

## 四、实施步骤（优先级排序）

### 阶段 A：关键性能优化（P0）

#### A1. 列表查询字段优化（预计收益最大）
1. [ ] **Go 后端**：
   - `internal/store/models.go`：新增 `ItemLight` 结构体（不含 `Content` 字段）
   - `internal/store/items.go`：新增 `ListAllItemsLight(limit, offset)`、`ListUnreadItemsLight`、`ListStarredItemsLight`、`ListItemsByFeedLight`
     - SELECT 字段列表去掉 `content`，其余同现有函数
     - 返回 `[]ItemLight`
   - `api/item.go`：新增 `ItemService.ListItemsLight(feedID, limit, offset)`（类似现有 `ListItems` 逻辑）
   - 运行 `wails3 generate bindings` 生成前端绑定
   - **测试**：`internal/store/items_test.go` 新增 `TestListItemsLight`

2. [ ] **前端**：
   - `Utils/Api/index.ts`：导出新的 `ItemService.ListItemsLight`
   - `Stores/ArticleStore.ts`：
     - 修改 `load()` / `reload()` 调用 `ListItemsLight`（类型改为 `ItemLight[]`）
     - 新增 `loadFullContent(id)` 方法：调用 `GetItem(id)` 获取完整 item，patch 到列表
     - `selectItem(id)` 触发时检查该 item 是否 `content` 为空，若空则先 `loadFullContent`
   - `Types/Article.ts`：新增 `ItemLight` 类型（从 bindings 导入）
   - **测试**：`Stores/ArticleStore.test.ts` 更新 mock

3. [ ] **验证**：
   - 启动应用，查看 Network tab / Go log：列表请求体积
   - 点击文章，确认 content 正确加载
   - 测试搜索、星标、筛选功能无回归

#### A2. 启动延迟加载
1. [ ] `frontend/src/App.tsx`：
   - mount 时只调 `useSidebarStore.getState().load()`
   - 移除 `useArticleStore.getState().load(...)` 的提前调用（依赖 useEffect in ArticleList）
2. [ ] 验证：空窗口启动时间 <1s，点击 feed 后列表加载

#### A3. ArticleRow memo 化
1. [ ] `Components/ArticleList/ArticleRow.tsx`：用 `React.memo()` 包裹导出
2. [ ] 检查 props 传递：确保 callbacks 已 useCallback（`ArticleList.tsx`）
3. [ ] 验证：React DevTools Profiler 观察筛选切换时的 render 次数

### 阶段 B：离线支持（P0）

#### B1. 前端离线检测
1. [ ] `frontend/src/Hooks/useOnlineStatus.ts`：实现 `navigator.onLine` + 事件监听
2. [ ] `Hooks/index.ts`：导出 `useOnlineStatus`
3. [ ] 测试：`Hooks/useOnlineStatus.test.ts`（模拟 online/offline 事件）

#### B2. 离线 UI 提示
1. [ ] `Components/OfflineBanner/OfflineBanner.tsx`：
   - 使用 `useOnlineStatus()`，离线时显示黄色横幅
   - 样式：fixed top, z-index 高于 toolbar
2. [ ] `Components/OfflineBanner/OfflineBanner.module.scss`
3. [ ] `Components/index.ts`：导出 `OfflineBanner`
4. [ ] `Components/Layout/Layout.tsx`：在 Toolbar 之前插入 `<OfflineBanner />`
5. [ ] i18n：`locales/en.json` + `zh.json` 新增：
   ```json
   "offline": {
     "banner": "Offline Mode - Automatic updates paused",
     "syncPaused": "订阅更新已暂停"
   }
   ```

#### B3. 后端离线检测
1. [ ] `api/system.go`：新增 `SystemService.IsOnline() bool`
   - 方法：尝试 DNS lookup `8.8.8.8` 或 HEAD `https://www.google.com`（超时 2s）
   - 返回 true/false
2. [ ] 生成 bindings：`wails3 generate bindings`
3. [ ] 测试：`api/system_test.go`（mock net.Dial）

#### B4. 调度器离线暂停
1. [ ] `internal/scheduler/scheduler.go`：
   - 新增字段 `offlineMode bool` + `SetOfflineMode(offline bool)`（加锁）
   - `tickSafe()` 开头检查：`if s.offlineMode { log.Println("offline, skip tick"); return }`
2. [ ] `main.go`（或 app 初始化）：
   - 启动 goroutine，每 30s 调用 `api/system.IsOnline()`
   - 状态变化时：调 `scheduler.SetOfflineMode(!online)` + `app.Event.Emit("network:status", online)`
3. [ ] 前端监听 `network:status` 事件，更新 store（可选，已有 `navigator.onLine` 足够）
4. [ ] 测试：`scheduler_test.go` 新增 `TestOfflineMode`

### 阶段 C：增量优化（P1）

#### C1. 内存优化（已基本达标，补充文档）
1. [ ] 文档：在 `need/dev-log/19-performance-offline.md` 记录：
   - LOAD_LIMIT=2000 限制内存
   - 数据库 `PruneReadItems()` 清理机制
   - 前端 GC 依赖 Zustand 替换旧数组

#### C2. 虚拟滚动优化（可选）
1. [ ] （若未来需卡片视图）升级为动态高度：
   - `ArticleList.tsx`：切换 `useVirtualizer` 为 `measureElement: (el) => el.getBoundingClientRect().height`
   - 增加 `ResizeObserver` polyfill（若需兼容旧浏览器）
2. [ ] **本期跳过**（固定行高已满足需求）

#### C3. 图片缓存（可选，后期）
1. [ ] **本期不实施**，标记为 TODO：
   - 方案：`internal/cache/image.go` 实现 LRU 磁盘缓存 + 反向代理
   - 前端 `<img src>` 改为 `wails://image-proxy?url=<encoded>`
   - 需额外 5-10h 工时
2. [ ] 在 `19-performance-offline.md` 标注："图片离线缓存（可选）— 留待阶段 21+"

### 阶段 D：测试与验证（P0）

#### D1. 性能测试
1. [ ] **启动性能**：
   - 测试数据：100 feeds，每个 50 篇文章（共 5000 篇）
   - 测量：应用启动到首屏可交互时间
   - 目标：<2s（冷启动），<1s（热启动）
2. [ ] **列表滚动**：
   - 测试：加载 2000 篇文章，快速滚动
   - 工具：React DevTools Profiler，Chrome Performance
   - 目标：60fps，无明显卡顿
3. [ ] **内存占用**：
   - 测试：加载 2000 篇文章后，Chrome Task Manager 查看内存
   - 目标：Renderer 进程 <100MB

#### D2. 离线功能测试
1. [ ] **离线检测**：
   - 断开网络，检查横幅显示
   - 恢复网络，检查横幅消失
2. [ ] **调度器暂停**：
   - 离线时查看 Go log，确认 tick 被 skip
   - 恢复后确认自动恢复更新
3. [ ] **离线阅读**：
   - 加载文章列表后断网
   - 切换文章，确认已加载的 content 可正常显示
   - 未加载的文章显示"离线时无法加载"提示（需补充 error handling）

#### D3. 回归测试
1. [ ] 运行现有测试套件：
   - Go：`go test ./internal/... ./api/...`
   - 前端：`pnpm test`（vitest）
2. [ ] 手动测试核心流程：
   - 添加 feed → 刷新 → 阅读 → 标记已读 → 搜索 → 星标
   - 切换主题、语言
   - 专注模式

---

## 五、技术细节与约束

### 5.1 Go 后端规范
- **命名**：`ItemLight` 结构体，`ListItemsLight` 函数（与现有风格一致）
- **测试**：table-driven tests，使用 `t.Run(name, func(t *testing.T) {...})`
- **错误处理**：`fmt.Errorf("failed to ...: %w", err)`

### 5.2 前端规范
- **组件**：`function OfflineBanner(): JSX.Element`（不用箭头函数）
- **文件命名**：`OfflineBanner/OfflineBanner.tsx` + `.module.scss`（大驼峰）
- **导出**：`Components/index.ts` 统一导出
- **Hooks**：`use` 前缀，返回类型明确
- **测试**：vitest，mock Wails bindings via `vi.mock('../Utils/Api')`

### 5.3 i18n 规范
- 结构：`{ "offline": { "banner": "...", "syncPaused": "..." } }`
- 同时更新 `en.json` 和 `zh.json`
- 中文提示：简洁、斜体（遵循 `design.md §5`）

### 5.4 性能指标验证
- **启动时间**：macOS Activity Monitor / Windows Task Manager 记录 app 启动到首次绘制
- **内存占用**：Chrome DevTools Memory Profiler（前端）+ Go pprof（后端，若需）
- **滚动 fps**：Chrome Performance tab，录制滚动操作，查看 fps 图表

---

## 六、风险与缓解

### 6.1 风险：ItemLight 导致类型复杂度增加
- **描述**：前端需处理 `Item` 和 `ItemLight` 两种类型
- **缓解**：
  - 统一 store 内部类型为 `Item`（content 可为空字符串）
  - `ListItemsLight` 返回后立即转换为 `Item`（content=""）
  - 仅在 load/patch 时区分，其他逻辑无感知

### 6.2 风险：离线检测误报（网络不稳定）
- **描述**：短暂网络抖动导致频繁切换离线/在线
- **缓解**：
  - 前端 `useOnlineStatus` 增加 debounce（2s）
  - 后端 `IsOnline()` 失败重试 2 次

### 6.3 风险：图片离线失败用户体验差
- **描述**：离线时文章图片全部 broken image
- **缓解**：
  - **本期**：CSS 隐藏 broken image 图标，显示占位文字"图片离线时不可用"
  - **后期**：实施图片缓存（阶段 C3）

---

## 七、验收标准

### 7.1 性能指标
- [ ] 启动时间（100 feeds）：<2s
- [ ] 列表滚动（2000 篇）：60fps 无卡顿
- [ ] 内存占用（2000 篇）：前端 <100MB

### 7.2 离线功能
- [ ] 离线横幅正确显示/隐藏
- [ ] 离线时调度器暂停（无请求超时日志）
- [ ] 已加载文章离线可读（文字部分）

### 7.3 无回归
- [ ] 所有现有测试通过（Go + 前端）
- [ ] 手动测试核心流程无异常

---

## 八、时间估算

| 任务 | 预计工时 |
|:-----|:--------|
| A1. 列表查询字段优化 | 4h |
| A2. 启动延迟加载 | 1h |
| A3. ArticleRow memo | 1h |
| B1-B2. 前端离线检测 + UI | 3h |
| B3-B4. 后端离线检测 + 调度器暂停 | 3h |
| C1. 内存优化文档 | 0.5h |
| D1-D3. 测试与验证 | 4h |
| **总计** | **16.5h** |

---

## 九、遗留问题（后续阶段处理）

1. **图片本地缓存**（可选）：需要 LRU 缓存 + 反向代理，工作量 5-10h，留待用户需求明确后实施
2. **真实分页/无限滚动**：当前 LOAD_LIMIT=2000 已满足 PRD，若未来需支持数万篇文章，再重构为增量加载
3. **后端全文提取（Readability）**：`reader.go` 为 stub，若需"阅读器模式"需集成 readability 算法（Go port 或调 Node.js）
4. **图片防盗链处理**：PRD 提到"可选"，当前无实现，用户遇到防盗链问题时再补充

---

## 十、Plan Mode 输出

**已完成探索**：
- ✅ 后端数据层（`store/items.go`、schema、API）
- ✅ 前端数据流（`ArticleStore`、虚拟滚动、渲染逻辑）
- ✅ 调度器结构（`Scheduler` 启停机制）
- ✅ 离线检测现状（无现有实现）
- ✅ 图片处理（无缓存/代理）
- ✅ PRD 性能指标与离线要求
- ✅ 编码规范、测试约定、i18n 结构

**待用户确认**：
1. **图片离线缓存是否本期实施**？（建议后置，本期仅支持文字离线阅读）
2. **启动性能目标是否调整**？（当前 <2s，实际可能优化到 <1s）
3. **是否需要后端 Readability 提取**？（当前 RSS content 已存储，是否还需再提取）

**推荐优先级**：
- **P0（必做）**：A1（字段优化）、B1-B4（离线支持）、D1-D3（测试）
- **P1（建议）**：A2（启动优化）、A3（memo）
- **P2（可选）**：C2（动态高度）、C3（图片缓存）
