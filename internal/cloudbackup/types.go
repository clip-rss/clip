// Package cloudbackup 把一致性的 Clip 数据库快照版本化备份到 WebDAV。
//
// 它提供的是单向云备份与手动恢复，不尝试合并两台机器的 SQLite 文件。每次备份
// 使用不可变文件，清单通过 ETag 乐观并发合并，避免设备之间覆盖数据库快照。
package cloudbackup

import (
	"context"
	"io"
	"time"

	"github.com/clip-rss/clip/internal/webdav"
)

const (
	remoteDir    = "clip/backups/"
	manifestFile = remoteDir + "manifest.json"

	manifestSchemaVersion = 1
	MaxBackupBytes        = int64(4 << 30) // 4 GiB，兼顾大型缓存与异常下载防护。

	configKey = "cloud_backup_config"
	stateKey  = "cloud_backup_state"
)

const (
	IntervalDaily  = "daily"
	IntervalWeekly = "weekly"
)

// Config 自动云备份配置。手动备份不受 Enabled 限制。
type Config struct {
	Enabled   bool   `json:"enabled"`
	Interval  string `json:"interval"`
	Retention int    `json:"retention"`
}

func DefaultConfig() Config {
	return Config{Interval: IntervalDaily, Retention: 5}
}

// BackupInfo 描述一个不可变数据库快照。
type BackupInfo struct {
	ID              string    `json:"id"`
	File            string    `json:"file"`
	DeviceName      string    `json:"deviceName"`
	CreatedAt       time.Time `json:"createdAt"`
	Size            int64     `json:"size"`
	SHA256          string    `json:"sha256"`
	DatabaseVersion int       `json:"databaseVersion"`
}

// Status 本机最近一次云备份/恢复状态。远端历史由 List 单独读取。
type Status struct {
	LastBackupAt  *time.Time  `json:"lastBackupAt"`
	LastAttemptAt *time.Time  `json:"lastAttemptAt"`
	LastBackup    *BackupInfo `json:"lastBackup"`
	LastRestoreAt *time.Time  `json:"lastRestoreAt"`
	LastError     string      `json:"lastError"`
	NextBackupAt  *time.Time  `json:"nextBackupAt"`
	RollbackPath  string      `json:"rollbackPath"`
}

// RestoreResult 表示快照已安全暂存，需重启应用完成换库。
type RestoreResult struct {
	RestartRequired bool   `json:"restartRequired"`
	RollbackPath    string `json:"rollbackPath"`
}

type dataStore interface {
	GetJSONSetting(key string, out any) (bool, error)
	SetJSONSetting(key string, value any) error
	BackupForCloud(dest string) (int, error)
	StageCloudRestore(src string) (string, error)
	SetPendingJSONSetting(key string, value any) error
	DiscardPendingRestore() error
}

// Remote 是云备份需要的 WebDAV 能力。流式方法保证大型数据库不进入内存。
type Remote interface {
	Get(ctx context.Context, path string) ([]byte, string, error)
	PutStream(
		ctx context.Context,
		path string,
		src io.Reader,
		size int64,
		opts webdav.PutOptions,
	) (string, error)
	GetTo(ctx context.Context, path string, dst io.Writer, maxBytes int64) (string, int64, error)
	MkcolAll(ctx context.Context, path string) error
	Delete(ctx context.Context, path string) error
	Stat(ctx context.Context, path string) (webdav.Stat, error)
}

// RemoteDir 返回数据库云备份目录，供连接探针与设置页展示。
func RemoteDir() string { return remoteDir }

// ManifestFile 返回远端版本清单路径。
func ManifestFile() string { return manifestFile }
