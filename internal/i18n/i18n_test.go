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
		{"zh_Hant_TW", English},
		{"zh-HK", English},
		{"en-US", English},
		{"ja-JP", English},
		{"", English},
	} {
		if got := Normalize(tt.input); got != tt.want {
			t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
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
}
