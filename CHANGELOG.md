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

