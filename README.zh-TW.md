<div align="center">

<img src="applogo.png" alt="Clip Logo" width="128" />

# Clip

**簡單好用的跨平台 RSS 閱讀器** —— 支援 macOS / Windows，專注於快速瀏覽與閱讀。

<img alt="Static Badge" src="https://img.shields.io/badge/Go-1.25.0-blue.svg"> <img alt="Static Badge" src="https://img.shields.io/badge/Wails-v3.0.0--alpha.98-blue.svg"> <img alt="Static Badge" src="https://img.shields.io/badge/React-18.2-blue.svg"> <img alt="Static Badge" src="https://img.shields.io/badge/TypeScript-6.0-blue.svg"> <img alt="Static Badge" src="https://img.shields.io/badge/Vite-8.0-blue.svg"> <img alt="Static Badge" src="https://img.shields.io/badge/Tailwind%20CSS-4.0-blue.svg"> <img alt="Static Badge" src="https://img.shields.io/badge/SQLite-1.52%20%28modernc%29-blue.svg">

[简体中文](README.zh.md) | 繁體中文 | [English](README.md)

</div>

## 螢幕截圖

<div align="center">

<img src="screenshot/screenshot_1.png" alt="Clip 螢幕截圖 1" width="49%" />
<img src="screenshot/screenshot_2.png" alt="Clip 螢幕截圖 2" width="49%" />

</div>

## 主要功能

- **三欄式佈局**：左側來源／資料夾樹狀清單 · 中間文章清單 · 右側閱讀檢視
- **訂閱管理**：新增訂閱來源、分類整理、OPML 匯入匯出
- **定時抓取**：RSS / Atom 解析，背景定時自動更新
- **專注閱讀**：全文閱讀檢視 + 無干擾專注模式
- **快速搜尋**：涵蓋標題、摘要與筆記的全域全文搜尋
- **筆記記錄**：為文章加上筆記，隨時回顧
- **桌面通知**：新文章到達提醒，支援 macOS Dock / Windows 工作列圖示標記
- **鍵盤驅動**：完善的快速鍵系統
- **主題與國際化**：淺色 / 深色 / 跟隨系統，支援多語言
- **效能與離線**：輕量清單查詢、本機快取、離線可讀

## 從原始碼建置

### 1. 準備開發環境

| 相依套件 | 版本要求 | 安裝方式 |
|:---------|:---------|:---------|
| Go | ≥ 1.25 | https://go.dev/dl/ |
| Node.js | ≥ 20 | https://nodejs.org/ |
| pnpm | ≥ 9 | `npm install -g pnpm` |
| Wails v3 CLI | latest | `go install github.com/wailsapp/wails/v3/cmd/wails3@latest` |

平台額外要求：

- **macOS**：安裝 Xcode Command Line Tools —— `xcode-select --install`
- **Windows**：確認已安裝 WebView2 Runtime（Windows 11 已預先安裝）

### 2. 複製儲存庫並安裝相依套件

```bash
# 複製儲存庫
git clone https://github.com/clip-rss/clip.git
cd clip

# 安裝前端相依套件
cd frontend
pnpm install
cd ..

# 拉取 Go 相依套件
go mod tidy
```

### 3. 開發模式（熱重載）

```bash
wails3 task dev
```

> 啟動 Vite 開發伺服器與 Go 後端，修改程式碼即時生效。

### 4. 編譯與打包

```bash
# 編譯目前平台的執行檔
wails3 task build

# 打包成可散布的安裝檔（macOS 產生 .app / Windows 產生安裝程式）
wails3 task package
```

產物輸出到 `bin/` 目錄。

### 5. 測試

```bash
# Go 後端測試
go test ./...

# 前端型別檢查 + 單元測試
cd frontend
pnpm typecheck
pnpm test
```

### 補充：同步前端繫結

修改 `api/` 下暴露給前端的繫結方法或 Go 模型後，需重新產生前端 TS 繫結：

```bash
wails3 generate bindings
```

不要手動修改 `frontend/bindings/` 下的檔案。

## LICENSE

[MIT](LICENSE)
