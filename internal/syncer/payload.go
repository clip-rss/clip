package syncer

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/clip-rss/clip/internal/store"
)

// SchemaVersion 当前同步载荷的格式版本。
//
// 递增规则：只在**旧客户端无法安全读取新载荷**时才加（如字段语义变更）。
// 单纯新增字段不需要递增 —— 旧客户端反序列化时会忽略不认识的字段，
// 且 Decode 以本端当前设置为基底，缺字段自然保持本机原值。
const SchemaVersion = 1

// Payload 存放在 WebDAV 上的同步文件内容。
//
// DeviceName / UpdatedAt 不参与任何同步决策，只用于冲突弹窗向用户交代
// 「远端由哪台机器、什么时候写的」—— 没有这两项，用户面对二选一时无从判断。
type Payload struct {
	SchemaVersion int              `json:"schemaVersion"`
	Revision      int              `json:"revision"`   // 单调递增，仅供展示与诊断
	UpdatedAt     time.Time        `json:"updatedAt"`  // UTC
	DeviceName    string           `json:"deviceName"` // 写入方机器名
	Settings      SyncableSettings `json:"settings"`
}

// Encode 序列化为待上传的字节。
//
// 带缩进：这是一份放在用户自己网盘里的配置文件，用户会直接打开看。
// 体积从约 400 字节涨到约 600 字节，无关紧要。
func Encode(p Payload) ([]byte, error) {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("同步载荷序列化失败: %w", err)
	}
	return append(b, '\n'), nil
}

// Decode 解析远端载荷。base 提供缺失字段的缺省值。
//
// ⚠️ 以 base 为基底反序列化，与 store.GetSettings 同样的思路：旧版本客户端推上来的
// 载荷可能缺新增字段。若从零值开始，缺失的 bool 字段会变成 false —— 而 Apply
// 无法区分「显式设为 false」与「字段不存在」，用户本机的开关会被静默关掉。
func Decode(data []byte, base store.Settings) (Payload, error) {
	// 先探测版本，再决定是否解析其余部分：版本不兼容时不该把任何值读进来。
	var probe struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return Payload{}, fmt.Errorf("%w: %v", ErrMalformedPayload, err)
	}
	switch {
	case probe.SchemaVersion < 1:
		// 从未有版本写出过 0。缺字段说明这不是 Clip 的同步文件
		// （用户可能把同步目录指到了别的东西上），不能覆盖它。
		return Payload{}, ErrForeignPayload
	case probe.SchemaVersion > SchemaVersion:
		return Payload{}, fmt.Errorf("%w（远端格式版本 %d，本端支持 %d）",
			ErrSchemaTooNew, probe.SchemaVersion, SchemaVersion)
	}

	p := Payload{Settings: From(base)}
	if err := json.Unmarshal(data, &p); err != nil {
		return Payload{}, fmt.Errorf("%w: %v", ErrMalformedPayload, err)
	}
	return p, nil
}

// newPayload 组装本机当前配置的载荷。revision 由调用方按远端值递增后传入。
func newPayload(s SyncableSettings, revision int, now time.Time) Payload {
	return Payload{
		SchemaVersion: SchemaVersion,
		Revision:      revision,
		UpdatedAt:     now.UTC(), // 统一存 UTC，展示时由前端转本地时区
		DeviceName:    DeviceName(),
		Settings:      s,
	}
}

// DeviceName 返回本机名称，取不到时给一个占位值。
//
// 只用于展示，拿不到不构成同步失败。
func DeviceName() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "未知设备"
	}
	return name
}
