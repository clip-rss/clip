## 0.6.0

#### 2026-09-05

### 新增
- 侧栏支持订阅源排序
- 文章列表支持按时间排序
- 新增查看订阅源信息功能
- 设置新增「无障碍」分区

### 优化
- 部分 UI 优化

### New Features
- Sidebar feeds can be sorted
- Article list can be sorted by time
- Added the ability to view feed info
- Added an "Accessibility" section in settings

### Improvements
- Minor UI polish

### 新增
- 側欄支援訂閱源排序
- 文章列表支援依時間排序
- 新增查看訂閱來源資訊功能
- 設定新增「無障礙」分區

### 優化
- 部分 UI 優化

## 0.5.0

#### 2026-09-02

### 新增
- 文章列表默认筛选改为「全部」
- 侧栏未读角标接近单源上限时将变色提醒

### 修复
- 修复自动刷新时选中文章被重置、阅读被打断的问题
- 修复搜索结果点击后未在阅读器中打开的问题

### 优化
- 部分 UI 优化

### New Features
- The article list filter now defaults to "All"
- The sidebar unread badge changes color as a feed approaches its item cap

### Bug Fixes
- Fixed auto refresh resetting the selected article and interrupting reading
- Fixed search results not opening in the reader when clicked

### Improvements
- Minor UI polish

### 新增
- 文章列表預設篩選改為「全部」
- 側欄未讀角標接近單源上限時將變色提醒

### 修復
- 修復自動重新整理時選中文章被重置、閱讀被打斷的問題
- 修復搜尋結果點擊後未在閱讀器中開啟的問題

### 優化
- 部分 UI 優化

## 0.4.0

#### 2026-09-01

### 新增
- 侧栏支持多选批量删除订阅源，右键菜单新增「删除全部出错订阅源」
- 支持从远程 OPML 链接导入订阅；批量导入改为事务式并显示进度，过程中侧栏逐步合并结果
- 新增应用内 toast 通知；后端错误改为以可读文案弹出，多条提示堆叠展示、悬停展开
- 刷新时对应订阅源显示加载动画
- 侧栏标题旁显示订阅源数量
- 抓取请求伪装为浏览器流量，降低被站点反爬拦截的概率
- 全局刷新间隔：移除 15 分钟选项，新增 2 小时

### 修复
- 修复摘要类订阅源多条条目共用同一链接时只保存部分文章的问题

### New Features
- Multi-select batch deletion of feeds in the sidebar, plus a context-menu action to delete all errored feeds at once
- Import subscriptions from a remote OPML URL; bulk import is now transactional with progress events, and the sidebar merges results incrementally during import
- Added in-app toast notifications; backend errors now surface as readable toasts that stack and expand on hover
- A per-feed spinner shows while that feed is refreshing
- Feed count shown next to the sidebar header title
- HTTP requests now masquerade as browser traffic to reduce blocking by anti-bot protection
- Refresh interval options: removed 15 minutes, added 2 hours

### Bug Fixes
- Fixed digest feeds storing only some entries when multiple entries share the same URL

### 新增
- 側欄支援多選批次刪除訂閱源，右鍵選單新增「刪除全部出錯訂閱源」
- 支援從遠端 OPML 連結匯入訂閱；批次匯入改為交易式並顯示進度，過程中側欄逐步合併結果
- 新增應用內 toast 通知；後端錯誤改為以可讀文字彈出，多條提示堆疊顯示、懸停展開
- 重新整理時對應訂閱源顯示載入動畫
- 側欄標題旁顯示訂閱源數量
- 抓取請求偽裝為瀏覽器流量，降低被網站反爬攔截的機率
- 全域重新整理間隔：移除 15 分鐘選項，新增 2 小時

### 修復
- 修復摘要類訂閱源多條條目共用同一連結時只儲存部分文章的問題

## 0.3.0

#### 2026-08-23

### 新增
- 更新下载失败时可改用浏览器下载安装包
- 无新版时更新日志读取本地缓存

### 修复
- 修复慢速网络下软件更新几乎必然下载失败的问题，现已支持断点续传与自动重试
- 修复代理设置对软件更新与更新日志抓取不生效的问题
- 修复更新失败时提示显示为内部错误代码而非可读文案的问题
- 修复更新日志显示为 Markdown 原文而非渲染后内容的问题

### New Features
- Added a browser fallback for downloading the installer when an update download fails
- Added local caching of the changelog when no update is available

### Bug Fixes
- Fixed update downloads almost always failing on slow networks; resumable downloads and automatic retries are now supported
- Fixed the proxy setting not applying to software updates or changelog fetches
- Fixed update failures showing an internal error code instead of a readable message
- Fixed the changelog rendering as raw Markdown instead of formatted text

### 新增
- 更新下載失敗時可改用瀏覽器下載安裝套件
- 無新版時更新日志讀取本機快取

### 修復
- 修復慢速網路下軟體更新幾乎必然下載失敗的問題，現已支援續傳與自動重試
- 修復代理伺服器設定對軟體更新與更新日志抓取不生效的問題
- 修復更新失敗時提示顯示為內部錯誤代碼而非可讀文案的問題
- 修復更新日志顯示為 Markdown 原文而非轉譯後內容的問題

## 0.2.0

#### 2026-08-21

### 新增
- 新增下载文章内图片
- 新增显示文章主题标签
- 新增历史搜索关键词
- 新增繁体中文

### 修复
- 修复某些站点（如 36kr）因反爬验证导致订阅无法更新的问题
- 修复图片查看器按钮的提示在英文界面下显示中文的问题

### 优化
- 统一了部分图标的风格
- 优化了部分 UI 样式

### New Features
- Added image download from articles
- Added article topic tags
- Added recent search keyword history
- Added Traditional Chinese

### Bug Fixes
- Fixed subscriptions on some sites (e.g. 36kr) failing to update due to anti-bot verification
- Fixed image viewer button tooltips showing Chinese text in the English UI

### Improvements
- Unified the style of some icons
- Polished some UI styles

### 新增
- 新增下載文章內圖片
- 新增顯示文章主題標籤
- 新增歷史搜尋關鍵詞
- 新增繁體中文

### 修復
- 修復某些站點（如 36kr）因反爬蟲驗證導致訂閱無法更新的問題
- 修復圖片檢視器按鈕的提示在英文介面下顯示中文的問題

### 優化
- 統一了部分圖示的風格
- 優化了部分 UI 樣式

## 0.1.0

#### 2026-08-12

First release

