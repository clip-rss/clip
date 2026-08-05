// Package secret 用一份本地密钥文件加解密需要落库的敏感字段（目前只有
// WebDAV 同步密码）。
//
// # 为什么需要它
//
// 备份数据库会把整个 clip.db 导出成一个文件，用户排查问题时也常直接把库发出来。
// 密码若以明文存在库里，会随之一起泄露。加密后即使库被单独拿走，也拿不到密码。
//
// # 防护边界（不要在界面上暗示比这更强的保护）
//
// 密钥文件与数据库在同一个目录下，权限 0600。这挡的是「**库文件**被单独拿走」
// —— 备份、发给别人排查、云盘同步误上传。它挡不住「能读取整个用户目录的攻击者」：
// 那种情况下密钥和密文一起被拿走，加密不提供任何额外保护。
//
// 要真正防住后者，得把密钥交给系统钥匙串（macOS Keychain / Windows DPAPI），
// 那会引入平台相关依赖，本项目有意不做。
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// KeyFileName 密钥文件名。调用方把它拼到数据库所在目录下 ——
// 密钥与它保护的库放在一起，语义清晰，也让测试天然隔离在临时目录里。
const KeyFileName = ".synckey"

// keySize AES-256 的密钥长度。这个长度同时也是密钥文件的格式约定：
// 长度不符即视为不可用（见 loadOrCreateKey）。
const keySize = 32

// ErrCredentialsLost 密文无法解密：密钥文件丢失后被重建、或密文本身损坏。
//
// 调用方应把它当作「凭据失效，请用户重新输入密码」，不是崩溃条件 ——
// 用户换机器、清配置目录、或恢复了一份旧数据库都会走到这里。
var ErrCredentialsLost = errors.New("同步密码已失效，请重新输入")

// Cipher 基于本地密钥文件的对称加解密器。零值不可用，须经 NewCipher 构造。
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher 打开 keyPath 处的密钥文件，不存在则生成一份新的（权限 0600）。
//
// 密钥缺失时**新建而不报错**，是为了让应用始终能启动并接受新密码：
// 报错会让用户卡在一个只能手动删文件才能解开的状态里。代价是原有密文
// 从此解不开 —— 那由 Decrypt 返回 ErrCredentialsLost 来表达。
func NewCipher(keyPath string) (*Cipher, error) {
	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("初始化加密器失败: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("初始化加密器失败: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt 加密明文，返回 base64 文本（可直接放进 JSON）。
// 空串原样返回空串，代表「没有设置密码」。
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("生成随机数失败: %w", err)
	}
	// 密文追加在 nonce 之后，一并 base64：解密时按 NonceSize 切开即可，
	// 不需要额外的分隔符或长度字段。
	sealed := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt 解密 Encrypt 产生的文本。空串返回空串。
//
// 任何解不开的情况（密钥换过、密文被截断、根本不是本函数的产物）都返回
// ErrCredentialsLost —— 对调用方而言这些情形的处置完全一样：让用户重新输入密码。
func (c *Cipher) Decrypt(token string) (string, error) {
	if token == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", ErrCredentialsLost
	}
	if len(raw) < c.aead.NonceSize() {
		return "", ErrCredentialsLost
	}
	nonce, sealed := raw[:c.aead.NonceSize()], raw[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		// GCM 的认证失败在这里收敛成同一个哨兵：区分「密钥不对」与
		// 「密文被改过」对用户没有意义，两者的下一步都是重新输入密码。
		return "", ErrCredentialsLost
	}
	return string(plaintext), nil
}

/* ---------- 密钥文件 ---------- */

// loadOrCreateKey 读取密钥；文件缺失或长度不符时生成并写入一份新的。
//
// 长度不符时**替换**而非报错：AES-256 只接受 32 字节，长度不对的文件解不开
// 任何东西，留着它只会挡住恢复路径（用户重新输入密码也存不进去）。
func loadOrCreateKey(keyPath string) ([]byte, error) {
	key, err := os.ReadFile(keyPath)
	switch {
	case err == nil && len(key) == keySize:
		return key, nil
	case err != nil && !os.IsNotExist(err):
		return nil, fmt.Errorf("读取密钥文件失败: %w", err)
	case err == nil:
		// 文件在但长度不对，走替换路径（覆盖写）。
		return replaceKey(keyPath, nil, "")
	}
	return createKey(keyPath)
}

// createKey 生成新密钥并「独占发布」到 keyPath；已被别人抢先建好则改用那一份。
//
// ⚠️ 这里的顺序很讲究，处理的是两个实例几乎同时首次启动的情形：
//
// 不能用 O_CREATE|O_EXCL 打开再往里写。那样文件会先出现、后有内容，
// 另一个实例撞到「已存在」转而去读，读到的是长度为 0 的文件 ——
// 按长度不符的规则它会**替换**掉赢家的密钥，于是赢家内存里的密钥与文件不一致，
// 它随后加密存库的密码，重启后永远解不开。
//
// 改成先把完整内容写进临时文件，再用硬链接发布：os.Link 在目标已存在时失败
// （即独占创建），而且一旦出现在 keyPath 上就必定是完整的 32 字节 ——
// 别人不可能读到中间状态。
func createKey(keyPath string) ([]byte, error) {
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成密钥失败: %w", err)
	}

	tmp, err := writeTempKey(keyPath, key)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)

	switch err := os.Link(tmp, keyPath); {
	case err == nil:
		return key, nil

	case os.IsExist(err):
		// 另一个实例抢先了。它发布的内容一定是完整的，直接用。
		existing, readErr := os.ReadFile(keyPath)
		if readErr != nil {
			return nil, fmt.Errorf("读取密钥文件失败: %w", readErr)
		}
		if len(existing) != keySize {
			// 不是并发造成的，是本来就有一份坏文件。
			return replaceKey(keyPath, key, tmp)
		}
		return existing, nil

	default:
		// 少数文件系统不支持硬链接。退回 rename 发布：失去「独占」语义
		// （并发首启时后写的胜出），但这是能用的最好选择，且不留半截文件。
		return replaceKey(keyPath, key, tmp)
	}
}

// replaceKey 用 key 覆盖 keyPath。tmp 为已写好完整内容的临时文件，
// 为空时自行写一份。
func replaceKey(keyPath string, key []byte, tmp string) ([]byte, error) {
	if key == nil {
		key = make([]byte, keySize)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("生成密钥失败: %w", err)
		}
	}
	if tmp == "" {
		var err error
		if tmp, err = writeTempKey(keyPath, key); err != nil {
			return nil, err
		}
		defer os.Remove(tmp)
	}
	// rename 覆盖目标，在同一文件系统内是原子的。
	if err := os.Rename(tmp, keyPath); err != nil {
		return nil, fmt.Errorf("保存密钥文件失败: %w", err)
	}
	return key, nil
}

// writeTempKey 把密钥完整写入 keyPath 同目录下的一个临时文件，返回其路径。
//
// 内容写完并落盘后才返回，调用方随后用 link 或 rename 把它发布出去 ——
// 「先备好完整内容，再一步发布」保证任何时刻从 keyPath 读到的都是完整的密钥。
// 直接往目标文件写的话，写入中途崩溃会留下截断的密钥，等于永久丢掉已存的密码。
func writeTempKey(keyPath string, key []byte) (string, error) {
	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("创建配置目录失败: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".synckey-*")
	if err != nil {
		return "", fmt.Errorf("创建密钥临时文件失败: %w", err)
	}
	tmpName := tmp.Name()

	cleanup := func(err error) (string, error) {
		tmp.Close()
		os.Remove(tmpName)
		return "", err
	}

	// 显式收紧权限：CreateTemp 建出来已是 0600，但发布后这个 inode 就是
	// 密钥文件本身（硬链接路径下尤其如此），权限必须在此处就位。
	if err := tmp.Chmod(0o600); err != nil {
		return cleanup(fmt.Errorf("设置密钥文件权限失败: %w", err))
	}
	if _, err := tmp.Write(key); err != nil {
		return cleanup(fmt.Errorf("写入密钥失败: %w", err))
	}
	// 落盘后再发布：否则宿主断电时可能链接已建立而内容还在页缓存里，
	// 留下一个长度为 0 的密钥文件。
	if err := tmp.Sync(); err != nil {
		return cleanup(fmt.Errorf("写入密钥失败: %w", err))
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("写入密钥失败: %w", err)
	}
	return tmpName, nil
}
