package fetcher

import (
	"errors"
	"testing"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
    <title>Example Blog</title>
    <link>https://example.com/</link>
    <description>An example feed</description>
    <lastBuildDate>Mon, 02 Jan 2006 15:04:05 -0700</lastBuildDate>
    <item>
      <title>First Post</title>
      <link>https://example.com/posts/1</link>
      <description>Short summary one</description>
      <content:encoded><![CDATA[<p>Full <b>body</b> one</p>]]></content:encoded>
      <dc:creator>Alice</dc:creator>
      <guid isPermaLink="false">tag:example.com,2006:1</guid>
      <pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate>
      <category>Tech</category>
      <category>Go</category>
      <enclosure url="https://example.com/audio/1.mp3" type="audio/mpeg"/>
    </item>
    <item>
      <title>Second Post</title>
      <link>https://example.com/posts/2</link>
      <description>Short summary two</description>
      <pubDate>Tue, 03 Jan 2006 15:04:05 -0700</pubDate>
    </item>
  </channel>
</rss>`

const sampleAtom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Example</title>
  <subtitle>Atom subtitle</subtitle>
  <link href="https://atom.example.com/" rel="alternate"/>
  <link href="https://atom.example.com/feed.xml" rel="self"/>
  <updated>2020-01-02T15:04:05Z</updated>
  <entry>
    <title>Atom Entry</title>
    <link href="https://atom.example.com/e/1" rel="alternate"/>
    <id>urn:uuid:1234</id>
    <published>2020-01-01T10:00:00Z</published>
    <updated>2020-01-02T11:00:00Z</updated>
    <summary type="text">Entry summary</summary>
    <content type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml"><p>Hello Atom</p></div></content>
    <author><name>Bob</name></author>
    <category term="news"/>
  </entry>
</feed>`

func TestParseRSS(t *testing.T) {
	feed, err := Parse([]byte(sampleRSS))
	if err != nil {
		t.Fatalf("Parse RSS: %v", err)
	}
	if feed.Title != "Example Blog" {
		t.Errorf("title = %q", feed.Title)
	}
	if feed.Link != "https://example.com/" {
		t.Errorf("link = %q", feed.Link)
	}
	if len(feed.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(feed.Items))
	}

	it := feed.Items[0]
	if it.Title != "First Post" {
		t.Errorf("item title = %q", it.Title)
	}
	if it.GUID != "tag:example.com,2006:1" {
		t.Errorf("guid = %q", it.GUID)
	}
	if it.Author != "Alice" {
		t.Errorf("author(dc:creator) = %q", it.Author)
	}
	if it.Content != "<p>Full <b>body</b> one</p>" {
		t.Errorf("content(content:encoded) = %q", it.Content)
	}
	if it.Enclosure != "https://example.com/audio/1.mp3" {
		t.Errorf("enclosure = %q", it.Enclosure)
	}
	if len(it.Categories) != 2 || it.Categories[0] != "Tech" || it.Categories[1] != "Go" {
		t.Errorf("categories = %v", it.Categories)
	}
	if it.Published.IsZero() {
		t.Error("published should be parsed")
	}
}

func TestParseAtom(t *testing.T) {
	feed, err := Parse([]byte(sampleAtom))
	if err != nil {
		t.Fatalf("Parse Atom: %v", err)
	}
	if feed.Title != "Atom Example" {
		t.Errorf("title = %q", feed.Title)
	}
	if feed.Description != "Atom subtitle" {
		t.Errorf("subtitle = %q", feed.Description)
	}
	if feed.Link != "https://atom.example.com/" {
		t.Errorf("alternate link = %q", feed.Link)
	}
	if feed.FeedLink != "https://atom.example.com/feed.xml" {
		t.Errorf("self link = %q", feed.FeedLink)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(feed.Items))
	}
	it := feed.Items[0]
	if it.GUID != "urn:uuid:1234" {
		t.Errorf("guid(id) = %q", it.GUID)
	}
	if it.Link != "https://atom.example.com/e/1" {
		t.Errorf("link = %q", it.Link)
	}
	if it.Author != "Bob" {
		t.Errorf("author = %q", it.Author)
	}
	if it.Content != "<p>Hello Atom</p>" {
		t.Errorf("xhtml content = %q", it.Content)
	}
	if len(it.Categories) != 1 || it.Categories[0] != "news" {
		t.Errorf("categories = %v", it.Categories)
	}
	if it.Published.IsZero() || it.Updated.IsZero() {
		t.Error("published/updated should be parsed")
	}
}

func TestParseUnknownFormat(t *testing.T) {
	cases := [][]byte{
		[]byte("just plain text, not xml"),
		[]byte("<foo><bar>nope</bar></foo>"),
	}
	for _, data := range cases {
		_, err := Parse(data)
		if err == nil {
			t.Errorf("expected error for %q", data)
			continue
		}
		if errors.Is(err, ErrHTMLResponse) {
			t.Errorf("%q should not be reported as an HTML response", data)
		}
	}
}

// 返回网页时必须与「格式未知」区分开：用户的处置办法不同（换地址 / 该站拦截了阅读器），
// 调用方靠 errors.Is 给出针对性提示。
func TestParseHTMLResponse(t *testing.T) {
	cases := [][]byte{
		[]byte(`<html><body>nope</body></html>`),
		[]byte(`<!DOCTYPE html><html lang="en"><head></head><body>waf</body></html>`),
		// 根元素名大小写不敏感：XML 解码器原样返回本地名。
		[]byte(`<!DOCTYPE HTML><HTML><BODY>waf</BODY></HTML>`),
	}
	for _, data := range cases {
		_, err := Parse(data)
		if !errors.Is(err, ErrHTMLResponse) {
			t.Errorf("Parse(%.40q) error = %v, want ErrHTMLResponse", data, err)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []string{
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"2020-01-02T15:04:05Z",
		"2020-01-02 15:04:05",
		"2020-01-02",
	}
	for _, c := range cases {
		if parseDate(c).IsZero() {
			t.Errorf("parseDate(%q) returned zero", c)
		}
	}
	if !parseDate("not a date").IsZero() {
		t.Error("invalid date should be zero")
	}
	if !parseDate("").IsZero() {
		t.Error("empty date should be zero")
	}
}

func TestParseCharsetConversion(t *testing.T) {
	// ISO-8859-1 文档，标题含 0xE9（é）。
	head := []byte(`<?xml version="1.0" encoding="ISO-8859-1"?><rss version="2.0"><channel><title>caf`)
	tail := []byte(`</title><link>https://x</link><description>d</description></channel></rss>`)
	data := append(append(head, 0xE9), tail...)

	feed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse ISO-8859-1: %v", err)
	}
	if feed.Title != "café" {
		t.Errorf("title = %q, want café", feed.Title)
	}
}

func TestParseGBKAsUTF8(t *testing.T) {
	// 模拟 36kr 等中文网站的场景：XML 声明为 UTF-8，但实际字节是 GBK。
	// GBK 编码的 "文章内容测试" → utf8.RuneError 的输入验证。
	//
	// "新闻" 的 GBK 字节：D0 C2 CE C5
	//
	// 构造一个 RSS feed，XML 声明 encoding="UTF-8"，但标题以 GBK 编码。
	head := []byte(`<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>`)
	// GBK 编码的 "新闻" = D0 C2 CE C5
	gbkTitle := []byte{0xD0, 0xC2, 0xCE, 0xC5}
	tail := []byte(`</title><link>https://x</link><description>d</description></channel></rss>`)
	data := append(append(head, gbkTitle...), tail...)

	// 直接 Parse：ensureUTF8 应检测到非 UTF-8 字节并自动转换为 GBK。
	feed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse GBK-as-UTF-8: %v", err)
	}
	if feed.Title != "新闻" {
		t.Errorf("title = %q, want 新闻 (got apparent length %d runes)", feed.Title, len([]rune(feed.Title)))
	}
}

func TestEnsureUTF8PreservesValidUTF8(t *testing.T) {
	// 有效 UTF-8 应原样通过。
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>Hello 世界</title></channel></rss>`)
	got := ensureUTF8(data)
	if string(got) != string(data) {
		t.Errorf("ensureUTF8 should not modify valid UTF-8")
	}
}

func TestEnsureUTF8PreservesExplicitNonUTF8(t *testing.T) {
	// 声明为 gb2312 的文档应原样保留（由 XML 解码器的 CharsetReader 处理）。
	data := []byte(`<?xml version="1.0" encoding="gb2312"?><rss version="2.0"><channel><title>`)
	// GBK 字节（gb2312 的超集）
	gbkBytes := []byte{0xD0, 0xC2, 0xCE, 0xC5}
	data = append(data, gbkBytes...)
	data = append(data, []byte(`</title></channel></rss>`)...)

	got := ensureUTF8(data)
	if string(got) != string(data) {
		t.Errorf("ensureUTF8 should not modify data with explicit non-UTF8 encoding declaration")
	}
}
