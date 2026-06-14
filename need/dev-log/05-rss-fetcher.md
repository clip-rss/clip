# 阶段 5：RSS 抓取与解析

## 概述
实现 RSS/Atom Feed 的 HTTP 抓取、解析、去重、条件 GET 等核心逻辑。

## 步骤清单

- [x] RSS 2.0 解析器实现
- [x] Atom 解析器实现
- [x] 统一 Feed 数据模型（兼容 RSS/Atom 差异）
- [x] HTTP 客户端封装（超时、重试、User-Agent）
- [x] 条件 GET 支持（`ETag` / `If-Modified-Since` 头）
- [x] Feed URL 自动发现（解析 HTML `<link rel="alternate">` 标签）
- [x] 文章去重逻辑（guid + publishedAt；无 guid 时用 link + title 指纹）
- [x] HTML 内容清洗（防 XSS，移除脚本/样式标签）
- [x] 摘要生成（纯文本前 200 字）
- [x] Favicon 提取与缓存
- [x] 并发控制（最多 5 个协程同时抓取）
- [x] 错误处理与智能退避（连续失败指数退避，最多 24h）
- [x] 单元测试：解析各种 Feed 格式
- [x] 单元测试：去重逻辑
- [x] 单元测试：条件 GET

## 验收标准
- 可正确解析主流 RSS 2.0 和 Atom Feed
- 条件 GET 减少不必要带宽
- 去重无误，无重复文章入库
- 并发安全，无 race condition
