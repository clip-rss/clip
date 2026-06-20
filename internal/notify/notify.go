// Package notify 负责新文章系统原生通知的决策与发送。
//
// 核心纯函数 Plan 按通知模式将新文章清单转换为若干条通知消息，
// 便于单元测试验证决策逻辑；Service 读取设置并协调发送。
package notify

import (
	"context"
	"fmt"

	"github.com/clip-rss/clip/internal/scheduler"
	"github.com/clip-rss/clip/internal/store"
)

// 通知模式常量（与 store 中定义一致）。
const (
	ModeEach    = store.NotifyEach
	ModeSummary = store.NotifySummary
	ModeOff     = store.NotifyOff
)

// 摘要阈值：单次刷新新文章超过此数时，each 模式也会合并为摘要。
const summaryThreshold = 5

// Message 一条待发送的通知。
type Message struct {
	ID    string // 通知标识，如 "article:42"；点击回调借此定位文章
	Title string
	Body  string
}

// Sender 发送系统通知的能力（生产由 Wails notifications 服务实现）。
type Sender interface {
	Send(msg Message) error
}

// SettingsProvider 读取当前通知偏好（由 store 实现）。
type SettingsProvider interface {
	GetSettings() (store.Settings, error)
}

// Plan 根据通知模式、源标题与新建文章列表决出待发消息。
//
// 规则：
//   - off    → 空
//   - summary → 一条摘要消息，格式 “《源标题》新增 N 篇: title1, title2...”
//   - each   → 每篇一条，但若单次 > summaryThreshold 篇则自动合并为摘要
func Plan(mode string, feedTitle string, items []scheduler.NewItem) []Message {
	if mode == ModeOff || mode == "" || len(items) == 0 {
		return nil
	}
	if mode == ModeSummary || (mode == ModeEach && len(items) > summaryThreshold) {
		return []Message{summaryMsg(feedTitle, items)}
	}
	return eachMsgs(feedTitle, items)
}

func summaryMsg(feedTitle string, items []scheduler.NewItem) Message {
	id := items[0].ID
	titles := make([]string, len(items))
	for i, it := range items {
		titles[i] = it.Title
	}
	return Message{
		ID:    fmt.Sprintf("article:%d", id),
		Title: fmt.Sprintf("%s 新增 %d 篇", feedTitle, len(items)),
		Body:  joinTitles(titles, 3),
	}
}

func eachMsgs(feedTitle string, items []scheduler.NewItem) []Message {
	msgs := make([]Message, len(items))
	for i, it := range items {
		msgs[i] = Message{
			ID:    fmt.Sprintf("article:%d", it.ID),
			Title: feedTitle,
			Body:  it.Title,
		}
	}
	return msgs
}

// joinTitles 拼接标题列表，最多展示前 n 条，超出加省略。
func joinTitles(titles []string, max int) string {
	n := len(titles)
	if n > max {
		return fmt.Sprintf("%s… 等 %d 篇", joinFirst(titles[:max], "、"), n)
	}
	return joinFirst(titles, "、")
}

func joinFirst(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for i := 1; i < len(ss); i++ {
		out += sep + ss[i]
	}
	return out
}

// Service 通知服务：读取设置并根据计划发送。
type Service struct {
	settings SettingsProvider
	sender   Sender
}

// NewService 创建通知服务。
func NewService(sp SettingsProvider, sd Sender) *Service {
	return &Service{settings: sp, sender: sd}
}

// Notify 实现 scheduler.Notifier 接口。
func (s *Service) Notify(ctx context.Context, feed store.Feed, items []scheduler.NewItem) {
	cfg, err := s.settings.GetSettings()
	if err != nil {
		return
	}
	msgs := Plan(cfg.NotificationMode, feed.Title, items)
	for _, msg := range msgs {
		_ = s.sender.Send(msg)
	}
}
