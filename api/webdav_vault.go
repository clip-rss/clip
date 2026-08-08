package api

import (
	"errors"
	"strings"

	"github.com/clip-rss/clip/internal/secret"
	"github.com/clip-rss/clip/internal/store"
)

// WebDAVVault WebDAV 凭据的加密存储管理。
type WebDAVVault struct {
	store  *store.Store
	cipher *secret.Cipher
}

// NewWebDAVVault 创建凭据管理器。
func NewWebDAVVault(st *store.Store, cipher *secret.Cipher) *WebDAVVault {
	return &WebDAVVault{store: st, cipher: cipher}
}

// View 返回不含密码的配置视图。
func (v *WebDAVVault) View() (WebDAVView, error) {
	cfg, err := v.store.GetWebDAVConfig()
	if err != nil {
		return WebDAVView{}, err
	}
	return WebDAVView{
		URL:         cfg.URL,
		Username:    cfg.Username,
		HasPassword: cfg.EncryptedPassword != "",
	}, nil
}

// Save 保存配置。密码为空串时保持原密码不变。
func (v *WebDAVVault) Save(input WebDAVInput) error {
	cfg, err := v.store.GetWebDAVConfig()
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}

	cfg.URL = strings.TrimSpace(input.URL)
	cfg.Username = strings.TrimSpace(input.Username)

	// 密码非空时加密后更新
	if input.Password != "" {
		encrypted, err := v.cipher.Encrypt(input.Password)
		if err != nil {
			return err
		}
		cfg.EncryptedPassword = encrypted
	}

	return v.store.SaveWebDAVConfig(cfg)
}

// Clear 删除全部配置。
func (v *WebDAVVault) Clear() error {
	return v.store.ClearWebDAVConfig()
}

// Credentials 读取完整凭据（含密码）。
func (v *WebDAVVault) Credentials() (WebDAVCredentials, error) {
	cfg, err := v.store.GetWebDAVConfig()
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return WebDAVCredentials{}, errors.New("尚未配置 WebDAV 服务器")
		}
		return WebDAVCredentials{}, err
	}

	password := ""
	if cfg.EncryptedPassword != "" {
		decrypted, err := v.cipher.Decrypt(cfg.EncryptedPassword)
		if err != nil {
			return WebDAVCredentials{}, err
		}
		password = decrypted
	}

	return WebDAVCredentials{
		URL:      cfg.URL,
		Username: cfg.Username,
		Password: password,
	}, nil
}

// CredentialsFor 根据表单输入返回凭据。密码为空时回落到已存密码。
func (v *WebDAVVault) CredentialsFor(input WebDAVInput) (WebDAVCredentials, error) {
	url := strings.TrimSpace(input.URL)
	username := strings.TrimSpace(input.Username)
	password := input.Password

	// 密码为空时尝试读取已存密码
	if password == "" {
		existing, err := v.store.GetWebDAVConfig()
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return WebDAVCredentials{}, err
		}
		if existing.EncryptedPassword != "" {
			decrypted, err := v.cipher.Decrypt(existing.EncryptedPassword)
			if err != nil {
				return WebDAVCredentials{}, err
			}
			password = decrypted
		}
	}

	if password == "" {
		return WebDAVCredentials{}, errors.New("请输入密码")
	}

	return WebDAVCredentials{
		URL:      url,
		Username: username,
		Password: password,
	}, nil
}
