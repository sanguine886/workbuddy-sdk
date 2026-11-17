package wbsdk

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newReplay 起一个按路径回放固定响应的测试服务器，并记录每个请求。
func newReplay(t *testing.T, handlers map[string]func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, *[]recorded) {
	t.Helper()
	recs := &[]recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*recs = append(*recs, recorded{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery})
		if h, ok := handlers[r.URL.Path]; ok {
			h(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)
	return srv, recs
}

// 余额端点：/api/status 是**公开**元数据；/api/usage/token/ 挂在 /api 下但
// 鉴权用的是网关密钥（不是管理面会话）。
func TestBalanceAuthRouting(t *testing.T) {
	var gotAuth string
	srv, _ := newReplay(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/status": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"success":true,"data":{"quota_per_unit":1,`+
				`"quota_display_type":"CUSTOM","custom_currency_symbol":"UNIT",`+
				`"custom_currency_exchange_rate":1}}`)
		},
		"/api/usage/token/": func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"code":true,"data":{"object":"token_usage",`+
				`"unlimited_quota":false,"total_granted":10,"total_used":3,`+
				`"total_available":7,"scope":"merged"}}`)
		},
		"/v1/credits": func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"code":true,"data":{"object":"credits",`+
				`"total_credits":10,"total_used":3,"total_available":7,"total_usage":7,`+
				`"scope":"merged"}}`)
		},
	})

	ctx := context.Background()

	// 公开端点：什么凭据都不配也能调。
	bs, err := NewClient(srv.URL).BalanceStatus(ctx)
	if err != nil {
		t.Fatalf("BalanceStatus: %v", err)
	}
	if !bs.Success || bs.Data.CustomCurrencySymbol != "UNIT" {
		t.Fatalf("BalanceStatus decode: %+v", bs)
	}

	// 余额端点必须用网关密钥：只配了管理面 Cookie 时应直接报错，而不是
	// 拿 Cookie 去撞一个只认 Bearer 的端点。
	if _, err := NewClient(srv.URL, WithAdminCookie("wb_session=x")).TokenUsage(ctx); err == nil {
		t.Fatal("TokenUsage 只配管理面凭据时应当报错")
	}

	g := NewClient(srv.URL, WithGatewayKey("wbk_k"))
	tu, err := g.TokenUsage(ctx)
	if err != nil {
		t.Fatalf("TokenUsage: %v", err)
	}
	if gotAuth != "Bearer wbk_k" {
		t.Fatalf("token usage auth = %q", gotAuth)
	}
	if tu.Data.TotalAvailable != 7 || tu.Data.Scope != "merged" {
		t.Fatalf("TokenUsage decode: %+v", tu)
	}

	cr, err := g.Credits(ctx)
	if err != nil {
		t.Fatalf("Credits: %v", err)
	}
	if gotAuth != "Bearer wbk_k" {
		t.Fatalf("credits auth = %q", gotAuth)
	}
	if cr.Data.TotalUsage != 7 || cr.Data.TotalAvailable != 7 {
		t.Fatalf("Credits decode: %+v", cr)
	}
}

// 监控（/api/watchdog）：路径与打码值的处理。
func TestWatchdogEndpoints(t *testing.T) {
	srv, recs := newReplay(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/watchdog/config": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"config":{"enabled":true,"interval_minutes":360,`+
				`"targets":[{"name":"t","url":"https://x.invalid/","kind":"html","enabled":true}],`+
				`"timeout_seconds":20,"asset_limit":4,"notify_on":"change",`+
				`"mail":{"enabled":false,"host":"","port":465,"security":"ssl","password":""},`+
				`"llm":{"enabled":false,"base_url":"","api_key":"","model":""},`+
				`"qq":{"enabled":false,"base_url":"","api_key":"","umo":""}}}`)
		},
		"/api/watchdog/qq/test": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ok":false,"stage":"auth","error":"密钥没有 im 权限"}`)
		},
	})

	c := NewClient(srv.URL, WithAdminCookie("wb_session=x"))
	ctx := context.Background()

	cfg, err := c.Watchdog().Config(ctx)
	if err != nil {
		t.Fatalf("Watchdog.Config: %v", err)
	}
	if lastPath(t, *recs) != "/api/watchdog/config" {
		t.Fatalf("config path = %q", lastPath(t, *recs))
	}
	if !cfg.Config.Enabled || cfg.Config.IntervalMinutes != 360 || len(cfg.Config.Targets) != 1 {
		t.Fatalf("watchdog config decode: %+v", cfg.Config)
	}
	if cfg.Config.Mail.Port != 465 || cfg.Config.Mail.Security != "ssl" {
		t.Fatalf("watchdog mail decode: %+v", cfg.Config.Mail)
	}

	// 测试端点用**表单当前值**（不落库）：请求体带 base_url / api_key / umo。
	qq, err := c.Watchdog().TestQQ(ctx, &WatchdogQQPatch{
		BaseURL: "https://astrbot.invalid",
		APIKey:  "abk_x",
		UMO:     "qq_official:GroupMessage:123",
	})
	if err != nil {
		t.Fatalf("TestQQ: %v", err)
	}
	if qq.Stage != "auth" || qq.OK {
		t.Fatalf("qq test decode: %+v", qq)
	}
}

// 抓包（/api/tap）：列表筛选参数与导出的文件名。
func TestTapEndpoints(t *testing.T) {
	srv, recs := newReplay(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/tap/sessions": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"total":1,"items":[{"id":7,"fingerprint":"fp",`+
				`"client":"claude-code","kind":"main","model":"glm-5.2","turns":3}]}`)
		},
		"/api/tap/sessions/7/export": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="workbuddy-trace-7.html"`)
			_, _ = io.WriteString(w, "<html>trace</html>")
		},
	})

	c := NewClient(srv.URL, WithAdminCookie("wb_session=x"))
	ctx := context.Background()

	page, err := c.Tap().Sessions(ctx, TapSessionQuery{Page: 2, Size: 10, Client: "claude-code"})
	if err != nil {
		t.Fatalf("Tap.Sessions: %v", err)
	}
	if q := lastQuery(t, *recs); q != "client=claude-code&page=2&size=10" {
		t.Fatalf("sessions query = %q", q)
	}
	if page.Total != 1 || page.Items[0].ID != 7 || page.Items[0].Turns != 3 {
		t.Fatalf("sessions decode: %+v", page)
	}

	exp, err := c.Tap().ExportSession(ctx, 7)
	if err != nil {
		t.Fatalf("ExportSession: %v", err)
	}
	if exp.Filename != "workbuddy-trace-7.html" {
		t.Fatalf("export filename = %q", exp.Filename)
	}
	if !strings.Contains(string(exp.HTML), "<html>") {
		t.Fatalf("export body = %q", exp.HTML)
	}
}

// 账号风险预警（/api/account-risk）：筛选参数与恢复动作的路径。
func TestAccountRiskEndpoints(t *testing.T) {
	srv, recs := newReplay(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/account-risk": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"items":[{"id":"e1","uid":"u1","nickname":"n",`+
				`"realm":"cn","operation":"token_refresh","http_status":429,"ts":100,`+
				`"paused":true,"needs_review":true,"failures":2,"group":"default"}],`+
				`"failed_groups":[],"total":1}`)
		},
		"/api/account-risk/e1/resume": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ok":true}`)
		},
	})

	c := NewClient(srv.URL, WithAdminCookie("wb_session=x"))
	ctx := context.Background()

	list, err := c.AccountRisk().List(ctx, AccountRiskQuery{Realm: RealmCN, IncludeAcknowledged: true})
	if err != nil {
		t.Fatalf("AccountRisk.List: %v", err)
	}
	if q := lastQuery(t, *recs); q != "include_acknowledged=true&realm=cn" {
		t.Fatalf("risk query = %q", q)
	}
	if list.Total != 1 || list.Items[0].Paused == nil || !*list.Items[0].Paused {
		t.Fatalf("risk decode: %+v", list)
	}

	if err := c.AccountRisk().Resume(ctx, "e1"); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if p := lastPath(t, *recs); p != "/api/account-risk/e1/resume" {
		t.Fatalf("resume path = %q", p)
	}
}
