package api

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/clip-rss/clip/internal/opmlbackup"
	"github.com/clip-rss/clip/internal/store"
	"github.com/clip-rss/clip/internal/webdav"
)

const opmlBackupTimeout = 5 * time.Minute

// OPMLBackupService 提供 OPML 订阅列表的云备份能力。
//
// 备份为纯手动触发：没有后台定时器，也不在启动时安排任何任务。仍保留一个
// 取消钩子，供退出时打断正在进行的手动备份/恢复，之后 main 才能安全关库。
type OPMLBackupService struct {
	webdavConfig *WebDAVConfigService
	manager      *opmlbackup.Manager
	opmlService  *OPMLService

	mu     sync.Mutex
	cancel context.CancelFunc
	opMu   sync.Mutex
}

// NewOPMLBackupService 创建 OPML 云备份服务。
func NewOPMLBackupService(
	st *store.Store,
	webdavConfig *WebDAVConfigService,
	opmlService *OPMLService,
) *OPMLBackupService {
	return &OPMLBackupService{
		webdavConfig: webdavConfig,
		opmlService:  opmlService,
		manager: opmlbackup.New(
			&storeAdapter{store: st},
			opmlService,
			&opmlServiceAdapter{service: opmlService},
		),
	}
}

/* ---------- 前端绑定 ---------- */

// GetOPMLBackupConfig 读取 OPML 备份配置。
func (s *OPMLBackupService) GetOPMLBackupConfig() (opmlbackup.Config, error) {
	return s.manager.GetConfig()
}

// SaveOPMLBackupConfig 保存 OPML 备份配置（目前只有保留版本数）。
func (s *OPMLBackupService) SaveOPMLBackupConfig(cfg opmlbackup.Config) error {
	return s.manager.SaveConfig(cfg)
}

// GetOPMLBackupStatus 读取 OPML 备份状态。
func (s *OPMLBackupService) GetOPMLBackupStatus() (opmlbackup.Status, error) {
	return s.manager.Status()
}

// ListOPMLBackups 列出远端所有 OPML 备份。
func (s *OPMLBackupService) ListOPMLBackups() ([]opmlbackup.BackupInfo, error) {
	remote, err := s.getRemote()
	if err != nil {
		return nil, err
	}
	ctx, done := s.operationContext()
	defer done()
	backups, err := s.manager.List(ctx, remote)
	if err != nil {
		return nil, err
	}
	return backups, nil
}

// BackupOPMLToCloud 执行一次 OPML 备份。
func (s *OPMLBackupService) BackupOPMLToCloud() (opmlbackup.BackupInfo, error) {
	remote, err := s.getRemote()
	if err != nil {
		return opmlbackup.BackupInfo{}, err
	}
	ctx, done := s.operationContext()
	defer done()
	info, err := s.manager.Backup(ctx, remote)
	if err != nil {
		return opmlbackup.BackupInfo{}, err
	}
	return info, nil
}

// RestoreOPMLFromCloud 从云备份恢复订阅列表。
func (s *OPMLBackupService) RestoreOPMLFromCloud(id string) (opmlbackup.ImportResult, error) {
	if id == "" {
		return opmlbackup.ImportResult{}, errors.New("请选择要恢复的备份")
	}
	remote, err := s.getRemote()
	if err != nil {
		return opmlbackup.ImportResult{}, err
	}
	ctx, done := s.operationContext()
	defer done()
	result, err := s.manager.Restore(ctx, remote, id)
	if err != nil {
		return opmlbackup.ImportResult{}, err
	}
	return result, nil
}

// DeleteOPMLBackup 删除指定的远端 OPML 备份。
func (s *OPMLBackupService) DeleteOPMLBackup(id string) error {
	if id == "" {
		return errors.New("请选择要删除的备份")
	}
	remote, err := s.getRemote()
	if err != nil {
		return err
	}
	ctx, done := s.operationContext()
	defer done()
	if err := s.manager.Delete(ctx, remote, id); err != nil {
		// 多设备同时管理备份时，目标可能已被其他设备删除。
		if errors.Is(err, webdav.ErrNotFound) {
			return nil
		}
		return err
	}
	return nil
}

// OPMLBackupRemotePath 返回 OPML 备份远端目录。
func (s *OPMLBackupService) OPMLBackupRemotePath() string {
	return opmlbackup.RemoteDir()
}

/* ---------- 生命周期 ---------- */

// StopOPMLBackup 停止 OPML 备份服务（由 main 在退出钩子里调用）。
//
// 备份是纯手动触发，没有需要启动的后台任务，因此不再提供 Start。退出时仍要
// 打断可能正在进行的手动备份/恢复，等它观察到取消并退出后，main 才能安全关库。
func StopOPMLBackup(s *OPMLBackupService) { s.stop() }

func (s *OPMLBackupService) stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	// 等待正在进行的操作完成
	s.opMu.Lock()
	s.opMu.Unlock()
}

func (s *OPMLBackupService) operationContext() (context.Context, func()) {
	s.opMu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), opmlBackupTimeout)
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

func (s *OPMLBackupService) getRemote() (opmlbackup.Remote, error) {
	client, err := s.webdavConfig.GetWebDAVClient()
	if err != nil {
		return nil, err
	}
	return &webdavRemoteAdapter{client: client}, nil
}
