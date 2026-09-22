// Package reader 负责文章正文提取（Readability）。
//
// 用于「RSS 只给摘要」的订阅源：拿到文章原文页面的 HTML 后，用 Readability
// 算法剥掉导航、广告、评论区，只留正文。本包不发网络请求——抓取由
// internal/fetcher 负责，这里只做纯函数式的提取，便于单测。
package reader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	readability "codeberg.org/readeck/go-readability/v2"
	"golang.org/x/net/html/charset"
)

// ErrNoContent 表示页面解析成功但没有可识别的正文。
//
// 非文章页（首页、列表页）、纯客户端渲染的空壳、以及反爬拦截页都会落到这里。
// 与「抓取失败」区分开，调用方才能给出「这个页面提不出正文」而不是「网络错误」。
var ErrNoContent = errors.New("reader: no readable content")

// Extract 从文章原文页面的 HTML 中提取正文，返回正文 HTML 片段。
//
// pageURL 用于把正文里的相对链接解析为绝对地址（图片 src 尤其依赖它）。
// 为空或无法解析时退化为不解析相对链接，不视为错误——正文本身仍然有价值。
//
// 输入先按页面自己声明的编码转成 UTF-8 再解析：readability 与 x/net/html 都
// 假定输入是 UTF-8，直接喂 GBK 字节会得到满屏乱码。
//
// 返回的 HTML **未经 XSS 清洗**，落库或渲染前必须过 fetcher.Sanitize。
func Extract(page []byte, pageURL string) (string, error) {
	if len(bytes.TrimSpace(page)) == 0 {
		return "", ErrNoContent
	}

	var input io.Reader
	// charset.NewReader 依次看 BOM、<meta charset> 与内容嗅探。只有遇到无法识别的
	// 编码名才报错，此时退回原始字节——绝大多数页面本就是 UTF-8。
	decoded, err := charset.NewReader(bytes.NewReader(page), "text/html")
	if err != nil {
		input = bytes.NewReader(page)
	} else {
		input = decoded
	}

	article, err := readability.FromReader(input, parseBase(pageURL))
	if err != nil {
		return "", fmt.Errorf("reader: parse %q: %w", pageURL, err)
	}
	// Node 为 nil 表示 grabArticle 没找到正文容器（库的约定）。
	if article.Node == nil {
		return "", ErrNoContent
	}

	// 有节点不等于有内容：只剩图片说明或空壳 div 的情况下正文文本为空，
	// 这种结果给用户看还不如保留 RSS 摘要。
	var text bytes.Buffer
	if err := article.RenderText(&text); err != nil {
		return "", fmt.Errorf("reader: render text: %w", err)
	}
	if strings.TrimSpace(text.String()) == "" {
		return "", ErrNoContent
	}

	var out bytes.Buffer
	if err := article.RenderHTML(&out); err != nil {
		return "", fmt.Errorf("reader: render html: %w", err)
	}
	return out.String(), nil
}

// parseBase 解析用于相对链接的基地址。非法或非 http(s) 地址返回 nil，
// 库内部对 nil base 的处理是「原样保留相对链接」。
func parseBase(pageURL string) *url.URL {
	if strings.TrimSpace(pageURL) == "" {
		return nil
	}
	u, err := url.Parse(pageURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil
	}
	return u
}
