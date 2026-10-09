package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/clip-rss/clip/internal/fetcher"
)

// 远程字体目录清单 URL（clip-font 仓库 main 分支）。
//
// 必须用 main 分支这版：release 资产里附带的那份 manifest.json 会把 files[].file 写成
// 带目录前缀（如 lxgw-wenkai/LXGWWenKai-Regular.ttf），与 manifest.url 拼接出的地址
// 在 GitHub release asset 上是 404；main 分支这份是裸文件名，配合下载地址才有效。
const fontCatalogURL = "https://raw.githubusercontent.com/clip-rss/clip-font/main/manifest.json"

// FontFile 单个字体文件（一个字重）。
type FontFile struct {
	Style  string `json:"style,omitempty"` // OTF 专用变体名，如 Regular / ConRegular / Bold
	Weight int    `json:"weight"`          // 字重数值：300 / 400 / 500 / 700
	Format string `json:"format"`          // ttf | otf
	File   string `json:"file"`            // 文件 basename，配合 FontDef.URL 拼下载地址
	Size   int64  `json:"size"`            // 字节数
	SHA256 string `json:"sha256"`          // 下载后校验用
}

// FontDef 一个可安装的字体族。
type FontDef struct {
	ID          string     `json:"id"`          // 稳定标识，如 lxgw-wenkai
	Name        string     `json:"name"`        // 显示名（中文），如「霞鹜文楷」
	Family      string     `json:"family"`      // postscript family，如 LXGW WenKai
	Version     string     `json:"version"`     // 字体版本号，非仓库 tag
	License     string     `json:"license"`     // 许可证名，如 SIL OFL 1.1
	LicenseFile string     `json:"licenseFile"` // 仓库内许可证文本路径
	URL         string     `json:"url"`         // 下载前缀（…/releases/latest/download）
	Files       []FontFile `json:"files"`       // 该族的字重文件
}

// FontCatalog 字体仓库的目录清单。
type FontCatalog struct {
	Version int       `json:"version"`
	Release string    `json:"release"`
	Fonts   []FontDef `json:"fonts"`
}

// FontService 远程字体目录服务。
type FontService struct {
	fetch      *fetcher.Client
	catalogURL string
}

// NewFontService 创建 FontService。复用抓取客户端，用户配置的代理 / UA / 超时一致生效。
func NewFontService(fetch *fetcher.Client) *FontService {
	return &FontService{fetch: fetch, catalogURL: fontCatalogURL}
}

// FetchFontCatalog 拉取并解析远程字体目录清单。
func (s *FontService) FetchFontCatalog() (*FontCatalog, error) {
	body, _, err := s.fetch.Get(context.Background(), s.catalogURL)
	if err != nil {
		return nil, err
	}
	var cat FontCatalog
	if err := json.Unmarshal(body, &cat); err != nil {
		return nil, fmt.Errorf("fonts: parse catalog: %w", err)
	}
	return &cat, nil
}
