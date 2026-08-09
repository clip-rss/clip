// Package opmlbackup 提供 OPML 订阅列表的云端备份与恢复能力。
//
// 与配置同步不同，OPML 云备份是版本化快照 + 手动恢复的模式，不会自动合并多台机器的订阅。
package opmlbackup

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

// 远端布局：所有 OPML 备份文件存于统一目录。
const (
	remoteDir = "clip/opml/"
)

// RemoteDir 返回 OPML 备份目录（相对用户填写的 WebDAV 地址）。
func RemoteDir() string { return remoteDir }

// Remote OPML 云备份需要的 WebDAV 能力。
type Remote interface {
	Get(ctx context.Context, path string) ([]byte, string, error)
	Put(ctx context.Context, path string, data []byte, ifMatch string) (string, error)
	List(ctx context.Context, dir string) ([]ListEntry, error)
	Delete(ctx context.Context, path string) error
	MkcolAll(ctx context.Context, path string) error
}

// ListEntry 远端文件信息。
type ListEntry struct {
	Path         string
	Size         int64
	LastModified time.Time
	IsDir        bool
}

// Store OPML 云备份需要的数据库能力。
type Store interface {
	GetOPMLBackupConfig() (OPMLBackupConfig, error)
	SaveOPMLBackupConfig(OPMLBackupConfig) error
	GetOPMLBackupStatus() (OPMLBackupStatus, error)
	SaveOPMLBackupStatus(OPMLBackupStatus) error
}

// OPMLBackupConfig OPML 云备份配置。
//
// 只保留清理策略：备份改为纯手动触发，不再有自动周期。Retention 控制每次
// 备份（含手动）后保留的历史版本数，避免快照无限堆积。
type OPMLBackupConfig struct {
	Retention int `json:"retention"` // 保留版本数
}

// OPMLBackupStatus OPML 云备份状态。
type OPMLBackupStatus struct {
	LastBackupAt time.Time `json:"lastBackupAt"` // 上次备份时间
	LastError    string    `json:"lastError"`    // 上次错误信息
}

// OPMLBuilder 生成 OPML 内容的能力。
type OPMLBuilder interface {
	BuildOPML() (string, error)
}

// OPMLImporter 导入 OPML 内容的能力。
type OPMLImporter interface {
	ImportOPML(content string) (OPMLImportResult, error)
}

// OPMLImportResult OPML 导入结果。
type OPMLImportResult struct {
	Categories int
	Feeds      int
	Skipped    int
}

// ImportResult OPML 导入结果（对外导出）。
type ImportResult = OPMLImportResult

// Config OPML 云备份配置（对外导出）。
type Config = OPMLBackupConfig

// Status OPML 云备份状态（对外导出）。
type Status = OPMLBackupStatus

// DefaultConfig 返回默认配置。
func DefaultConfig() Config {
	return Config{
		Retention: 7,
	}
}

// BackupInfo 单份备份的信息。
type BackupInfo struct {
	ID           string    `json:"id"`           // 备份 ID（文件名）
	DeviceName   string    `json:"deviceName"`   // 创建备份的设备名
	CreatedAt    time.Time `json:"createdAt"`    // 创建时间
	Size         int64     `json:"size"`         // 文件大小（字节）
	LastModified time.Time `json:"lastModified"` // 最后修改时间
}

// Manager OPML 云备份管理器。
type Manager struct {
	store    Store
	builder  OPMLBuilder
	importer OPMLImporter
}

// New 创建 OPML 云备份管理器。
func New(store Store, builder OPMLBuilder, importer OPMLImporter) *Manager {
	return &Manager{
		store:    store,
		builder:  builder,
		importer: importer,
	}
}

// GetConfig 读取配置。
func (m *Manager) GetConfig() (Config, error) {
	return m.store.GetOPMLBackupConfig()
}

// SaveConfig 保存配置。
func (m *Manager) SaveConfig(cfg Config) error {
	return m.store.SaveOPMLBackupConfig(cfg)
}

// Status 读取状态。
func (m *Manager) Status() (Status, error) {
	return m.store.GetOPMLBackupStatus()
}

// Backup 执行一次备份：生成 OPML → 上传到 WebDAV → 清理旧备份。
func (m *Manager) Backup(ctx context.Context, remote Remote) (BackupInfo, error) {
	// 生成 OPML 内容
	content, err := m.builder.BuildOPML()
	if err != nil {
		return BackupInfo{}, fmt.Errorf("生成 OPML 失败: %w", err)
	}

	// 构造文件名：clip-feeds-<timestamp>-<device>.opml
	now := time.Now()
	deviceName := deviceName()
	filename := fmt.Sprintf("clip-feeds-%s-%s.opml",
		now.Format("20060102T150405"),
		sanitizeDeviceName(deviceName))
	remotePath := remoteDir + filename

	// 确保目录存在
	if err := remote.MkcolAll(ctx, remoteDir); err != nil {
		return BackupInfo{}, fmt.Errorf("创建备份目录失败: %w", err)
	}

	// 上传文件
	_, err = remote.Put(ctx, remotePath, []byte(content), "")
	if err != nil {
		return BackupInfo{}, fmt.Errorf("上传备份失败: %w", err)
	}

	// 更新状态
	status := OPMLBackupStatus{
		LastBackupAt: now,
		LastError:    "",
	}
	if err := m.store.SaveOPMLBackupStatus(status); err != nil {
		// 状态保存失败不影响备份本身
		_ = err
	}

	// 清理旧备份
	cfg, _ := m.GetConfig()
	if cfg.Retention > 0 {
		if err := m.cleanup(ctx, remote, cfg.Retention); err != nil {
			// 清理失败不影响本次备份
			_ = err
		}
	}

	return BackupInfo{
		ID:         filename,
		DeviceName: deviceName,
		CreatedAt:  now,
		Size:       int64(len(content)),
	}, nil
}

// List 列出远端所有 OPML 备份。
func (m *Manager) List(ctx context.Context, remote Remote) ([]BackupInfo, error) {
	files, err := remote.List(ctx, remoteDir)
	if err != nil {
		return nil, fmt.Errorf("列出备份失败: %w", err)
	}

	var backups []BackupInfo
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".opml") {
			continue
		}
		filename := path.Base(f.Path)
		info := BackupInfo{
			ID:           filename,
			DeviceName:   extractDeviceName(filename),
			CreatedAt:    extractTimestamp(filename),
			Size:         f.Size,
			LastModified: f.LastModified,
		}
		backups = append(backups, info)
	}

	// 按创建时间倒序排列
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// Restore 从指定备份恢复订阅：下载 OPML → 导入。
func (m *Manager) Restore(ctx context.Context, remote Remote, id string) (ImportResult, error) {
	remotePath := remoteDir + id

	// 下载备份文件
	data, _, err := remote.Get(ctx, remotePath)
	if err != nil {
		return ImportResult{}, fmt.Errorf("下载备份失败: %w", err)
	}

	// 导入 OPML
	result, err := m.importer.ImportOPML(string(data))
	if err != nil {
		return ImportResult{}, fmt.Errorf("导入 OPML 失败: %w", err)
	}

	return result, nil
}

// Delete 删除指定的远端 OPML 备份。
func (m *Manager) Delete(ctx context.Context, remote Remote, id string) error {
	remotePath, err := backupRemotePath(id)
	if err != nil {
		return err
	}
	if err := remote.Delete(ctx, remotePath); err != nil {
		return fmt.Errorf("删除备份失败: %w", err)
	}
	return nil
}

// backupRemotePath 将前端传入的备份 ID 限制为备份目录下的单个 OPML 文件。
func backupRemotePath(id string) (string, error) {
	if id == "" || path.Base(id) != id ||
		strings.ContainsAny(id, `/\%?#`) || !strings.HasSuffix(id, ".opml") {
		return "", fmt.Errorf("无效的备份 ID")
	}
	return remoteDir + id, nil
}

// cleanup 清理超过保留数量的旧备份。
func (m *Manager) cleanup(ctx context.Context, remote Remote, retention int) error {
	backups, err := m.List(ctx, remote)
	if err != nil {
		return err
	}

	if len(backups) <= retention {
		return nil
	}

	// 删除最旧的备份
	toDelete := backups[retention:]
	for _, backup := range toDelete {
		if err := m.Delete(ctx, remote, backup.ID); err != nil {
			// 删除失败不中断清理流程
			_ = err
		}
	}

	return nil
}

// extractTimestamp 从文件名中提取时间戳。
// 格式：clip-feeds-20060102T150405-device.opml
func extractTimestamp(filename string) time.Time {
	parts := strings.Split(filename, "-")
	if len(parts) < 3 {
		return time.Time{}
	}
	timestampStr := parts[2]
	t, err := time.Parse("20060102T150405", timestampStr)
	if err != nil {
		return time.Time{}
	}
	return t
}

// extractDeviceName 从文件名中提取设备名。
func extractDeviceName(filename string) string {
	parts := strings.Split(filename, "-")
	if len(parts) < 4 {
		return ""
	}
	// 移除 .opml 后缀
	device := strings.TrimSuffix(parts[3], ".opml")
	return device
}

// sanitizeDeviceName 清理设备名，移除不适合作为文件名的字符。
func sanitizeDeviceName(name string) string {
	// 替换空格和特殊字符为下划线
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, ":", "_")
	return name
}

// deviceName 返回当前设备名称（主机名）。
//
// 写入备份文件名与历史列表展示。主机名不可用时回退 "Device" —— 展示与文件
// 命名都只是辅助信息，拿不到时不该让备份本身失败。
func deviceName() string {
	if name, err := os.Hostname(); err == nil && strings.TrimSpace(name) != "" {
		return name
	}
	return "Device"
}
