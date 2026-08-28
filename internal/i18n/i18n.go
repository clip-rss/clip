// Package i18n contains the small set of translations that must be produced
// by the Go backend. UI translations remain in the frontend locale files.
package i18n

import (
	"errors"
	"fmt"
	"strings"
)

const (
	English            = "en"
	SimplifiedChinese  = "zh-CN"
	TraditionalChinese = "zh-TW"
)

var messages = map[string]map[string]string{
	English: {
		"feed.urlEmpty":                  "Feed URL is required",
		"feed.notFound":                  "No subscribable feed was found at this address",
		"feed.htmlResponse":              "The server returned a web page instead of a feed. The site may be blocking feed readers, or this is not a feed address.",
		"feed.fetchFailed":               "Failed to fetch feed",
		"feed.emptyResponse":             "The feed returned an empty response",
		"feed.alreadyExists":             "This feed is already subscribed: %s",
		"category.nameEmpty":             "Category name is required",
		"opml.contentEmpty":              "OPML content is empty",
		"opml.unnamedCategory":           "Untitled category",
		"opml.export":                    "Export subscriptions",
		"opml.fileFilter":                "OPML files",
		"opml.urlEmpty":                  "OPML URL is required",
		"opml.urlInvalid":                "The OPML address must start with http:// or https://",
		"opml.fetchFailed":               "Failed to fetch the OPML file",
		"opml.clientUnavailable":         "The HTTP client is unavailable; restart the application",
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
		"changelog.clientUnavailable":    "The HTTP client is unavailable; restart the application",
		"changelog.fetchFailed":          "Failed to fetch changelog",
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
		"image.save":                     "Save image",
		"image.fileFilter":               "Image files",
		"image.urlEmpty":                 "Image URL is empty",
		"image.downloadUnavailable":      "Image download is unavailable",
		"image.downloadFailed":           "Failed to download image",
		"image.writeFailed":              "Failed to save image",
	},
	SimplifiedChinese: {
		"feed.urlEmpty":                  "订阅地址不能为空",
		"feed.notFound":                  "未在该地址找到可订阅的源",
		"feed.htmlResponse":              "服务器返回的是网页而不是订阅源。该站点可能拦截了阅读器访问，或这个地址不是订阅地址。",
		"feed.fetchFailed":               "获取订阅源失败",
		"feed.emptyResponse":             "订阅源返回了空响应",
		"feed.alreadyExists":             "该订阅源已存在：%s",
		"category.nameEmpty":             "分类名称不能为空",
		"opml.contentEmpty":              "OPML 内容不能为空",
		"opml.unnamedCategory":           "未命名分类",
		"opml.export":                    "导出订阅",
		"opml.fileFilter":                "OPML 文件",
		"opml.urlEmpty":                  "请输入 OPML 地址",
		"opml.urlInvalid":                "OPML 地址无效，请使用 http:// 或 https:// 开头的地址",
		"opml.fetchFailed":               "获取 OPML 文件失败",
		"opml.clientUnavailable":         "HTTP 客户端不可用，请重启应用",
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
		"changelog.clientUnavailable":    "HTTP 客户端不可用，请重启应用",
		"changelog.fetchFailed":          "获取更新日志失败",
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
		"image.save":                     "保存图片",
		"image.fileFilter":               "图片文件",
		"image.urlEmpty":                 "图片地址为空",
		"image.downloadUnavailable":      "图片下载不可用",
		"image.downloadFailed":           "下载图片失败",
		"image.writeFailed":              "保存图片失败",
	},
	TraditionalChinese: {}, // populated from the complete catalog in init
}

// traditionalChineseMessages starts with the complete Simplified Chinese
// catalog so a newly added backend key never falls through to English. The
// replacements below use Taiwan terminology for the UI phrases shared by the
// backend (the frontend locale contains the larger user-facing catalog).
func traditionalChineseMessages() map[string]string {
	translations := []struct{ from, to string }{
		{"请输入密码", "請輸入密碼"}, {"下载更新包", "下載更新套件"},
		{"安全选项", "安全選項"}, {"两步验证", "兩步驗證"},
		{"服务器地址", "伺服器位址"}, {"上级目录", "上層目錄"},
		{"重新定向", "重新導向"}, {"重定向", "重新導向"},
		{"本机", "本機"}, {"凭据密钥", "認證金鑰"}, {"密钥", "金鑰"},
		{"存储", "儲存"}, {"读写", "讀寫"}, {"专用", "專用"},
		{"简单好用", "簡單好用"}, {"请检查", "請檢查"}, {"请确认", "請確認"},
		{"尚未配置", "尚未設定"}, {"未配置", "未設定"}, {"必须", "必須"},
		{"开头", "開頭"}, {"开启", "開啟"}, {"同样", "同樣"}, {"常见", "常見"},
		{"上级", "上層"}, {"填到", "填寫至"}, {"漏掉", "遺漏"}, {"网盘", "雲端硬碟"},
		{"换机器", "更換電腦"}, {"清理后", "清理後"}, {"已被其他设备", "已被其他裝置"},
		{"锁定", "鎖定"}, {"写入", "寫入"}, {"拒绝", "拒絕"}, {"过于频繁", "過於頻繁"},
		{"账号", "帳號"}, {"权限", "權限"}, {"链路", "連線"},
		{"稍后", "稍後"}, {"无可用", "無可用"},
		{"设备", "裝置"}, {"配置", "設定"}, {"请", "請"}, {"确认", "確認"},
		{"验证", "驗證"},
		{"文件夹", "資料夾"}, {"数据库", "資料庫"},
		{"剪贴板", "剪貼簿"}, {"阅读器", "閱讀器"}, {"服务器", "伺服器"},
		{"软件", "軟體"}, {"默认", "預設"}, {"文件", "檔案"}, {"设置", "設定"},
		{"用户", "使用者"}, {"数据", "資料"}, {"信息", "訊息"}, {"支持", "支援"},
		{"搜索", "搜尋"}, {"网络", "網路"}, {"缓存", "快取"}, {"字体", "字型"},
		{"签名", "簽章"}, {"解压", "解壓縮"}, {"屏幕", "螢幕"}, {"复制", "複製"},
		{"订阅", "訂閱"}, {"地址", "位址"}, {"连接", "連線"}, {"失败", "失敗"},
		{"错误", "錯誤"}, {"检查", "檢查"}, {"获取", "取得"}, {"请输入", "請輸入"},
		{"请选择", "請選擇"}, {"无效", "無效"}, {"无法", "無法"}, {"为空", "不可為空"},
		{"返回", "回傳"}, {"响应", "回應"}, {"请求", "請求"}, {"目录", "目錄"},
		{"路径", "路徑"}, {"远端", "遠端"}, {"资源", "資源"}, {"重试", "重試"},
		{"导出", "匯出"}, {"导入", "匯入"}, {"备份", "備份"}, {"恢复", "還原"},
		{"生成", "產生"}, {"删除", "刪除"},
		{"安装", "安裝"}, {"分类", "分類"}, {"名称", "名稱"}, {"密码", "密碼"},
		{"凭据", "認證資訊"}, {"清理", "清理"}, {"空间", "空間"}, {"内部", "內部"},
		{"权限", "權限"}, {"应用", "應用程式"}, {"跨平台", "跨平台"}, {"图片", "圖片"},
		{"新增", "新增"}, {"篇", "篇"}, {"等", "等"},
	}
	result := make(map[string]string, len(messages[SimplifiedChinese]))
	for key, value := range messages[SimplifiedChinese] {
		translated := value
		for _, translation := range translations {
			translated = strings.ReplaceAll(translated, translation.from, translation.to)
		}
		result[key] = translated
	}
	return result
}

func init() {
	messages[TraditionalChinese] = traditionalChineseMessages()
}

// Normalize converts locale names to canonical supported language tags.
func Normalize(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(lang, "_", "-")))
	if idx := strings.IndexAny(lang, ".@"); idx >= 0 {
		lang = lang[:idx]
	}
	switch {
	case lang == "zh-tw", strings.HasPrefix(lang, "zh-tw-"),
		lang == "zh-hk", strings.HasPrefix(lang, "zh-hk-"),
		lang == "zh-mo", strings.HasPrefix(lang, "zh-mo-"),
		strings.HasPrefix(lang, "zh-hant"):
		return TraditionalChinese
	case lang == "zh", lang == "zh-cn", strings.HasPrefix(lang, "zh-cn-"),
		lang == "zh-sg", strings.HasPrefix(lang, "zh-sg-"),
		strings.HasPrefix(lang, "zh-hans"):
		return SimplifiedChinese
	}
	return English
}

// IsChinese reports whether notifications should use Chinese punctuation.
func IsChinese(lang string) bool {
	return UsesChineseSeparator(lang)
}

// UsesChineseSeparator reports whether Chinese list punctuation should be used.
// Both Simplified and Traditional Chinese use the ideographic comma.
func UsesChineseSeparator(lang string) bool {
	normalized := Normalize(lang)
	return normalized == SimplifiedChinese || normalized == TraditionalChinese
}

func isSimplifiedChinese(lang string) bool {
	return Normalize(lang) == SimplifiedChinese
}

// T returns a translated message. Missing keys intentionally fall back to the
// key itself so a newly added backend message is diagnosable during development.
func T(lang, key string, args ...any) string {
	lang = Normalize(lang)
	template, ok := messages[lang][key]
	if !ok {
		if lang == TraditionalChinese {
			template, ok = messages[SimplifiedChinese][key]
		}
	}
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
	if err == nil || isSimplifiedChinese(lang) {
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
