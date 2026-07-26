package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestUpdaterI18nInjection 验证更新窗口的 i18n 注入：占位符被替换、字典是合法 JSON、
// 且 en/zh 两语言的 "updater" key 集合一致（防止本地化漏项 / locale 结构漂移）。
func TestUpdaterI18nInjection(t *testing.T) {
	dict := updaterI18nDict()

	// 字典应是 {"en":{...},"zh":{...}} 的合法 JSON。
	var parsed map[string]map[string]string
	if err := json.Unmarshal([]byte(dict), &parsed); err != nil {
		t.Fatalf("dict 不是合法 JSON: %v", err)
	}
	en, ok := parsed["en"]
	if !ok || len(en) == 0 {
		t.Fatal("dict 缺少非空 en 段")
	}
	zh, ok := parsed["zh"]
	if !ok || len(zh) == 0 {
		t.Fatal("dict 缺少非空 zh 段")
	}

	// en/zh 的 key 必须一致。
	for k := range en {
		if _, ok := zh[k]; !ok {
			t.Errorf("zh 缺少 key %q", k)
		}
	}
	for k := range zh {
		if _, ok := en[k]; !ok {
			t.Errorf("en 缺少 key %q", k)
		}
	}

	// 注入后占位符应全部消失，且选定语言出现在结果中。
	html := buildSoftwareUpdateHTML(dict, "zh")
	if strings.Contains(html, updI18nDictMarker) {
		t.Errorf("注入后仍残留字典占位符 %s", updI18nDictMarker)
	}
	if strings.Contains(html, updI18nLangMarker) {
		t.Errorf("注入后仍残留语言占位符 %s", updI18nLangMarker)
	}
	if !strings.Contains(html, `var lang = "zh"`) {
		t.Error("注入后未见 var lang = \"zh\"")
	}
	// 抽查一个已知 key 的中文文案确实进入了 HTML。
	if want := zh["skipVersion"]; want != "" && !strings.Contains(html, want) {
		t.Errorf("注入后未见 zh.skipVersion 文案 %q", want)
	}
}
