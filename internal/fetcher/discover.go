package fetcher

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// DiscoveredFeed 表示从 HTML 页面中发现的一个 Feed 链接。
type DiscoveredFeed struct {
	Title string // <link> 的 title 属性
	URL   string // 已解析为绝对地址的 Feed URL
	Type  string // MIME 类型，如 application/rss+xml
}

// feedLinkTypes 视为 Feed 的 <link> MIME 类型。
var feedLinkTypes = map[string]bool{
	"application/rss+xml":   true,
	"application/atom+xml":  true,
	"application/feed+json": true,
	"application/json":      true,
}

// DiscoverFeeds 解析 HTML 页面，提取 <link rel="alternate"> 中声明的 Feed 链接，
// 相对地址会基于 baseURL 解析为绝对地址。
func DiscoverFeeds(htmlData []byte, baseURL string) []DiscoveredFeed {
	doc, err := html.Parse(strings.NewReader(string(htmlData)))
	if err != nil {
		return nil
	}
	base := parseBase(baseURL)

	var feeds []DiscoveredFeed
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "link" {
			if f, ok := feedFromLink(n, base); ok {
				feeds = append(feeds, f)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return feeds
}

// feedFromLink 判断 <link> 节点是否声明了 Feed，并解析其属性。
func feedFromLink(n *html.Node, base *url.URL) (DiscoveredFeed, bool) {
	var rel, typ, href, title string
	for _, a := range n.Attr {
		switch strings.ToLower(a.Key) {
		case "rel":
			rel = strings.ToLower(strings.TrimSpace(a.Val))
		case "type":
			typ = strings.ToLower(strings.TrimSpace(a.Val))
		case "href":
			href = strings.TrimSpace(a.Val)
		case "title":
			title = strings.TrimSpace(a.Val)
		}
	}
	if !strings.Contains(rel, "alternate") || !feedLinkTypes[typ] || href == "" {
		return DiscoveredFeed{}, false
	}
	return DiscoveredFeed{
		Title: title,
		URL:   resolveURL(base, href),
		Type:  typ,
	}, true
}

// parseBase 解析基准 URL，失败返回 nil。
func parseBase(raw string) *url.URL {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	return u
}

// resolveURL 基于 base 解析可能为相对路径的 href。
func resolveURL(base *url.URL, href string) string {
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	if base == nil {
		return href
	}
	return base.ResolveReference(ref).String()
}
