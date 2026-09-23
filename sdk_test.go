package wbsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func jsonHandler(t *testing.T, status int, body string, check func(*http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			check(r)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

func TestAccountsListWithStaticSession(t *testing.T) {
	var gotCookie, gotPath string
	srv := jsonHandler(t, 200,
		`{"total":1,"pool_available":true,"accounts":[{"file":"a.json","uid":"u1","nickname":"小明","realm":"cn","credits":123.5}]}`,
		func(r *http.Request) {
			gotCookie = r.Header.Get("Cookie")
			gotPath = r.URL.Path
		})
	defer srv.Close()

	c := NewClient(srv.URL, WithAdminCookie("wb_session=abc"))
	res, err := c.Accounts().List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotPath != "/api/accounts" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotCookie != "wb_session=abc" {
		t.Fatalf("cookie = %q", gotCookie)
	}
	if res.Total != 1 || len(res.Accounts) != 1 {
		t.Fatalf("unexpected response: %+v", res)
	}
	acc := res.Accounts[0]
	if acc.UID != "u1" || acc.Nickname != "小明" || acc.Realm != RealmCN {
		t.Fatalf("unexpected account: %+v", acc)
	}
	if acc.Credits == nil || *acc.Credits != 123.5 {
		t.Fatalf("credits = %v", acc.Credits)
	}
}

func TestPasswordAuthLoginAndRelogin(t *testing.T) {
	var logins int32
	const wantCookie = "wb_session=c2"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/login":
			n := atomic.AddInt32(&logins, 1)
			http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: fmt.Sprintf("c%d", n), Path: "/"})
			_, _ = io.WriteString(w, `{"ok":true,"username":"admin","role":"admin"}`)
		case "/api/me":
			if r.Header.Get("Cookie") != wantCookie {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, `{"detail":"未登录"}`)
				return
			}
			_, _ = io.WriteString(w, `{"username":"admin","role":"admin"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAdminCredentials(srv.URL, "admin", "pw"))
	me, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me.Username != "admin" || me.Role != "admin" {
		t.Fatalf("unexpected me: %+v", me)
	}
	// 第一次 cookie c1 被拒 → 自动重登拿到 c2 → 成功；共登录 2 次。
	if got := atomic.LoadInt32(&logins); got != 2 {
		t.Fatalf("logins = %d, want 2", got)
	}
}

func TestStaticSession401IsAuthError(t *testing.T) {
	srv := jsonHandler(t, http.StatusUnauthorized, `{"detail":"未登录"}`, nil)
	defer srv.Close()

	c := NewClient(srv.URL, WithAdminCookie("wb_session=stale"))
	_, err := c.Me(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
	var ae *AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AuthError, got %T", err)
	}
}

func TestGatewayKeyInjection(t *testing.T) {
	var gotAuth, gotPath string
	srv := jsonHandler(t, 200, `{"models":[]}`, func(r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
	})
	defer srv.Close()

	c := NewClient(srv.URL, WithGatewayKey("wbk_test123"))
	res, err := c.Do(context.Background(), http.MethodGet, "/v1/models", nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if gotPath != "/v1/models" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer wbk_test123" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
}

func TestErrorShapes(t *testing.T) {
	// 网关面：OpenAI 形状（含 code 与 gateway_hint）。
	gwSrv := jsonHandler(t, http.StatusForbidden,
		`{"error":{"message":"来源 IP 1.2.3.4 被安全策略拦截","type":"permission_error","code":"ip_blocked","gateway_hint":"check ip rules"}}`, nil)
	defer gwSrv.Close()
	gw := NewClient(gwSrv.URL, WithGatewayKey("k"))
	res, doErr := gw.Do(context.Background(), http.MethodPost, "/v1/chat/completions", []byte(`{}`))
	if doErr != nil {
		t.Fatalf("Do transport error: %v", doErr)
	}
	err := res.Err()
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T (%v)", err, err)
	}
	if apiErr.Code != "ip_blocked" || apiErr.Hint != "check ip rules" || apiErr.StatusCode != 403 {
		t.Fatalf("unexpected APIError: %+v", apiErr)
	}
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	// 管理面：中文 detail 原样暴露；429 映射为 QuotaError。
	admSrv := jsonHandler(t, http.StatusTooManyRequests, `{"detail":"失败次数过多，请 10 分钟后再试"}`, nil)
	defer admSrv.Close()
	adm := NewClient(admSrv.URL, WithAdminCookie("c"))
	_, err = adm.Me(context.Background())
	var qe *QuotaError
	if !errors.As(err, &qe) {
		t.Fatalf("expected *QuotaError, got %T", err)
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	if apiErr := qe.APIError; apiErr.Message != "失败次数过多，请 10 分钟后再试" {
		t.Fatalf("message = %q", apiErr.Message)
	}
}

func TestCreateKeyRoundTrip(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || r.URL.Path != "/api/keys" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = io.WriteString(w, `{"id":7,"name":"ci","prefix":"wbk_abcd","enabled":true,"realm":"cn","quota_credit":50,"used_credit":0,"used_tokens":0,"key":"wbk_secret_once"}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAdminCookie("c"))
	key, err := c.Keys().Create(context.Background(), KeyIn{
		Name:        "ci",
		Realm:       RealmCN,
		QuotaCredit: 50,
		Models:      []string{"glm-5.2"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if key.Key != "wbk_secret_once" || key.ID != 7 || key.Realm != RealmCN {
		t.Fatalf("unexpected key: %+v", key)
	}
	if gotBody["quota_credit"] != float64(50) {
		t.Fatalf("quota_credit not sent: %+v", gotBody)
	}
	if _, ok := gotBody["models"]; !ok {
		t.Fatalf("models not sent: %+v", gotBody)
	}
}

func TestExtractSSEDataLine(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"data: {\"a\":1}", `{"a":1}`, true},
		{"data:{\"a\":1}", `{"a":1}`, true},
		{"data: [DONE]\r", `[DONE]`, true},
		{": keep-alive", "", false},
		{"event: message", "", false},
		{"", "", false},
		{"datax: nope", "", false},
	}
	for _, tc := range cases {
		got, ok := extractSSEDataLine([]byte(tc.in))
		if ok != tc.wantOK || string(got) != tc.want {
			t.Fatalf("extract(%q) = (%q,%v), want (%q,%v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestDoJSONRequiresAuth(t *testing.T) {
	c := NewClient("http://127.0.0.1:1")
	_, err := c.Accounts().List(context.Background())
	if err == nil {
		t.Fatal("expected auth error for missing admin auth")
	}
}
