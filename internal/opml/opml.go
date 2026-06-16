// Package opml 负责 OPML 文件的导入与导出。
//
// OPML（Outline Processor Markup Language）是订阅源列表的通用交换格式：
// <outline> 节点可嵌套，含 xmlUrl 属性者表示一个订阅源，否则视为分组（分类）。
package opml

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// OPML 表示一份 OPML 文档。
type OPML struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    Head     `xml:"head"`
	Body    Body     `xml:"body"`
}

// Head OPML 头部元信息。
type Head struct {
	Title string `xml:"title,omitempty"`
}

// Body OPML 主体，包含顶层 outline 列表。
type Body struct {
	Outlines []Outline `xml:"outline"`
}

// Outline 大纲节点：可表示订阅源（含 XMLURL）或分组（含子节点）。
type Outline struct {
	Text     string    `xml:"text,attr,omitempty"`
	Title    string    `xml:"title,attr,omitempty"`
	Type     string    `xml:"type,attr,omitempty"`
	XMLURL   string    `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string    `xml:"htmlUrl,attr,omitempty"`
	Outlines []Outline `xml:"outline"`
}

// IsFeed 判断该节点是否为订阅源（带 xmlUrl）。
func (o Outline) IsFeed() bool {
	return strings.TrimSpace(o.XMLURL) != ""
}

// Label 返回节点的展示名称，优先 text，回退 title。
func (o Outline) Label() string {
	if t := strings.TrimSpace(o.Text); t != "" {
		return t
	}
	return strings.TrimSpace(o.Title)
}

// Parse 解析 OPML 字节流。
func Parse(data []byte) (*OPML, error) {
	var doc OPML
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("opml: parse: %w", err)
	}
	return &doc, nil
}

// Marshal 将 OPML 文档序列化为带 XML 声明、缩进的字节流。
func Marshal(doc *OPML) ([]byte, error) {
	if doc.Version == "" {
		doc.Version = "2.0"
	}
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("opml: marshal: %w", err)
	}
	return append([]byte(xml.Header), out...), nil
}
