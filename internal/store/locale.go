package store

import "strings"

var systemLocale = detectSystemLocale

func detectDefaultLanguage() string {
	if isChineseLocale(systemLocale()) {
		return "zh"
	}
	return "en"
}

func isChineseLocale(locale string) bool {
	normalized := strings.ToLower(strings.TrimSpace(locale))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	return normalized == "zh" || strings.HasPrefix(normalized, "zh-")
}
