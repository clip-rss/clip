package fetcher

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// Parse 解析 RSS 2.0 或 Atom Feed 字节流，返回统一数据模型。
// 自动嗅探根元素以区分格式，并支持非 UTF-8 编码（依据 XML 声明转换）。
// 当 XML 声明为 UTF-8 但实际字节并非有效 UTF-8 时（常见于中文站点用 GBK 编码
// 却声明为 UTF-8），自动尝试编码检测与转换。
func Parse(data []byte) (*ParsedFeed, error) {
	data = ensureUTF8(data)

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
		// 根元素名大小写不敏感：XML 解码器原样返回本地名，`<HTML>` 与 `<html>` 都可能出现。
		if strings.EqualFold(root, "html") {
			// 反爬校验页 / 网站首页 / 失效后的错误页
			return nil, ErrHTMLResponse
		}
		return nil, fmt.Errorf("%w: <%s>", ErrUnknownFormat, root)
	}
}

// reXMLEnc 匹配 XML 声明中的 encoding 属性。
var reXMLEnc = regexp.MustCompile(`\bencoding\s*=\s*["']([^"']+)["']`)

// sniffXMLEncoding 提取 XML 声明中声明的编码名称。无声明或未找到时返回空串。
func sniffXMLEncoding(data []byte) string {
	// XML 声明总是出现在文档开头，限制搜索范围。
	limit := 200
	if len(data) < limit {
		limit = len(data)
	}
	m := reXMLEnc.FindSubmatch(data[:limit])
	if len(m) < 2 {
		return ""
	}
	return string(m[1])
}

// ensureUTF8 检测字节流编码，若实际并非 UTF-8 则尝试自动转换。
//
// 如果 XML 声明明确指定了非 UTF-8 编码（如 gb2312），保留原始字节交由解码器的
// CharsetReader 处理。仅当声明为 UTF-8、无声明、且字节不合法 UTF-8 时，
// 才依次尝试 GBK / GB18030 转换——这是中文站点最常见的问题模式：
// 内容以 GBK 编码，但 XML 声明却写 encoding="UTF-8"。
func ensureUTF8(data []byte) []byte {
	if len(data) == 0 || utf8.Valid(data) {
		return data
	}

	// 检查 XML 声明的编码：非 UTF-8 时信任声明，让 CharsetReader 处理。
	enc := sniffXMLEncoding(data)
	if enc != "" && !strings.EqualFold(enc, "utf-8") && !strings.EqualFold(enc, "utf8") {
		return data
	}

	// 依次尝试常见中文编码。
	decoders := []transform.Transformer{
		simplifiedchinese.GBK.NewDecoder(),
		simplifiedchinese.GB18030.NewDecoder(),
	}
	for _, dec := range decoders {
		result, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), dec))
		if err == nil && utf8.Valid(result) {
			return result
		}
	}

	return data
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
