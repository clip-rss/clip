package api

import (
	"sort"
	"testing"

	"github.com/clip-rss/clip/internal/store"
)

func TestChangedSettingsFields(t *testing.T) {
	base := store.Settings{Theme: "dark", Language: "en"}

	// 无变化 → 空切片。
	if got := changedSettingsFields(base, base); len(got) != 0 {
		t.Errorf("no change should yield empty, got %v", got)
	}

	// 改两个字段 → 返回其 json tag 名（只记名字，不记值）。
	changed := base
	changed.Language = "zh"
	changed.ProxyHost = "127.0.0.1"
	got := changedSettingsFields(base, changed)
	sort.Strings(got)
	want := []string{"language", "proxyHost"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("changedSettingsFields = %v, want %v", got, want)
	}
}
