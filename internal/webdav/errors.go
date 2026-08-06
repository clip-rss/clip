package webdav

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// 哨兵错误。调用方用 errors.Is 判别，不要匹配错误文本。
var (
	// ErrNotFound 资源不存在（404）。首次同步时远端配置文件必然不存在，
	// 属正常情况，调用方应据此走「首次推送」分支而非报错。
	ErrNotFound = errors.New("webdav: resource not found")

	// ErrUnauthorized 认证失败（401 / 403）。
	ErrUnauthorized = errors.New("webdav: authentication failed")

	// ErrConflict 前置条件失败（412）。带 If-Match 的 PUT 遇此说明期间远端已被改写。
	ErrConflict = errors.New("webdav: precondition failed")

	// ErrInsufficientStorage 空间不足（507）。
	ErrInsufficientStorage = errors.New("webdav: insufficient storage")

	// ErrLocked 资源被锁定（423）。
	ErrLocked = errors.New("webdav: resource locked")

	// ErrNotCollection 父目录不存在（409）。MKCOL 遇此说明上级目录还没建。
	ErrNotCollection = errors.New("webdav: parent collection missing")

	// ErrInvalidConfig 配置不合法（地址为空 / 非 https / 缺主机名等）。
	ErrInvalidConfig = errors.New("webdav: invalid configuration")

	// ErrNetwork 网络层失败（连接不上、超时、TLS 握手失败等）。
	ErrNetwork = errors.New("webdav: network failure")

	// ErrBadResponse 响应无法解析（非法 XML、缺必要字段等）。
	ErrBadResponse = errors.New("webdav: malformed response")

	// ErrResponseTooLarge 下载内容超过调用方给定的上限。大文件下载必须显式给出
	// 上限，避免错误地址或恶意服务器耗尽磁盘。
	ErrResponseTooLarge = errors.New("webdav: response too large")
)

// Error 本包的统一错误类型。
//
// Msg 是面向用户的中文说明，与 api 层其他用户可见错误（如 api/settings.go 的
// 「代理连接失败」）保持一致的语言；Op / Path / Status 供诊断；sentinel 供
// errors.Is 判别。
//
// ⚠️ Msg 只描述「发生了什么」，不含「该怎么办」的服务商特定建议
// （如坚果云需用应用密码）。那类提示依赖用户所用服务，且需要随界面语言切换，
// 由 api / 前端按 errors.Is 的判别结果渲染。
type Error struct {
	Op     string // 操作名：config / resolve / propfind / get / put / mkcol / delete
	Path   string // 相对路径，不含凭据
	Status int    // HTTP 状态码；非 HTTP 阶段的错误为 0
	Msg    string // 面向用户的说明
	Err    error  // 底层原因，可为 nil

	sentinel error // 供 errors.Is 匹配；由构造处设定
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("webdav: ")
	b.WriteString(e.Op)
	if e.Path != "" {
		b.WriteString(" ")
		b.WriteString(e.Path)
	}
	if e.Status != 0 {
		fmt.Fprintf(&b, " [%d]", e.Status)
	}
	if e.Msg != "" {
		b.WriteString(": ")
		b.WriteString(e.Msg)
	}
	if e.Err != nil {
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

// Unwrap 同时暴露哨兵错误与底层原因，使两类判别都可用：
//
//	errors.Is(err, ErrNetwork)      // 按类判别，用于决定重试 / 提示文案
//	errors.Is(err, context.Canceled) // 穿透到根因，用于区分用户取消与真实故障
//
// 返回切片而非单个错误（Go 1.20+ 的多父形式）：只返回哨兵会截断错误链，
// 让根因不可达；只返回根因则按类判别失效。二者都需要。
func (e *Error) Unwrap() []error {
	var out []error
	if e.sentinel != nil {
		out = append(out, e.sentinel)
	}
	if e.Err != nil {
		out = append(out, e.Err)
	}
	return out
}

// configError 构造配置类错误。
func configError(msg string) *Error {
	return &Error{Op: "config", Msg: msg, sentinel: ErrInvalidConfig}
}

// statusError 把非 2xx 状态码转成 *Error；2xx 返回 nil。
//
// body 仅取首段用于诊断：WebDAV 服务器的错误体常是整页 HTML 或长 XML，
// 全量塞进错误信息会污染日志且无助于定位。
func statusError(op, path string, status int, body []byte) error {
	if status >= 200 && status < 300 {
		return nil
	}

	sentinel, msg := classifyStatus(status)
	return &Error{
		Op:       op,
		Path:     path,
		Status:   status,
		Msg:      msg,
		Err:      bodySnippet(body),
		sentinel: sentinel,
	}
}

// classifyStatus 把状态码映射到哨兵错误与用户可见说明。
func classifyStatus(status int) (error, string) {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrUnauthorized, "认证失败，请检查用户名与密码"
	case http.StatusNotFound:
		return ErrNotFound, "资源不存在"
	case http.StatusMethodNotAllowed:
		return nil, "服务器不支持该操作"
	case http.StatusConflict:
		return ErrNotCollection, "上级目录不存在"
	case http.StatusPreconditionFailed:
		return ErrConflict, "远端已被其他设备修改"
	case http.StatusLocked:
		return ErrLocked, "资源被锁定，可能有其他客户端正在写入"
	case http.StatusRequestEntityTooLarge:
		return nil, "文件超出服务器允许的大小"
	case http.StatusTooManyRequests:
		return nil, "请求过于频繁，请稍后再试"
	case http.StatusInsufficientStorage:
		return ErrInsufficientStorage, "网盘空间不足"
	}

	switch {
	case status >= 500:
		return nil, "服务器内部错误"
	case status >= 400:
		return nil, "请求被服务器拒绝"
	}
	// 3xx：已跟随重定向后仍是 3xx，通常是地址不对（如把网页地址填成了 WebDAV 地址）。
	return nil, "服务器返回了意外的重定向，请检查地址是否为 WebDAV 路径"
}

// bodySnippet 截取响应体首段作为诊断信息；无有效内容返回 nil。
func bodySnippet(body []byte) error {
	const limit = 200
	s := strings.TrimSpace(string(body))
	if s == "" {
		return nil
	}
	// 折叠换行，避免多行 HTML 把单条错误撑成一屏。
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > limit {
		s = s[:limit] + "…"
	}
	return errors.New(s)
}
