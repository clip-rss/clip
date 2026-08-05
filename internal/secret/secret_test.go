package secret

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func keyPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), KeyFileName)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	c, err := NewCipher(keyPath(t))
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}

	for _, want := range []string{
		"hunter2",
		"含中文的密码",
		"with spaces and 符号 !@#$%^&*()",
		strings.Repeat("x", 500), // 坚果云的应用密码不短
	} {
		token, err := c.Encrypt(want)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", want, err)
		}
		if strings.Contains(token, want) {
			t.Errorf("密文里出现了明文: %q", token)
		}
		got, err := c.Decrypt(token)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if got != want {
			t.Errorf("往返 = %q, want %q", got, want)
		}
	}
}

// TestEmptyStringMeansNoPassword 空串表示「没设密码」，两个方向都原样透传。
// 若加密空串得到一段密文，hasPassword 的判断（密文非空）就会永远为真。
func TestEmptyStringMeansNoPassword(t *testing.T) {
	c, err := NewCipher(keyPath(t))
	if err != nil {
		t.Fatal(err)
	}

	token, err := c.Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt(\"\"): %v", err)
	}
	if token != "" {
		t.Errorf("加密空串得到 %q, want 空串", token)
	}
	got, err := c.Decrypt("")
	if err != nil {
		t.Fatalf("Decrypt(\"\"): %v", err)
	}
	if got != "" {
		t.Errorf("解密空串得到 %q, want 空串", got)
	}
}

// TestNonceIsRandom 同一明文两次加密必须得到不同密文。
// 固定 nonce 会让相同密码产生相同密文，可被用来判断两个用户是否同密码。
func TestNonceIsRandom(t *testing.T) {
	c, err := NewCipher(keyPath(t))
	if err != nil {
		t.Fatal(err)
	}

	a, err := c.Encrypt("same-password")
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.Encrypt("same-password")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("同一明文两次加密得到相同密文，nonce 未随机化")
	}
}

// TestKeyPersistsAcrossInstances 重启后必须还能解开上次存的密码。
func TestKeyPersistsAcrossInstances(t *testing.T) {
	path := keyPath(t)

	first, err := NewCipher(path)
	if err != nil {
		t.Fatal(err)
	}
	token, err := first.Encrypt("hunter2")
	if err != nil {
		t.Fatal(err)
	}

	second, err := NewCipher(path) // 模拟下次启动
	if err != nil {
		t.Fatalf("NewCipher (第二次): %v", err)
	}
	got, err := second.Decrypt(token)
	if err != nil {
		t.Fatalf("重启后解密失败: %v", err)
	}
	if got != "hunter2" {
		t.Errorf("重启后解出 %q, want hunter2", got)
	}
}

/* ---------- 密钥文件缺失 / 损坏 ---------- */

// TestMissingKeyMeansCredentialsLost 密钥文件被删后不能崩，
// 而要报成「凭据失效」，让用户重新输入密码。
func TestMissingKeyMeansCredentialsLost(t *testing.T) {
	path := keyPath(t)
	first, err := NewCipher(path)
	if err != nil {
		t.Fatal(err)
	}
	token, err := first.Encrypt("hunter2")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	// 重建：不报错，且会生成一把新密钥。
	second, err := NewCipher(path)
	if err != nil {
		t.Fatalf("密钥缺失时 NewCipher 应重建而非报错: %v", err)
	}
	if _, err := second.Decrypt(token); !errors.Is(err, ErrCredentialsLost) {
		t.Errorf("err = %v, want ErrCredentialsLost", err)
	}

	// 新密钥本身可用：用户重新输入的密码存得进、取得出。
	fresh, err := second.Encrypt("new-password")
	if err != nil {
		t.Fatal(err)
	}
	got, err := second.Decrypt(fresh)
	if err != nil || got != "new-password" {
		t.Errorf("重建后 = (%q, %v), want (new-password, nil)", got, err)
	}
}

// TestWrongSizeKeyIsReplaced 长度不符的密钥文件必须被替换。
//
// AES-256 只接受 32 字节，长度不对的文件解不开任何东西 —— 报错留着它
// 只会挡住恢复路径：用户重新输入的密码也存不进去。
func TestWrongSizeKeyIsReplaced(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{"空文件", nil},
		{"被截断", make([]byte, 7)},
		{"过长", make([]byte, keySize+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := keyPath(t)
			if err := os.WriteFile(path, tc.body, 0o600); err != nil {
				t.Fatal(err)
			}

			c, err := NewCipher(path)
			if err != nil {
				t.Fatalf("NewCipher: %v", err)
			}
			token, err := c.Encrypt("hunter2")
			if err != nil {
				t.Fatal(err)
			}
			if got, err := c.Decrypt(token); err != nil || got != "hunter2" {
				t.Errorf("替换后 = (%q, %v), want (hunter2, nil)", got, err)
			}

			// 文件已被换成合法长度。
			key, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(key) != keySize {
				t.Errorf("密钥文件长度 = %d, want %d", len(key), keySize)
			}
		})
	}
}

// TestTamperedCiphertextIsRejected GCM 的认证标签必须真的在校验。
// 少了它，被改过的密文会解出一段垃圾，然后被当成密码发给服务器。
func TestTamperedCiphertextIsRejected(t *testing.T) {
	c, err := NewCipher(keyPath(t))
	if err != nil {
		t.Fatal(err)
	}
	token, err := c.Encrypt("hunter2")
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"改密文中段":    flipByteAt(t, token, 20), // 落在密文区
		"改 nonce":  flipByteAt(t, token, 0),  // 落在 nonce 区
		"改认证标签":    flipByteAt(t, token, -1), // 末字节属 GCM 标签
		"截断":       token[:len(token)/2],
		"非 base64": "这不是 base64!!",
		"太短":       "AAAA",
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			if bad == token {
				t.Fatal("构造的坏数据与原密文相同，测试无效")
			}
			if _, err := c.Decrypt(bad); !errors.Is(err, ErrCredentialsLost) {
				t.Errorf("err = %v, want ErrCredentialsLost", err)
			}
		})
	}
}

// flipByteAt 解出 base64、翻掉第 i 个字节（负数从末尾算）、再编回去。
//
// 直接改 base64 字符是不行的：末尾字符的低位在解码时会被丢弃，
// 改了可能解出完全相同的字节序列，测试就成了假绿。所以在字节层面动手。
func flipByteAt(t *testing.T, token string, i int) string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("解码测试密文失败: %v", err)
	}
	if i < 0 {
		i += len(raw)
	}
	if i < 0 || i >= len(raw) {
		t.Fatalf("下标 %d 超出密文长度 %d", i, len(raw))
	}
	raw[i] ^= 0xff
	return base64.StdEncoding.EncodeToString(raw)
}

// TestKeyFromOtherMachineCannotDecrypt 换一把密钥就解不开 ——
// 这正是「库被单独拿走也拿不到密码」所依赖的性质。
func TestKeyFromOtherMachineCannotDecrypt(t *testing.T) {
	mine, err := NewCipher(keyPath(t))
	if err != nil {
		t.Fatal(err)
	}
	token, err := mine.Encrypt("hunter2")
	if err != nil {
		t.Fatal(err)
	}

	// 攻击者拿到了库（密文），但没有密钥文件，只能用自己的。
	theirs, err := NewCipher(keyPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := theirs.Decrypt(token); err == nil {
		t.Errorf("用别的密钥解出了 %q，加密形同虚设", got)
	} else if !errors.Is(err, ErrCredentialsLost) {
		t.Errorf("err = %v, want ErrCredentialsLost", err)
	}
}

/* ---------- 文件权限与落盘 ---------- */

// TestKeyFilePermissions 密钥文件必须是 0600：同机器上的其他用户不该读到它。
func TestKeyFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 的权限模型不同，ACL 不体现在 FileMode 上")
	}

	path := keyPath(t)
	if _, err := NewCipher(path); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("密钥文件权限 = %04o, want 0600", perm)
	}
}

// TestReplacedKeyFilePermissions 走替换路径（原子写）时权限同样要收紧。
func TestReplacedKeyFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 的权限模型不同")
	}

	path := keyPath(t)
	// 故意留一个宽权限的坏文件，逼 NewCipher 走替换路径。
	if err := os.WriteFile(path, []byte("too short"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewCipher(path); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("替换后权限 = %04o, want 0600", perm)
	}
}

// TestCreatesMissingDirectory 密钥可能先于数据库目录被创建，不该因此失败。
func TestCreatesMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip", "nested", KeyFileName)

	if _, err := NewCipher(path); err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("密钥文件未创建: %v", err)
	}
}

// TestNoTempFilesLeftBehind 中转用的临时文件不能残留 ——
// 那是一把完整有效的密钥，留在磁盘上等于多一份泄露面。
//
// ⚠️ 两条发布路径都要覆盖，它们的清理机制不同：
//   - 首次创建走 os.Link，链接建好后临时文件**依然存在**，必须显式删
//   - 替换坏文件走 os.Rename，重命名本身就消耗掉了临时文件
//
// 只测替换路径是不够的（这个漏洞是变异验证撞出来的：删掉 createKey 里的
// os.Remove，只有替换路径的测试照样绿）。
func TestNoTempFilesLeftBehind(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(t *testing.T, path string)
	}{
		{
			name:    "首次创建（走 link）",
			prepare: func(t *testing.T, path string) {},
		},
		{
			name: "替换坏文件（走 rename）",
			prepare: func(t *testing.T, path string) {
				if err := os.WriteFile(path, []byte("bad"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, KeyFileName)
			tc.prepare(t, path)

			if _, err := NewCipher(path); err != nil {
				t.Fatal(err)
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				if e.Name() != KeyFileName {
					t.Errorf("残留文件 %q（内含一份可用的密钥）", e.Name())
				}
			}
			if len(entries) != 1 {
				t.Errorf("目录内有 %d 个文件, want 1", len(entries))
			}
		})
	}
}

/* ---------- 文件系统异常 ---------- */

// TestKeyPathIsDirectory 密钥路径被一个目录占住时要报错，不能崩、也不能
// 悄悄把那个目录删掉。
//
// 真实成因：用户用同步盘同步过配置目录、或某次恢复操作留下了错误的条目。
// 这种情况我们无权自行清理 —— 目录里可能有别的东西。
func TestKeyPathIsDirectory(t *testing.T) {
	path := keyPath(t)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := NewCipher(path)
	if err == nil {
		t.Fatal("密钥路径是目录时应报错")
	}
	if !strings.Contains(err.Error(), "密钥") {
		t.Errorf("错误信息应说明是密钥文件的问题: %v", err)
	}
	// 目录必须还在。
	fi, statErr := os.Stat(path)
	if statErr != nil || !fi.IsDir() {
		t.Error("原有目录被删掉了")
	}
}

// TestKeyDirectoryBlockedByFile 上级路径被普通文件占住时报错而非崩溃。
func TestKeyDirectoryBlockedByFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "clip")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := NewCipher(filepath.Join(blocker, KeyFileName)); err == nil {
		t.Error("上级路径是文件时应报错")
	}
}

/* ---------- 并发 ---------- */

// TestConcurrentNewCipherAgreesOnOneKey 多个实例同时首次启动时，
// 必须都用上同一把密钥。
//
// 各自生成再覆盖写的话，后写的那把胜出，而先到者内存里的密钥与文件不一致 ——
// 它随后加密存库的密码，重启后永远解不开。用 -race 跑这个测试。
func TestConcurrentNewCipherAgreesOnOneKey(t *testing.T) {
	path := keyPath(t)

	const n = 8
	tokens := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c, err := NewCipher(path)
			if err != nil {
				t.Errorf("NewCipher: %v", err)
				return
			}
			token, err := c.Encrypt("hunter2")
			if err != nil {
				t.Errorf("Encrypt: %v", err)
				return
			}
			tokens[i] = token
		}(i)
	}
	wg.Wait()

	// 最终落盘的那把密钥必须能解开每个实例加密的内容。
	final, err := NewCipher(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, token := range tokens {
		if token == "" {
			continue // 该 goroutine 已单独报错
		}
		got, err := final.Decrypt(token)
		if err != nil {
			t.Errorf("实例 %d 加密的内容解不开：%v（并发首启时密钥被覆盖）", i, err)
			continue
		}
		if got != "hunter2" {
			t.Errorf("实例 %d 解出 %q, want hunter2", i, got)
		}
	}
}

// TestConcurrentEncryptDecrypt Cipher 会被多个 goroutine 共用
// （手动同步与 debounce 推送各自取凭据）。用 -race 跑。
func TestConcurrentEncryptDecrypt(t *testing.T) {
	c, err := NewCipher(keyPath(t))
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				token, err := c.Encrypt("hunter2")
				if err != nil {
					t.Errorf("Encrypt: %v", err)
					return
				}
				if got, err := c.Decrypt(token); err != nil || got != "hunter2" {
					t.Errorf("往返 = (%q, %v)", got, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
