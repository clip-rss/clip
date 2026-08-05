package syncer

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPayloadRoundTrip(t *testing.T) {
	base := settingsA()
	now := time.Date(2026, 8, 5, 10, 30, 0, 0, time.UTC)
	want := newPayload(syncableB(), 7, now)

	data, err := Encode(want)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := Decode(data, base)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if got.SchemaVersion != SchemaVersion {
		t.Errorf("schemaVersion = %d, want %d", got.SchemaVersion, SchemaVersion)
	}
	if got.Revision != 7 {
		t.Errorf("revision = %d, want 7", got.Revision)
	}
	if !got.UpdatedAt.Equal(now) {
		t.Errorf("updatedAt = %v, want %v", got.UpdatedAt, now)
	}
	if got.DeviceName == "" {
		t.Error("deviceName 为空")
	}
	if got.Settings != syncableB() {
		t.Errorf("settings = %+v, want %+v", got.Settings, syncableB())
	}
}

// TestPayloadStoresUTC 载荷里的时间统一存 UTC：两台机器在不同时区，
// 展示时由前端转本地时区，存储层不掺时区。
func TestPayloadStoresUTC(t *testing.T) {
	tokyo := time.FixedZone("JST", 9*3600)
	local := time.Date(2026, 8, 5, 19, 0, 0, 0, tokyo)

	p := newPayload(syncableB(), 1, local)
	if p.UpdatedAt.Location() != time.UTC {
		t.Errorf("updatedAt 时区 = %v, want UTC", p.UpdatedAt.Location())
	}
	if !p.UpdatedAt.Equal(local) {
		t.Errorf("updatedAt = %v，与原时刻 %v 不等", p.UpdatedAt, local)
	}
}

// TestDecodeSeedsMissingFieldsFromBase 旧版本客户端推上来的载荷缺字段时，
// 必须保持本机原值，不能退化成零值。
//
// 这条是布尔字段的命门：若从零值开始反序列化，缺失的 showUnreadBadge 会变成
// false，而 Apply 无法区分「对方显式关掉了」与「对方那版还没这个字段」，
// 用户本机的开关就被静默关掉了。
func TestDecodeSeedsMissingFieldsFromBase(t *testing.T) {
	base := settingsA() // 各布尔字段均为 true
	if !base.ShowUnreadBadge || !base.ReduceMotion {
		t.Fatal("fixture 前提不成立：settingsA 的布尔字段应为 true")
	}

	// 只带主题的老载荷：其余字段一概没有。
	old := `{"schemaVersion":1,"revision":3,"deviceName":"old-mac","settings":{"theme":"dark"}}`

	got, err := Decode([]byte(old), base)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Settings.Theme != "dark" {
		t.Errorf("theme = %q, want dark", got.Settings.Theme)
	}
	if !got.Settings.ShowUnreadBadge {
		t.Error("showUnreadBadge 被缺失字段冲成 false，应保持本机的 true")
	}
	if !got.Settings.ReduceMotion {
		t.Error("reduceMotion 被缺失字段冲成 false，应保持本机的 true")
	}
	if got.Settings.ReaderFontSize != base.ReaderFontSize {
		t.Errorf("readerFontSize = %d, want %d（本机原值）",
			got.Settings.ReaderFontSize, base.ReaderFontSize)
	}

	// 应用后本机配置除主题外不应有任何变化。
	out := Apply(base, got.Settings)
	want := base
	want.Theme = "dark"
	if out != want {
		t.Errorf("应用老载荷后设置 = %+v，应仅主题变化 %+v", out, want)
	}
}

// TestDecodeRejectsNewerSchema 高版本载荷拒绝应用。
//
// 不能降级读取：高版本可能改了字段语义，按本端理解应用等于静默改坏配置。
func TestDecodeRejectsNewerSchema(t *testing.T) {
	future := `{"schemaVersion":99,"revision":1,"settings":{"theme":"dark"}}`

	_, err := Decode([]byte(future), settingsA())
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("err = %v, want ErrSchemaTooNew", err)
	}
	// 错误信息要带上两个版本号，用户才知道该升到哪儿去。
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("错误信息应包含远端版本号 99: %v", err)
	}
}

// TestDecodeRejectsForeignFile 缺 schemaVersion 的文件不是 Clip 写的，
// 必须拒绝 —— 同步目录指错时覆盖它会毁掉用户的其他文件。
func TestDecodeRejectsForeignFile(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"无版本字段", `{"hello":"world"}`},
		{"版本为 0", `{"schemaVersion":0,"settings":{}}`},
		{"空对象", `{}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Decode([]byte(c.data), settingsA()); !errors.Is(err, ErrForeignPayload) {
				t.Errorf("err = %v, want ErrForeignPayload", err)
			}
		})
	}
}

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	for _, data := range []string{`{not json`, ``, `<html>登录页</html>`} {
		if _, err := Decode([]byte(data), settingsA()); !errors.Is(err, ErrMalformedPayload) {
			t.Errorf("data %q: err = %v, want ErrMalformedPayload", data, err)
		}
	}
}

// TestDecodeIgnoresUnknownFields 高版本新增的字段不该让解析失败 ——
// 只要 schemaVersion 兼容，多出来的字段忽略即可。
func TestDecodeIgnoresUnknownFields(t *testing.T) {
	data := `{"schemaVersion":1,"revision":2,"deviceName":"new-pc",
		"settings":{"theme":"sepia","futureOption":"whatever"},
		"futureTopLevel":{"a":1}}`

	got, err := Decode([]byte(data), settingsA())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Settings.Theme != "sepia" {
		t.Errorf("theme = %q, want sepia", got.Settings.Theme)
	}
}

// TestEncodeIsHumanReadable 这份文件放在用户自己的网盘里，用户会直接打开看。
func TestEncodeIsHumanReadable(t *testing.T) {
	data, err := Encode(newPayload(syncableB(), 1, time.Now()))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(string(data), "\n  ") {
		t.Error("载荷未缩进")
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Error("载荷未以换行结尾")
	}
	// 载荷体积应远小于 webdav 包的 1 MiB 响应上限。
	if len(data) > 4096 {
		t.Errorf("载荷 %d 字节，意外地大", len(data))
	}

	// 不得含任何凭据字段：这份文件在网盘上，任何能访问该网盘的人都能读。
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("载荷不是合法 JSON: %v", err)
	}
	for _, banned := range []string{"password", "passwordCipher", "username", "url"} {
		if _, ok := raw[banned]; ok {
			t.Errorf("载荷含敏感字段 %q", banned)
		}
	}
}

func TestDeviceNameNeverEmpty(t *testing.T) {
	if DeviceName() == "" {
		t.Error("DeviceName 返回空串；取不到机器名时应给占位值")
	}
}
