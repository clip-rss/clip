package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/fonts"
	"github.com/clip-rss/clip/internal/logging"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// 远程字体目录清单 URL：clip-font 仓库**最新 release** 里的 manifest.json。
//
// 用 release 资产而不是 main 分支那份：仓库里的 manifest 版本不固定，可能与已发布
// 的字体资产不一致（仓库更新了、还没打 tag，清单就会指向尚不存在的文件）。
// release 里的清单与它描述的那批资产是同一个 tag，天然自洽。
//
// ⚠️ 两份清单的 files[].file 形态不同：release 版带目录前缀（lxgw-wenkai/xxx.ttf），
// 而 release 资产是扁平裸名，直接拼接会 404。assetName() 统一取 basename 兜住这个差异。
const fontCatalogURL = "https://github.com/clip-rss/clip-font/releases/latest/download/manifest.json"

// FontFile 单个字体文件（一个字重）。
type FontFile struct {
	Style  string `json:"style,omitempty"` // OTF 专用变体名，如 Regular / ConRegular / Bold
	Weight int    `json:"weight"`          // 字重数值：300 / 400 / 500 / 700
	Format string `json:"format"`          // ttf | otf
	File   string `json:"file"`            // 清单内路径；拼下载地址前须经 assetName 取 basename
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

// fontCatalogVersion 是本构建期望的清单 schema 版本（clip-font 顶层 version 字段）。
//
// 上游只在「字段语义变化、旧 App 会解析坏」时递增它（约定见 clip-font 的 updateFont
// skill）。这里**只记录不拦截**：硬失败等于把发布方控制的字段变成字体库的远程开关 ——
// 上游只是递增版本、形状却没变时，功能会对所有未更新的 App 直接死掉，比它要解决的问题更糟。
const fontCatalogVersion = 1

// FontService 远程字体目录服务。
type FontService struct {
	fetch      *fetcher.Client
	dl         *http.Client
	catalogURL string
	// fontDir 覆盖落盘目录，仅测试用；空值时用 fonts.Dir()。
	fontDir string
}

// NewFontService 创建 FontService。fetch 用于拉清单（复用抓取的代理 / UA / 超时），
// dl 用于下字体资产（updatesrc 的弱网客户端），两者用途不同故分开注入。
func NewFontService(fetch *fetcher.Client, dl *http.Client) *FontService {
	return &FontService{fetch: fetch, dl: dl, catalogURL: fontCatalogURL}
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
	if cat.Version != fontCatalogVersion {
		// 不返回错误，理由见 fontCatalogVersion 的注释。日志是排查「某几个文件 404」
		// 类症状时唯一能说明「上游换 schema 了」的线索。
		logging.Printf("fonts", "catalog schema version %d, this build expects %d — continuing",
			cat.Version, fontCatalogVersion)
	}
	return &cat, nil
}

// FontDownloadProgressEvent 是字体下载进度事件名，前端 Events.ts 里有一份同名镜像。
const FontDownloadProgressEvent = "fonts:download:progress"

// DownloadFont 把指定字体族的全部字重下载到本地字体目录并逐个校验 SHA256。
// 已存在且校验通过的文件跳过，因此重复调用不会重下。
//
// 目录现取现用、不缓存：清单可能随 release 更新，缓存会让「装某款字体」和「看到它
// 在列表里」的版本错位。进度按累计字节数推送 FontDownloadProgressEvent。
func (s *FontService) DownloadFont(id string) error {
	cat, err := s.FetchFontCatalog()
	if err != nil {
		return err
	}
	var def *FontDef
	for i := range cat.Fonts {
		if cat.Fonts[i].ID == id {
			def = &cat.Fonts[i]
			break
		}
	}
	if def == nil {
		return fmt.Errorf("fonts: unknown font id %q", id)
	}

	files := make([]fonts.File, 0, len(def.Files))
	for _, f := range def.Files {
		name := assetName(f.File)
		if name == "" {
			return fmt.Errorf("fonts: %s has invalid file name %q", id, f.File)
		}
		files = append(files, fonts.File{
			URL:    strings.TrimSuffix(def.URL, "/") + "/" + name,
			Name:   name,
			SHA256: f.SHA256,
			Size:   f.Size,
		})
	}

	dir := s.fontDir
	if dir == "" {
		if dir, err = fonts.Dir(); err != nil {
			return err
		}
	}
	app := application.Get()
	return fonts.Download(context.Background(), s.dl, dir, files, func(done, total int64) {
		emitFontProgress(app, id, done, total)
	})
}

// assetName 取清单里 files[].file 的裸文件名。
//
// release 清单写的是带目录前缀的仓库内路径（lxgw-wenkai/LXGWWenKai-Regular.ttf），
// 而 release 资产是扁平裸名，只有 basename 才拼得出不会 404 的地址。
// 用 path 而非 filepath：这里处理的是 URL 路径，分隔符恒为 "/"，与宿主平台无关。
func assetName(file string) string {
	name := path.Base(strings.TrimSpace(file))
	if name == "." || name == ".." || name == "/" {
		return ""
	}
	if strings.ContainsAny(name, `/\`) {
		return ""
	}
	return name
}

// emitFontProgress 推一条下载进度事件。percent 由后端算好，前端不必处理 total 为 0
// （资产大小未知）时的兜底。与 emitRestoreProgress 同构。
func emitFontProgress(app *application.App, id string, downloaded, total int64) {
	if app == nil {
		return
	}
	percent := 0
	if total > 0 {
		percent = int(downloaded * 100 / total)
	}
	if percent > 100 {
		percent = 100
	}
	app.Event.Emit(FontDownloadProgressEvent, map[string]any{
		"id":         id,
		"downloaded": downloaded,
		"total":      total,
		"percent":    percent,
	})
}
