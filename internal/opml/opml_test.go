package opml

import "testing"

const sample = `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head><title>My Feeds</title></head>
  <body>
    <outline text="科技" title="科技">
      <outline text="Hacker News" type="rss" xmlUrl="https://news.ycombinator.com/rss" htmlUrl="https://news.ycombinator.com"/>
      <outline text="Lobsters" type="rss" xmlUrl="https://lobste.rs/rss"/>
    </outline>
    <outline text="独立博客" type="rss" xmlUrl="https://example.com/feed.xml"/>
  </body>
</opml>`

func TestParseNestedAndFlat(t *testing.T) {
	doc, err := Parse([]byte(sample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Head.Title != "My Feeds" {
		t.Errorf("head title = %q", doc.Head.Title)
	}
	if len(doc.Body.Outlines) != 2 {
		t.Fatalf("top-level outlines = %d, want 2", len(doc.Body.Outlines))
	}

	group := doc.Body.Outlines[0]
	if group.IsFeed() {
		t.Error("group node should not be a feed")
	}
	if group.Label() != "科技" {
		t.Errorf("group label = %q, want 科技", group.Label())
	}
	if len(group.Outlines) != 2 {
		t.Fatalf("group feeds = %d, want 2", len(group.Outlines))
	}
	if hn := group.Outlines[0]; !hn.IsFeed() || hn.XMLURL != "https://news.ycombinator.com/rss" {
		t.Errorf("first feed wrong: %+v", hn)
	}

	loose := doc.Body.Outlines[1]
	if !loose.IsFeed() || loose.XMLURL != "https://example.com/feed.xml" {
		t.Errorf("loose feed wrong: %+v", loose)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	doc := &OPML{
		Head: Head{Title: "Export"},
		Body: Body{Outlines: []Outline{
			{Text: "技术", Outlines: []Outline{
				{Text: "A", Type: "rss", XMLURL: "https://a.example/feed"},
			}},
			{Text: "B", Type: "rss", XMLURL: "https://b.example/feed"},
		}},
	}

	data, err := Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) < len(`<?xml`) || string(data[:5]) != "<?xml" {
		t.Errorf("output should start with XML header, got %q", string(data[:min(20, len(data))]))
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if parsed.Version != "2.0" {
		t.Errorf("version = %q, want 2.0", parsed.Version)
	}
	if len(parsed.Body.Outlines) != 2 {
		t.Fatalf("outlines = %d, want 2", len(parsed.Body.Outlines))
	}
	if parsed.Body.Outlines[0].Outlines[0].XMLURL != "https://a.example/feed" {
		t.Errorf("nested feed url lost: %+v", parsed.Body.Outlines[0])
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := Parse([]byte("not xml <<<")); err == nil {
		t.Error("expected error for invalid OPML")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
