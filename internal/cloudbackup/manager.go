package cloudbackup

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/clip-rss/clip/internal/webdav"
)

const (
	manifestRetries = 4
	failureBackoff  = time.Hour
)

type persistedState struct {
	LastBackupAt  time.Time   `json:"lastBackupAt"`
	LastAttemptAt time.Time   `json:"lastAttemptAt"`
	LastBackup    *BackupInfo `json:"lastBackup"`
	LastRestoreAt time.Time   `json:"lastRestoreAt"`
	LastError     string      `json:"lastError"`
	RollbackPath  string      `json:"rollbackPath"`
}

type manifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	Backups       []BackupInfo `json:"backups"`
}

// Manager 串行化数据库快照、清单更新与恢复。零值不可用。
type Manager struct {
	store dataStore
	now   func() time.Time
	mu    sync.Mutex
}

func New(store dataStore) *Manager {
	return &Manager{store: store, now: time.Now}
}

func (m *Manager) GetConfig() (Config, error) {
	cfg := DefaultConfig()
	if _, err := m.store.GetJSONSetting(configKey, &cfg); err != nil {
		return Config{}, err
	}
	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (m *Manager) SaveConfig(cfg Config) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}
	return m.store.SetJSONSetting(configKey, cfg)
}

func validateConfig(cfg Config) error {
	if cfg.Interval != IntervalDaily && cfg.Interval != IntervalWeekly {
		return fmt.Errorf("不支持的云备份周期 %q", cfg.Interval)
	}
	if cfg.Retention != 3 && cfg.Retention != 5 && cfg.Retention != 10 {
		return fmt.Errorf("云备份保留数量必须为 3、5 或 10")
	}
	return nil
}

func (m *Manager) Status() (Status, error) {
	cfg, err := m.GetConfig()
	if err != nil {
		return Status{}, err
	}
	st, err := m.loadState()
	if err != nil {
		return Status{}, err
	}
	out := Status{
		LastBackup:   st.LastBackup,
		LastError:    st.LastError,
		RollbackPath: st.RollbackPath,
	}
	if !st.LastBackupAt.IsZero() {
		v := st.LastBackupAt
		out.LastBackupAt = &v
	}
	if !st.LastAttemptAt.IsZero() {
		v := st.LastAttemptAt
		out.LastAttemptAt = &v
	}
	if !st.LastRestoreAt.IsZero() {
		v := st.LastRestoreAt
		out.LastRestoreAt = &v
	}
	if cfg.Enabled {
		v := nextBackupAt(cfg, st, m.now())
		out.NextBackupAt = &v
	}
	return out, nil
}

// DueIn 返回自动备份距离下次执行的时间。未启用时第二个返回值为 false。
func (m *Manager) DueIn(now time.Time) (time.Duration, bool, error) {
	cfg, err := m.GetConfig()
	if err != nil {
		return 0, false, err
	}
	if !cfg.Enabled {
		return 0, false, nil
	}
	st, err := m.loadState()
	if err != nil {
		return 0, false, err
	}
	d := nextBackupAt(cfg, st, now).Sub(now)
	if d < 0 {
		d = 0
	}
	return d, true, nil
}

func nextBackupAt(cfg Config, st persistedState, now time.Time) time.Time {
	interval := 24 * time.Hour
	if cfg.Interval == IntervalWeekly {
		interval = 7 * 24 * time.Hour
	}
	next := now
	if !st.LastBackupAt.IsZero() {
		next = st.LastBackupAt.Add(interval)
	}
	if st.LastError != "" && !st.LastAttemptAt.IsZero() {
		retry := st.LastAttemptAt.Add(failureBackoff)
		if retry.After(next) {
			next = retry
		}
	}
	return next
}

// Backup 创建快照、上传不可变文件，并用 ETag 合并远端清单。
func (m *Manager) Backup(ctx context.Context, remote Remote) (info BackupInfo, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now().UTC()
	m.recordAttempt(now)
	defer func() {
		if err != nil {
			m.recordFailure(now, err)
		}
	}()

	cfg, err := m.GetConfig()
	if err != nil {
		return BackupInfo{}, err
	}
	tempDir, err := os.MkdirTemp("", "clip-cloud-backup-")
	if err != nil {
		return BackupInfo{}, fmt.Errorf("创建云备份临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	snapshot := filepath.Join(tempDir, "snapshot.db")
	dbVersion, err := m.store.BackupForCloud(snapshot)
	if err != nil {
		return BackupInfo{}, err
	}
	file, err := os.Open(snapshot)
	if err != nil {
		return BackupInfo{}, fmt.Errorf("打开云备份快照失败: %w", err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return BackupInfo{}, fmt.Errorf("读取云备份大小失败: %w", err)
	}
	if stat.Size() <= 0 || stat.Size() > MaxBackupBytes {
		return BackupInfo{}, fmt.Errorf("云备份大小 %d 字节超出允许范围", stat.Size())
	}

	hash, err := hashReader(file)
	if err != nil {
		return BackupInfo{}, fmt.Errorf("计算云备份校验和失败: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return BackupInfo{}, fmt.Errorf("重置云备份读取位置失败: %w", err)
	}
	id, err := newBackupID(now)
	if err != nil {
		return BackupInfo{}, err
	}
	filePath := remoteDir + id + ".db"
	info = BackupInfo{
		ID:              id,
		File:            filePath,
		DeviceName:      deviceName(),
		CreatedAt:       now,
		Size:            stat.Size(),
		SHA256:          hash,
		DatabaseVersion: dbVersion,
	}

	if err := remote.MkcolAll(ctx, remoteDir); err != nil {
		return BackupInfo{}, err
	}
	if _, err := remote.PutStream(ctx, filePath, file, stat.Size(), webdav.PutOptions{
		ContentType: "application/vnd.sqlite3",
		IfNoneMatch: true,
	}); err != nil {
		return BackupInfo{}, err
	}

	dropped, err := m.publishManifest(ctx, remote, info, cfg.Retention)
	if err != nil {
		// 快照尚未进入清单，没有任何恢复入口。删除它避免远端长期积累孤立文件。
		_ = remote.Delete(ctx, filePath)
		return BackupInfo{}, err
	}
	for _, old := range dropped {
		if deleteErr := remote.Delete(ctx, old.File); deleteErr != nil &&
			!errors.Is(deleteErr, webdav.ErrNotFound) {
			// 清理失败不使已发布且可恢复的新备份失败。旧文件会成为清单外的
			// 不可见孤立文件，用户可在 WebDAV 管理端手动清理。
			continue
		}
	}

	st, stateErr := m.loadState()
	if stateErr != nil {
		return BackupInfo{}, stateErr
	}
	st.LastBackupAt = now
	st.LastAttemptAt = now
	st.LastBackup = &info
	st.LastError = ""
	if err := m.saveState(st); err != nil {
		return BackupInfo{}, err
	}
	return info, nil
}

// List 返回远端清单中仍保留的快照，新到旧排序。
func (m *Manager) List(ctx context.Context, remote Remote) ([]BackupInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	manifest, _, _, err := loadManifest(ctx, remote)
	if err != nil {
		return nil, err
	}
	return append([]BackupInfo(nil), manifest.Backups...), nil
}

// Restore 下载并校验指定快照，再暂存到下次启动。当前数据库会先留下回滚副本。
func (m *Manager) Restore(
	ctx context.Context,
	remote Remote,
	id string,
) (result RestoreResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now().UTC()
	m.recordAttempt(now)
	defer func() {
		if err != nil {
			m.recordFailure(now, err)
		}
	}()

	remoteManifest, _, _, err := loadManifest(ctx, remote)
	if err != nil {
		return RestoreResult{}, err
	}
	var selected *BackupInfo
	for i := range remoteManifest.Backups {
		if remoteManifest.Backups[i].ID == id {
			v := remoteManifest.Backups[i]
			selected = &v
			break
		}
	}
	if selected == nil {
		return RestoreResult{}, fmt.Errorf("找不到指定的云备份")
	}
	if selected.Size <= 0 || selected.Size > MaxBackupBytes {
		return RestoreResult{}, fmt.Errorf("云备份记录的大小无效")
	}

	tempDir, err := os.MkdirTemp("", "clip-cloud-restore-")
	if err != nil {
		return RestoreResult{}, fmt.Errorf("创建云恢复临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)
	target := filepath.Join(tempDir, "restore.db")
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("创建云恢复临时文件失败: %w", err)
	}

	_, written, downloadErr := remote.GetTo(ctx, selected.File, file, MaxBackupBytes)
	if downloadErr == nil && written != selected.Size {
		downloadErr = fmt.Errorf("云备份大小不匹配：实际 %d，预期 %d", written, selected.Size)
	}
	if downloadErr == nil {
		downloadErr = file.Sync()
	}
	if closeErr := file.Close(); downloadErr == nil && closeErr != nil {
		downloadErr = closeErr
	}
	if downloadErr != nil {
		return RestoreResult{}, downloadErr
	}

	check, err := os.Open(target)
	if err != nil {
		return RestoreResult{}, err
	}
	actualHash, hashErr := hashReader(check)
	closeErr := check.Close()
	if hashErr != nil {
		return RestoreResult{}, fmt.Errorf("计算下载文件校验和失败: %w", hashErr)
	}
	if closeErr != nil {
		return RestoreResult{}, closeErr
	}
	if !strings.EqualFold(actualHash, selected.SHA256) {
		return RestoreResult{}, fmt.Errorf("云备份校验和不匹配，文件可能未完整下载")
	}

	rollback, err := m.store.StageCloudRestore(target)
	if err != nil {
		return RestoreResult{}, err
	}
	st, stateErr := m.loadState()
	if stateErr != nil {
		_ = m.store.DiscardPendingRestore()
		return RestoreResult{}, stateErr
	}
	st.LastAttemptAt = now
	st.LastRestoreAt = now
	st.LastError = ""
	st.RollbackPath = rollback
	if err := m.store.SetPendingJSONSetting(stateKey, st); err != nil {
		_ = m.store.DiscardPendingRestore()
		return RestoreResult{}, err
	}
	// 待恢复库已经完整就绪；当前库中的状态只用于重启前展示，失败不应把一份
	// 确定会在下次启动生效的恢复误报成失败。
	_ = m.saveState(st)
	return RestoreResult{RestartRequired: true, RollbackPath: rollback}, nil
}

func (m *Manager) publishManifest(
	ctx context.Context,
	remote Remote,
	info BackupInfo,
	retention int,
) ([]BackupInfo, error) {
	for attempt := 0; attempt < manifestRetries; attempt++ {
		current, etag, exists, err := loadManifest(ctx, remote)
		if err != nil {
			return nil, err
		}
		if exists && etag == "" {
			stat, err := remote.Stat(ctx, manifestFile)
			if err != nil {
				return nil, err
			}
			etag = stat.ETag
			if etag == "" {
				return nil, fmt.Errorf("WebDAV 服务器未提供清单 ETag，无法安全处理多设备并发备份")
			}
		}
		next, dropped := mergeManifest(current, info, retention)
		data, err := json.MarshalIndent(next, "", "  ")
		if err != nil {
			return nil, err
		}
		data = append(data, '\n')
		opts := webdav.PutOptions{ContentType: "application/json; charset=utf-8"}
		if exists {
			opts.IfMatch = etag
		} else {
			opts.IfNoneMatch = true
		}
		_, err = remote.PutStream(ctx, manifestFile, strings.NewReader(string(data)), int64(len(data)), opts)
		if errors.Is(err, webdav.ErrConflict) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return dropped, nil
	}
	return nil, fmt.Errorf("云备份清单被其他设备连续修改，请稍后重试")
}

func loadManifest(
	ctx context.Context,
	remote Remote,
) (manifest, string, bool, error) {
	data, etag, err := remote.Get(ctx, manifestFile)
	if errors.Is(err, webdav.ErrNotFound) {
		return manifest{SchemaVersion: manifestSchemaVersion}, "", false, nil
	}
	if err != nil {
		return manifest{}, "", false, err
	}
	var out manifest
	if err := json.Unmarshal(data, &out); err != nil {
		return manifest{}, "", false, fmt.Errorf("云备份清单损坏: %w", err)
	}
	if out.SchemaVersion != manifestSchemaVersion {
		return manifest{}, "", false, fmt.Errorf(
			"不支持的云备份清单版本 %d",
			out.SchemaVersion,
		)
	}
	if len(out.Backups) > 100 {
		return manifest{}, "", false, fmt.Errorf("云备份清单条目过多")
	}
	for _, item := range out.Backups {
		if err := validateBackupInfo(item); err != nil {
			return manifest{}, "", false, err
		}
	}
	sort.SliceStable(out.Backups, func(i, j int) bool {
		return out.Backups[i].CreatedAt.After(out.Backups[j].CreatedAt)
	})
	return out, etag, true, nil
}

func validateBackupInfo(info BackupInfo) error {
	if info.ID == "" || info.File != remoteDir+info.ID+".db" {
		return fmt.Errorf("云备份清单包含非法文件路径")
	}
	if path.Clean(info.File) != info.File || !strings.HasPrefix(info.File, remoteDir) {
		return fmt.Errorf("云备份清单包含越界路径")
	}
	if info.Size <= 0 || info.Size > MaxBackupBytes {
		return fmt.Errorf("云备份清单包含非法文件大小")
	}
	if len(info.SHA256) != sha256.Size*2 {
		return fmt.Errorf("云备份清单包含非法校验和")
	}
	if _, err := hex.DecodeString(info.SHA256); err != nil {
		return fmt.Errorf("云备份清单包含非法校验和")
	}
	return nil
}

func mergeManifest(current manifest, info BackupInfo, retention int) (manifest, []BackupInfo) {
	all := make([]BackupInfo, 0, len(current.Backups)+1)
	all = append(all, info)
	seen := map[string]bool{info.ID: true}
	for _, old := range current.Backups {
		if !seen[old.ID] {
			seen[old.ID] = true
			all = append(all, old)
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	var dropped []BackupInfo
	if len(all) > retention {
		dropped = append(dropped, all[retention:]...)
		all = all[:retention]
	}
	return manifest{SchemaVersion: manifestSchemaVersion, Backups: all}, dropped
}

func (m *Manager) loadState() (persistedState, error) {
	var st persistedState
	if _, err := m.store.GetJSONSetting(stateKey, &st); err != nil {
		return persistedState{}, err
	}
	return st, nil
}

func (m *Manager) saveState(st persistedState) error {
	return m.store.SetJSONSetting(stateKey, st)
}

func (m *Manager) recordAttempt(now time.Time) {
	st, err := m.loadState()
	if err != nil {
		return
	}
	st.LastAttemptAt = now
	_ = m.saveState(st)
}

func (m *Manager) recordFailure(now time.Time, cause error) {
	st, err := m.loadState()
	if err != nil {
		return
	}
	st.LastAttemptAt = now
	st.LastError = cause.Error()
	_ = m.saveState(st)
}

func hashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func newBackupID(now time.Time) (string, error) {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("生成云备份标识失败: %w", err)
	}
	return now.Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(random), nil
}

func deviceName() string {
	name, err := os.Hostname()
	if err != nil || strings.TrimSpace(name) == "" {
		return "未知设备"
	}
	return name
}
