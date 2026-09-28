package fetcher

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
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

func TestSanitizeKeepsImgSrcset(t *testing.T) {
	// 响应式图片的候选地址必须留着：前端要逐个改写成代理地址，浏览器才不会从
	// srcset 里挑一个未代理的候选去加载（那样防盗链照样 403）。
	in := `<img src="https://ok.com/a.png" srcset="https://ok.com/a.png 1x, https://ok.com/a@2x.png 2x" alt="a">`

	out := Sanitize(in)

	if !strings.Contains(out, `srcset="https://ok.com/a.png 1x, https://ok.com/a@2x.png 2x"`) {
		t.Errorf("img 的 srcset 应被保留：%q", out)
	}
}

func TestSanitizeKeepsID(t *testing.T) {
	// 正文里的锚点靠 id 落点：站点给标题/引用目标带 id，`<a href="#…">` 才跳得过去。
	// 剥掉 id 会让阅读视图里的目录项与引用链接点了毫无反应（issue #7）。
	in := `<h2 id="570-个漏洞都有哪些？">570 个漏洞都有哪些？</h2>` +
		`<a href="#570-个漏洞都有哪些？">570 个漏洞都有哪些？</a>` +
		`<a id="cite:rfc9114">[7]</a>` +
		`<div id="wrapper"><span id="note">x</span></div>`

	out := Sanitize(in)

	// 标题与引用目标：既有逐标签白名单里的 h2/a，也有原本 allowed==nil 的 div/span
	for _, want := range []string{
		`id="570-个漏洞都有哪些？"`,
		`id="cite:rfc9114"`,
		`id="wrapper"`,
		`id="note"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("应保留 %s：%q", want, out)
		}
	}
	if !strings.Contains(out, `href="#570-个漏洞都有哪些？"`) {
		t.Errorf("锚点链接本身应保留：%q", out)
	}
}

func TestSanitizeIDDoesNotSmuggleAnything(t *testing.T) {
	// 放行 id 不能顺手放过同元素上的事件处理器/style，也不能让 id 的值在输出里
	// 撑开一个新属性（值里带引号的情形）。
	in := `<h2 id="x" onclick="evil()" style="color:red">t</h2>` +
		`<a id="a&quot; onmouseover=&quot;evil()" href="https://ok.com">l</a>`

	out := Sanitize(in)

	if !strings.Contains(out, `id="x"`) {
		t.Errorf("id 应保留：%q", out)
	}
	// 把输出重新解析一遍：只要 DOM 里多出一个 on*/style 属性就说明逃逸了。
	// （不能直接对字符串断言 onmouseover —— 它会作为 id 的值被转义后留在正文里。）
	doc, err := html.Parse(strings.NewReader(out))
	if err != nil {
		t.Fatalf("输出应仍是可解析的 HTML: %v", err)
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, a := range n.Attr {
				k := strings.ToLower(a.Key)
				if strings.HasPrefix(k, "on") || k == "style" {
					t.Errorf("输出里解析出了 %s 属性：%q", k, out)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
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
