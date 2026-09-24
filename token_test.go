package wbsdk

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// 默认 UA 必须带上当前版本号（唯一来源：version.go / ldflags）。
func TestDefaultUserAgentCarriesVersion(t *testing.T) {
	want := "workbuddy-sdk/" + Version() + " (+https://github.com/sanguine886/workbuddy-sdk)"
	if got := defaultUserAgent(); got != want {
		t.Fatalf("defaultUserAgent() = %q, want %q", got, want)
	}

	var gotUA string
	srv := jsonHandler(t, 200, `{}`, func(r *http.Request) { gotUA = r.Header.Get("User-Agent") })
	defer srv.Close()

	c := NewClient(srv.URL, WithAdminCookie("c"))
	if _, err := c.Me(context.Background()); err != nil {
		t.Fatalf("Me: %v", err)
	}
	if gotUA != want {
		t.Fatalf("UA = %q, want %q", gotUA, want)
	}
}

// 管理面 Token 走 Authorization: Bearer，且不带 Cookie。
func TestAdminTokenInjection(t *testing.T) {
	var auth, cookie string
	srv := jsonHandler(t, 200, `{"username":"ci","role":"admin"}`, func(r *http.Request) {
		auth = r.Header.Get("Authorization")
		cookie = r.Header.Get("Cookie")
	})
	defer srv.Close()

	c := NewClient(srv.URL, WithAdminToken("wbt_abcdef"))
	me, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me.Username != "ci" {
		t.Fatalf("unexpected me: %+v", me)
	}
	if auth != "Bearer wbt_abcdef" {
		t.Fatalf("Authorization = %q", auth)
	}
	if cookie != "" {
		t.Fatalf("不应同时带 Cookie，得到 %q", cookie)
	}
}

// Token 存在时优先于 Cookie。
func TestAdminTokenPriorityOverCookie(t *testing.T) {
	var auth, cookie string
	srv := jsonHandler(t, 200, `{}`, func(r *http.Request) {
		auth = r.Header.Get("Authorization")
		cookie = r.Header.Get("Cookie")
	})
	defer srv.Close()

	c := NewClient(srv.URL, WithAdminToken("wbt_x"), WithAdminCookie("wb_session=y"))
	if _, err := c.Me(context.Background()); err != nil {
		t.Fatalf("Me: %v", err)
	}
	if cookie != "" {
		t.Fatalf("token 存在时不应带 Cookie，得到 %q", cookie)
	}
	if auth != "Bearer wbt_x" {
		t.Fatalf("Authorization = %q", auth)
	}
}

// Token 是静态凭据：401 不应触发任何重登重试。
func TestAdminTokenNoReloginOn401(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"detail":"token 无效"}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAdminToken("wbt_bad"))
	_, err := c.Me(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
	if got := atomic.LoadInt32(&n); got != 1 {
		t.Fatalf("不应重试，请求数 = %d", got)
	}
}

// 管理面 Token 不得串到数据面：数据面仍用网关密钥。
func TestAdminTokenNotAppliedToGateway(t *testing.T) {
	var auth string
	srv := jsonHandler(t, 200, `{"data":[]}`, func(r *http.Request) {
		auth = r.Header.Get("Authorization")
	})
	defer srv.Close()

	c := NewClient(srv.URL, WithAdminToken("wbt_admin"), WithGatewayKey("wbk_gw"))
	if _, err := c.Models(context.Background()); err != nil {
		t.Fatalf("Models: %v", err)
	}
	if auth != "Bearer wbk_gw" {
		t.Fatalf("数据面 Authorization = %q, want Bearer wbk_gw", auth)
	}
	if strings.Contains(auth, "wbt_admin") {
		t.Fatal("管理面 token 泄漏到了数据面")
	}
}
