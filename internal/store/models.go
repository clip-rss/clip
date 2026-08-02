package store

import "time"

// Feed RSS/Atom 订阅源
type Feed struct {
	ID             int64      `json:"id"`
	URL            string     `json:"url"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Link           string     `json:"link"`
	Icon           string     `json:"icon"`           // favicon URL
	CategoryID     *int64     `json:"categoryId"`     // 可选分类
	UpdateInterval int        `json:"updateInterval"` // 更新间隔（分钟）
	MaxItems       int        `json:"maxItems"`       // 最大保留条目数
	LastUpdated    *time.Time `json:"lastUpdated"`    // 可为 NULL
	LastAttempted  *time.Time `json:"-"`              // 最近一次抓取尝试时间（成功或失败）
	ErrorCount     int        `json:"errorCount"`     // 连续错误次数
	LastError      *string    `json:"lastError"`      // 可为 NULL
	Status         string     `json:"status"`         // active/paused/error
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// RefreshItem 是一次 Feed 刷新准备写入的文章及其稳定去重键。
// Keys 同时包含源提供的 GUID 指纹和 URL 别名，用于避免已清理旧文被再次视为新文。
type RefreshItem struct {
	Item *Item
	Keys []string
}

// Item RSS 文章条目
type Item struct {
	ID          int64      `json:"id"`
	FeedID      int64      `json:"feedId"`
	Title       string     `json:"title"`
	Author      string     `json:"author"`
	PublishedAt time.Time  `json:"publishedAt"`
	UpdatedAt   *time.Time `json:"updatedAt"` // 可为 NULL
	URL         string     `json:"url"`
	Content     string     `json:"content"`    // 完整内容
	Summary     string     `json:"summary"`    // 摘要
	Enclosure   string     `json:"enclosure"`  // 附件 URL（音频/视频）
	Categories  string     `json:"categories"` // JSON 数组字符串
	IsRead      bool       `json:"isRead"`
	IsStarred   bool       `json:"isStarred"`
	ReadAt      *time.Time `json:"readAt"` // 可为 NULL
	Note        string     `json:"note"`   // 用户笔记
	CreatedAt   time.Time  `json:"createdAt"`
}

// Category 订阅源分类（支持树形结构）
type Category struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	ParentID  *int64    `json:"parentId"`  // null 表示根分类
	SortOrder int       `json:"sortOrder"` // 排序权重
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// FeedWithUnread Feed 附带未读计数
type FeedWithUnread struct {
	Feed
	UnreadCount int `json:"unreadCount"`
}

// CategoryWithFeeds 分类附带订阅源列表
type CategoryWithFeeds struct {
	Category
	Feeds []FeedWithUnread `json:"feeds"`
}

// ItemLight 文章条目轻量版本（不含 content 字段，用于列表视图）
type ItemLight struct {
	ID          int64      `json:"id"`
	FeedID      int64      `json:"feedId"`
	Title       string     `json:"title"`
	Author      string     `json:"author"`
	PublishedAt time.Time  `json:"publishedAt"`
	UpdatedAt   *time.Time `json:"updatedAt"`
	URL         string     `json:"url"`
	Summary     string     `json:"summary"`
	Enclosure   string     `json:"enclosure"`
	Categories  string     `json:"categories"`
	IsRead      bool       `json:"isRead"`
	IsStarred   bool       `json:"isStarred"`
	ReadAt      *time.Time `json:"readAt"`
	Note        string     `json:"note"`
	CreatedAt   time.Time  `json:"createdAt"`
}
