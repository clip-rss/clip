package fetcher

import (
	"fmt"
	"strings"
	"time"
)

// rssRoot RSS 2.0 根元素。
type rssRoot struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title string `xml:"title"`
	// SelfLink 显式接住 Atom 的 rel="self" 自引用链接（RSSHub 等会在 channel 里加
	// <atom:link href=… rel="self"/>）。**必须声明在 Link 之前**：
	// Go 的 encoding/xml 按本地名匹配、对空命名空间字段不校验命名空间，所以
	// `xml:"link"` 会把 <atom:link> 一并吃下 —— 而后者没有 chardata，会把真正的
	// <link> 覆盖成空串（实测 RSSHub 的财联社源即如此）。
	SelfLink      string    `xml:"http://www.w3.org/2005/Atom link"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	LastBuildDate string    `xml:"lastBuildDate"`
	PubDate       string    `xml:"pubDate"`
	Image         *rssImage `xml:"image"`
	Items         []rssItem `xml:"item"`
}

// rssImage RSS <channel><image> 子元素，用于声明频道图标/徽标。
type rssImage struct {
	URL   string `xml:"url"`
	Title string `xml:"title"`
	Link  string `xml:"link"`
}

type rssItem struct {
	Title       string       `xml:"title"`
	Link        string       `xml:"link"`
	Description string       `xml:"description"`
	Encoded     string       `xml:"http://purl.org/rss/1.0/modules/content/ encoded"` // content:encoded
	Author      string       `xml:"author"`
	Creator     string       `xml:"http://purl.org/dc/elements/1.1/ creator"` // dc:creator
	GUID        rssGUID      `xml:"guid"`
	PubDate     string       `xml:"pubDate"`
	Date        string       `xml:"http://purl.org/dc/elements/1.1/ date"` // dc:date
	Categories  []string     `xml:"category"`
	Enclosure   rssEnclosure `xml:"enclosure"`
}

type rssGUID struct {
	Value       string `xml:",chardata"`
	IsPermaLink string `xml:"isPermaLink,attr"`
}

type rssEnclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

// parseRSS 解析 RSS 2.0 文档。
func parseRSS(data []byte) (*ParsedFeed, error) {
	var root rssRoot
	if err := decodeInto(data, &root); err != nil {
		return nil, fmt.Errorf("fetcher: parse rss: %w", err)
	}
	ch := root.Channel

	feed := &ParsedFeed{
		Title:       strings.TrimSpace(ch.Title),
		Description: strings.TrimSpace(ch.Description),
		Link:        strings.TrimSpace(ch.Link),
		Icon:        channelImageURL(ch.Image),
		Updated:     firstDate(ch.LastBuildDate, ch.PubDate),
		Items:       make([]ParsedItem, 0, len(ch.Items)),
	}

	for _, it := range ch.Items {
		content := it.Encoded
		if content == "" {
			content = it.Description
		}
		author := it.Author
		if author == "" {
			author = it.Creator
		}

		feed.Items = append(feed.Items, ParsedItem{
			GUID:       resolveRSSGUID(it),
			Title:      strings.TrimSpace(it.Title),
			Link:       strings.TrimSpace(it.Link),
			Author:     strings.TrimSpace(author),
			Published:  firstDate(it.PubDate, it.Date),
			Content:    content,
			Summary:    it.Description,
			Enclosure:  strings.TrimSpace(it.Enclosure.URL),
			Categories: trimAll(it.Categories),
		})
	}

	return feed, nil
}

// resolveRSSGUID 返回条目的去重标识：优先 guid，其次 link。
func resolveRSSGUID(it rssItem) string {
	if g := strings.TrimSpace(it.GUID.Value); g != "" {
		return g
	}
	return strings.TrimSpace(it.Link)
}

// firstDate 返回第一个可成功解析的日期。
func firstDate(candidates ...string) time.Time {
	for _, c := range candidates {
		if d := parseDate(c); !d.IsZero() {
			return d
		}
	}
	return time.Time{}
}

// trimAll 去除字符串切片中每个元素的首尾空白，并丢弃空项。
func trimAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// channelImageURL 从 RSS <channel><image> 中提取图标地址。
// 元素存在但 <url> 为空时返回空串。
func channelImageURL(img *rssImage) string {
	if img == nil {
		return ""
	}
	return strings.TrimSpace(img.URL)
}
