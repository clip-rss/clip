// Package fonts 负责远程字体库资产的下载与落盘。
//
// 只做「把文件安全地放到本地目录」，不涉及系统字体注册、也不涉及前端生效 —— 那些
// 各自有独立的失败面（注册需要平台 API，生效需要排版白名单放行）。
//
// 下载走 updatesrc.Download，而不是 fetcher.Client：字体单字重 25–32 MB，
// fetcher 的 http.Client 带整体超时且把响应体读进内存，在弱网下必然失败。
// 详见 internal/updatesrc/client.go 的包注释。
package fonts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/clip-rss/clip/internal/updatesrc"
)

// File 描述一个待下载的字体文件。
type File struct {
	// URL 是直接下载地址，调用方需已完成 URL 拼接与文件名归一化。
	URL string
	// Name 是落盘文件名，必须是裸文件名（不含路径分隔符）。
	Name string
	// SHA256 是期望的十六进制摘要。为空时报错，不做「跳过校验」的静默降级。
	SHA256 string
	// Size 是期望字节数，0 表示未知（不校验，也不进入进度分母）。
	Size int64
}

// DirName 是字体目录名，与数据库同处一个配置目录下。
const DirName = "fonts"

// Dir 返回字体目录 <configDir>/clip/fonts，不存在则创建。
func Dir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("fonts: locate config dir: %w", err)
	}
	dir := filepath.Join(configDir, "clip", DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("fonts: create font dir: %w", err)
	}
	return dir, nil
}

// Download 把 files 依次下载到 <dir>/<family>。family 是按字体族分目录的稳定目录名
// （用清单里的 id，不用字族显示名 —— 后者含空格与大写，且与 id 非一一对应）。
// 已存在且校验通过的文件跳过，其余逐个「下载到临时文件 → 校验 → 原子改名」，
// 中途失败不会在目录里留下半个文件。
//
// onProgress 报告的是**跨文件累计**已写字节数与总量（总量未知时为 0），可为 nil。
func Download(ctx context.Context, client *http.Client, dir, family string, files []File, onProgress func(done, total int64)) error {
	if !validName(family) {
		return fmt.Errorf("fonts: invalid family name %q", family)
	}
	if len(files) == 0 {
		return fmt.Errorf("fonts: no files to download")
	}
	familyDir := filepath.Join(dir, family)
	if err := os.MkdirAll(familyDir, 0o755); err != nil {
		return fmt.Errorf("fonts: create family dir: %w", err)
	}
	total := int64(0)
	for _, f := range files {
		if f.Size > 0 {
			total += f.Size
		}
	}

	var base int64 // 已完成文件的字节数，作为当前文件进度的起点
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		// 文件名必须是裸名。不能只靠 filepath.Base 判定：它在 macOS/Linux 上不把 `\`
		// 当分隔符，`sub\a.ttf` 会原样返回，而在 Windows 上那是一个真实路径。
		if !validName(f.Name) {
			return fmt.Errorf("fonts: invalid file name %q", f.Name)
		}
		target := filepath.Join(familyDir, f.Name)

		// 已下载且校验通过：跳过。这让「重试安装」不必重下 80 MB。
		if verify(target, f) == nil {
			base += f.Size
			reportProgress(onProgress, base, total)
			continue
		}

		if err := downloadOne(ctx, client, familyDir, target, f, func(written int64) {
			reportProgress(onProgress, base+written, total)
		}); err != nil {
			return err
		}
		base += f.Size
		reportProgress(onProgress, base, total)
	}
	return nil
}

// downloadOne 下载单个文件到 target。写入落在同目录的临时文件里，校验通过后才改名，
// 因此 target 要么是完整的旧文件、要么是完整的新文件，不会是半截。
func downloadOne(
	ctx context.Context,
	client *http.Client,
	dir, target string,
	f File,
	onWritten func(int64),
) error {
	tmp, err := os.CreateTemp(dir, ".font-*.part")
	if err != nil {
		return fmt.Errorf("fonts: create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	// 任一失败路径都要清掉临时文件；成功路径改名后 removed 已为 false。
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	hasher := sha256.New()
	dst := io.MultiWriter(tmp, hasher)
	if err := updatesrc.Download(ctx, client, f.URL, f.Size, dst, func(written, _ int64) {
		onWritten(written)
	}); err != nil {
		return fmt.Errorf("fonts: download %s: %w", f.Name, err)
	}

	got := hex.EncodeToString(hasher.Sum(nil))
	if f.SHA256 == "" {
		return fmt.Errorf("fonts: %s has no expected checksum", f.Name)
	}
	if !strings.EqualFold(got, f.SHA256) {
		return fmt.Errorf("fonts: %s checksum mismatch: got %s, want %s", f.Name, got, f.SHA256)
	}
	if f.Size > 0 {
		if info, err := tmp.Stat(); err != nil {
			return fmt.Errorf("fonts: stat temp file: %w", err)
		} else if info.Size() != f.Size {
			return fmt.Errorf("fonts: %s size mismatch: got %d, want %d", f.Name, info.Size(), f.Size)
		}
	}
	// Windows 上 rename 到已存在的目标会失败；旧文件此时已确认校验不通过，直接删。
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("fonts: remove stale %s: %w", f.Name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("fonts: close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return fmt.Errorf("fonts: install %s: %w", f.Name, err)
	}
	committed = true
	return nil
}

// verify 校验 path 处已有的文件；通过返回 nil。Size 为 0 时只比对摘要。
func verify(path string, f File) error {
	if f.SHA256 == "" {
		return fmt.Errorf("fonts: %s has no expected checksum", f.Name)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if f.Size > 0 {
		if info, err := file.Stat(); err != nil {
			return err
		} else if info.Size() != f.Size {
			return fmt.Errorf("fonts: %s size mismatch", f.Name)
		}
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	if got := hex.EncodeToString(hasher.Sum(nil)); !strings.EqualFold(got, f.SHA256) {
		return fmt.Errorf("fonts: %s checksum mismatch", f.Name)
	}
	return nil
}

func reportProgress(onProgress func(done, total int64), done, total int64) {
	if onProgress != nil {
		onProgress(done, total)
	}
}

// validName 校验族目录名与文件名：必须是单段裸名，挡 `..`、路径分隔符与绝对路径。
// 族 id 与文件名都来自远端清单，download / list / remove 三个入口共用这一关。
// 不能用 filepath.Base 单独判定：它在 macOS/Linux 上不把 `\` 当分隔符，
// `sub\a.ttf` 会原样返回，而在 Windows 上那是一个真实路径。
func validName(name string) bool {
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) ||
		name == "." || name == ".." {
		return false
	}
	return true
}

// InstalledFile 已安装字体族里的单个文件。
type InstalledFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// Installed 已安装的一个字体族。安装清单靠扫描目录得出，不落安装元数据：
// 删目录即卸载，无状态、无迁移；代价是无法显示「有新版本可升级」，本次不做。
type Installed struct {
	ID        string          `json:"id"`
	Files     []InstalledFile `json:"files"`
	TotalSize int64           `json:"totalSize"`
}

// List 扫描 <dir> 的一级子目录，把每个目录当作一个已安装字体族。
// 只统计通过 validName 校验的目录名；<dir> 下的散文件（旧扁平布局残留）忽略。
// 结果按 ID 排序保证稳定。目录不存在时返回空列表（等价于「什么都没装」）。
func List(dir string) ([]Installed, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("fonts: scan %s: %w", dir, err)
	}
	var out []Installed
	for _, e := range entries {
		if !e.IsDir() || !validName(e.Name()) {
			continue
		}
		files, err := os.ReadDir(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("fonts: scan %s: %w", e.Name(), err)
		}
		inst := Installed{ID: e.Name()}
		for _, fe := range files {
			if fe.IsDir() || !validName(fe.Name()) {
				continue // 只统计合法裸文件，防远端清单之外的奇怪名字混进清单
			}
			info, err := fe.Info()
			if err != nil {
				return nil, fmt.Errorf("fonts: stat %s: %w", fe.Name(), err)
			}
			inst.Files = append(inst.Files, InstalledFile{Name: fe.Name(), Size: info.Size()})
			inst.TotalSize += info.Size()
		}
		out = append(out, inst)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Remove 删除指定字体族目录及其全部文件，亦即「卸载」。目标不存在时返回 nil（幂等）。
func Remove(dir, id string) error {
	if !validName(id) {
		return fmt.Errorf("fonts: invalid family name %q", id)
	}
	target := filepath.Join(dir, id)
	// 单段 id 经 Join 不可能逃逸 dir，仍显式确认一遍：RemoveAll 是破坏性操作，
	// 双保险确认最终路径仍在 dir 之内。
	rel, err := filepath.Rel(dir, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("fonts: unsafe family path %q", id)
	}
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("fonts: stat %s: %w", id, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("fonts: %s is not a directory", id)
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("fonts: remove %s: %w", id, err)
	}
	return nil
}
