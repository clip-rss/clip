package fetcher

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// dangerousTags 这些元素及其子树将被整体移除。
var dangerousTags = map[string]bool{
	"script": true, "style": true, "iframe": true, "object": true,
	"embed": true, "applet": true, "form": true, "input": true,
	"textarea": true, "button": true, "select": true, "option": true,
	"link": true, "meta": true, "base": true, "frame": true,
	"frameset": true, "noscript": true, "title": true, "head": true,
	"svg": true, "math": true,
}

// allowedTags 允许保留的元素白名单。
var allowedTags = map[string]bool{
	"p": true, "br": true, "a": true, "img": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "li": true, "blockquote": true, "pre": true,
	"code": true, "strong": true, "em": true, "b": true, "i": true,
	"u": true, "s": true, "strike": true, "span": true, "div": true,
	"table": true, "thead": true, "tbody": true, "tfoot": true, "tr": true,
	"td": true, "th": true, "caption": true, "colgroup": true, "col": true,
	"figure": true, "figcaption": true, "hr": true, "sup": true, "sub": true,
	"dl": true, "dt": true, "dd": true, "abbr": true, "cite": true,
	"kbd": true, "mark": true, "small": true, "time": true,
	"video": true, "audio": true, "source": true, "picture": true,
}

// allowedAttrs 每个标签允许保留的属性。
var allowedAttrs = map[string]map[string]bool{
	"a":      {"href": true, "title": true, "target": true, "rel": true},
	"img":    {"src": true, "alt": true, "title": true, "width": true, "height": true},
	"video":  {"src": true, "controls": true, "width": true, "height": true, "poster": true},
	"audio":  {"src": true, "controls": true},
	"source": {"src": true, "srcset": true, "type": true},
	"time":   {"datetime": true},
	"td":     {"colspan": true, "rowspan": true},
	"th":     {"colspan": true, "rowspan": true},
	"col":    {"span": true},
}

// urlAttrs 需要做协议安全检查的属性。
var urlAttrs = map[string]bool{"href": true, "src": true, "poster": true}

// Sanitize 清洗 HTML：移除脚本/样式等危险元素、事件处理属性与危险协议，
// 仅保留白名单内的标签与属性，以防止 XSS。
func Sanitize(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}

	context := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(input), context)
	if err != nil {
		return StripTags(input)
	}

	var buf bytes.Buffer
	for _, n := range nodes {
		for _, c := range cleanNode(n) {
			_ = html.Render(&buf, c)
		}
	}
	return buf.String()
}

// cleanNode 递归清洗节点，返回应保留的节点列表（可能展开子节点）。
func cleanNode(n *html.Node) []*html.Node {
	switch n.Type {
	case html.TextNode:
		return []*html.Node{{Type: html.TextNode, Data: n.Data}}
	case html.ElementNode:
		// 危险元素：整体丢弃（含子树）。
		if dangerousTags[n.Data] {
			return nil
		}
		children := cleanChildren(n)
		// 非白名单元素：展开，保留其子节点。
		if !allowedTags[n.Data] {
			return children
		}
		el := &html.Node{
			Type:     html.ElementNode,
			Data:     n.Data,
			DataAtom: n.DataAtom,
			Attr:     filterAttrs(n.Data, n.Attr),
		}
		for _, c := range children {
			el.AppendChild(c)
		}
		return []*html.Node{el}
	default:
		// 注释、文档类型等一律丢弃。
		return nil
	}
}

// cleanChildren 清洗并返回脱离父节点的子节点切片。
func cleanChildren(n *html.Node) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out = append(out, cleanNode(c)...)
	}
	return out
}

// filterAttrs 过滤属性：丢弃事件处理器、style 及危险协议，仅保留白名单属性。
func filterAttrs(tag string, attrs []html.Attribute) []html.Attribute {
	allowed := allowedAttrs[tag]
	var out []html.Attribute
	for _, a := range attrs {
		key := strings.ToLower(a.Key)
		if strings.HasPrefix(key, "on") || key == "style" {
			continue
		}
		if allowed == nil || !allowed[key] {
			continue
		}
		if urlAttrs[key] && !safeURL(a.Val) {
			continue
		}
		out = append(out, html.Attribute{Key: key, Val: a.Val})
	}
	return out
}

// safeURL 拒绝 javascript:/vbscript: 等危险协议，data: 仅允许图片。
func safeURL(raw string) bool {
	v := strings.TrimSpace(raw)
	lower := strings.ToLower(v)
	if i := strings.IndexByte(lower, ':'); i >= 0 {
		// 仅当冒号出现在路径分隔符之前才视为协议。
		if !strings.ContainsAny(lower[:i], "/?#") {
			scheme := lower[:i]
			switch scheme {
			case "javascript", "vbscript", "file":
				return false
			case "data":
				return strings.HasPrefix(lower, "data:image/")
			}
		}
	}
	return true
}

// StripTags 移除所有 HTML 标签，返回归一化空白后的纯文本。
func StripTags(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	tokenizer := html.NewTokenizer(strings.NewReader(input))
	var buf strings.Builder
	skip := 0 // script/style 内部文本计数
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return normalizeSpace(buf.String())
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == "script" || string(name) == "style" {
				if tt == html.StartTagToken {
					skip++
				}
			}
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			if (string(name) == "script" || string(name) == "style") && skip > 0 {
				skip--
			}
		case html.TextToken:
			if skip == 0 {
				buf.Write(tokenizer.Text())
			}
		}
	}
}

// Summarize 生成纯文本摘要：剥离标签后截取前 max 个字符（按 rune），超出加省略号。
func Summarize(input string, max int) string {
	text := StripTags(input)
	if max <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return strings.TrimSpace(string(runes[:max])) + "…"
}

// normalizeSpace 将连续空白折叠为单个空格并去除首尾空白。
func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
