package notify

import (
	"errors"
	"strings"
	"testing"

	"github.com/clip-rss/clip/internal/scheduler"
	"github.com/clip-rss/clip/internal/store"
)

type fakeSettingsProvider struct {
	settings store.Settings
	err      error
}

func (p fakeSettingsProvider) GetSettings() (store.Settings, error) {
	return p.settings, p.err
}

type fakeSender struct {
	err error
}

func (s fakeSender) Send(Message) error { return s.err }

func items(n int) []scheduler.NewItem {
	out := make([]scheduler.NewItem, n)
	for i := 0; i < n; i++ {
		out[i] = scheduler.NewItem{ID: int64(i + 1), Title: "文章标题" + strings.Repeat("A", i%3+1)}
	}
	return out
}

func TestPlanOff(t *testing.T) {
	msgs := Plan(ModeOff, "科技", items(3))
	if len(msgs) != 0 {
		t.Errorf("off mode should return 0 messages, got %d", len(msgs))
	}
}

func TestPlanEmpty(t *testing.T) {
	msgs := Plan(ModeEach, "科技", nil)
	if len(msgs) != 0 {
		t.Errorf("empty items should return 0, got %d", len(msgs))
	}
}

func TestPlanEach(t *testing.T) {
	msgs := Plan(ModeEach, "科技", items(3))
	if len(msgs) != 3 {
		t.Fatalf("each mode 3 items → 3 msgs, got %d", len(msgs))
	}
	for i, m := range msgs {
		if m.Title != "科技" {
			t.Errorf("msg[%d].Title = %q, want 科技", i, m.Title)
		}
		if !strings.Contains(m.ID, "article:") {
			t.Errorf("msg[%d].ID = %q, want article: prefix", i, m.ID)
		}
	}
}

func TestPlanEachThreshold(t *testing.T) {
	// >5 篇 → 自动合并为摘要
	msgs := Plan(ModeEach, "科技", items(6))
	if len(msgs) != 1 {
		t.Fatalf("each with 6 items → 1 summary, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0].Body, "等 6 篇") {
		t.Errorf("body should mention 等 6 篇, got %q", msgs[0].Body)
	}
	if strings.Contains(msgs[0].Body, "文章标题A") {
		// 正文含标题仍 OK，这里不做格式强校验
	}
}

func TestPlanSummary(t *testing.T) {
	msgs := Plan(ModeSummary, "科技", items(2))
	if len(msgs) != 1 {
		t.Fatalf("summary mode → 1 msg, got %d", len(msgs))
	}
	m := msgs[0]
	if m.Title != "科技 新增 2 篇" {
		t.Errorf("title = %q, want 科技 新增 2 篇", m.Title)
	}
	if !strings.Contains(m.Body, "文章标题A") {
		t.Errorf("body should contain item title, got %q", m.Body)
	}
}

func TestPlanLocalizedEnglish(t *testing.T) {
	msgs := PlanLocalized(ModeSummary, "en", "Tech", items(2))
	if len(msgs) != 1 || msgs[0].Title != "Tech: 2 new items" {
		t.Fatalf("english summary = %+v", msgs)
	}
}

func TestJoinTitles(t *testing.T) {
	in := []string{"a", "b", "c", "d", "e"}
	got := joinTitles(in, 3)
	if !strings.Contains(got, "等 5 篇") {
		t.Errorf("should show 等 5 篇, got %q", got)
	}
	short := joinTitles([]string{"x", "y"}, 3)
	if strings.Contains(short, "等") {
		t.Errorf("should not truncate 2 items, got %q", short)
	}
}

func TestServiceReportsSettingsAndSendErrors(t *testing.T) {
	t.Run("settings", func(t *testing.T) {
		want := errors.New("settings unavailable")
		svc := NewService(fakeSettingsProvider{err: want}, fakeSender{})
		var got error
		svc.reportError = func(err error) { got = err }

		svc.Notify(t.Context(), store.Feed{Title: "feed"}, items(1))

		if !errors.Is(got, want) {
			t.Fatalf("reported error = %v, want settings error", got)
		}
	})

	t.Run("send", func(t *testing.T) {
		want := errors.New("notification rejected")
		svc := NewService(
			fakeSettingsProvider{settings: store.Settings{NotificationMode: ModeEach}},
			fakeSender{err: want},
		)
		var got error
		svc.reportError = func(err error) { got = err }

		svc.Notify(t.Context(), store.Feed{Title: "feed"}, items(1))

		if !errors.Is(got, want) {
			t.Fatalf("reported error = %v, want send error", got)
		}
	})
}
