package syncer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/clip-rss/clip/internal/secret"
)

// webdavKey WebDAV 连接配置在 settings 表中的键名。
const webdavKey = "webdav"

// ErrNoPassword 已启用同步但没有存下密码。
//
// 与 secret.ErrCredentialsLost 分开：那个是「存过但解不开了」，这个是「从未存过」。
// 两者给用户的提示不同（重新输入 vs 先填密码），故不合并。
var ErrNoPassword = errors.New("尚未设置同步密码")

// ConfigStore 连接配置的持久化能力，由 *store.Store 实现。
type ConfigStore interface {
	GetJSONSetting(key string, out any) (bool, error)
	SetJSONSetting(key string, value any) error
	DeleteSetting(key string) error
}

// webdavConfig 落库的连接配置。密码以密文存放，故本类型不导出 ——
// 一旦导出，它就可能被顺手当成返回给前端的结构，那样密文（乃至日后误改成明文）
// 会直接出现在 IPC 里。对外只有 WebDAVView（读）与 WebDAVInput（写）两个形状。
type webdavConfig struct {
	Enabled        bool   `json:"enabled"`
	URL            string `json:"url"`
	Username       string `json:"username"`
	PasswordCipher string `json:"passwordCipher"`
}

// WebDAVInput 前端提交的连接配置。
//
// Password 为空表示**保持原密码不变** —— 前端拿不到现有密码，
// 用户只改地址时不该被迫重新输入一遍。要清空密码得用 Clear。
type WebDAVInput struct {
	Enabled  bool   `json:"enabled"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// WebDAVView 回给前端的连接配置。
//
// ⚠️ 只有 HasPassword，没有任何密码字段。密码不出现在返回给前端的任何结构里：
// 一进 IPC 就会落到 webview 的内存与可能的日志里。
// TestViewHasNoPasswordField 用反射守住这条。
type WebDAVView struct {
	Enabled     bool   `json:"enabled"`
	URL         string `json:"url"`
	Username    string `json:"username"`
	HasPassword bool   `json:"hasPassword"`
}

// WebDAVCredentials 解密后的凭据，仅在进程内传给 webdav.New。绝不返回给前端。
type WebDAVCredentials struct {
	URL      string
	Username string
	Password string
}

// String 遮蔽密码。
//
// 这个结构体会流经错误信息与调试输出，而 fmt 的 %v / %+v 默认会把每个字段
// 原样打出来。实现 Stringer 让默认打印路径就是安全的 —— 靠「记得别打印它」
// 是靠不住的。
func (c WebDAVCredentials) String() string {
	pw := "<未设置>"
	if c.Password != "" {
		pw = "<已隐藏>"
	}
	return fmt.Sprintf("WebDAVCredentials{URL: %s, Username: %s, Password: %s}",
		c.URL, c.Username, pw)
}

// Vault 同步凭据的读写。密码在存入前加密，取出时解密。
type Vault struct {
	store  ConfigStore
	cipher *secret.Cipher
}

// NewVault 创建凭据仓库。
func NewVault(store ConfigStore, cipher *secret.Cipher) *Vault {
	return &Vault{store: store, cipher: cipher}
}

// View 返回可安全交给前端的配置。
func (v *Vault) View() (WebDAVView, error) {
	cfg, err := v.load()
	if err != nil {
		return WebDAVView{}, err
	}
	return WebDAVView{
		Enabled:     cfg.Enabled,
		URL:         cfg.URL,
		Username:    cfg.Username,
		HasPassword: cfg.PasswordCipher != "",
	}, nil
}

// Save 保存配置。in.Password 为空时保留原密码。
func (v *Vault) Save(in WebDAVInput) error {
	current, err := v.load()
	if err != nil {
		return err
	}

	next := webdavConfig{
		Enabled: in.Enabled,
		// 地址与用户名去掉首尾空白：从网页复制粘贴时极易带上，
		// 而带空白的地址会以一个难以理解的解析错误告终。
		URL:            strings.TrimSpace(in.URL),
		Username:       strings.TrimSpace(in.Username),
		PasswordCipher: current.PasswordCipher,
	}
	// ⚠️ 密码不做 TrimSpace：空白可能是密码的一部分，替用户"清理"会造成
	// 一个查不出原因的认证失败。
	if in.Password != "" {
		cipher, err := v.cipher.Encrypt(in.Password)
		if err != nil {
			return err
		}
		next.PasswordCipher = cipher
	}

	if next.Enabled && next.URL == "" {
		return errors.New("启用同步前请先填写服务器地址")
	}
	// 地址的格式规则（必须 https 等）由 webdav.New 独家判定，此处不重复一遍 ——
	// 两处各写一份迟早会不一致。
	return v.store.SetJSONSetting(webdavKey, next)
}

// Credentials 取出解密后的凭据，供组装 webdav.Client。
//
// 密钥文件丢失或密文损坏时返回 secret.ErrCredentialsLost，调用方应提示用户
// 重新输入密码 —— 这不是崩溃条件，换机器或清过配置目录都会走到这里。
func (v *Vault) Credentials() (WebDAVCredentials, error) {
	cfg, err := v.load()
	if err != nil {
		return WebDAVCredentials{}, err
	}
	if cfg.PasswordCipher == "" {
		return WebDAVCredentials{}, ErrNoPassword
	}
	password, err := v.cipher.Decrypt(cfg.PasswordCipher)
	if err != nil {
		return WebDAVCredentials{}, err
	}
	return WebDAVCredentials{
		URL:      cfg.URL,
		Username: cfg.Username,
		Password: password,
	}, nil
}

// Enabled 报告用户是否开启了同步。
func (v *Vault) Enabled() (bool, error) {
	cfg, err := v.load()
	if err != nil {
		return false, err
	}
	return cfg.Enabled, nil
}

// Clear 删除全部连接配置（含密文）。用于设置页的「移除配置」。
//
// 与 Save(Enabled: false) 不同：后者只是停用，密码留着，重新启用时不用再输一遍。
func (v *Vault) Clear() error {
	return v.store.DeleteSetting(webdavKey)
}

// load 读取落库的配置；从未保存过时返回零值。
func (v *Vault) load() (webdavConfig, error) {
	var cfg webdavConfig
	if _, err := v.store.GetJSONSetting(webdavKey, &cfg); err != nil {
		return webdavConfig{}, err
	}
	return cfg, nil
}
