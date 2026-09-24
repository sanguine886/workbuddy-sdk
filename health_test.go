package wbsdk

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 存活探测不需要任何凭据：既不该报「缺少鉴权配置」，也不该把凭据发出去。
func TestHealthzNeedsNoCredentials(t *testing.T) {
	var auth, cookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		cookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/healthz":
			_, _ = io.WriteString(w, `{"service":"workbuddy-manager","upstream_ok":false}`)
		case "/api/healthz":
			_, _ = io.WriteString(w, `{"ok":true,"service":"workbuddy-manager"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// 刻意**不配置任何凭据**
	c := NewClient(srv.URL)

	gh, err := c.Healthz(context.Background())
	if err != nil {
		t.Fatalf("Healthz: %v", err)
	}
	if gh.Service == "" || gh.UpstreamOK {
		t.Fatalf("unexpected gateway health: %+v", gh)
	}
	if auth != "" || cookie != "" {
		t.Fatalf("存活探测不该带凭据：auth=%q cookie=%q", auth, cookie)
	}

	ah, err := c.AdminHealthz(context.Background())
	if err != nil {
		t.Fatalf("AdminHealthz: %v", err)
	}
	if !ah.OK || ah.Service == "" {
		t.Fatalf("unexpected admin health: %+v", ah)
	}
}

// 但**非公开**的管理路径仍然要求配好鉴权（不能因为加了公开路径白名单就放松）。
func TestNonPublicAdminPathStillRequiresAuth(t *testing.T) {
	c := NewClient("http://127.0.0.1:1")
	if _, err := c.Me(context.Background()); err == nil {
		t.Fatal("未配置鉴权时应报错")
	}
}
