package wbsdk

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

// 用 WithAdminLogin 时登录地址应取自 Client 的 baseURL，无需重复传。
func TestWithAdminLoginUsesClientBaseURL(t *testing.T) {
	var logins int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			atomic.AddInt32(&logins, 1)
			http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "s1", Path: "/"})
			_, _ = io.WriteString(w, `{"ok":true}`)
		case "/api/me":
			if r.Header.Get("Cookie") != "wb_session=s1" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = io.WriteString(w, `{"username":"admin","role":"admin"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAdminLogin("admin", "pw"))
	me, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me.Username != "admin" {
		t.Fatalf("unexpected me: %+v", me)
	}
	if atomic.LoadInt32(&logins) != 1 {
		t.Fatalf("logins = %d, want 1", logins)
	}
}

// PasswordAuth 漏配 baseURL 应给出可执行的错误，而不是发出一个非法 URL。
func TestPasswordAuthMissingBaseURLErrors(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", WithAdminAuth(NewPasswordAuth("", "u", "p")))
	_, err := c.Me(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "baseURL") {
		t.Fatalf("error should mention baseURL, got: %v", err)
	}
}

// WithHeader("User-Agent") 不应被默认 UA 覆盖。
func TestUserAgentOverride(t *testing.T) {
	var gotUA string
	srv := jsonHandler(t, 200, `{}`, func(r *http.Request) { gotUA = r.Header.Get("User-Agent") })
	defer srv.Close()

	c := NewClient(srv.URL, WithAdminCookie("c"), WithHeader("User-Agent", "my-ua/9"))
	if _, err := c.Me(context.Background()); err != nil {
		t.Fatalf("Me: %v", err)
	}
	if gotUA != "my-ua/9" {
		t.Fatalf("User-Agent = %q, want my-ua/9", gotUA)
	}
}

// 只在路径段边界上注入鉴权：/v1abc、/apifoo 都不应带鉴权头。
func TestPathPrefixPrecision(t *testing.T) {
	var authHdr, cookieHdr string
	srv := jsonHandler(t, 200, `{}`, func(r *http.Request) {
		authHdr = r.Header.Get("Authorization")
		cookieHdr = r.Header.Get("Cookie")
	})
	defer srv.Close()

	c := NewClient(srv.URL, WithGatewayKey("k"), WithAdminCookie("wb_session=x"))

	if _, err := c.Do(context.Background(), http.MethodGet, "/v1abc", nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if authHdr != "" {
		t.Fatalf("/v1abc 不应带 Authorization，得到 %q", authHdr)
	}

	if _, err := c.Do(context.Background(), http.MethodGet, "/apifoo", nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if cookieHdr != "" {
		t.Fatalf("/apifoo 不应带 Cookie，得到 %q", cookieHdr)
	}

	// 正常前缀仍然注入。
	if _, err := c.Do(context.Background(), http.MethodGet, "/v1/models", nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if authHdr != "Bearer k" {
		t.Fatalf("/v1/models Authorization = %q", authHdr)
	}
}

// Chat 必须忽略 Extra 里的 stream，避免非流式请求被当成流式。
func TestChatExtraDoesNotLeakStream(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"1","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hi"}}]}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithGatewayKey("k"))
	resp, err := c.Chat(context.Background(), &ChatRequest{
		Model:    "m",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
		Extra:    map[string]any{"stream": true, "foo": "bar"},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Choices[0].Message == nil {
		t.Fatal("empty choice")
	}
	if strings.Contains(gotBody, `"stream"`) {
		t.Fatalf("请求体不该含 stream: %s", gotBody)
	}
	if !strings.Contains(gotBody, `"foo":"bar"`) {
		t.Fatalf("Extra 的其它键应保留: %s", gotBody)
	}
}

// ChatStream 应交付 SSE data: 载荷，并带上网关密钥。
func TestChatStreamDecodesSSE(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, ": keep-alive\n\n")
		_, _ = io.WriteString(w, "data: {\"x\":1}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\r\n\r\n")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithGatewayKey("k"))
	var got []string
	err := c.ChatStream(context.Background(), &ChatRequest{
		Model:    "m",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	}, func(raw []byte) error {
		got = append(got, string(raw))
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if gotAuth != "Bearer k" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if len(got) != 2 || got[0] != `{"x":1}` || got[1] != "[DONE]" {
		t.Fatalf("payloads = %#v", got)
	}
}

// 流式接口的非 2xx 应映射为对应的错误类型。
func TestDoStreamErrorMapsToQuota(t *testing.T) {
	srv := jsonHandler(t, http.StatusTooManyRequests,
		`{"error":{"message":"额度用尽","type":"rate_limit_error","code":"quota_exceeded"}}`, nil)
	defer srv.Close()

	c := NewClient(srv.URL, WithGatewayKey("k"))
	err := c.DoStream(context.Background(), http.MethodPost, "/v1/chat/completions", []byte(`{}`),
		func([]byte) error { return nil })
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	var qe *QuotaError
	if !errors.As(err, &qe) || qe.Code != "quota_exceeded" {
		t.Fatalf("expected *QuotaError with code, got %v", err)
	}
}

// 开启重试后，GET 遇到 5xx 应重试并最终成功。
func TestRetryOnIdempotentGet(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAdminCookie("c"),
		WithRetry(RetryPolicy{MaxRetries: 3, BaseDelay: time.Millisecond}))
	res, err := c.Do(context.Background(), http.MethodGet, "/api/me", nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if got := atomic.LoadInt32(&n); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
}

// 非幂等方法即使开启重试也绝不重试（避免副作用重复）。
func TestNoRetryOnPost(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"detail":"boom"}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAdminCookie("c"),
		WithRetry(RetryPolicy{MaxRetries: 5, BaseDelay: time.Millisecond}))
	res, err := c.Do(context.Background(), http.MethodPost, "/api/accounts/checkin-all", nil)
	if err != nil {
		t.Fatalf("Do transport error: %v", err)
	}
	// Do 对非 2xx 不返回 error（原始通道语义），用 res.Err() 检查。
	if res.Err() == nil {
		t.Fatal("expected API error on 503")
	}
	if got := atomic.LoadInt32(&n); got != 1 {
		t.Fatalf("POST 被重试了 %d 次，应为 1", got)
	}
}

// 重试应尊重 ctx 取消，而不是傻等。
func TestRetryRespectsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	c := NewClient(srv.URL, WithAdminCookie("c"),
		WithRetry(RetryPolicy{MaxRetries: 10, BaseDelay: 200 * time.Millisecond}))
	if _, err := c.Do(ctx, http.MethodGet, "/api/me", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

// truncate 不应把 UTF-8 字符切成两半。
func TestTruncateUTF8Safe(t *testing.T) {
	s := strings.Repeat("你好世界", 10) // 每个汉字 3 字节
	got := truncate(s, 7)           // 7 不是 3 的倍数，会落在字符中间
	if !utf8.ValidString(got) {
		t.Fatalf("截断结果不是合法 UTF-8: %q", got)
	}
	if len(got) > 7 {
		t.Fatalf("len = %d, want <= 7", len(got))
	}
}

// SetRealm / Realm 在并发下应当是安全的（配合 go test -race 验证）。
func TestSetRealmConcurrent(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", WithRealm(RealmCN))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if i%2 == 0 {
					c.SetRealm(RealmGlobal)
				} else {
					c.SetRealm(RealmCN)
				}
				_ = c.Realm()
			}
		}(i)
	}
	wg.Wait()
}
