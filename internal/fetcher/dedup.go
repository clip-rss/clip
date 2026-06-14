package fetcher

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

// Fingerprint 计算文章的稳定去重指纹。
// 优先使用 guid；guid 为空时退化为 link + title 的哈希指纹。
// 既无 guid 又无 link 的条目无法稳定标识，返回空串（调用方应丢弃）。
func Fingerprint(item ParsedItem) string {
	if g := strings.TrimSpace(item.GUID); g != "" {
		return g
	}
	link := strings.TrimSpace(item.Link)
	if link == "" {
		return ""
	}
	title := strings.TrimSpace(item.Title)
	sum := sha1.Sum([]byte(link + "\x00" + title))
	return "fp:" + hex.EncodeToString(sum[:])
}

// Dedup 移除一批文章中的重复项，保留首次出现的条目，顺序不变。
// 指纹为空（无法识别）的条目一并丢弃。
func Dedup(items []ParsedItem) []ParsedItem {
	seen := make(map[string]struct{}, len(items))
	out := make([]ParsedItem, 0, len(items))
	for _, it := range items {
		fp := Fingerprint(it)
		if fp == "" {
			continue
		}
		if _, ok := seen[fp]; ok {
			continue
		}
		seen[fp] = struct{}{}
		out = append(out, it)
	}
	return out
}
