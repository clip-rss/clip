package fetcher

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// faviconRels <link rel> 中表示站点图标的取值。
var faviconRels = map[string]bool{
	"icon":             true,
	"shortcut icon":    true,
	"apple-touch-icon": true,
	"mask-icon":        true,
}

// DiscoverFavicon 从页面 HTML 中提取站点图标地址。
// 若页面未声明，则回退到 <scheme>://<host>/favicon.ico。
func DiscoverFavicon(htmlData []byte, baseURL string) string {
	base := parseBase(baseURL)
	if icon := faviconFromHTML(htmlData, base); icon != "" {
		return icon
	}
	return defaultFaviconURL(base)
}

// faviconFromHTML 解析 <link rel="icon"> 等节点，返回首个图标地址。
func faviconFromHTML(htmlData []byte, base *url.URL) string {
	doc, err := html.Parse(strings.NewReader(string(htmlData)))
	if err != nil {
		return ""
	}

	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "link" {
			var rel, href string
			for _, a := range n.Attr {
				switch strings.ToLower(a.Key) {
				case "rel":
					rel = strings.ToLower(strings.TrimSpace(a.Val))
				case "href":
					href = strings.TrimSpace(a.Val)
				}
			}
			if href != "" && faviconRels[rel] {
				found = resolveURL(base, href)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found
}

// defaultFaviconURL 返回站点根目录下的默认 favicon.ico 地址。
func defaultFaviconURL(base *url.URL) string {
	if base == nil || base.Host == "" {
		return ""
	}
	scheme := base.Scheme
	if scheme == "" {
		scheme = "https"
	}
	return scheme + "://" + base.Host + "/favicon.ico"
}
