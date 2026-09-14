package fetcher

import (
	"context"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// faviconTimeout 解析站点图标时的总超时。
// 命中反爬挑战时需要「求解 + 重取」两个往返，故比单次请求宽松。
const faviconTimeout = 8 * time.Second

// faviconRels <link rel> 中表示站点图标的取值。
var faviconRels = map[string]bool{
	"icon":             true,
	"shortcut icon":    true,
	"apple-touch-icon": true,
	"mask-icon":        true,
}

// ResolveFavicon 解析站点图标，依次尝试三层：
//
//  1. declared —— Feed 自身声明的图标（RSS <image> / Atom <icon>），最权威且零额外请求
//  2. 抓取 siteURL 页面，解析其中的 <link rel="icon">（经 FetchPage，含反爬挑战求解）
//  3. 回退到 <scheme>://<host>/favicon.ico
//
// siteURL 为空时返回空串（无从推断站点）。网络或解析失败静默降级到下一层 ——
// 拿不到图标好过让订阅流程失败。
//
// 注意第 3 层对任意非空 siteURL 都会给出地址（哪怕该地址实际 404），
// 前端在图片加载失败时自行回退到内置图标。
func (f *Fetcher) ResolveFavicon(ctx context.Context, declared, siteURL string) string {
	if declared != "" {
		return declared
	}
	if siteURL == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(ctx, faviconTimeout)
	defer cancel()

	if body, err := f.FetchPage(ctx, siteURL); err == nil && len(body) > 0 {
		if icon := DiscoverFavicon(body, siteURL); icon != "" {
			return icon
		}
	}
	return DiscoverFavicon(nil, siteURL)
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
