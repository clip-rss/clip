# 阶段 4：Go 后端数据层

## 概述
实现 SQLite 数据库初始化、表设计、基础 CRUD 操作。

## 步骤清单

- [x] 选定 SQLite 库（modernc.org/sqlite 纯 Go，无 CGO）
- [x] 数据库初始化与迁移机制
- [x] `feeds` 表创建（id, url, title, description, link, icon, category, updateInterval, maxItems, lastUpdated, errorCount, lastError, status）
- [x] `items` 表创建（id, feedId, title, author, publishedAt, updatedAt, url, content, summary, enclosure, categories, isRead, isStarred, readAt, note）
- [x] `feed_categories` 表创建（支持树形嵌套分类）
- [x] FTS5 全文搜索索引配置（文章标题、摘要、笔记）
- [x] Feed CRUD 操作封装
- [x] Item CRUD 操作封装
- [x] Category CRUD 操作封装
- [x] 批量标记已读/未读方法
- [x] 星标操作方法
- [x] 数据库连接池配置与关闭处理
- [x] 单元测试：CRUD 操作

## 验收标准
- [x] 数据库文件正确创建在 `os.UserConfigDir` 下
- [x] 所有表结构正确，FTS5 索引可用
- [x] CRUD 操作通过单元测试

## 实现说明

- **驱动**：`modernc.org/sqlite` v1.52.0（纯 Go，无需 CGO，跨平台编译友好）
- **连接配置**：单连接池（SQLite 推荐）+ WAL 模式 + 外键约束 + 5s 忙等待超时
- **数据库位置**：`os.UserConfigDir()/clip/clip.db`
- **NULL 处理**：`LastUpdated`、`UpdatedAt`、`ReadAt`、`LastError` 使用指针类型以正确映射 SQL NULL
- **FTS5 关键点**：外部内容表（external content table）的 UPDATE/DELETE 触发器必须使用 `INSERT INTO items_fts(items_fts, ...) VALUES('delete', ...)` 特殊命令，普通 UPDATE/DELETE 语句会损坏索引（"database disk image is malformed"）
- **文件结构**：
  - `store.go` — Store 初始化、连接池、迁移
  - `models.go` — Feed / Item / Category 数据模型
  - `feeds.go` — Feed CRUD + 状态/错误管理 + 待更新查询
  - `items.go` — Item CRUD + 已读/星标/笔记 + FTS5 搜索 + 旧条目清理
  - `categories.go` — Category 树形 CRUD + 订阅源归类
  - `store_test.go` — 单元测试（5 个测试用例全部通过）
