package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/clip-rss/clip/internal/cloudbackup"
	"github.com/clip-rss/clip/internal/store"
)

const cloudBackupTimeout = 30 * time.Minute

// CloudBackupService 提供独立于配置同步的数据库云备份能力。
//
// 云备份是不可变快照 + 手动恢复，不会把数据库文件加入配置同步引擎，也不会
// 自动覆盖本机正在使用的数据库。
type CloudBackupService struct {
	config  *SyncService
	manager *cloudbackup.Manager

	mu      sync.Mutex
	timer   *time.Timer
	cancel  context.CancelFunc
	started bool
	opMu    sync.Mutex
}

func NewCloudBackupService(st *store.Store, syncSvc *SyncService) *CloudBackupService {
	return &CloudBackupService{
		config:  syncSvc,
		manager: cloudbackup.New(st),
	}
}

/* ---------- 前端绑定 ---------- */

func (s *CloudBackupService) GetCloudBackupConfig() (cloudbackup.Config, error) {
	return s.manager.GetConfig()
}

func (s *CloudBackupService) SaveCloudBackupConfig(cfg cloudbackup.Config) error {
	if cfg.Enabled {
		if _, err := s.remote(); err != nil {
			return describeCloudBackupError(err)
		}
	}
	if err := s.manager.SaveConfig(cfg); err != nil {
		return err
	}
	s.arm()
	return nil
}

func (s *CloudBackupService) GetCloudBackupStatus() (cloudbackup.Status, error) {
	return s.manager.Status()
}

func (s *CloudBackupService) ListCloudBackups() ([]cloudbackup.BackupInfo, error) {
	remote, err := s.remote()
	if err != nil {
		return nil, describeCloudBackupError(err)
	}
	ctx, done := s.operationContext()
	defer done()
	backups, err := s.manager.List(ctx, remote)
	if err != nil {
		return nil, describeCloudBackupError(err)
	}
	return backups, nil
}

func (s *CloudBackupService) BackupDatabaseToCloud() (cloudbackup.BackupInfo, error) {
	defer s.arm()
	remote, err := s.remote()
	if err != nil {
		return cloudbackup.BackupInfo{}, describeCloudBackupError(err)
	}
	ctx, done := s.operationContext()
	defer done()
	info, err := s.manager.Backup(ctx, remote)
	if err != nil {
		return cloudbackup.BackupInfo{}, describeCloudBackupError(err)
	}
	return info, nil
}

func (s *CloudBackupService) RestoreDatabaseFromCloud(id string) (cloudbackup.RestoreResult, error) {
	if id == "" {
		return cloudbackup.RestoreResult{}, errors.New("请选择要恢复的云备份")
	}
	remote, err := s.remote()
	if err != nil {
		return cloudbackup.RestoreResult{}, describeCloudBackupError(err)
	}
	ctx, done := s.operationContext()
	defer done()
	result, err := s.manager.Restore(ctx, remote, id)
	if err != nil {
		return cloudbackup.RestoreResult{}, describeCloudBackupError(err)
	}
	return result, nil
}

// CloudBackupRemotePath 返回备份远端目录，供设置页说明实际存储位置。
func (s *CloudBackupService) CloudBackupRemotePath() string {
	return cloudbackup.RemoteDir()
}

/* ---------- 生命周期 ---------- */

func StartCloudBackup(s *CloudBackupService) { s.start() }

func StopCloudBackup(s *CloudBackupService) { s.stop() }

func (s *CloudBackupService) start() {
	s.mu.Lock()
	s.started = true
	s.mu.Unlock()
	s.arm()
}

func (s *CloudBackupService) stop() {
	s.mu.Lock()
	timer := s.timer
	s.timer = nil
	cancel := s.cancel
	s.started = false
	s.mu.Unlock()
	if timer != nil {
		timer.Stop()
	}
	if cancel != nil {
		cancel()
	}
	// 等正在进行的快照/传输观察到取消并退出，main 随后才可安全关闭数据库。
	s.opMu.Lock()
	s.opMu.Unlock()
}

func (s *CloudBackupService) disarm() {
	s.mu.Lock()
	timer := s.timer
	s.timer = nil
	s.mu.Unlock()
	if timer != nil {
		timer.Stop()
	}
}

func (s *CloudBackupService) arm() {
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if !started {
		return
	}
	delay, enabled, err := s.manager.DueIn(time.Now())
	if err != nil || !enabled {
		s.disarm()
		return
	}
	if delay < time.Minute {
		delay = time.Minute
	}
	s.mu.Lock()
	old := s.timer
	s.timer = time.AfterFunc(delay, s.autoBackup)
	s.mu.Unlock()
	if old != nil {
		old.Stop()
	}
}

func (s *CloudBackupService) autoBackup() {
	if _, err := s.BackupDatabaseToCloud(); err != nil {
		log.Printf("cloud backup: automatic backup failed: %v", err)
	}
}

func (s *CloudBackupService) operationContext() (context.Context, func()) {
	s.opMu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), cloudBackupTimeout)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	return ctx, func() {
		cancel()
		s.mu.Lock()
		s.cancel = nil
		s.mu.Unlock()
		s.opMu.Unlock()
	}
}

func (s *CloudBackupService) remote() (cloudbackup.Remote, error) {
	remote, err := s.config.cloudRemote()
	if err != nil {
		return nil, err
	}
	return remote, nil
}

func describeCloudBackupError(err error) error {
	if hint := hintFor(err); hint != "" {
		return fmt.Errorf("%w（%s）", err, hint)
	}
	return err
}
