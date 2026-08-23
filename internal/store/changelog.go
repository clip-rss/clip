package store

// changelogCacheKey 更新日志缓存在 settings 表中的键名。
const changelogCacheKey = "changelog_cache"

// ChangelogCache 缓存一次成功抓取到的更新日志原文。
//
// changelogURL 指向仓库 main 分支的 CHANGELOG.md，内容始终是「最新已发布版本」的日志。
// 因此只要本地版本已是最新（更新检查确认无新版），远端内容就是稳定的，可以直接复用；
// 一旦检出新版，远端日志必然已变，缓存即失效。Version 记录抓取时的应用版本号，
// 用于在升级后丢弃上一版的缓存。
//
// 存原始 Markdown 而不是渲染后的 HTML：渲染是前端 markdownToHtml 的纯函数变换，
// 缓存 HTML 会让渲染逻辑改动后仍吐出按旧规则生成的结构。
type ChangelogCache struct {
	Version  string `json:"version"`  // 抓取时的应用版本号
	Markdown string `json:"markdown"` // 更新日志原始 Markdown 文本
}

// GetChangelogCache 读取更新日志缓存。
//
// 第二个返回值表示缓存是否存在：不存在时返回零值 + false + nil，属正常初始状态。
// 值损坏（无法解析）时返回错误，由调用方决定是否忽略后重新抓取。
func (s *Store) GetChangelogCache() (ChangelogCache, bool, error) {
	var cache ChangelogCache
	found, err := s.GetJSONSetting(changelogCacheKey, &cache)
	if err != nil {
		return ChangelogCache{}, false, err
	}
	if !found {
		return ChangelogCache{}, false, nil
	}
	return cache, true, nil
}

// SaveChangelogCache 保存更新日志缓存（整体覆盖）。
func (s *Store) SaveChangelogCache(cache ChangelogCache) error {
	return s.SetJSONSetting(changelogCacheKey, cache)
}
