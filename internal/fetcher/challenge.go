package fetcher

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// 反爬 JS 挑战的求解（目前覆盖火山引擎 WAF，36kr 等站点在用）。
//
// 拦截页返回 HTTP 200 + text/html，内嵌一段混淆脚本，其 base64 载荷形如
//
//	{"v":{"a":<32 字节种子 base64>,"b":<时间戳>,"c":<目标摘要 base64>},"s":"..."}
//
// 脚本从 0 递增 counter，求 sha256(a ‖ counter 的十进制字符串)，命中 c 后把
// d = btoa(counter) 写回该对象，整体 base64 作为 _wafchallengeid cookie 并重新加载。
// 本文件用纯 Go 复刻这一步，无需 JS 引擎或无头浏览器。
//
// counter 实测只有个位数（4、8），求解耗时微秒级 —— wafMaxCounter 是防死循环的
// 安全阀而非工作量证明，WAF 检测的是「能否执行 JS」而不是算力。
const (
	wafCookieName = "_wafchallengeid"
	// 与挑战脚本自身的循环上界一致。最坏情况约 100 万次 SHA-256（数百毫秒）。
	wafMaxCounter = 1000000
)

var reAtobPayload = regexp.MustCompile(`atob\('([A-Za-z0-9+/=]+)'\)`)

// looksLikeWAFChallenge 判断响应体是否为已知的反爬挑战页。
// 用 cookie 名做判别，避免对普通网页（用户填错地址）做无谓的求解尝试。
func looksLikeWAFChallenge(body []byte) bool {
	return bytes.Contains(body, []byte(wafCookieName))
}

// solveWAFChallenge 求解挑战页，返回 _wafchallengeid 的 cookie 值。
// 载荷结构不符或算不出时返回 false，调用方按原错误处理。
func solveWAFChallenge(body []byte) (string, bool) {
	payload, ok := longestAtobPayload(body)
	if !ok {
		return "", false
	}
	raw, err := base64.StdEncoding.DecodeString(padBase64(payload))
	if err != nil {
		return "", false
	}

	// 用 RawMessage 保留未知字段：cookie 要把整个对象原样带回，
	// 只额外追加 d。（键顺序会被 Go 按字典序重排，实测服务端不校验顺序。）
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", false
	}
	var v struct {
		A string `json:"a"`
		C string `json:"c"`
	}
	if err := json.Unmarshal(obj["v"], &v); err != nil {
		return "", false
	}
	seed, err := base64.StdEncoding.DecodeString(v.A)
	if err != nil || len(seed) == 0 {
		return "", false
	}
	target, err := base64.StdEncoding.DecodeString(v.C)
	if err != nil || len(target) != sha256.Size {
		return "", false
	}

	counter, ok := findCounter(seed, target)
	if !ok {
		return "", false
	}

	d, err := json.Marshal(base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(counter))))
	if err != nil {
		return "", false
	}
	obj["d"] = d
	out, err := json.Marshal(obj)
	if err != nil {
		return "", false
	}
	return base64.StdEncoding.EncodeToString(out), true
}

// findCounter 暴力搜索满足 sha256(seed ‖ counter) == target 的最小 counter。
func findCounter(seed, target []byte) (int, bool) {
	h := sha256.New()
	sum := make([]byte, 0, sha256.Size)
	for i := 0; i <= wafMaxCounter; i++ {
		h.Reset()
		h.Write(seed)
		h.Write(strconv.AppendInt(nil, int64(i), 10))
		if bytes.Equal(h.Sum(sum[:0]), target) {
			return i, true
		}
	}
	return 0, false
}

// longestAtobPayload 取页面中最长的 atob('...') 参数。
// 挑战页里还有其他短 atob 调用（混淆用的字符串表），载荷是最长的那个。
func longestAtobPayload(body []byte) (string, bool) {
	matches := reAtobPayload.FindAllSubmatch(body, -1)
	best := ""
	for _, m := range matches {
		if s := string(m[1]); len(s) > len(best) {
			best = s
		}
	}
	return best, best != ""
}

// padBase64 补齐 base64 尾部 padding —— 页面里的载荷可能省略 '='。
func padBase64(s string) string {
	return s + strings.Repeat("=", (4-len(s)%4)%4)
}
