// Package fontproxy 把已下载的字体文件交给 WebView 加载。
//
// 解决的接入问题：正文自定义字体需要以 URL 形式进 @font-face，而字体文件落在
// 本地配置目录（<configDir>/clip/fonts/），webview 无权直接读文件系统。做法与
// mediaproxy 同构 —— 一个小中间件挂在 Wails 资产服务器中间件链上，接管
// /__clip/font/<id>/<file>，把 <fontsDir>/<id>/<file> 读盘回包。
//
// 为什么用相对路径前缀：Windows 上页面源是 http://wails.localhost，
// macOS/Linux 上才是 wails://localhost；相对路径在两端都解析到资产服务器。
// dev 模式下资产服务器的兜底 handler 本身就是指向 Vite 的反向代理，本中间件排
// 在它之前，所以 dev 与生产走同一条路，无需 Vite 侧配置。（同 mediaproxy 包注释）
//
// 字体是 immutable 资产：同一 id/文件名的字节不会变，回长缓存即可，WebView 只需
// 拉一次。
package fontproxy

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/clip-rss/clip/internal/fonts"
)

// PathPrefix 是字体资产路由。⚠️ 与 frontend/src/Stores/FontStore.ts 的
// FONT_ASSET_PREFIX 是一份契约，两侧必须同时改。
const PathPrefix = "/__clip/font"

// cacheMaxAge 回给 WebView 的缓存时长（秒）。字体文件不可变，参考 mediaproxy
// 的 cacheMaxAge 取值。
const cacheMaxAge = 86400

// FontProxy 提供已下载字体资产。root 是字体根目录（<configDir>/clip/fonts），
// 由调用方注入 —— 便于测试。
type FontProxy struct {
	root string
}

// New 创建 FontProxy。
func New(root string) *FontProxy {
	return &FontProxy{root: root}
}

// Middleware 返回 Wails 资产服务器中间件：只接管 PathPrefix，其余原样交给 next。
func (p *FontProxy) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, PathPrefix+"/") {
			next.ServeHTTP(w, r)
			return
		}
		p.serve(w, r)
	})
}

// serve 处理一次字体资产请求。
func (p *FontProxy) serve(w http.ResponseWriter, r *http.Request) {
	// 失败一律不缓存：路径非法或文件缺失可能是目录变动，被 WebView 缓存住会让
	// 已显示的内容一直错下去。
	fail := func(msg string, code int) {
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, msg, code)
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		fail("method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, PathPrefix+"/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		fail("malformed font path", http.StatusBadRequest)
		return
	}
	target, err := fonts.ServeAssetPath(p.root, parts[0], parts[1])
	if err != nil {
		fail("invalid font path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			fail("font not found", http.StatusNotFound)
			return
		}
		fail("font stat failed", http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		fail("not a font file", http.StatusNotFound)
		return
	}

	f, err := os.Open(target)
	if err != nil {
		fail("font open failed", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	// Content-Type 按扩展名；窥探 MIME 不可靠（TTF/OTF 常被识别成 octet-stream）。
	// 字体文件不可变，加长缓存。
	w.Header().Set("Content-Type", contentType(target))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(cacheMaxAge))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(w, f)
}

// contentType 按扩展名回字体 MIME。
func contentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	default:
		// 未知扩展名也能读，只是 WebView 不一定识别；交给 mime 兜底。
		if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
			return ct
		}
		return "application/octet-stream"
	}
}
