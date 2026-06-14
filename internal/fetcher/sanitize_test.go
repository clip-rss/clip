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
		`<img src="https://ok.com/a.png" alt="a" style="x">`

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
