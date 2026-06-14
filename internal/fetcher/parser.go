package fetcher

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
)

// Parse 解析 RSS 2.0 或 Atom Feed 字节流，返回统一数据模型。
// 自动嗅探根元素以区分格式，并支持非 UTF-8 编码（依据 XML 声明转换）。
func Parse(data []byte) (*ParsedFeed, error) {
	root, err := sniffRoot(data)
	if err != nil {
		return nil, err
	}

	switch root {
	case "feed": // Atom
		return parseAtom(data)
	case "rss": // RSS 2.0
		return parseRSS(data)
	default:
		return nil, fmt.Errorf("%w: <%s>", ErrUnknownFormat, root)
	}
}

// sniffRoot 读取到第一个起始元素，返回其本地名（rss / feed 等）。
func sniffRoot(data []byte) (string, error) {
	dec := newDecoder(data)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return "", ErrUnknownFormat
		}
		if err != nil {
			return "", fmt.Errorf("fetcher: sniff root: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name.Local, nil
		}
	}
}

// newDecoder 创建容错的 XML 解码器：支持字符集转换、HTML 实体、非严格模式。
func newDecoder(data []byte) *xml.Decoder {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charset.NewReaderLabel
	dec.Strict = false
	dec.Entity = xml.HTMLEntity
	return dec
}

// decodeInto 将 XML 解码到目标结构体。
func decodeInto(data []byte, v any) error {
	return newDecoder(data).Decode(v)
}

// dateLayouts 覆盖 RSS（RFC822/1123）与 Atom（RFC3339）常见日期格式。
var dateLayouts = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC822Z,
	time.RFC822,
	time.RFC3339,
	time.RFC3339Nano,
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"Mon, 02 Jan 2006 15:04:05 -0700",
	"Mon, 02 Jan 2006 15:04:05 MST",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	time.ANSIC,
	time.UnixDate,
}

// parseDate 尝试多种布局解析日期字符串，失败返回零值。
func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
