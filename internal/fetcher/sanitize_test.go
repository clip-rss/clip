package fetcher

import (
	"strings"
	"testing"
)

func TestSanitizeRemovesDangerous(t *testing.T) {
	in := `<p>hello</p><script>alert('xss')</script><style>.x{}</style>` +
		`<p onclick="evil()">click</p>` +
		`<a href="javascript:alert(1)">bad</a>` +
		`<a href="https://ok.com" onmouseover="x()">good</a>` +
		`<img src="https://ok.com/a.png" alt="a" style="x">` +
		`<iframe src="https://evil.example/embed/1"></iframe>`

	out := Sanitize(in)

	if strings.Contains(out, "<script") || strings.Contains(out, "alert") {
		t.Errorf("script not removed: %q", out)
	}
	if strings.Contains(out, "<style") {
		t.Errorf("style not removed: %q", out)
	}
	if strings.Contains(out, "onclick") || strings.Contains(out, "onmouseover") {
		t.Errorf("event handler not removed: %q", out)
	}
	if strings.Contains(out, "javascript:") {
		t.Errorf("javascript URL not removed: %q", out)
	}
	if strings.Contains(out, "evil.example") {
		t.Errorf("untrusted iframe should be removed: %q", out)
	}
	if strings.Contains(out, "style=") {
		t.Errorf("style attribute not removed: %q", out)
	}
	if !strings.Contains(out, `href="https://ok.com"`) {
		t.Errorf("safe link should be kept: %q", out)
	}
	if !strings.Contains(out, `<img`) || !strings.Contains(out, `src="https://ok.com/a.png"`) {
		t.Errorf("safe img should be kept: %q", out)
	}
	if !strings.Contains(out, "click") {
		t.Errorf("text content should be preserved: %q", out)
	}
}

func TestSanitizeAllowsTrustedEmbed(t *testing.T) {
	out := Sanitize(`<iframe src="https://www.youtube.com/embed/demo" allowfullscreen></iframe>`)
	if !strings.Contains(out, `<iframe`) || !strings.Contains(out, `src="https://www.youtube.com/embed/demo"`) {
		t.Errorf("trusted iframe should be preserved: %q", out)
	}
}

func TestSanitizePromotesLazyMediaURL(t *testing.T) {
	input := `<video data-video-src="https://cloudvideo.thepaper.cn/demo.mp4" controls></video>` +
		`<iframe data-src="https://www.thepaper.cn/video/player?vid=1"></iframe>`
	out := Sanitize(input)
	for _, want := range []string{
		`src="https://cloudvideo.thepaper.cn/demo.mp4"`,
		`src="https://www.thepaper.cn/video/player?vid=1"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("lazy media URL missing %q: %s", want, out)
		}
	}
}

func TestSanitizeKeepsLazyMediaContainer(t *testing.T) {
	out := Sanitize(`<div class="video-player" data-video-url="/media/demo.m3u8"></div>`)
	if !strings.Contains(out, `data-video-url="/media/demo.m3u8"`) {
		t.Errorf("lazy media container URL should be retained: %q", out)
	}
}

func TestSanitizeWithBaseResolvesLegacyContent(t *testing.T) {
	out := SanitizeWithBase(
		`<video src="../media/demo.mp4"></video><iframe src="//www.thepaper.cn/video/player"></iframe>`,
		"https://www.thepaper.cn/news/123",
	)
	for _, want := range []string{
		`src="https://www.thepaper.cn/media/demo.mp4"`,
		`src="https://www.thepaper.cn/video/player"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("legacy content missing %q: %s", want, out)
		}
	}
}

func TestStripTags(t *testing.T) {
	in := `<p>Hello   <b>World</b></p><script>ignore()</script>`
	got := StripTags(in)
	if got != "Hello World" {
		t.Errorf("StripTags = %q, want %q", got, "Hello World")
	}
}

func TestSummarize(t *testing.T) {
	long := "<p>" + strings.Repeat("a", 250) + "</p>"
	got := Summarize(long, 200)
	// 200 个字符 + 省略号
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis suffix: %q", got)
	}
	if n := len([]rune(strings.TrimSuffix(got, "…"))); n != 200 {
		t.Errorf("summary length = %d, want 200", n)
	}

	short := Summarize("<p>tiny</p>", 200)
	if short != "tiny" {
		t.Errorf("short summary = %q", short)
	}
}

func TestSummarizeRuneSafe(t *testing.T) {
	// 多字节字符不应被截断为半个。
	in := strings.Repeat("中", 10)
	got := Summarize(in, 5)
	if []rune(strings.TrimSuffix(got, "…"))[0] != '中' {
		t.Errorf("multibyte truncation broken: %q", got)
	}
	if n := len([]rune(strings.TrimSuffix(got, "…"))); n != 5 {
		t.Errorf("rune length = %d, want 5", n)
	}
}
