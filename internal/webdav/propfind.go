package webdav

import (
	"encoding/xml"
	"strings"
	"time"
)

// propfindBody 请求的属性集。只要同步判定用得到的三项，不用 <allprop> ——
// 后者在大目录上会让服务器吐出大量无用数据。
const propfindBody = `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:">
  <d:prop>
    <d:getetag/>
    <d:getcontentlength/>
    <d:getlastmodified/>
    <d:resourcetype/>
  </d:prop>
</d:propfind>`

// Stat 资源元信息。
type Stat struct {
	ETag         string    // 已归一化（见 normalizeETag）
	Size         int64     // 字节数；服务器未给时为 0
	LastModified time.Time // 服务器未给或无法解析时为零值
	IsDir        bool      // 是否为集合（目录）
}

/* ---------- XML 绑定 ---------- */

// 用命名空间 URI（DAV:）而非前缀匹配元素：各家服务器前缀不一（d: / D: / lp1: / 无前缀），
// encoding/xml 按 URI 解析，故一份结构体即可通吃。
type multistatus struct {
	XMLName   xml.Name      `xml:"DAV: multistatus"`
	Responses []davResponse `xml:"DAV: response"`
}

type davResponse struct {
	Href      string        `xml:"DAV: href"`
	Propstats []davPropstat `xml:"DAV: propstat"`
}

type davPropstat struct {
	Status string  `xml:"DAV: status"`
	Prop   davProp `xml:"DAV: prop"`
}

type davProp struct {
	ETag          string       `xml:"DAV: getetag"`
	ContentLength *int64       `xml:"DAV: getcontentlength"`
	LastModified  string       `xml:"DAV: getlastmodified"`
	ResourceType  *davResource `xml:"DAV: resourcetype"`
}

type davResource struct {
	Collection *struct{} `xml:"DAV: collection"`
}

/* ---------- 解析 ---------- */

// parsePropfind 从 PROPFIND 的 207 响应中取第一个资源的属性。
//
// 只取第一条 response：调用方一律用 Depth: 0，正常只会有一条。若服务器忽略
// Depth 返回了整个目录（确有此类实现），第一条即所请求资源本身。
func parsePropfind(path string, data []byte) (Stat, error) {
	var ms multistatus
	if err := xml.Unmarshal(data, &ms); err != nil {
		return Stat{}, &Error{
			Op:       "propfind",
			Path:     path,
			Msg:      "服务器返回的内容无法解析，请确认地址指向 WebDAV 路径",
			Err:      err,
			sentinel: ErrBadResponse,
		}
	}
	if len(ms.Responses) == 0 {
		return Stat{}, &Error{
			Op:       "propfind",
			Path:     path,
			Msg:      "服务器未返回资源信息",
			sentinel: ErrBadResponse,
		}
	}

	resp := ms.Responses[0]
	var out Stat
	for _, ps := range resp.Propstats {
		// 属性可分散在多个 propstat 中，未取到的那组状态为 404，需跳过，
		// 否则它的空值会覆盖掉 200 那组的真实值。
		if !isOKStatus(ps.Status) {
			continue
		}
		if ps.Prop.ETag != "" {
			out.ETag = normalizeETag(ps.Prop.ETag)
		}
		if ps.Prop.ContentLength != nil {
			out.Size = *ps.Prop.ContentLength
		}
		if ps.Prop.LastModified != "" {
			out.LastModified = parseHTTPTime(ps.Prop.LastModified)
		}
		if ps.Prop.ResourceType != nil && ps.Prop.ResourceType.Collection != nil {
			out.IsDir = true
		}
	}
	return out, nil
}

// isOKStatus 判断 propstat 的状态行（形如 "HTTP/1.1 200 OK"）是否为 2xx。
func isOKStatus(status string) bool {
	// 无状态行时按成功处理：属性已经在 prop 里给出了，缺状态行属服务器不规范，
	// 直接丢弃反而会把可用数据判成空。
	if strings.TrimSpace(status) == "" {
		return true
	}
	fields := strings.Fields(status)
	for _, f := range fields {
		if len(f) == 3 && f[0] == '2' {
			return true
		}
	}
	return false
}

// normalizeETag 剥离弱标记与引号，得到可直接比较的裸值。
//
// 同一个 ETag 在不同响应里可能写作 "abc"、W/"abc"、abc —— 不归一化就会把
// 「没变」误判成「变了」，进而触发无谓的冲突提示。
func normalizeETag(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "W/")
	s = strings.TrimPrefix(s, "w/")
	return strings.Trim(s, `"`)
}

// parseHTTPTime 解析 HTTP 日期。失败返回零值 —— 时间戳仅用于向用户展示
// 「远端于何时被改」，解析不出来不该让整次同步失败。
func parseHTTPTime(raw string) time.Time {
	layouts := []string{
		time.RFC1123,  // Mon, 02 Jan 2006 15:04:05 MST
		time.RFC1123Z, // Mon, 02 Jan 2006 15:04:05 -0700
		time.RFC850,
		time.ANSIC,
	}
	s := strings.TrimSpace(raw)
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
