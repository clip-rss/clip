package fetcher

import (
	"errors"
	"time"
)

// ErrUnknownFormat 表示无法识别的 Feed 格式（既非 RSS 也非 Atom）。
var ErrUnknownFormat = errors.New("fetcher: unknown feed format")

// ParsedFeed 统一的 Feed 数据模型，兼容 RSS 2.0 与 Atom 的字段差异。
type ParsedFeed struct {
	Title       string       // 频道标题
	Description string       // 频道描述（RSS description / Atom subtitle）
	Link        string       // 站点主页链接
	FeedLink    string       // Feed 自身链接（Atom rel="self"）
	Updated     time.Time    // 频道最后更新时间
	Items       []ParsedItem // 文章条目
}

// ParsedItem 统一的文章条目模型。
type ParsedItem struct {
	GUID       string    // 全局唯一标识（RSS guid / Atom id），可能为空
	Title      string    // 标题
	Link       string    // 文章链接
	Author     string    // 作者
	Published  time.Time // 发布时间（可能为零值）
	Updated    time.Time // 更新时间（可能为零值）
	Content    string    // 正文 HTML（已清洗）
	Summary    string    // 摘要（纯文本）
	Enclosure  string    // 附件 URL（音频/视频）
	Categories []string  // 分类标签
}
