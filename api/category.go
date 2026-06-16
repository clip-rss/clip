package api

import (
	"errors"
	"strings"

	"github.com/clip-rss/clip/internal/store"
)

// CategoryService 分类管理相关的绑定方法。
type CategoryService struct {
	store *store.Store
}

// NewCategoryService 创建 CategoryService。
func NewCategoryService(st *store.Store) *CategoryService {
	return &CategoryService{store: st}
}

// AddCategory 新增分类。parentID 为 0 表示根分类。
func (s *CategoryService) AddCategory(name string, parentID int64) (*store.Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("category name is empty")
	}
	cat := &store.Category{Name: name, ParentID: nullableID(parentID)}
	if err := s.store.CreateCategory(cat); err != nil {
		return nil, err
	}
	return cat, nil
}

// UpdateCategory 更新分类（名称、父级、排序）。
func (s *CategoryService) UpdateCategory(category store.Category) error {
	return s.store.UpdateCategory(&category)
}

// DeleteCategory 删除分类（级联删除子分类，其下订阅源置为未分类）。
func (s *CategoryService) DeleteCategory(id int64) error {
	return s.store.DeleteCategory(id)
}

// ListCategories 列出全部分类。
func (s *CategoryService) ListCategories() ([]store.Category, error) {
	return s.store.ListCategories()
}

// GetCategoryWithFeeds 获取分类及其下订阅源（含未读计数）。
func (s *CategoryService) GetCategoryWithFeeds(id int64) (*store.CategoryWithFeeds, error) {
	return s.store.GetCategoryWithFeeds(id)
}

// GetUncategorizedFeeds 获取未分类的订阅源。
func (s *CategoryService) GetUncategorizedFeeds() ([]store.FeedWithUnread, error) {
	return s.store.GetUncategorizedFeeds()
}

// MoveToCategory 将订阅源移动到指定分类。categoryID 为 0 表示移出分类（未分类）。
func (s *CategoryService) MoveToCategory(feedID int64, categoryID int64) error {
	return s.store.MoveFeedToCategory(feedID, nullableID(categoryID))
}
