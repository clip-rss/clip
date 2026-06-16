// Package api 提供 Wails 绑定方法，暴露给前端调用。
//
// 设计约定：
//   - 每个领域一个 Service（FeedService / ItemService / CategoryService /
//     SettingsService / OPMLService），由 main 注入 store/fetcher/scheduler 依赖。
//   - 方法统一返回 (结果, error)：Wails 会把非 nil error 转为前端 Promise 的 reject，
//     前端用 try/catch 即可捕获，无需自定义返回信封。
//   - 入参/返回使用 store 包的模型，经 wails3 generate bindings 生成对应 TS 类型。
package api

import (
	"strings"

	"github.com/clip-rss/clip/internal/scheduler"
)

// RefreshOutcome 单个订阅源刷新结果（对前端友好的可序列化形态）。
type RefreshOutcome struct {
	FeedID      int64  `json:"feedId"`
	NewItems    int    `json:"newItems"`
	NotModified bool   `json:"notModified"`
	Error       string `json:"error"` // 该源抓取失败时的错误信息，空表示成功
}

// toOutcome 将调度器结果转换为可序列化结果。
func toOutcome(r scheduler.RefreshResult) RefreshOutcome {
	o := RefreshOutcome{
		FeedID:      r.FeedID,
		NewItems:    r.NewItems,
		NotModified: r.NotModified,
	}
	if r.Err != nil {
		o.Error = r.Err.Error()
	}
	return o
}

func toOutcomes(rs []scheduler.RefreshResult) []RefreshOutcome {
	out := make([]RefreshOutcome, len(rs))
	for i, r := range rs {
		out[i] = toOutcome(r)
	}
	return out
}

// firstNonEmpty 返回首个去空白后非空的字符串。
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

// nullableID 将前端传入的 ID 转为可空指针：0 表示“无/根”。
func nullableID(id int64) *int64 {
	if id <= 0 {
		return nil
	}
	return &id
}
