package reader

import (
	"errors"
	"strings"
	"testing"
)

// articlePage 是一份最小但结构完整的文章页：真实站点上 readability 依赖的
// 信号（<article> 容器、成段正文、导航与脚本噪声）都在，才能验出「提取出来的是
// 正文而不是整页」。
func articlePage(body string) string {
	return `<!doctype html>
<html><head><title>标题</title></head>
<body>
<nav><a href="/">首页</a><a href="/about">关于</a></nav>
<article>
  <h1>标题</h1>
  ` + body + `
</article>
<footer>版权所有</footer>
<script>tracker()</script>
</body></html>`
}

func TestExtractArticle(t *testing.T) {
	page := articlePage(`
  <p>第一段正文，讲清楚一件事。</p>
  <p>第二段正文，继续把这件事讲完。</p>
  <p>第三段收尾。</p>`)

	got, err := Extract([]byte(page), "https://example.com/post/1")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, want := range []string{"第一段正文", "第二段正文", "第三段收尾"} {
		if !strings.Contains(got, want) {
			t.Errorf("提取结果缺少正文段落 %q，得到：%s", want, got)
		}
	}
	// 导航与页脚属于噪声，不该混进正文。
	if strings.Contains(got, "版权所有") {
		t.Errorf("提取结果不应包含页脚：%s", got)
	}
}

func TestExtractResolvesRelativeLinks(t *testing.T) {
	// 相对链接必须解析成绝对地址，否则前端渲染时图片一律裂图。
	page := articlePage(`
  <p><img src="/img/a.png" alt="图"></p>
  <p>正文段落一。</p>
  <p>正文段落二。</p>`)

	got, err := Extract([]byte(page), "https://example.com/post/1")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.Contains(got, "https://example.com/img/a.png") {
		t.Errorf("相对图片地址未解析为绝对地址：%s", got)
	}
}

func TestExtractInvalidPageURLKeepsRelativeLinks(t *testing.T) {
	// URL 非法不该报错——正文本身仍有价值，只是链接解析不了。
	page := articlePage(`<p>正文段落一。</p><p>正文段落二。</p>`)

	got, err := Extract([]byte(page), "::not a url::")
	if err != nil {
		t.Fatalf("非法 URL 不应导致提取失败：%v", err)
	}
	if !strings.Contains(got, "正文段落一") {
		t.Errorf("非法 URL 时正文仍应提取出来：%s", got)
	}
}

func TestExtractEmptyInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		page []byte
	}{
		{"nil", nil},
		{"空白", []byte("   \n\t ")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Extract(tc.page, "https://example.com/"); !errors.Is(err, ErrNoContent) {
				t.Errorf("期望 ErrNoContent，得到 %v", err)
			}
		})
	}
}

func TestExtractPageWithoutArticle(t *testing.T) {
	// 首页/列表页：没有可识别的正文容器。允许两种结果——要么报 ErrNoContent，
	// 要么提出点东西；但绝不能 panic 或返回无意义的超长整页。
	page := `<!doctype html><html><head><title>站点首页</title></head>
<body><ul><li><a href="/a">文章 A</a></li><li><a href="/b">文章 B</a></li></ul></body></html>`

	got, err := Extract([]byte(page), "https://example.com/")
	if err != nil {
		if !errors.Is(err, ErrNoContent) {
			t.Fatalf("期望 ErrNoContent 或成功，得到 %v", err)
		}
		return
	}
	if len(got) > 4096 {
		t.Errorf("无正文页不应返回整页 HTML，长度 %d", len(got))
	}
}

func TestExtractNonHTMLResponse(t *testing.T) {
	// 非 HTML 响应（这里是 JSON）不该 panic。
	body := []byte(`{"error":"not found","code":404}`)
	if _, err := Extract(body, "https://example.com/api"); err != nil && !errors.Is(err, ErrNoContent) {
		t.Logf("非 HTML 输入返回了错误（可接受）：%v", err)
	}
}

func TestExtractGBKPage(t *testing.T) {
	// 中文站点常把 GBK 字节标成 UTF-8。Extract 需先按声明/嗅探转码，
	// 否则正文全是乱码（阶段 27 同类问题）。
	utf8Text := "这是一段中文正文，用于验证编码转换是否正确。"
	// GBK 编码的等价字节。
	gbk := []byte{
		0xd5, 0xe2, 0xca, 0xc7, 0xd2, 0xbb, 0xb6, 0xce, 0xd6, 0xd0,
		0xce, 0xc4, 0xd5, 0xfd, 0xce, 0xc4, 0xa3, 0xac, 0xd3, 0xc3,
		0xd3, 0xda, 0xd1, 0xe9, 0xd6, 0xa4, 0xb1, 0xe0, 0xc2, 0xeb,
		0xd7, 0xaa, 0xbb, 0xbb, 0xca, 0xc7, 0xb7, 0xf1, 0xd5, 0xfd,
		0xc8, 0xb7, 0xa1, 0xa3,
	}
	page := append([]byte(`<!doctype html><html><head><meta charset="gbk"><title>编码</title></head><body><article><p>`), gbk...)
	page = append(page, []byte(`</p><p>第二段补足长度，避免提取器判定内容过短。</p></article></body></html>`)...)

	got, err := Extract(page, "https://example.com/gbk")
	if err != nil {
		t.Fatalf("GBK 页面提取失败：%v", err)
	}
	if !strings.Contains(got, utf8Text) {
		t.Errorf("GBK 正文未正确转码，得到：%s", got)
	}
}
