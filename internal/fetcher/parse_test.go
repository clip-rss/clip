package fetcher

import (
	"bytes"
	"errors"
	"testing"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
    <title>Example Blog</title>
    <link>https://example.com/</link>
    <description>An example feed</description>
    <image>
      <url>https://example.com/logo.png</url>
      <title>Example Blog</title>
      <link>https://example.com/</link>
    </image>
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
  <icon>https://atom.example.com/icon.png</icon>
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
	if feed.Icon != "https://example.com/logo.png" {
		t.Errorf("rss icon = %q, want https://example.com/logo.png", feed.Icon)
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
	if feed.Icon != "https://atom.example.com/icon.png" {
		t.Errorf("atom icon = %q, want https://atom.example.com/icon.png", feed.Icon)
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

// RSSHub 等生成的 RSS 会在 channel 里同时给出 <link> 与 <atom:link rel="self"/>。
// Go 的 encoding/xml 按本地名匹配，对空命名空间字段不校验命名空间，`xml:"link"`
// 因而把 <atom:link> 一并吃下 —— 它没有 chardata，会把真正的 <link> 覆盖成空串。
// 实测财联社源因此拿到空 Link，favicon 无从解析（回退到 globe 图标）。
func TestParseRSSChannelLinkSurvivesAtomSelfLink(t *testing.T) {
	const doc = `<?xml version="1.0" encoding="UTF-8"?>
	<rss xmlns:atom="http://www.w3.org/2005/Atom" version="2.0">
	  <channel>
	    <title>财联社 - 头条</title>
	    <link>https://www.cls.cn/depth?id=1000</link>
	    <atom:link href="http://rsshub.example/cls/depth/1000" rel="self" type="application/rss+xml"></atom:link>
	    <description>d</description>
	    <item><title>t</title><link>https://example.com/1</link></item>
	  </channel>
	</rss>`

	feed, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if feed.Link != "https://www.cls.cn/depth?id=1000" {
		t.Errorf("channel link = %q, want https://www.cls.cn/depth?id=1000（是否被 <atom:link> 覆盖？）", feed.Link)
	}
	if feed.Items[0].Link != "https://example.com/1" {
		t.Errorf("item link = %q", feed.Items[0].Link)
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

func TestStripInvalidXMLChars(t *testing.T) {
	t.Run("clean data unchanged", func(t *testing.T) {
		data := []byte(`<rss><channel><title>Hello 世界</title></channel></rss>`)
		got := stripInvalidXMLChars(data)
		if string(got) != string(data) {
			t.Errorf("clean data was modified: got %q", got)
		}
	})

	t.Run("removes U+001E", func(t *testing.T) {
		// 模拟 36kr 问题：feed 中包含 U+001E (Record Separator)
		data := []byte("<rss><channel><title>标题\x1E副标题</title></channel></rss>")
		got := stripInvalidXMLChars(data)
		if bytes.Contains(got, []byte{0x1E}) {
			t.Error("U+001E should have been removed")
		}
		if !bytes.Contains(got, []byte("标题副标题")) {
			t.Errorf("expected concatenated title, got %q", got)
		}
	})

	t.Run("preserves allowed control chars", func(t *testing.T) {
		data := []byte("<rss><channel><title>line1\nline2\r\ttab</title></channel></rss>")
		got := stripInvalidXMLChars(data)
		if !bytes.Contains(got, []byte{0x0A}) {
			t.Error("LF should be preserved")
		}
		if !bytes.Contains(got, []byte{0x0D}) {
			t.Error("CR should be preserved")
		}
		if !bytes.Contains(got, []byte{0x09}) {
			t.Error("Tab should be preserved")
		}
	})

	t.Run("removes various control chars U+0000-U+001F", func(t *testing.T) {
		data := []byte("a\x00b\x01c\x0Bd\x0Ce\x1Bf")
		got := stripInvalidXMLChars(data)
		if !bytes.Equal(got, []byte("abcdef")) {
			t.Errorf("expected abcdef, got %q", got)
		}
	})
}

func TestParseStripsInvalidXMLChars(t *testing.T) {
	// 完整 Parse 路径验证：含 U+001E 的 RSS 应成功解析而非报错
	data := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<rss version=\"2.0\"><channel>\n" +
		"<title>test\x1Efeed</title>\n" +
		"<link>https://example.com</link>\n" +
		"<description>d</description>\n" +
		"<item><title>item\x1Eone</title><link>https://example.com/1</link></item>\n" +
		"</channel></rss>")

	feed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse with U+001E: %v", err)
	}
	if feed.Title != "testfeed" {
		t.Errorf("title = %q, want testfeed", feed.Title)
	}
	if feed.Items[0].Title != "itemone" {
		t.Errorf("item title = %q, want itemone", feed.Items[0].Title)
	}
}
