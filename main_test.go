package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestUpdaterI18nInjection 验证更新窗口的 i18n 注入：占位符被替换、字典是合法 JSON、
// 且 en/zh 两语言的 "updater" key 集合一致（防止本地化漏项 / locale 结构漂移）。
func TestUpdaterI18nInjection(t *testing.T) {
	dict := updaterI18nDict()

	// 字典应是 {"en":{...},"zh":{...}} 的合法 JSON。
	var parsed map[string]map[string]interface{}
	if err := json.Unmarshal([]byte(dict), &parsed); err != nil {
		t.Fatalf("dict 不是合法 JSON: %v", err)
	}
	en, ok := parsed["en"]
	if !ok || len(en) == 0 {
		t.Fatal("dict 缺少非空 en 段")
	}
	zh, ok := parsed["zh"]
	if !ok || len(zh) == 0 {
		t.Fatal("dict 缺少非空 zh 段")
	}

	// en/zh 的 key 必须一致（递归比较支持嵌套结构）。
	var compareKeys func(path string, a, b map[string]interface{})
	compareKeys = func(path string, a, b map[string]interface{}) {
		for k := range a {
			fullKey := path + k
			if _, ok := b[k]; !ok {
				t.Errorf("zh 缺少 key %q", fullKey)
				continue
			}
			// 如果值是嵌套对象，递归比较
			if aMap, aOk := a[k].(map[string]interface{}); aOk {
				if bMap, bOk := b[k].(map[string]interface{}); bOk {
					compareKeys(fullKey+".", aMap, bMap)
				}
			}
		}
		for k := range b {
			fullKey := path + k
			if _, ok := a[k]; !ok {
				t.Errorf("en 缺少 key %q", fullKey)
			}
		}
	}
	compareKeys("", en, zh)

	// 注入后占位符应全部消失，且选定语言出现在结果中。
	html := buildSoftwareUpdateHTML(dict, "zh")
	if strings.Contains(html, updI18nDictMarker) {
		t.Errorf("注入后仍残留字典占位符 %s", updI18nDictMarker)
	}
	if strings.Contains(html, updI18nLangMarker) {
		t.Errorf("注入后仍残留语言占位符 %s", updI18nLangMarker)
	}
	if !strings.Contains(html, `var lang = "zh"`) {
		t.Error("注入后未见 var lang = \"zh\"")
	}
	// 抽查一个已知 key 的中文文案确实进入了 HTML（检查顶级 key）。
	if zhClose, ok := zh["close"].(string); ok && zhClose != "" && !strings.Contains(html, zhClose) {
		t.Errorf("注入后未见 zh.close 文案 %q", zhClose)
	}
}

// TestCleanOrphanedUpdateDirs 验证孤儿更新包清理逻辑。
func TestCleanOrphanedUpdateDirs(t *testing.T) {
	// 创建临时测试目录（模拟系统临时目录）
	testTempDir := t.TempDir()

	now := time.Now()

	// 创建测试用的 wails-update-* 目录
	dirs := []struct {
		name    string
		age     time.Duration
		wantDel bool
	}{
		{"wails-update-newest", 0, false},                 // 最新的，保留
		{"wails-update-recent", 12 * time.Hour, false},    // 12h 内，保留
		{"wails-update-old-1", 25 * time.Hour, true},      // 超过 24h 且非最新，删除
		{"wails-update-old-2", 48 * time.Hour, true},      // 超过 24h 且非最新，删除
		{"not-wails-update", 50 * time.Hour, false},       // 非 wails-update-* 前缀，不处理
		{"wails-update-boundary", 24*time.Hour + 1, true}, // 刚超过 24h，删除
	}

	for _, d := range dirs {
		dirPath := filepath.Join(testTempDir, d.name)
		if err := os.Mkdir(dirPath, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d.name, err)
		}
		// 设置修改时间
		modTime := now.Add(-d.age)
		if err := os.Chtimes(dirPath, modTime, modTime); err != nil {
			t.Fatalf("chtimes %s: %v", d.name, err)
		}
	}

	// 执行清理
	cleanOrphanedUpdateDirsIn(testTempDir)

	// 验证结果
	for _, d := range dirs {
		dirPath := filepath.Join(testTempDir, d.name)
		_, err := os.Stat(dirPath)
		exists := err == nil

		if d.wantDel && exists {
			t.Errorf("dir %s should be deleted but still exists", d.name)
		}
		if !d.wantDel && !exists {
			t.Errorf("dir %s should be kept but was deleted", d.name)
		}
	}
}

// TestCleanOrphanedUpdateDirs_NoDirectories 验证没有孤儿目录时不报错。
func TestCleanOrphanedUpdateDirs_NoDirectories(t *testing.T) {
	testTempDir := t.TempDir()

	// 不创建任何目录，直接清理
	cleanOrphanedUpdateDirsIn(testTempDir) // 应该不 panic 或报错
}

// TestCleanOrphanedUpdateDirs_OnlyNewest 验证只有 1 个目录时不删除。
func TestCleanOrphanedUpdateDirs_OnlyNewest(t *testing.T) {
	testTempDir := t.TempDir()

	// 创建一个超过 24h 的目录
	oldDir := filepath.Join(testTempDir, "wails-update-old")
	if err := os.Mkdir(oldDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	modTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldDir, modTime, modTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	// 执行清理
	cleanOrphanedUpdateDirsIn(testTempDir)

	// 验证：因为只有 1 个目录（是最新的），应该保留
	if _, err := os.Stat(oldDir); os.IsNotExist(err) {
		t.Errorf("the only directory should be kept as newest")
	}
}


