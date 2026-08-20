package store

import "testing"

func TestDetectDefaultLanguageTraditionalChinese(t *testing.T) {
	for _, locale := range []string{"zh_TW.UTF-8", "zh-Hant-TW", "zh-HK", "zh-MO"} {
		restore := stubSystemLocale(t, locale)
		if got := detectDefaultLanguage(); got != "zh-TW" {
			t.Errorf("detectDefaultLanguage(%q) = %q, want zh-TW", locale, got)
		}
		restore()
	}
}

func TestDetectDefaultLanguageSimplifiedChineseUnchanged(t *testing.T) {
	for _, locale := range []string{"zh", "zh_CN.UTF-8", "zh-Hans-SG"} {
		restore := stubSystemLocale(t, locale)
		if got := detectDefaultLanguage(); got != "zh" {
			t.Errorf("detectDefaultLanguage(%q) = %q, want zh", locale, got)
		}
		restore()
	}
}
