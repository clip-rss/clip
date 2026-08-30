package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// 导入进度事件推送间隔（每 N 条 feed 推送一次）。
// 仅在有 emitter 时生效。
const importProgressInterval = 50

// OPMLImportProgressEvent OPML 导入进度事件名（前端用）。
const OPMLImportProgressEvent = "opml:import:progress"

// opmlFetchTimeout 远程拉取 OPML 的整体超时（含 Client 内部的退避重试）。
const opmlFetchTimeout = 30 * time.Second

// OPMLService OPML 导入导出相关的绑定方法。
type OPMLService struct {
	store   *store.Store
	http    *fetcher.Client
	emitter func(name string, data any) // 进度事件推送，nil 时不推送
}

// NewOPMLService 创建 OPMLService。
func NewOPMLService(st *store.Store, client *fetcher.Client) *OPMLService {
	return &OPMLService{store: st, http: client}
}

// WithEmitter 设置进度事件推送函数。参数签名与 scheduler.Emitter.Emit 一致。
func (s *OPMLService) WithEmitter(emit func(name string, data any)) *OPMLService {
	s.emitter = emit
	return s
}

// ImportResult 导入结果统计。
type ImportResult struct {
	Categories int `json:"categories"` // 新建分类数
	Feeds      int `json:"feeds"`      // 新增订阅源数
	Skipped    int `json:"skipped"`    // 已存在而跳过的订阅源数

	// 新建的订阅源与分类详情，供前端增量追加（无需全量 reload）。
	NewFeeds     []NewFeed     `json:"newFeeds"`
	NewCategories []NewCategory `json:"newCategories"`
}

// NewFeed 新建订阅源概要（供前端增量追加到 Store）。
type NewFeed struct {
	ID             int64  `json:"id"`
	URL            string `json:"url"`
	Title          string `json:"title"`
	Link           string `json:"link"`
	CategoryID     *int64 `json:"categoryId"`
	UpdateInterval int    `json:"updateInterval"`
	MaxItems       int    `json:"maxItems"`
	Status         string `json:"status"`
}

// NewCategory 新建分类概要（供前端增量追加到 Store）。
type NewCategory struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ParentID *int64 `json:"parentId"`
}

// ImportOPML 解析 OPML 文本并导入分类与订阅源。
// 导入仅根据 OPML 元信息建源，不发起网络抓取；文章将在下次调度或手动刷新时拉取。
// 整个导入过程在一个数据库事务中完成。
func (s *OPMLService) ImportOPML(content string) (ImportResult, error) {
	if strings.TrimSpace(content) == "" {
		return ImportResult{}, errors.New(i18n.T(backendLanguage(s.store), "opml.contentEmpty"))
	}
	doc, err := opml.Parse([]byte(content))
	if err != nil {
		return ImportResult{}, err
	}

	// 在事务外读取设置与语言，避免与事务争用单 SQLite 连接。
	settings, _ := s.store.GetSettings()
	lang := backendLanguage(s.store)

	tx, err := s.store.Begin()
	if err != nil {
		return ImportResult{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // no-op after Commit

	res := ImportResult{}
	total := countFeeds(doc.Body.Outlines)
	feedBatch := make([]store.Feed, 0, importProgressInterval)
	var newFeeds []NewFeed
	var newCategories []NewCategory
	if err := s.importOutlines(tx, doc.Body.Outlines, nil, lang, settings, &res, 0, total, &feedBatch, &newFeeds, &newCategories); err != nil {
		return res, err
	}
	// 刷新剩余批次。
	if len(feedBatch) > 0 {
		if err := store.BulkCreateFeeds(tx, feedBatch); err != nil {
			return res, err
		}
		res.Feeds += len(feedBatch)
		for i := range feedBatch {
			newFeeds = append(newFeeds, feedToNewFeed(feedBatch[i]))
		}
	}
	res.NewFeeds = newFeeds
	res.NewCategories = newCategories
	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("failed to commit transaction: %w", err)
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

// countFeeds 递归统计 outline 树中的 feed 节点总数。
func countFeeds(outlines []opml.Outline) int {
	n := 0
	for _, o := range outlines {
		if o.IsFeed() {
			n++
		} else {
			n += countFeeds(o.Outlines)
		}
	}
	return n
}

// importOutlines 递归导入大纲：feed 节点建源，分组节点建分类后递归其子节点。
// 所有数据库操作使用同一个事务 tx。新 feed 暂存到 feedBatch，积累至 importProgressInterval
// 条后执行一次批量写入并将结果追加到 newFeeds。
// processed 是已处理的 feed 数（不含分类），用于进度计算。
func (s *OPMLService) importOutlines(tx *sql.Tx, outlines []opml.Outline, parentID *int64, lang string, settings store.Settings, res *ImportResult, processed int, total int, feedBatch *[]store.Feed, newFeeds *[]NewFeed, newCategories *[]NewCategory) error {
	for _, o := range outlines {
		if o.IsFeed() {
			url := strings.TrimSpace(o.XMLURL)
			existing, err := store.TxGetFeedByURL(tx, url)
			if err != nil {
				return err
			}
			if existing != nil {
				res.Skipped++
			} else {
				*feedBatch = append(*feedBatch, store.Feed{
					URL:            url,
					Title:          firstNonEmpty(o.Label(), url),
					Link:           o.HTMLURL,
					CategoryID:     parentID,
					UpdateInterval: settings.DefaultUpdateInterval,
					MaxItems:       settings.DefaultMaxItems,
					Status:         "active",
				})
			}
			processed++

			// 批次满了就 flush。
			if len(*feedBatch) >= importProgressInterval {
				if err := store.BulkCreateFeeds(tx, *feedBatch); err != nil {
					return err
				}
				res.Feeds += len(*feedBatch)
				for i := range *feedBatch {
					*newFeeds = append(*newFeeds, feedToNewFeed((*feedBatch)[i]))
				}
				*feedBatch = (*feedBatch)[:0]
			}

			if processed%importProgressInterval == 0 || processed == total {
				s.emitProgress(res, processed, total)
			}
			continue
		}

		// 分组节点 → 分类。
		cat := &store.Category{Name: firstNonEmpty(o.Label(), i18n.T(lang, "opml.unnamedCategory")), ParentID: parentID}
		if err := store.TxCreateCategory(tx, cat); err != nil {
			return err
		}
		*newCategories = append(*newCategories, NewCategory{
			ID:       cat.ID,
			Name:     cat.Name,
			ParentID: cat.ParentID,
		})
		res.Categories++
		if err := s.importOutlines(tx, o.Outlines, &cat.ID, lang, settings, res, processed, total, feedBatch, newFeeds, newCategories); err != nil {
			return err
		}
	}
	return nil
}

// feedToNewFeed 将 store.Feed 转为前端可消费的 NewFeed。
func feedToNewFeed(f store.Feed) NewFeed {
	return NewFeed{
		ID:             f.ID,
		URL:            f.URL,
		Title:          f.Title,
		Link:           f.Link,
		CategoryID:     f.CategoryID,
		UpdateInterval: f.UpdateInterval,
		MaxItems:       f.MaxItems,
		Status:         f.Status,
	}
}

// emitProgress 推导入进度事件（emitter 为 nil 时不推）。
func (s *OPMLService) emitProgress(res *ImportResult, processed, total int) {
	if s.emitter == nil {
		return
	}
	s.emitter(OPMLImportProgressEvent, map[string]any{
		"processed":  processed,
		"total":      total,
		"feeds":      res.Feeds,
		"skipped":    res.Skipped,
		"categories": res.Categories,
	})
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
