package api

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/i18n"
	"github.com/clip-rss/clip/internal/opml"
	"github.com/clip-rss/clip/internal/store"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// opmlFetchTimeout 远程拉取 OPML 的整体超时（含 Client 内部的退避重试）。
const opmlFetchTimeout = 30 * time.Second

// OPMLService OPML 导入导出相关的绑定方法。
type OPMLService struct {
	store *store.Store
	// http 用于拉取远程 OPML 文件。复用抓取用的客户端以共享超时、重试、代理与响应体上限；
	// 为 nil 时（如单测）远程导入不可用，本地导入不受影响。
	http *fetcher.Client
}

// NewOPMLService 创建 OPMLService。
func NewOPMLService(st *store.Store, client *fetcher.Client) *OPMLService {
	return &OPMLService{store: st, http: client}
}

// ImportResult 导入结果统计。
type ImportResult struct {
	Categories int `json:"categories"` // 新建分类数
	Feeds      int `json:"feeds"`      // 新增订阅源数
	Skipped    int `json:"skipped"`    // 已存在而跳过的订阅源数
}

// ImportOPML 解析 OPML 文本并导入分类与订阅源。
// 导入仅根据 OPML 元信息建源，不发起网络抓取；文章将在下次调度或手动刷新时拉取。
func (s *OPMLService) ImportOPML(content string) (ImportResult, error) {
	if strings.TrimSpace(content) == "" {
		return ImportResult{}, errors.New(i18n.T(backendLanguage(s.store), "opml.contentEmpty"))
	}
	doc, err := opml.Parse([]byte(content))
	if err != nil {
		return ImportResult{}, err
	}

	settings, _ := s.store.GetSettings()
	res := ImportResult{}
	if err := s.importOutlines(doc.Body.Outlines, nil, settings, &res); err != nil {
		return res, err
	}
	return res, nil
}

// ImportOPMLFromURL 拉取远程 OPML 文件并导入。
//
// 与 ImportOPML 只差内容来源：取到文本后交给 ImportOPML 走同一条导入路径，
// 因此解析、建分类、去重跳过的语义完全一致。
//
// 用 Client.Fetch 而非 Client.Get：前者的 Accept 头含 application/xml、text/xml
// （OPML 是 XML），且 10 MiB 的响应上限对订阅列表足够；后者面向图片下载。
func (s *OPMLService) ImportOPMLFromURL(rawURL string) (ImportResult, error) {
	lang := backendLanguage(s.store)
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ImportResult{}, errors.New(i18n.T(lang, "opml.urlEmpty"))
	}
	if !isHTTPURL(rawURL) {
		return ImportResult{}, errors.New(i18n.T(lang, "opml.urlInvalid"))
	}
	if s.http == nil {
		return ImportResult{}, errors.New(i18n.T(lang, "opml.clientUnavailable"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), opmlFetchTimeout)
	defer cancel()
	res, err := s.http.Fetch(ctx, rawURL, fetcher.ConditionalHeaders{})
	if err != nil {
		return ImportResult{}, i18n.Error(lang, "opml.fetchFailed", err)
	}
	// 空响应（含服务端无视空校验头硬回 304 的情形）交给 ImportOPML 的空内容分支报错。
	return s.ImportOPML(string(res.Body))
}

// isHTTPURL 仅接受带主机名的 http/https 地址，挡掉 file:// 等本地协议与相对路径。
func isHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// importOutlines 递归导入大纲：feed 节点建源，分组节点建分类后递归其子节点。
func (s *OPMLService) importOutlines(outlines []opml.Outline, parentID *int64, settings store.Settings, res *ImportResult) error {
	for _, o := range outlines {
		if o.IsFeed() {
			url := strings.TrimSpace(o.XMLURL)
			if existing, _ := s.store.GetFeedByURL(url); existing != nil {
				res.Skipped++
				continue
			}
			feed := &store.Feed{
				URL:            url,
				Title:          firstNonEmpty(o.Label(), url),
				Link:           o.HTMLURL,
				CategoryID:     parentID,
				UpdateInterval: settings.DefaultUpdateInterval,
				MaxItems:       settings.DefaultMaxItems,
				Status:         "active",
			}
			if err := s.store.CreateFeed(feed); err != nil {
				return err
			}
			res.Feeds++
			continue
		}

		// 分组节点 → 分类。
		cat := &store.Category{Name: firstNonEmpty(o.Label(), i18n.T(backendLanguage(s.store), "opml.unnamedCategory")), ParentID: parentID}
		if err := s.store.CreateCategory(cat); err != nil {
			return err
		}
		res.Categories++
		if err := s.importOutlines(o.Outlines, &cat.ID, settings, res); err != nil {
			return err
		}
	}
	return nil
}

// ExportOPML 将当前全部分类与订阅源导出为 OPML：弹出系统保存对话框选择位置后写盘。
// 用户取消返回 (false, nil)；成功返回 (true, nil)。
func (s *OPMLService) ExportOPML() (bool, error) {
	out, err := s.buildOPML()
	if err != nil {
		return false, err
	}
	app := application.Get()
	if app == nil {
		return false, errors.New(i18n.T(backendLanguage(s.store), "app.unavailable"))
	}
	dest, err := app.Dialog.SaveFile().
		SetMessage(i18n.T(backendLanguage(s.store), "opml.export")).
		SetFilename("clip-feeds.opml").
		AddFilter(i18n.T(backendLanguage(s.store), "opml.fileFilter"), "*.opml").
		PromptForSingleSelection()
	if err != nil {
		return false, err
	}
	if dest == "" {
		return false, nil // 用户取消
	}
	if err := os.WriteFile(dest, []byte(out), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// buildOPML 生成当前订阅的 OPML 文本。
func (s *OPMLService) buildOPML() (string, error) {
	cats, err := s.store.ListCategories()
	if err != nil {
		return "", err
	}
	feeds, err := s.store.ListFeeds()
	if err != nil {
		return "", err
	}

	// 按分类聚合订阅源。
	feedsByCat := map[int64][]store.Feed{}
	var uncategorized []store.Feed
	for _, f := range feeds {
		if f.CategoryID != nil {
			feedsByCat[*f.CategoryID] = append(feedsByCat[*f.CategoryID], f)
		} else {
			uncategorized = append(uncategorized, f)
		}
	}

	// 构建分类树（parentID -> 子分类）。
	childCats := map[int64][]store.Category{}
	var roots []store.Category
	for _, c := range cats {
		if c.ParentID != nil {
			childCats[*c.ParentID] = append(childCats[*c.ParentID], c)
		} else {
			roots = append(roots, c)
		}
	}

	var build func(cat store.Category) opml.Outline
	build = func(cat store.Category) opml.Outline {
		o := opml.Outline{Text: cat.Name, Title: cat.Name}
		for _, child := range childCats[cat.ID] {
			o.Outlines = append(o.Outlines, build(child))
		}
		for _, f := range feedsByCat[cat.ID] {
			o.Outlines = append(o.Outlines, feedOutline(f))
		}
		return o
	}

	doc := &opml.OPML{Head: opml.Head{Title: "Clip Feeds"}}
	for _, c := range roots {
		doc.Body.Outlines = append(doc.Body.Outlines, build(c))
	}
	for _, f := range uncategorized {
		doc.Body.Outlines = append(doc.Body.Outlines, feedOutline(f))
	}

	out, err := opml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// BuildOPML 生成当前订阅的 OPML 文本（导出给 opmlbackup 使用）。
func (s *OPMLService) BuildOPML() (string, error) {
	return s.buildOPML()
}

// feedOutline 将订阅源映射为 OPML 大纲节点。
func feedOutline(f store.Feed) opml.Outline {
	return opml.Outline{
		Text:    f.Title,
		Title:   f.Title,
		Type:    "rss",
		XMLURL:  f.URL,
		HTMLURL: f.Link,
	}
}
