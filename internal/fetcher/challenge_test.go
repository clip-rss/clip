package fetcher

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// buildChallengePage 合成一份与线上同构的挑战页：种子固定，目标摘要按 wantCounter 算出。
// 不打真实网络。
func buildChallengePage(t *testing.T, seed string, wantCounter int) []byte {
	t.Helper()
	seedBytes := []byte(seed)
	h := sha256.New()
	h.Write(seedBytes)
	h.Write([]byte(strconv.Itoa(wantCounter)))
	target := h.Sum(nil)

	payload := map[string]any{
		"v": map[string]any{
			"a": base64.StdEncoding.EncodeToString(seedBytes),
			"b": 1787153313,
			"c": base64.StdEncoding.EncodeToString(target),
		},
		"s": "c2lnbmF0dXJl",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	b64 := base64.StdEncoding.EncodeToString(raw)

	// 混入一个更短的 atob 调用，验证「取最长载荷」的逻辑。
	return []byte(fmt.Sprintf(
		`<!DOCTYPE html><html><head><script>var x=atob('c2hvcnQ=');`+
			`var o=JSON.parse(atob('%s'));document.cookie='%s='+btoa(JSON.stringify(o));`+
			`</script></head><body>安全检测</body></html>`,
		b64, wafCookieName))
}

// decodeCookie 还原 cookie 值里的对象，供断言。
func decodeCookie(t *testing.T, value string) map[string]any {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("cookie base64: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("cookie json: %v", err)
	}
	return obj
}

func TestSolveWAFChallenge(t *testing.T) {
	page := buildChallengePage(t, "0123456789abcdef0123456789abcdef", 7)

	value, ok := solveWAFChallenge(page)
	if !ok {
		t.Fatal("solveWAFChallenge failed on a well-formed page")
	}

	obj := decodeCookie(t, value)

	// d 必须是 btoa(counter)，且 counter 正确。
	d, _ := obj["d"].(string)
	counter, err := base64.StdEncoding.DecodeString(d)
	if err != nil {
		t.Fatalf("d is not base64: %q", d)
	}
	if string(counter) != "7" {
		t.Errorf("counter = %q, want \"7\"", counter)
	}

	// 原有字段必须原样带回，否则服务端校验不过。
	if _, has := obj["v"]; !has {
		t.Error("v field dropped from cookie payload")
	}
	if s, _ := obj["s"].(string); s != "c2lnbmF0dXJl" {
		t.Errorf("s field = %q, want it preserved", s)
	}
}

func TestSolveWAFChallengeRejectsNonChallenge(t *testing.T) {
	cases := map[string][]byte{
		"普通网页":       []byte(`<html><body><h1>hello</h1></body></html>`),
		"无 atob 载荷":  []byte(`<html><script>var a=1</script>` + wafCookieName + `</html>`),
		"载荷非 base64": []byte(`<html><script>atob('!!!!not-base64!!!!')</script></html>`),
		"载荷非 JSON": []byte(`<html><script>atob('` +
			base64.StdEncoding.EncodeToString([]byte("plain text")) + `')</script></html>`),
	}
	for name, body := range cases {
		if _, ok := solveWAFChallenge(body); ok {
			t.Errorf("%s: 不应求解成功", name)
		}
	}
}

func TestSolveWAFChallengeUnsolvable(t *testing.T) {
	// 目标摘要随机 —— 循环内必然找不到，必须干净地返回 false 而不是卡死或 panic。
	payload := map[string]any{"v": map[string]any{
		"a": base64.StdEncoding.EncodeToString([]byte("seed")),
		"c": base64.StdEncoding.EncodeToString(make([]byte, sha256.Size)),
	}}
	raw, _ := json.Marshal(payload)
	body := []byte(`<html>` + wafCookieName + `<script>atob('` +
		base64.StdEncoding.EncodeToString(raw) + `')</script></html>`)

	if _, ok := solveWAFChallenge(body); ok {
		t.Error("全零目标摘要不应被求解出来")
	}
}

func TestLooksLikeWAFChallenge(t *testing.T) {
	if looksLikeWAFChallenge([]byte(`<html><body>plain</body></html>`)) {
		t.Error("普通网页被误判为挑战页")
	}
	if !looksLikeWAFChallenge([]byte(`x` + wafCookieName + `y`)) {
		t.Error("含 cookie 名的页面应判为挑战页")
	}
}

// 端到端：首次请求返回挑战页，带上正确 cookie 后返回真实 Feed。
// 验证 fetch 的「求解 → 重取」接线，以及凭据能被 jar 复用。
func TestFetchSolvesChallengeAndRetries(t *testing.T) {
	page := buildChallengePage(t, "seed-for-e2e-test-32bytes-long!!", 3)

	var challengeServed, feedServed int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ck, err := r.Cookie(wafCookieName); err == nil && ck.Value != "" {
			feedServed++
			w.Header().Set("Content-Type", "application/rss+xml")
			w.Write([]byte(sampleRSS))
			return
		}
		challengeServed++
		w.Header().Set("Content-Type", "text/html")
		w.Write(page)
	}))
	defer srv.Close()

	f := New(WithClient(NewClient(WithMaxRetry(0))))

	feed, _, err := f.FetchFeed(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchFeed = %v, want success after solving challenge", err)
	}
	if feed == nil || feed.Title != "Example Blog" {
		t.Fatalf("feed = %+v, want parsed sampleRSS", feed)
	}
	if challengeServed != 1 || feedServed != 1 {
		t.Errorf("challengeServed=%d feedServed=%d, want 1/1", challengeServed, feedServed)
	}

	// 第二次抓取应直接带 jar 里的凭据，不再触发挑战页。
	if _, _, err := f.FetchFeedForce(context.Background(), srv.URL); err != nil {
		t.Fatalf("second fetch = %v", err)
	}
	if challengeServed != 1 {
		t.Errorf("challengeServed=%d，凭据未被复用（应仍为 1）", challengeServed)
	}
}

// 挑战页无法求解时不得无限重试，且必须保留原始的 ErrHTMLResponse。
func TestFetchKeepsHTMLErrorWhenUnsolvable(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body>普通网页，不是挑战页</body></html>`))
	}))
	defer srv.Close()

	f := New(WithClient(NewClient(WithMaxRetry(0))))
	_, _, err := f.FetchFeed(context.Background(), srv.URL)
	if !errors.Is(err, ErrHTMLResponse) {
		t.Fatalf("err = %v, want ErrHTMLResponse", err)
	}
	if hits != 1 {
		t.Errorf("hits = %d, want 1（不应重试）", hits)
	}
}
