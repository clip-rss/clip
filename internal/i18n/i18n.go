// Package i18n contains the small set of translations that must be produced
// by the Go backend. UI translations remain in the frontend locale files.
package i18n

import (
	"errors"
	"fmt"
	"strings"
)

const (
	English           = "en"
	SimplifiedChinese = "zh-CN"
)

var messages = map[string]map[string]string{
	English: {
		"feed.urlEmpty":                  "Feed URL is required",
		"feed.notFound":                  "No subscribable feed was found at this address",
		"feed.fetchFailed":               "Failed to fetch feed",
		"feed.emptyResponse":             "The feed returned an empty response",
		"feed.alreadyExists":             "This feed is already subscribed: %s",
		"category.nameEmpty":             "Category name is required",
		"opml.contentEmpty":              "OPML content is empty",
		"opml.unnamedCategory":           "Untitled category",
		"opml.export":                    "Export subscriptions",
		"opml.fileFilter":                "OPML files",
		"backup.restoreSelection":        "Select a backup to restore",
		"backup.deleteSelection":         "Select a backup to delete",
		"proxy.invalid":                  "Proxy host or port is invalid",
		"proxy.invalidAddress":           "Invalid proxy address",
		"proxy.connectionFailed":         "Proxy connection failed",
		"proxy.badStatus":                "Proxy returned an unexpected status: %s",
		"app.unavailable":                "Application is not available",
		"app.description":                "A simple cross-platform RSS reader",
		"updater.title":                  "Software Update",
		"database.backup":                "Back up database",
		"database.restore":               "Restore database",
		"database.fileFilter":            "Clip databases",
		"changelog.notConfigured":        "Changelog URL is not configured",
		"changelog.fetchFailed":          "Failed to fetch changelog",
		"changelog.badStatus":            "Changelog server returned HTTP %d",
		"changelog.readFailed":           "Failed to read changelog",
		"webdav.credentialsUnavailable":  "Credential storage is unavailable; check the configuration directory permissions and restart",
		"webdav.notConfigured":           "No WebDAV server is configured",
		"webdav.passwordRequired":        "Password is required",
		"webdav.hintUnauthorized":        "For Jianguoyun, create an app password in Security instead of using your login password. Nextcloud with two-factor authentication also requires an app password.",
		"webdav.hintNotCollection":       "The parent directory does not exist. Make sure the address points to the WebDAV root, for example https://<host>/remote.php/dav/files/<user>/ for Nextcloud.",
		"webdav.hintNotFound":            "The address does not exist. A common cause is entering only the domain and omitting the WebDAV path.",
		"webdav.hintInvalidConfig":       "Check the address format; it must start with https://.",
		"webdav.hintInsufficientStorage": "Not enough storage space. Free some space and try again.",
		"webdav.hintCredentialsLost":     "The local credential key is no longer valid (often caused by moving the machine or clearing the configuration directory). Enter the password again.",
		"webdav.hintNetwork":             "Check your network connection and proxy settings.",
		"webdav.urlEmpty":                "Server address is required",
		"webdav.invalidAddress":          "Invalid server address",
		"webdav.httpsRequired":           "The server address must start with https://",
		"webdav.hostRequired":            "The server address is missing a host name",
		"webdav.networkFailure":          "Unable to connect to the server",
		"webdav.readResponseFailed":      "Failed to read the server response",
		"webdav.badResponse":             "The server returned an unreadable response",
		"webdav.noResourceInfo":          "The server returned no resource information",
		"webdav.parentMissing":           "The parent directory does not exist",
		"webdav.remoteChanged":           "The remote data was changed by another device",
		"webdav.locked":                  "The resource is locked, possibly by another client",
		"webdav.insufficientStorage":     "The remote storage is full",
		"webdav.serverError":             "The server encountered an internal error",
		"webdav.requestRejected":         "The server rejected the request",
		"webdav.pathTraversal":           "The path cannot contain ..",
		"webdav.authFailed":              "Authentication failed; check the username and password",
		"webdav.notFound":                "Resource not found",
		"webdav.unsupported":             "The server does not support this operation",
		"webdav.tooLarge":                "The file exceeds the server's size limit",
		"webdav.rateLimited":             "Too many requests; try again later",
		"webdav.redirect":                "The server returned an unexpected redirect; check that this is a WebDAV path",
		"webdav.insecureHTTP":            "HTTPS is required because WebDAV credentials are sent with every request and plain HTTP would expose them in transit",
		"webdav.badWebDAVPath":           "The server response could not be parsed; make sure the address points to a WebDAV path",
		"backup.generateOPMLFailed":      "Failed to generate OPML",
		"backup.createDirectoryFailed":   "Failed to create backup directory",
		"backup.uploadFailed":            "Failed to upload backup",
		"backup.listFailed":              "Failed to list backups",
		"backup.downloadFailed":          "Failed to download backup",
		"backup.importFailed":            "Failed to import OPML backup",
		"backup.deleteFailed":            "Failed to delete backup",
		"backup.invalidID":               "Invalid backup ID",
		"secret.credentialsLost":         "The sync password is no longer valid; enter it again",
		"notify.newItems":                "%s: %d new items",
		"notify.moreItems":               "%s… and %d more",
	},
	SimplifiedChinese: {
		"feed.urlEmpty":                  "订阅地址不能为空",
		"feed.notFound":                  "未在该地址找到可订阅的源",
		"feed.fetchFailed":               "获取订阅源失败",
		"feed.emptyResponse":             "订阅源返回了空响应",
		"feed.alreadyExists":             "该订阅源已存在：%s",
		"category.nameEmpty":             "分类名称不能为空",
		"opml.contentEmpty":              "OPML 内容不能为空",
		"opml.unnamedCategory":           "未命名分类",
		"opml.export":                    "导出订阅",
		"opml.fileFilter":                "OPML 文件",
		"backup.restoreSelection":        "请选择要恢复的备份",
		"backup.deleteSelection":         "请选择要删除的备份",
		"proxy.invalid":                  "代理地址或端口无效",
		"proxy.invalidAddress":           "代理地址格式错误",
		"proxy.connectionFailed":         "代理连接失败",
		"proxy.badStatus":                "代理返回异常状态：%s",
		"app.unavailable":                "应用不可用",
		"app.description":                "简单好用的跨平台 RSS 阅读器",
		"updater.title":                  "软件更新",
		"database.backup":                "备份数据库",
		"database.restore":               "恢复数据库",
		"database.fileFilter":            "Clip 数据库",
		"changelog.notConfigured":        "未配置更新日志地址",
		"changelog.fetchFailed":          "获取更新日志失败",
		"changelog.badStatus":            "更新日志服务器返回 HTTP %d",
		"changelog.readFailed":           "读取更新日志失败",
		"webdav.credentialsUnavailable":  "凭据存储不可用，无法读写 WebDAV 密码；请检查配置目录权限后重启",
		"webdav.notConfigured":           "尚未配置 WebDAV 服务器",
		"webdav.passwordRequired":        "请输入密码",
		"webdav.hintUnauthorized":        "若使用坚果云，请在「安全选项」里生成应用密码，不要用登录密码。Nextcloud 开启两步验证后同样需要应用专用密码。",
		"webdav.hintNotCollection":       "服务器地址指向的上级目录不存在。请确认地址填到了 WebDAV 根目录，例如 Nextcloud 形如 https://<域名>/remote.php/dav/files/<用户名>/",
		"webdav.hintNotFound":            "地址不存在。常见原因是只填了域名而漏掉了 WebDAV 路径。",
		"webdav.hintInvalidConfig":       "请检查地址格式，必须以 https:// 开头。",
		"webdav.hintInsufficientStorage": "网盘空间不足，清理后重试。",
		"webdav.hintCredentialsLost":     "本机的凭据密钥已失效（常见于换机器或清理过配置目录），请重新输入密码。",
		"webdav.hintNetwork":             "请检查网络连接与代理设置。",
		"webdav.urlEmpty":                "服务器地址不能为空",
		"webdav.invalidAddress":          "服务器地址格式错误",
		"webdav.httpsRequired":           "服务器地址必须以 https:// 开头",
		"webdav.hostRequired":            "服务器地址缺少主机名",
		"webdav.networkFailure":          "无法连接服务器",
		"webdav.readResponseFailed":      "读取响应失败",
		"webdav.badResponse":             "服务器返回的内容无法解析",
		"webdav.noResourceInfo":          "服务器未返回资源信息",
		"webdav.parentMissing":           "上级目录不存在",
		"webdav.remoteChanged":           "远端已被其他设备修改",
		"webdav.locked":                  "资源被锁定，可能有其他客户端正在写入",
		"webdav.insufficientStorage":     "网盘空间不足",
		"webdav.serverError":             "服务器内部错误",
		"webdav.requestRejected":         "请求被服务器拒绝",
		"webdav.pathTraversal":           "路径不能包含 ..",
		"webdav.authFailed":              "认证失败，请检查用户名与密码",
		"webdav.notFound":                "资源不存在",
		"webdav.unsupported":             "服务器不支持该操作",
		"webdav.tooLarge":                "文件超出服务器允许的大小",
		"webdav.rateLimited":             "请求过于频繁，请稍后再试",
		"webdav.redirect":                "服务器返回了意外的重定向，请检查地址是否为 WebDAV 路径",
		"webdav.insecureHTTP":            "必须使用 https：WebDAV 的账号密码随每个请求发送，明文 http 会在链路上暴露",
		"webdav.badWebDAVPath":           "服务器返回的内容无法解析，请确认地址指向 WebDAV 路径",
		"backup.generateOPMLFailed":      "生成 OPML 失败",
		"backup.createDirectoryFailed":   "创建备份目录失败",
		"backup.uploadFailed":            "上传备份失败",
		"backup.listFailed":              "列出备份失败",
		"backup.downloadFailed":          "下载备份失败",
		"backup.importFailed":            "导入 OPML 失败",
		"backup.deleteFailed":            "删除备份失败",
		"backup.invalidID":               "无效的备份 ID",
		"secret.credentialsLost":         "同步密码已失效，请重新输入",
		"notify.newItems":                "%s 新增 %d 篇",
		"notify.moreItems":               "%s… 等 %d 篇",
	},
}

// Normalize converts locale names to canonical supported language tags.
// Only Simplified Chinese is currently supported; other locales fall back to
// English.
func Normalize(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(lang, "_", "-")))
	switch {
	case lang == "zh", lang == "zh-cn", strings.HasPrefix(lang, "zh-cn-"),
		lang == "zh-sg", strings.HasPrefix(lang, "zh-sg-"),
		strings.HasPrefix(lang, "zh-hans"):
		return SimplifiedChinese
	}
	return English
}

// IsChinese reports whether lang is a supported Simplified Chinese locale.
func IsChinese(lang string) bool {
	return Normalize(lang) == SimplifiedChinese
}

// T returns a translated message. Missing keys intentionally fall back to the
// key itself so a newly added backend message is diagnosable during development.
func T(lang, key string, args ...any) string {
	lang = Normalize(lang)
	template, ok := messages[lang][key]
	if !ok {
		template, ok = messages[English][key]
		if !ok {
			return key
		}
	}
	if len(args) == 0 {
		return template
	}
	return fmt.Sprintf(template, args...)
}

// Error creates a localized user-facing error while retaining the underlying
// cause for errors.Is/errors.As and diagnostics.
func Error(lang, key string, cause error, args ...any) error {
	message := T(lang, key, args...)
	if cause == nil {
		return errors.New(message)
	}
	return fmt.Errorf("%s: %w", message, cause)
}

type localizedError struct {
	message string
	cause   error
}

func (e localizedError) Error() string { return e.message }
func (e localizedError) Unwrap() error { return e.cause }

// LocalizeError translates known backend phrases in an existing error chain.
// It is used at API boundaries for errors produced by lower-level packages
// that predate backend i18n; unknown diagnostic text is left untouched.
func LocalizeError(lang string, err error) error {
	if err == nil || IsChinese(lang) {
		return err
	}
	message := err.Error()
	for _, replacement := range legacyPhraseKeys {
		message = strings.ReplaceAll(message, replacement.phrase, T(lang, replacement.key))
	}
	if message == err.Error() {
		return err
	}
	return localizedError{message: message, cause: err}
}

type phraseTranslation struct {
	phrase string
	key    string
}

// Longer phrases must precede their shorter substrings.
var legacyPhraseKeys = []phraseTranslation{
	{"必须使用 https：WebDAV 的账号密码随每个请求发送，明文 http 会在链路上暴露", "webdav.insecureHTTP"},
	{"服务器返回的内容无法解析，请确认地址指向 WebDAV 路径", "webdav.badWebDAVPath"},
	{"服务器返回了意外的重定向，请检查地址是否为 WebDAV 路径", "webdav.redirect"},
	{"资源被锁定，可能有其他客户端正在写入", "webdav.locked"},
	{"认证失败，请检查用户名与密码", "webdav.authFailed"},
	{"feed url is empty", "feed.urlEmpty"},
	{"empty feed response", "feed.emptyResponse"},
	{"category name is empty", "category.nameEmpty"},
	{"opml content is empty", "opml.contentEmpty"},
	{"application not available", "app.unavailable"},
	{"代理地址或端口无效", "proxy.invalid"},
	{"代理地址格式错误", "proxy.invalidAddress"},
	{"代理连接失败", "proxy.connectionFailed"},
	{"服务器地址不能为空", "webdav.urlEmpty"},
	{"服务器地址格式错误", "webdav.invalidAddress"},
	{"服务器地址必须以 https:// 开头", "webdav.httpsRequired"},
	{"服务器地址缺少主机名", "webdav.hostRequired"},
	{"无法连接服务器", "webdav.networkFailure"},
	{"读取响应失败", "webdav.readResponseFailed"},
	{"服务器返回的内容无法解析", "webdav.badResponse"},
	{"服务器未返回资源信息", "webdav.noResourceInfo"},
	{"上级目录不存在", "webdav.parentMissing"},
	{"远端已被其他设备修改", "webdav.remoteChanged"},
	{"网盘空间不足", "webdav.insufficientStorage"},
	{"服务器内部错误", "webdav.serverError"},
	{"请求被服务器拒绝", "webdav.requestRejected"},
	{"路径不能包含 ..", "webdav.pathTraversal"},
	{"资源不存在", "webdav.notFound"},
	{"服务器不支持该操作", "webdav.unsupported"},
	{"文件超出服务器允许的大小", "webdav.tooLarge"},
	{"请求过于频繁，请稍后再试", "webdav.rateLimited"},
	{"生成 OPML 失败", "backup.generateOPMLFailed"},
	{"创建备份目录失败", "backup.createDirectoryFailed"},
	{"上传备份失败", "backup.uploadFailed"},
	{"列出备份失败", "backup.listFailed"},
	{"下载备份失败", "backup.downloadFailed"},
	{"导入 OPML 失败", "backup.importFailed"},
	{"删除备份失败", "backup.deleteFailed"},
	{"无效的备份 ID", "backup.invalidID"},
	{"同步密码已失效，请重新输入", "secret.credentialsLost"},
}
