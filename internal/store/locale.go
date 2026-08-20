package store

import "strings"

var systemLocale = detectSystemLocale

func detectDefaultLanguage() string {
	if isTraditionalChineseLocale(systemLocale()) {
		return "zh-TW"
	}
	if isChineseLocale(systemLocale()) {
		return "zh"
	}
	return "en"
}

func isChineseLocale(locale string) bool {
	normalized := normalizeSystemLocale(locale)
	return normalized == "zh" || strings.HasPrefix(normalized, "zh-") && !isTraditionalChineseNormalized(normalized)
}

func isTraditionalChineseLocale(locale string) bool {
	normalized := normalizeSystemLocale(locale)
	return isTraditionalChineseNormalized(normalized)
}

func normalizeSystemLocale(locale string) string {
	normalized := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(locale, "_", "-")))
	if idx := strings.IndexAny(normalized, ".@"); idx >= 0 {
		normalized = normalized[:idx]
	}
	return normalized
}

func isTraditionalChineseNormalized(normalized string) bool {
	return normalized == "zh-tw" || strings.HasPrefix(normalized, "zh-tw-") ||
		normalized == "zh-hk" || strings.HasPrefix(normalized, "zh-hk-") ||
		normalized == "zh-mo" || strings.HasPrefix(normalized, "zh-mo-") ||
		strings.HasPrefix(normalized, "zh-hant")
}
