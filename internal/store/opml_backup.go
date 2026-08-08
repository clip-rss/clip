package store

import "time"

// opmlBackupConfigKey OPML 云备份配置在 settings 表中的键名。
const opmlBackupConfigKey = "opml_backup_config"

// opmlBackupStatusKey OPML 云备份状态在 settings 表中的键名。
const opmlBackupStatusKey = "opml_backup_status"

// OPMLBackupConfig OPML 云备份配置。
//
// 备份改为纯手动触发，配置只保留清理策略：Retention 控制每次备份后
// 保留的历史版本数，避免快照无限堆积。
type OPMLBackupConfig struct {
	Retention int `json:"retention"` // 保留版本数
}

// OPMLBackupStatus OPML 云备份状态。
type OPMLBackupStatus struct {
	LastBackupAt time.Time `json:"lastBackupAt"` // 上次备份时间
	LastError    string    `json:"lastError"`    // 上次错误信息
}

// GetOPMLBackupConfig 读取 OPML 云备份配置。
func (s *Store) GetOPMLBackupConfig() (OPMLBackupConfig, error) {
	var cfg OPMLBackupConfig
	found, err := s.GetJSONSetting(opmlBackupConfigKey, &cfg)
	if err != nil {
		return OPMLBackupConfig{}, err
	}
	if !found {
		// 返回默认配置
		return OPMLBackupConfig{
			Retention: 7,
		}, nil
	}
	return cfg, nil
}

// SaveOPMLBackupConfig 保存 OPML 云备份配置。
func (s *Store) SaveOPMLBackupConfig(cfg OPMLBackupConfig) error {
	return s.SetJSONSetting(opmlBackupConfigKey, cfg)
}

// GetOPMLBackupStatus 读取 OPML 云备份状态。
func (s *Store) GetOPMLBackupStatus() (OPMLBackupStatus, error) {
	var status OPMLBackupStatus
	found, err := s.GetJSONSetting(opmlBackupStatusKey, &status)
	if err != nil {
		return OPMLBackupStatus{}, err
	}
	if !found {
		return OPMLBackupStatus{}, nil
	}
	return status, nil
}

// SaveOPMLBackupStatus 保存 OPML 云备份状态。
func (s *Store) SaveOPMLBackupStatus(status OPMLBackupStatus) error {
	return s.SetJSONSetting(opmlBackupStatusKey, status)
}
