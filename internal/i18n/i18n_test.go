package i18n

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	for _, tt := range []struct {
		input, want string
	}{
		{"zh", SimplifiedChinese},
		{"zh-CN", SimplifiedChinese},
		{"zh_Hans_SG", SimplifiedChinese},
		{"zh_Hant_TW", TraditionalChinese},
		{"zh-hant-tw", TraditionalChinese},
		{"ZH_tw", TraditionalChinese},
		{"zh_TW.UTF-8", TraditionalChinese},
		{"zh-HK", TraditionalChinese},
		{"zh-MO-x-private", TraditionalChinese},
		{"en-US", English},
		{"ja-JP", English},
		{"", English},
	} {
		if got := Normalize(tt.input); got != tt.want {
			t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTraditionalChineseUsesCompleteCatalog(t *testing.T) {
	if got := T(TraditionalChinese, "updater.title"); got != "軟體更新" {
		t.Fatalf("traditional updater title = %q", got)
	}
	if got := T(TraditionalChinese, "webdav.urlEmpty"); got == T(English, "webdav.urlEmpty") {
		t.Fatalf("traditional translation unexpectedly fell back to English: %q", got)
	}
	if len(messages[TraditionalChinese]) != len(messages[SimplifiedChinese]) {
		t.Fatalf("traditional catalog has %d keys, simplified has %d", len(messages[TraditionalChinese]), len(messages[SimplifiedChinese]))
	}
}

func TestTranslationsAndErrorCause(t *testing.T) {
	if got := T(English, "feed.alreadyExists", "https://example.test/feed"); !strings.Contains(got, "already subscribed") {
		t.Fatalf("english translation = %q", got)
	}
	cause := errors.New("connection refused")
	err := Error(English, "feed.fetchFailed", cause)
	if !strings.HasPrefix(err.Error(), "Failed to fetch feed:") || !errors.Is(err, cause) {
		t.Fatalf("localized error = %q, errors.Is = %v", err, errors.Is(err, cause))
	}
}

func TestLocalizeErrorKeepsUnknownDiagnostics(t *testing.T) {
	cause := errors.New("服务器地址格式错误")
	err := LocalizeError(English, cause)
	if err.Error() != "Invalid server address" {
		t.Errorf("localized = %q", err)
	}
	unknown := errors.New("some provider detail")
	if got := LocalizeError(English, unknown); got != unknown {
		t.Errorf("unknown error should be returned unchanged")
	}
	webdavErr := errors.New("webdav: config: 服务器返回的内容无法解析，请确认地址指向 WebDAV 路径")
	if got := LocalizeError(English, webdavErr).Error(); got != "webdav: config: The server response could not be parsed; make sure the address points to a WebDAV path" {
		t.Errorf("long phrase translation = %q", got)
	}
	traditional := LocalizeError(TraditionalChinese, errors.New("服务器地址格式错误"))
	if traditional.Error() != "伺服器位址格式錯誤" {
		t.Errorf("traditional phrase translation = %q", traditional)
	}
}
