package fetcher

import (
	"fmt"
	"strings"
)

// atomFeed Atom 根元素。
type atomFeed struct {
	Title    atomText    `xml:"title"`
	Subtitle atomText    `xml:"subtitle"`
	Links    []atomLink  `xml:"link"`
	Updated  string      `xml:"updated"`
	Entries  []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title      atomText       `xml:"title"`
	Links      []atomLink     `xml:"link"`
	ID         string         `xml:"id"`
	Published  string         `xml:"published"`
	Updated    string         `xml:"updated"`
	Summary    atomText       `xml:"summary"`
	Content    atomText       `xml:"content"`
	Authors    []atomAuthor   `xml:"author"`
	Categories []atomCategory `xml:"category"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type atomText struct {
	Type     string `xml:"type,attr"`
	Body     string `xml:",chardata"`
	InnerXML string `xml:",innerxml"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

type atomCategory struct {
	Term string `xml:"term,attr"`
}

// value 返回文本内容：type="xhtml" 时取内部 XML 并剥离外层包裹 <div>，
// 否则取字符数据。
func (t atomText) value() string {
	if strings.EqualFold(t.Type, "xhtml") {
		return unwrapXHTMLDiv(t.InnerXML)
	}
	return strings.TrimSpace(t.Body)
}

// unwrapXHTMLDiv 剥离 Atom xhtml 内容中规范要求的外层 <div> 包裹元素。
func unwrapXHTMLDiv(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "<div") || !strings.HasSuffix(s, "</div>") {
		return s
	}
	open := strings.IndexByte(s, '>')
	if open < 0 {
		return s
	}
	return strings.TrimSpace(s[open+1 : len(s)-len("</div>")])
}

// parseAtom 解析 Atom 文档。
func parseAtom(data []byte) (*ParsedFeed, error) {
	var af atomFeed
	if err := decodeInto(data, &af); err != nil {
		return nil, fmt.Errorf("fetcher: parse atom: %w", err)
	}

	feed := &ParsedFeed{
		Title:       af.Title.value(),
		Description: af.Subtitle.value(),
		Link:        atomAlternateHref(af.Links),
		FeedLink:    atomSelfHref(af.Links),
		Updated:     parseDate(af.Updated),
		Items:       make([]ParsedItem, 0, len(af.Entries)),
	}

	for _, e := range af.Entries {
		content := e.Content.value()
		summary := e.Summary.value()
		if content == "" {
			content = summary
		}
		if summary == "" {
			summary = e.Content.value()
		}

		feed.Items = append(feed.Items, ParsedItem{
			GUID:       resolveAtomGUID(e),
			Title:      e.Title.value(),
			Link:       atomAlternateHref(e.Links),
			Author:     atomAuthorName(e.Authors),
			Published:  firstDate(e.Published, e.Updated),
			Updated:    parseDate(e.Updated),
			Content:    content,
			Summary:    summary,
			Enclosure:  atomEnclosureHref(e.Links),
			Categories: atomCategoryTerms(e.Categories),
		})
	}

	return feed, nil
}

// atomAlternateHref 返回 rel="alternate"（或无 rel）的链接，作为正文链接。
func atomAlternateHref(links []atomLink) string {
	for _, l := range links {
		if l.Rel == "alternate" || l.Rel == "" {
			return strings.TrimSpace(l.Href)
		}
	}
	if len(links) > 0 {
		return strings.TrimSpace(links[0].Href)
	}
	return ""
}

// atomSelfHref 返回 rel="self" 的链接，即 Feed 自身地址。
func atomSelfHref(links []atomLink) string {
	for _, l := range links {
		if l.Rel == "self" {
			return strings.TrimSpace(l.Href)
		}
	}
	return ""
}

// atomEnclosureHref 返回 rel="enclosure" 的附件链接。
func atomEnclosureHref(links []atomLink) string {
	for _, l := range links {
		if l.Rel == "enclosure" {
			return strings.TrimSpace(l.Href)
		}
	}
	return ""
}

// resolveAtomGUID 返回条目去重标识：优先 id，其次正文链接。
func resolveAtomGUID(e atomEntry) string {
	if id := strings.TrimSpace(e.ID); id != "" {
		return id
	}
	return atomAlternateHref(e.Links)
}

func atomAuthorName(authors []atomAuthor) string {
	for _, a := range authors {
		if name := strings.TrimSpace(a.Name); name != "" {
			return name
		}
	}
	return ""
}

func atomCategoryTerms(cats []atomCategory) []string {
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		if term := strings.TrimSpace(c.Term); term != "" {
			out = append(out, term)
		}
	}
	return out
}
