package api

import (
	"strings"
	"testing"

	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/scheduler"
)

func TestBackendPromptsFollowSettingsLanguage(t *testing.T) {
	st := newTestStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.Language = "en"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	ft := fetcher.New()
	feedService := NewFeedService(st, ft, scheduler.New(st, ft))
	if _, err := feedService.PreviewFeed(" "); err == nil || err.Error() != "Feed URL is required" {
		t.Fatalf("english feed prompt = %v", err)
	}
	if _, err := NewCategoryService(st).AddCategory(" ", 0); err == nil || err.Error() != "Category name is required" {
		t.Fatalf("english category prompt = %v", err)
	}
	if _, err := NewOPMLService(st).ImportOPML(" "); err == nil || err.Error() != "OPML content is empty" {
		t.Fatalf("english OPML prompt = %v", err)
	}
	if _, err := NewWebDAVConfigService(st, nil).GetWebDAVConfig(); err == nil || !strings.HasPrefix(err.Error(), "Credential storage is unavailable") {
		t.Fatalf("english WebDAV prompt = %v", err)
	}

	settings.Language = "zh"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := feedService.PreviewFeed(" "); err == nil || err.Error() != "订阅地址不能为空" {
		t.Fatalf("chinese feed prompt = %v", err)
	}
}

func TestSystemPromptUsesCurrentLanguage(t *testing.T) {
	lang := "en"
	svc := &SystemService{LanguageFn: func() string { return lang }}
	if _, err := svc.FetchChangelog(); err == nil || err.Error() != "Changelog URL is not configured" {
		t.Fatalf("english changelog prompt = %v", err)
	}
	lang = "zh"
	if _, err := svc.FetchChangelog(); err == nil || err.Error() != "未配置更新日志地址" {
		t.Fatalf("chinese changelog prompt = %v", err)
	}
}
