package api

import (
	"github.com/clip-rss/clip/internal/opmlbackup"
	"github.com/clip-rss/clip/internal/store"
)

// storeAdapter 将 store.Store 适配为 opmlbackup.Store。
type storeAdapter struct {
	store *store.Store
}

func (a *storeAdapter) GetOPMLBackupConfig() (opmlbackup.OPMLBackupConfig, error) {
	cfg, err := a.store.GetOPMLBackupConfig()
	if err != nil {
		return opmlbackup.OPMLBackupConfig{}, err
	}
	return opmlbackup.OPMLBackupConfig{
		Retention: cfg.Retention,
	}, nil
}

func (a *storeAdapter) SaveOPMLBackupConfig(cfg opmlbackup.OPMLBackupConfig) error {
	return a.store.SaveOPMLBackupConfig(store.OPMLBackupConfig{
		Retention: cfg.Retention,
	})
}

func (a *storeAdapter) GetOPMLBackupStatus() (opmlbackup.OPMLBackupStatus, error) {
	status, err := a.store.GetOPMLBackupStatus()
	if err != nil {
		return opmlbackup.OPMLBackupStatus{}, err
	}
	return opmlbackup.OPMLBackupStatus{
		LastBackupAt: status.LastBackupAt,
		LastError:    status.LastError,
	}, nil
}

func (a *storeAdapter) SaveOPMLBackupStatus(status opmlbackup.OPMLBackupStatus) error {
	return a.store.SaveOPMLBackupStatus(store.OPMLBackupStatus{
		LastBackupAt: status.LastBackupAt,
		LastError:    status.LastError,
	})
}

// opmlServiceAdapter 将 OPMLService 适配为 opmlbackup.OPMLImporter。
type opmlServiceAdapter struct {
	service *OPMLService
}

func (a *opmlServiceAdapter) ImportOPML(content string) (opmlbackup.OPMLImportResult, error) {
	result, err := a.service.ImportOPML(content)
	if err != nil {
		return opmlbackup.OPMLImportResult{}, err
	}
	return opmlbackup.OPMLImportResult{
		Categories: result.Categories,
		Feeds:      result.Feeds,
		Skipped:    result.Skipped,
	}, nil
}
