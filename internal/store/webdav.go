package store

import "errors"

// webdavConfigKey WebDAV 配置在 settings 表中的键名。
const webdavConfigKey = "webdav"

// WebDAVConfig WebDAV 服务器配置。
type WebDAVConfig struct {
	URL               string `json:"url"`
	Username          string `json:"username"`
	EncryptedPassword string `json:"encryptedPassword"` // 密码密文
}

// GetWebDAVConfig 读取 WebDAV 配置。
func (s *Store) GetWebDAVConfig() (WebDAVConfig, error) {
	var cfg WebDAVConfig
	found, err := s.GetJSONSetting(webdavConfigKey, &cfg)
	if err != nil {
		return WebDAVConfig{}, err
	}
	if !found {
		return WebDAVConfig{}, ErrNotFound
	}
	return cfg, nil
}

// SaveWebDAVConfig 保存 WebDAV 配置。
func (s *Store) SaveWebDAVConfig(cfg WebDAVConfig) error {
	return s.SetJSONSetting(webdavConfigKey, cfg)
}

// ClearWebDAVConfig 删除 WebDAV 配置。
func (s *Store) ClearWebDAVConfig() error {
	_, err := s.db.Exec(`DELETE FROM settings WHERE key = ?`, webdavConfigKey)
	if err != nil {
		return err
	}
	return nil
}

// ErrNotFound 表示查询的记录不存在。
var ErrNotFound = errors.New("not found")
