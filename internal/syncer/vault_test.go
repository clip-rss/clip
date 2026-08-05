package syncer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/clip-rss/clip/internal/secret"
	"github.com/clip-rss/clip/internal/store"
)

func newTestVault(t *testing.T) (*Vault, *store.Store) {
	t.Helper()
	st := newTestStore(t)
	// 密钥与库同目录，与生产一致（见 secret.KeyFileName 的说明）。
	cipher, err := secret.NewCipher(filepath.Join(filepath.Dir(st.Path()), secret.KeyFileName))
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	return NewVault(st, cipher), st
}

/* ---------- 密码不外泄 ---------- */

// TestViewHasNoPasswordField 回给前端的结构里不能有任何密码字段。
//
// 这是阶段 D 的核心约束，也是最容易在日后被顺手破坏的一条：
// 有人为了「方便前端回填」加一个 password 字段，密码就进了 IPC 与 webview 内存。
// 用反射守住，比靠 code review 记得住可靠。
func TestViewHasNoPasswordField(t *testing.T) {
	viewType := reflect.TypeOf(WebDAVView{})

	for i := range viewType.NumField() {
		f := viewType.Field(i)
		name := strings.ToLower(f.Name)
		tag := strings.ToLower(f.Tag.Get("json"))

		// HasPassword 是唯一允许提及密码的字段，且必须是 bool。
		if f.Name == "HasPassword" {
			if f.Type.Kind() != reflect.Bool {
				t.Errorf("HasPassword 类型 = %s, want bool", f.Type)
			}
			continue
		}
		for _, banned := range []string{"password", "secret", "cipher", "token", "credential"} {
			if strings.Contains(name, banned) || strings.Contains(tag, banned) {
				t.Errorf("WebDAVView 含疑似凭据字段 %s（json:%q）——"+
					"密码不得出现在返回给前端的结构里", f.Name, f.Tag.Get("json"))
			}
		}
	}
}

// TestViewNeverCarriesPassword 存了密码之后，View 的序列化结果里
// 不能出现明文或密文。
func TestViewNeverCarriesPassword(t *testing.T) {
	v, _ := newTestVault(t)

	const password = "hunter2-very-distinctive"
	if err := v.Save(WebDAVInput{
		Enabled:  true,
		URL:      "https://dav.example.com/dav/",
		Username: "alice",
		Password: password,
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	view, err := v.View()
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	if !view.HasPassword {
		t.Error("hasPassword = false，应为 true")
	}

	// %+v 会把每个字段都打出来，足以覆盖「有没有藏着密码字段」。
	dump := fmt.Sprintf("%+v", view)
	if strings.Contains(dump, password) {
		t.Errorf("View 里出现了明文密码: %s", dump)
	}
	// 密文也不该给前端：它对界面毫无用处，只是多一条泄露路径。
	cfg := loadRawConfig(t, v)
	if cfg.PasswordCipher == "" {
		t.Fatal("测试前提不成立：密文未落库")
	}
	if strings.Contains(dump, cfg.PasswordCipher) {
		t.Errorf("View 里出现了密文: %s", dump)
	}
}

// TestCredentialsStringRedactsPassword 凭据结构体会流经错误信息与调试输出，
// 默认打印路径必须是安全的。
func TestCredentialsStringRedactsPassword(t *testing.T) {
	c := WebDAVCredentials{
		URL:      "https://dav.example.com/dav/",
		Username: "alice",
		Password: "hunter2",
	}

	for _, format := range []string{"%v", "%s", "%+v"} {
		got := fmt.Sprintf(format, c)
		if strings.Contains(got, "hunter2") {
			t.Errorf("%s 输出了明文密码: %s", format, got)
		}
		// 地址与用户名要留着，否则错误信息失去诊断价值。
		if !strings.Contains(got, "alice") {
			t.Errorf("%s 丢掉了用户名: %s", format, got)
		}
	}

	// 区分「没设密码」与「已设但隐藏」，便于排查。
	empty := fmt.Sprintf("%v", WebDAVCredentials{URL: "u", Username: "n"})
	if !strings.Contains(empty, "未设置") {
		t.Errorf("未设密码时应标明，得到: %s", empty)
	}
}

// TestPasswordIsEncryptedAtRest 落库的必须是密文 —— 备份数据库或把库发给别人
// 排查时，明文密码会随之泄露。这是本阶段存在的全部理由。
func TestPasswordIsEncryptedAtRest(t *testing.T) {
	v, st := newTestVault(t)

	const password = "hunter2-very-distinctive"
	if err := v.Save(WebDAVInput{
		Enabled: true, URL: "https://dav.example.com/dav/", Username: "alice", Password: password,
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 直接读原始存储，模拟拿到数据库的人。
	var raw map[string]any
	found, err := st.GetJSONSetting(webdavKey, &raw)
	if err != nil || !found {
		t.Fatalf("读取原始配置失败: found=%v err=%v", found, err)
	}
	dump := fmt.Sprintf("%v", raw)
	if strings.Contains(dump, password) {
		t.Fatalf("库里存着明文密码: %s", dump)
	}
	if !strings.Contains(dump, "passwordCipher") {
		t.Errorf("未找到 passwordCipher 字段: %s", dump)
	}
}

/* ---------- 保存语义 ---------- */

// TestSaveWithEmptyPasswordKeepsExisting 前端拿不到现有密码，
// 用户只改地址时不该被迫重新输入一遍。
func TestSaveWithEmptyPasswordKeepsExisting(t *testing.T) {
	v, _ := newTestVault(t)

	if err := v.Save(WebDAVInput{
		Enabled: true, URL: "https://a.example/dav/", Username: "alice", Password: "hunter2",
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 只改地址，密码留空。
	if err := v.Save(WebDAVInput{
		Enabled: true, URL: "https://b.example/dav/", Username: "alice",
	}); err != nil {
		t.Fatalf("Save (改地址): %v", err)
	}

	creds, err := v.Credentials()
	if err != nil {
		t.Fatalf("Credentials: %v", err)
	}
	if creds.Password != "hunter2" {
		t.Errorf("密码 = %q, want hunter2（留空应保持原密码）", creds.Password)
	}
	if creds.URL != "https://b.example/dav/" {
		t.Errorf("地址 = %q, 未更新", creds.URL)
	}
}

func TestSaveReplacesPasswordWhenProvided(t *testing.T) {
	v, _ := newTestVault(t)

	if err := v.Save(WebDAVInput{
		Enabled: true, URL: "https://a.example/dav/", Username: "alice", Password: "old",
	}); err != nil {
		t.Fatal(err)
	}
	if err := v.Save(WebDAVInput{
		Enabled: true, URL: "https://a.example/dav/", Username: "alice", Password: "new",
	}); err != nil {
		t.Fatal(err)
	}

	creds, err := v.Credentials()
	if err != nil {
		t.Fatalf("Credentials: %v", err)
	}
	if creds.Password != "new" {
		t.Errorf("密码 = %q, want new", creds.Password)
	}
}

// TestSaveTrimsURLAndUsernameButNotPassword 地址从网页复制时常带空白，
// 而替用户"清理"密码会造成一个查不出原因的认证失败。
func TestSaveTrimsURLAndUsernameButNotPassword(t *testing.T) {
	v, _ := newTestVault(t)

	if err := v.Save(WebDAVInput{
		Enabled:  true,
		URL:      "  https://dav.example.com/dav/\n",
		Username: " alice ",
		Password: "  pw with spaces  ",
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	view, err := v.View()
	if err != nil {
		t.Fatal(err)
	}
	if view.URL != "https://dav.example.com/dav/" {
		t.Errorf("地址 = %q, 未去空白", view.URL)
	}
	if view.Username != "alice" {
		t.Errorf("用户名 = %q, 未去空白", view.Username)
	}

	creds, err := v.Credentials()
	if err != nil {
		t.Fatal(err)
	}
	if creds.Password != "  pw with spaces  " {
		t.Errorf("密码 = %q, 空白被裁掉了", creds.Password)
	}
}

// TestDisableKeepsPassword 停用同步不该丢密码，否则重新启用要再输一遍。
func TestDisableKeepsPassword(t *testing.T) {
	v, _ := newTestVault(t)

	if err := v.Save(WebDAVInput{
		Enabled: true, URL: "https://a.example/dav/", Username: "alice", Password: "hunter2",
	}); err != nil {
		t.Fatal(err)
	}
	if err := v.Save(WebDAVInput{
		Enabled: false, URL: "https://a.example/dav/", Username: "alice",
	}); err != nil {
		t.Fatalf("Save (停用): %v", err)
	}

	enabled, err := v.Enabled()
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Error("停用后 Enabled 仍为 true")
	}
	view, err := v.View()
	if err != nil {
		t.Fatal(err)
	}
	if !view.HasPassword {
		t.Error("停用把密码丢了，重新启用需再输一遍")
	}
}

// TestEnableWithoutURLIsRejected 启用同步但没填地址是配置错误，
// 及早拒绝比留到同步时报网络错误清楚。
func TestEnableWithoutURLIsRejected(t *testing.T) {
	v, _ := newTestVault(t)

	err := v.Save(WebDAVInput{Enabled: true, Username: "alice", Password: "pw"})
	if err == nil {
		t.Fatal("启用同步却没有地址，应报错")
	}
	if !strings.Contains(err.Error(), "地址") {
		t.Errorf("错误信息应提到地址: %v", err)
	}

	// 未启用时允许留空（用户可能只是先填一半）。
	if err := v.Save(WebDAVInput{Enabled: false}); err != nil {
		t.Errorf("未启用时应允许留空地址: %v", err)
	}
}

func TestClearRemovesEverything(t *testing.T) {
	v, _ := newTestVault(t)

	if err := v.Save(WebDAVInput{
		Enabled: true, URL: "https://a.example/dav/", Username: "alice", Password: "hunter2",
	}); err != nil {
		t.Fatal(err)
	}
	if err := v.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	view, err := v.View()
	if err != nil {
		t.Fatalf("View after Clear: %v", err)
	}
	if view != (WebDAVView{}) {
		t.Errorf("清除后仍有残留: %+v", view)
	}
	if _, err := v.Credentials(); !errors.Is(err, ErrNoPassword) {
		t.Errorf("err = %v, want ErrNoPassword", err)
	}
}

/* ---------- 未配置 / 凭据失效 ---------- */

func TestViewOnFreshInstall(t *testing.T) {
	v, _ := newTestVault(t)

	view, err := v.View()
	if err != nil {
		t.Fatalf("从未配置时 View 不应报错: %v", err)
	}
	if view != (WebDAVView{}) {
		t.Errorf("view = %+v, want 零值", view)
	}
}

func TestCredentialsWithoutPassword(t *testing.T) {
	v, _ := newTestVault(t)

	if err := v.Save(WebDAVInput{Enabled: false, URL: "https://a.example/dav/"}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Credentials(); !errors.Is(err, ErrNoPassword) {
		t.Errorf("err = %v, want ErrNoPassword", err)
	}
}

// TestCredentialsAfterKeyLoss 密钥文件被删（换机器、清配置目录、恢复旧库）
// 之后不能崩，要报成「凭据失效」让用户重新输入。
func TestCredentialsAfterKeyLoss(t *testing.T) {
	st := newTestStore(t)
	keyPath := filepath.Join(filepath.Dir(st.Path()), secret.KeyFileName)

	cipher, err := secret.NewCipher(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	v := NewVault(st, cipher)
	if err := v.Save(WebDAVInput{
		Enabled: true, URL: "https://a.example/dav/", Username: "alice", Password: "hunter2",
	}); err != nil {
		t.Fatal(err)
	}

	// 模拟下次启动：密钥没了，重新生成一把。
	if err := os.Remove(keyPath); err != nil {
		t.Fatal(err)
	}
	newCipher, err := secret.NewCipher(keyPath)
	if err != nil {
		t.Fatalf("密钥缺失时不应报错: %v", err)
	}
	v2 := NewVault(st, newCipher)

	if _, err := v2.Credentials(); !errors.Is(err, secret.ErrCredentialsLost) {
		t.Errorf("err = %v, want secret.ErrCredentialsLost", err)
	}

	// 其余配置仍在，界面能显示地址并提示重新输入密码。
	view, err := v2.View()
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	if view.URL != "https://a.example/dav/" || !view.Enabled {
		t.Errorf("密钥丢失不该影响非敏感配置: %+v", view)
	}

	// 用户重新输入后即恢复。
	if err := v2.Save(WebDAVInput{
		Enabled: true, URL: "https://a.example/dav/", Username: "alice", Password: "again",
	}); err != nil {
		t.Fatalf("重新保存: %v", err)
	}
	creds, err := v2.Credentials()
	if err != nil {
		t.Fatalf("恢复后 Credentials: %v", err)
	}
	if creds.Password != "again" {
		t.Errorf("密码 = %q, want again", creds.Password)
	}
}

/* ---------- 辅助 ---------- */

// loadRawConfig 读出落库的原始配置（含密文），仅测试用。
func loadRawConfig(t *testing.T, v *Vault) webdavConfig {
	t.Helper()
	cfg, err := v.load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return cfg
}
