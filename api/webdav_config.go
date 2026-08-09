package api

import (
	"context"
	"errors"
	"time"

	"github.com/clip-rss/clip/internal/i18n"
	"github.com/clip-rss/clip/internal/secret"
	"github.com/clip-rss/clip/internal/store"
	"github.com/clip-rss/clip/internal/webdav"
)

// webdavTimeout WebDAV 操作的超时时间。
const webdavTimeout = 90 * time.Second

// WebDAVConfigService WebDAV 凭据与连接管理服务。
type WebDAVConfigService struct {
	store *store.Store
	vault *WebDAVVault
}

// NewWebDAVConfigService 创建 WebDAV 配置服务。
//
// cipher 为 nil 表示凭据加密不可用（密钥文件所在路径被占等）。此时服务照常构造，
// 但所有涉及凭据的方法返回明确错误。
func NewWebDAVConfigService(st *store.Store, cipher *secret.Cipher) *WebDAVConfigService {
	s := &WebDAVConfigService{store: st}
	if cipher != nil {
		s.vault = NewWebDAVVault(st, cipher)
	}
	return s
}

// WebDAVView 不含密码的 WebDAV 配置视图，供前端展示。
type WebDAVView struct {
	URL         string `json:"url"`
	Username    string `json:"username"`
	HasPassword bool   `json:"hasPassword"`
}

// WebDAVInput 前端提交的 WebDAV 配置表单。密码传空串表示保持原密码不变。
type WebDAVInput struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// WebDAVCredentials 完整的 WebDAV 凭据（含密码）。
type WebDAVCredentials struct {
	URL      string
	Username string
	Password string
}

// GetWebDAVConfig 读取 WebDAV 配置。不含密码，只带 hasPassword。
func (s *WebDAVConfigService) GetWebDAVConfig() (WebDAVView, error) {
	if s.vault == nil {
		return WebDAVView{}, credentialStoreUnavailable(s.store)
	}
	view, err := s.vault.View()
	return view, backendError(s.store, err)
}

// SaveWebDAVConfig 保存 WebDAV 配置。密码传空串表示保持原密码不变。
func (s *WebDAVConfigService) SaveWebDAVConfig(cfg WebDAVInput) error {
	if s.vault == nil {
		return credentialStoreUnavailable(s.store)
	}

	// ⚠️ 先校验地址，再落库。避免保存无效配置。
	creds, err := s.vault.CredentialsFor(cfg)
	if err != nil {
		return backendError(s.store, err)
	}
	if _, err := s.newClient(creds); err != nil {
		return backendError(s.store, err)
	}

	return backendError(s.store, s.vault.Save(cfg))
}

// ClearWebDAVConfig 删除全部 WebDAV 配置（含密码）。
func (s *WebDAVConfigService) ClearWebDAVConfig() error {
	if s.vault == nil {
		return credentialStoreUnavailable(s.store)
	}
	return backendError(s.store, s.vault.Clear())
}

// ConnectionTestResult 「测试连接」的逐步结果。
type ConnectionTestResult struct {
	OK bool `json:"ok"`

	// Step 失败的步骤：connect / mkcol / write；成功时为空。
	Step string `json:"step"`

	// Message 失败原因（面向用户）。
	Message string `json:"message"`

	// Hint 可操作的建议，可能为空。
	Hint string `json:"hint"`
}

// TestWebDAVConnection 建目录 + 写探针文件 + 删除，逐项报错。
//
// 传入的是**尚未保存**的表单配置：用户改完就点测试，不该先被迫保存。
// 密码留空时回落到已存的密码。
func (s *WebDAVConfigService) TestWebDAVConnection(cfg WebDAVInput) (ConnectionTestResult, error) {
	if s.vault == nil {
		return ConnectionTestResult{}, credentialStoreUnavailable(s.store)
	}
	creds, err := s.vault.CredentialsFor(cfg)
	if err != nil {
		return failedTest("connect", err, backendLanguage(s.store)), nil
	}
	client, err := s.newClient(creds)
	if err != nil {
		return failedTest("connect", err, backendLanguage(s.store)), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), webdavTimeout)
	defer cancel()

	testDir := "clip/"
	if err := client.MkcolAll(ctx, testDir); err != nil {
		return failedTest("mkcol", err, backendLanguage(s.store)), nil
	}
	probe := testDir + "clip-probe.tmp"
	if _, err := client.Put(ctx, probe, []byte("clip connection probe\n"), ""); err != nil {
		return failedTest("write", err, backendLanguage(s.store)), nil
	}
	// 清理探针文件，失败不影响测试结果
	_ = client.Delete(ctx, probe)

	return ConnectionTestResult{OK: true}, nil
}

// GetWebDAVClient 返回当前配置的 WebDAV 客户端，供其他服务使用。
func (s *WebDAVConfigService) GetWebDAVClient() (*webdav.Client, error) {
	if s.vault == nil {
		return nil, credentialStoreUnavailable(s.store)
	}
	creds, err := s.vault.Credentials()
	if err != nil {
		return nil, backendError(s.store, err)
	}
	client, err := s.newClient(creds)
	return client, backendError(s.store, err)
}

// newClient 用凭据与当前代理设置构造 WebDAV 客户端。
func (s *WebDAVConfigService) newClient(creds WebDAVCredentials) (*webdav.Client, error) {
	return s.newClientWithOptions(creds)
}

func (s *WebDAVConfigService) newClientWithOptions(
	creds WebDAVCredentials,
	extra ...webdav.Option,
) (*webdav.Client, error) {
	opts := []webdav.Option{}
	// 代理与 RSS 抓取共用同一份设置。
	if cfg, err := s.store.GetSettings(); err == nil && cfg.ProxyHost != "" && cfg.ProxyPort > 0 {
		opts = append(opts, webdav.WithProxy(cfg.ProxyHost, cfg.ProxyPort))
	}
	opts = append(opts, extra...)
	return webdav.New(webdav.Config{
		URL:      creds.URL,
		Username: creds.Username,
		Password: creds.Password,
	}, opts...)
}

func credentialStoreUnavailable(st *store.Store) error {
	return errors.New(i18n.T(backendLanguage(st), "webdav.credentialsUnavailable"))
}

// failedTest 把错误转成一步失败的测试结果。
func failedTest(step string, err error, lang string) ConnectionTestResult {
	return ConnectionTestResult{
		OK:      false,
		Step:    step,
		Message: i18n.LocalizeError(lang, err).Error(),
		Hint:    hintFor(err, lang),
	}
}

// hintFor 按错误类别给出可操作建议。
func hintFor(err error, lang string) string {
	switch {
	case errors.Is(err, webdav.ErrUnauthorized):
		return i18n.T(lang, "webdav.hintUnauthorized")
	case errors.Is(err, webdav.ErrNotCollection):
		return i18n.T(lang, "webdav.hintNotCollection")
	case errors.Is(err, webdav.ErrNotFound):
		return i18n.T(lang, "webdav.hintNotFound")
	case errors.Is(err, webdav.ErrInvalidConfig):
		return i18n.T(lang, "webdav.hintInvalidConfig")
	case errors.Is(err, webdav.ErrInsufficientStorage):
		return i18n.T(lang, "webdav.hintInsufficientStorage")
	case errors.Is(err, secret.ErrCredentialsLost):
		return i18n.T(lang, "webdav.hintCredentialsLost")
	case errors.Is(err, webdav.ErrNetwork):
		return i18n.T(lang, "webdav.hintNetwork")
	}
	return ""
}
