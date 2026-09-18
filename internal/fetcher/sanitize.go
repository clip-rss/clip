package fetcher

import (
	"bytes"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// dangerousTags 这些元素及其子树将被整体移除。
var dangerousTags = map[string]bool{
	"script": true, "style": true, "object": true,
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
	"iframe": true,
}

// allowedAttrs 每个标签允许保留的属性。
var allowedAttrs = map[string]map[string]bool{
	"a":      {"href": true, "title": true, "target": true, "rel": true},
	"img":    {"src": true, "alt": true, "title": true, "width": true, "height": true},
	"video":  {"src": true, "controls": true, "width": true, "height": true, "poster": true, "preload": true, "playsinline": true},
	"audio":  {"src": true, "controls": true, "preload": true},
	"source": {"src": true, "srcset": true, "type": true},
	"iframe": {"src": true, "title": true, "width": true, "height": true, "allow": true, "allowfullscreen": true, "frameborder": true, "loading": true, "referrerpolicy": true},
	"time":   {"datetime": true},
	"td":     {"colspan": true, "rowspan": true},
	"th":     {"colspan": true, "rowspan": true},
	"col":    {"span": true},
}

// urlAttrs 需要做协议安全检查的属性。
var urlAttrs = map[string]bool{"href": true, "src": true, "poster": true}

var mediaDataAttrs = map[string]bool{
	"data-src":       true,
	"data-video":     true,
	"data-video-src": true,
	"data-video-url": true,
	"data-hls":       true,
	"data-mp4":       true,
	"data-url":       true,
}

var mediaTags = map[string]bool{
	"video": true, "audio": true, "source": true, "iframe": true,
}

// Sanitize 清洗 HTML：移除脚本/样式等危险元素、事件处理属性与危险协议，
// 仅保留白名单内的标签与属性，以防止 XSS。
//
// 同时移除 U+FFFD（替换字符），该字符渲染为「��」方块。
// U+FFFD 可能在以下场景进入数据：
//   - 编码转换失败时由解码器注入（如旧版无 GBK 检测的解析器）
//   - 无效 UTF-8 字节经 xml.Decoder 读取后生成
//   - 来源于 Feed 本身携带的垃圾字节
func Sanitize(input string) string {
	return sanitizeWithBase(input, nil)
}

// SanitizeWithBase 清洗 HTML，并按文章 URL 解析其中的媒体与嵌入地址。
// 用于读取历史缓存文章时补齐旧版本未处理的媒体地址。
func SanitizeWithBase(input, baseURL string) string {
	return sanitizeWithBase(input, parseBase(baseURL))
}

// sanitizeWithBase 清洗 HTML，并将正文中的 URL 属性解析为绝对地址。
func sanitizeWithBase(input string, base *url.URL) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	// 移除替换字符 U+FFFD，避免渲染为「��」乱码。
	if strings.ContainsRune(input, '�') {
		input = strings.ReplaceAll(input, "�", "")
	}

	context := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(input), context)
	if err != nil {
		return StripTags(input)
	}

	var buf bytes.Buffer
	for _, n := range nodes {
		for _, c := range cleanNode(n, base) {
			_ = html.Render(&buf, c)
		}
	}
	return buf.String()
}

// cleanNode 递归清洗节点，返回应保留的节点列表（可能展开子节点）。
func cleanNode(n *html.Node, base *url.URL) []*html.Node {
	switch n.Type {
	case html.TextNode:
		return []*html.Node{{Type: html.TextNode, Data: n.Data}}
	case html.ElementNode:
		// 危险元素：整体丢弃（含子树）。
		if dangerousTags[n.Data] {
			return nil
		}
		children := cleanChildren(n, base)
		// 非白名单元素：展开，保留其子节点。
		if !allowedTags[n.Data] {
			return children
		}
		attrs := filterAttrs(n.Data, n.Attr, base)
		if n.Data == "iframe" && !hasAttr(attrs, "src") {
			return nil
		}
		el := &html.Node{
			Type:     html.ElementNode,
			Data:     n.Data,
			DataAtom: n.DataAtom,
			Attr:     attrs,
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
func cleanChildren(n *html.Node, base *url.URL) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out = append(out, cleanNode(c, base)...)
	}
	return out
}

// filterAttrs 过滤属性：丢弃事件处理器、style 及危险协议，仅保留白名单属性。
func filterAttrs(tag string, attrs []html.Attribute, base *url.URL) []html.Attribute {
	allowed := allowedAttrs[tag]
	var out []html.Attribute
	hasSrc := false
	var deferredSrc string
	for _, a := range attrs {
		key := strings.ToLower(a.Key)
		if strings.HasPrefix(key, "on") || key == "style" {
			continue
		}
		if mediaTags[tag] && mediaDataAttrs[key] {
			if deferredSrc == "" {
				deferredSrc = a.Val
			}
			continue
		}
		if mediaDataAttrs[key] {
			if key == "data-url" && !mediaTags[tag] {
				continue
			}
			if !safeMediaURL(a.Val) {
				continue
			}
			value := a.Val
			if base != nil {
				value = resolveURL(base, value)
			}
			out = append(out, html.Attribute{Key: key, Val: value})
			continue
		}
		if allowed == nil || !allowed[key] {
			continue
		}
		value := a.Val
		if key == "srcset" {
			value = sanitizeSrcset(value, base)
			if value == "" {
				continue
			}
		} else if urlAttrs[key] {
			if mediaTags[tag] && (key == "src" || key == "poster") {
				if !safeMediaURL(value) {
					continue
				}
			} else if !safeURL(value) {
				continue
			}
			if base != nil {
				value = resolveURL(base, value)
			}
			if tag == "iframe" && key == "src" && !safeEmbedURL(value) {
				continue
			}
		}
		if key == "src" {
			hasSrc = true
		}
		out = append(out, html.Attribute{Key: key, Val: value})
	}
	if mediaTags[tag] && !hasSrc && deferredSrc != "" && allowed != nil && allowed["src"] {
		if safeMediaURL(deferredSrc) {
			value := deferredSrc
			if base != nil {
				value = resolveURL(base, value)
			}
			if tag != "iframe" || safeEmbedURL(value) {
				out = append(out, html.Attribute{Key: "src", Val: value})
			}
		}
	}
	return out
}

func hasAttr(attrs []html.Attribute, key string) bool {
	for _, a := range attrs {
		if a.Key == key {
			return true
		}
	}
	return false
}

// sanitizeSrcset 清洗并解析 srcset 中的每个候选地址。
func sanitizeSrcset(raw string, base *url.URL) string {
	var out []string
	for _, candidate := range strings.Split(raw, ",") {
		fields := strings.Fields(strings.TrimSpace(candidate))
		if len(fields) == 0 || !safeURL(fields[0]) {
			continue
		}
		if base != nil {
			fields[0] = resolveURL(base, fields[0])
		}
		out = append(out, strings.Join(fields, " "))
	}
	return strings.Join(out, ", ")
}

var safeEmbedDomains = []string{
	"youtube.com", "youtube-nocookie.com", "youtu.be", "vimeo.com",
	"bilibili.com", "dailymotion.com", "twitch.tv", "thepaper.cn",
}

func safeMediaURL(raw string) bool {
	v := strings.TrimSpace(raw)
	lower := strings.ToLower(v)
	if strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "blob:") {
		return false
	}
	if i := strings.IndexByte(lower, ':'); i >= 0 && !strings.ContainsAny(lower[:i], "/?#") {
		scheme := lower[:i]
		return scheme == "http" || scheme == "https"
	}
	return safeURL(v)
}

// safeEmbedURL 只允许常见视频平台的 HTTPS/HTTP 播放器地址进入 iframe。
func safeEmbedURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	for _, domain := range safeEmbedDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
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
