package syncer

import "errors"

// 哨兵错误。调用方用 errors.Is 判别，不要匹配错误文本。
//
// 文案是中文，与 internal/webdav 及 api 层其他用户可见错误保持一致的语言。
// 但只描述「发生了什么」—— 具体该怎么办（换地址 / 升级客户端）由前端按
// errors.Is 的判别结果渲染，那部分需要随界面语言切换。
var (
	// ErrNotConfigured 未配置远端。调用方应先保存 WebDAV 配置。
	ErrNotConfigured = errors.New("尚未配置同步服务器")

	// ErrSchemaTooNew 远端载荷格式版本高于本端支持，拒绝应用。
	//
	// 不能降级读取：高版本可能变更了字段语义，按本端理解应用等于静默改坏用户配置。
	// 此时也**不推送** —— 推上去会把新版客户端的配置覆盖成旧格式。
	ErrSchemaTooNew = errors.New("远端配置由更新版本的 Clip 写入，请先升级本机客户端")

	// ErrForeignPayload 远端文件不是 Clip 的同步文件（缺格式版本字段）。
	// 多为同步路径指错，覆盖它可能毁掉用户的其他文件。
	ErrForeignPayload = errors.New("远端已存在同名文件且不是 Clip 的配置备份，请更换同步目录")

	// ErrMalformedPayload 远端载荷是 Clip 的文件但内容损坏。
	ErrMalformedPayload = errors.New("远端配置文件已损坏")

	// ErrNoConflict 没有待解决的冲突时调用了 Resolve。
	ErrNoConflict = errors.New("当前没有待解决的同步冲突")
)
