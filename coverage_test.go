package wbsdk

import (
	"context"
	"errors"
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
		body, _ := io.ReadAll(r.Body)
		*recs = append(*recs, recorded{
			Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: string(body),
		})
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
	var gotAuth, gotQuery string
	srv, _ := newReplay(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/status": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"success":true,"data":{"quota_per_unit":1,`+
				`"quota_display_type":"CUSTOM","custom_currency_symbol":"UNIT",`+
				`"custom_currency_exchange_rate":1}}`)
		},
		"/api/usage/token/": func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotQuery = r.URL.RawQuery
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

	// scope 口径：显式给了才拼参数，ScopeAuto 不拼（服务端自己决定）。
	gotQuery = ""
	if _, err := g.TokenUsageOf(ctx, ScopeGlobal); err != nil {
		t.Fatalf("TokenUsageOf: %v", err)
	}
	if gotQuery != "scope=global" {
		t.Fatalf("TokenUsageOf query = %q, 期望 scope=global", gotQuery)
	}
	gotQuery = ""
	if _, err := g.CreditsOf(ctx, ScopeAuto); err != nil {
		t.Fatalf("CreditsOf: %v", err)
	}
	if gotQuery != "" {
		t.Fatalf("ScopeAuto 不该带 scope 参数，实得 %q", gotQuery)
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
		BaseURL: Ptr("https://astrbot.invalid"),
		APIKey:  Ptr("abk_x"),
		UMO:     Ptr("qq_official:GroupMessage:123"),
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

// 定向重登：StartAuthFor 带 uid，realm 由服务端跟随目标账号。旧 StartAuth 不带 uid。
func TestStartAuthForSendsUID(t *testing.T) {
	srv, recs := newReplay(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/auth/start": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"state":"s1","authUrl":"https://qr.invalid/","realm":"global",`+
				`"target":{"uid":"u9","file":"a.json","nickname":"小明","realm":"global",`+
				`"panel_disabled":true}}`)
		},
	})
	c := NewClient(srv.URL, WithAdminCookie("wb_session=x"))
	ctx := context.Background()

	if _, err := c.Accounts().StartAuth(ctx, RealmCN); err != nil {
		t.Fatalf("StartAuth: %v", err)
	}
	if b := lastBody(t, *recs); strings.Contains(b, "uid") {
		t.Fatalf("StartAuth 不该带 uid：%s", b)
	}

	st, err := c.Accounts().StartAuthFor(ctx, RealmCN, "u9")
	if err != nil {
		t.Fatalf("StartAuthFor: %v", err)
	}
	if b := lastBody(t, *recs); !strings.Contains(b, `"uid":"u9"`) {
		t.Fatalf("StartAuthFor body = %s", b)
	}
	if st.Target == nil || st.Target.UID != "u9" || !st.Target.PanelDisabled {
		t.Fatalf("target decode: %+v", st.Target)
	}

	// 国际版新号必须带 region，否则聊天报 14017；分组要跟到 query 上。
	if _, err := c.Accounts().StartAuthIn(ctx, AuthStartIn{
		Realm: RealmGlobal, Region: "HK", UpstreamID: Ptr(7),
	}); err != nil {
		t.Fatalf("StartAuthIn: %v", err)
	}
	b := lastBody(t, *recs)
	if !strings.Contains(b, `"region":"HK"`) || !strings.Contains(b, `"realm":"global"`) {
		t.Fatalf("StartAuthIn body = %s", b)
	}
	if q := lastQuery(t, *recs); q != "upstream_id=7" {
		t.Fatalf("StartAuthIn query = %q", q)
	}

	// proxy 三态：nil 不写；Ptr("") 是显式切回默认线路，必须写进 body。
	if _, err := c.Accounts().StartAuth(ctx, RealmCN); err != nil {
		t.Fatalf("StartAuth: %v", err)
	}
	if b := lastBody(t, *recs); strings.Contains(b, "proxy") {
		t.Fatalf("Proxy 为 nil 时不该带 proxy：%s", b)
	}
	if _, err := c.Accounts().StartAuthIn(ctx, AuthStartIn{Realm: RealmCN, Proxy: Ptr("")}); err != nil {
		t.Fatalf("StartAuthIn(proxy): %v", err)
	}
	if b := lastBody(t, *recs); !strings.Contains(b, `"proxy":""`) {
		t.Fatalf("Ptr(\"\") 必须写成 proxy:\\\"\\\"：%s", b)
	}
}

// 统计的自定义区间：只有 start/end 都填才生效。
func TestStatsDateRange(t *testing.T) {
	emptyList := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}
	srv, recs := newReplay(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/stats/daily":    emptyList,
		"/api/stats/by-model": emptyList,
	})
	c := NewClient(srv.URL, WithAdminCookie("wb_session=x"))
	ctx := context.Background()

	if _, err := c.Stats().DailyRange(ctx, DateRange{Start: "2026-11-01", End: "2026-11-15"}, 0, RealmCN); err != nil {
		t.Fatalf("DailyRange: %v", err)
	}
	if q := lastQuery(t, *recs); q != "end=2026-11-15&realm=cn&start=2026-11-01" {
		t.Fatalf("daily range query = %q", q)
	}

	// 只填一端：serde 不进查询串，退回 days 口径。
	if _, err := c.Stats().ByModelRange(ctx, DateRange{Start: "2026-11-01"}, 7, RealmAny); err != nil {
		t.Fatalf("ByModelRange: %v", err)
	}
	if q := lastQuery(t, *recs); q != "days=7" {
		t.Fatalf("partial range query = %q", q)
	}
}

// 429 的 Retry-After 要能取到（服务端把上游文案换算成了秒数）。
func TestAPIErrorRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "42")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"message":"配额用尽","code":"insufficient_quota"}}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithGatewayKey("wbk_x"))
	_, err := c.Credits(context.Background())
	if err == nil {
		t.Fatal("期望报错")
	}
	var qe *QuotaError
	if !errors.As(err, &qe) {
		t.Fatalf("期望 *QuotaError，得到 %T", err)
	}
	if qe.RetryAfter != 42 {
		t.Fatalf("RetryAfter = %d", qe.RetryAfter)
	}
	if qe.Code != "insufficient_quota" {
		t.Fatalf("code = %q", qe.Code)
	}
}

// clear-cooling 的两条路径明细不同：旧三字段必须视为可选。
func TestClearCoolingDetailBothPaths(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		chk  func(*testing.T, *ClearCoolingResult)
	}{
		{
			name: "在线清除（不停上游）",
			body: `{"ok":true,"message":"已通过上游管理接口在线清除","uid":"u1",` +
				`"online":true,"models":["glm-5.2"],"unfrozen":true}`,
			chk: func(t *testing.T, r *ClearCoolingResult) {
				if r.Online == nil || !*r.Online {
					t.Fatalf("online = %v", r.Online)
				}
				if len(r.Models) != 1 || r.StateFile != "" || r.Before != nil {
					t.Fatalf("在线分支不该有离线明细: %+v", r)
				}
			},
		},
		{
			name: "离线清除（停上游改 state.json）",
			body: `{"ok":true,"message":"已强制清除冷却","uid":"u1",` +
				`"state_file":"/opt/wb/state.json","backup":"/opt/wb/state.json.bak",` +
				`"before":{"until":"2026-11-18T00:00:00Z","reason":"6004"}}`,
			chk: func(t *testing.T, r *ClearCoolingResult) {
				if r.Online != nil {
					t.Fatalf("离线分支不该有 online: %+v", r.Online)
				}
				if r.StateFile == "" || r.Backup == "" || r.Before == nil {
					t.Fatalf("离线明细缺失: %+v", r)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := newReplay(t, map[string]func(http.ResponseWriter, *http.Request){
				"/api/accounts/a.json/clear-cooling": func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, tc.body)
				},
			})
			c := NewClient(srv.URL, WithAdminCookie("wb_session=x"))
			got, err := c.Accounts().ClearCoolingDetail(context.Background(), "a.json")
			if err != nil {
				t.Fatalf("ClearCoolingDetail: %v", err)
			}
			tc.chk(t, got)
		})
	}
}

// 分组：client 级默认值要给所有账号级端点补上 upstream_id，显式传的优先。
func TestWithUpstreamID(t *testing.T) {
	srv, recs := newReplay(t, nil)
	ctx := context.Background()

	// 未设置：不带 upstream_id（服务端用默认分组）。
	c := NewClient(srv.URL, WithAdminCookie("wb_session=x"))
	if _, err := c.Accounts().List(ctx); err != nil {
		t.Fatalf("List: %v", err)
	}
	if q := lastQuery(t, *recs); q != "" {
		t.Fatalf("未设默认分组时不该带参数，实得 %q", q)
	}

	// 设了默认分组：连按文件名操作的端点也要带上——错分组的报错是
	// 「账号文件不存在」，会把人引到完全错误的方向。
	g := NewClient(srv.URL, WithAdminCookie("wb_session=x"), WithUpstreamID(5))
	if g.UpstreamID() != 5 {
		t.Fatalf("UpstreamID() = %d", g.UpstreamID())
	}
	if _, err := g.Accounts().Checkin(ctx, "a.json"); err != nil {
		t.Fatalf("Checkin: %v", err)
	}
	if q := lastQuery(t, *recs); q != "upstream_id=5" {
		t.Fatalf("Checkin query = %q", q)
	}
	if p := (*recs)[len(*recs)-1].Path; p != "/api/accounts/a.json/checkin" {
		t.Fatalf("Checkin path = %q", p)
	}

	// 显式传的覆盖默认值。
	if _, err := g.Accounts().Order(ctx, Ptr(9)); err != nil {
		t.Fatalf("Order: %v", err)
	}
	if q := lastQuery(t, *recs); q != "upstream_id=9" {
		t.Fatalf("Order query = %q", q)
	}

	// 运行期切换也生效。
	g.SetUpstreamID(11)
	if _, err := g.Accounts().List(ctx); err != nil {
		t.Fatalf("List: %v", err)
	}
	if q := lastQuery(t, *recs); q != "upstream_id=11" {
		t.Fatalf("List query = %q", q)
	}
}

// 抓包的轮次级视图：status 是**整数**（HTTP 状态码），token 计数可空。
//
// 这条 fixture 是照上游真实返回抄的：tap_turns.status 是 INTEGER 列
// （db.py），写成 string 会让 Turn() 对任何真实记录解码失败——而列表接口
// 不返回轮次详情，测试很容易绕开它。
func TestTapTurnDecode(t *testing.T) {
	srv, _ := newReplay(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/tap/turns/11": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":11,"session_id":7,"seq":1,"ts":1763300000,`+
				`"path":"/v1/chat/completions","protocol":"chat","model":"glm-5.2",`+
				`"status":200,"stream":true,"latency_ms":842,"first_token_ms":120,`+
				`"prompt_tokens":1024,"completion_tokens":256,`+
				`"cache_hit_tokens":null,"cache_miss_tokens":null,`+
				`"cost":0.12,"error":"","truncated":false,`+
				`"request":{"messages":[{"role":"user","content":"hi"}]},`+
				`"request_ok":true,"response":{"choices":[]},"response_ok":true,`+
				`"passthrough":true,"client_protocol":"chat",`+
				`"summary":{"messages":1,"roles":{"user":1},"tools":0,"system_chars":0}}`)
		},
		"/api/tap/sessions/7": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":7,"fingerprint":"fp","client":"codex",`+
				`"kind":"main","protocol":"chat","model":"glm-5.2","turns":1,`+
				`"items":[{"id":11,"seq":1,"ts":1763300000,"status":502,"stream":false,`+
				`"model":"glm-5.2","prompt_tokens":null,"completion_tokens":null,`+
				`"error":"upstream 502","truncated":false,"protocol":"chat"}]}`)
		},
	})

	c := NewClient(srv.URL, WithAdminCookie("wb_session=x"))
	ctx := context.Background()

	turn, err := c.Tap().Turn(ctx, 11)
	if err != nil {
		t.Fatalf("Tap.Turn: %v", err)
	}
	if turn.Status == nil || *turn.Status != 200 {
		t.Fatalf("Turn.Status = %v, 期望 200", turn.Status)
	}
	if turn.PromptTokens == nil || *turn.PromptTokens != 1024 {
		t.Fatalf("Turn.PromptTokens = %v", turn.PromptTokens)
	}
	// null != 0：没采集到的缓存计数必须留 nil，不能变成「缓存命中 0 次」。
	if turn.CacheHitTokens != nil || turn.CacheMissTokens != nil {
		t.Fatalf("null 的缓存计数应当是 nil：hit=%v miss=%v",
			turn.CacheHitTokens, turn.CacheMissTokens)
	}
	if !turn.Passthrough || turn.ClientProtocol != "chat" {
		t.Fatalf("两端视角: passthrough=%v clientProtocol=%q", turn.Passthrough, turn.ClientProtocol)
	}

	det, err := c.Tap().Session(ctx, 7)
	if err != nil {
		t.Fatalf("Tap.Session: %v", err)
	}
	if len(det.Items) != 1 {
		t.Fatalf("items = %d", len(det.Items))
	}
	if got := det.Items[0].Status; got == nil || *got != 502 {
		t.Fatalf("summary.Status = %v, 期望 502", got)
	}
}

// 轮询的 realm 是**一致性校验**，不是路由：RealmAny 时必须不发，
// 否则国际版的码会被判 realm_mismatch 并一直空转到过期。
func TestPollAuthRealm(t *testing.T) {
	srv, recs := newReplay(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/auth/poll": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"status":"waiting"}`)
		},
	})
	c := NewClient(srv.URL, WithAdminCookie("wb_session=x"), WithRealm(RealmGlobal))
	ctx := context.Background()

	if _, err := c.Accounts().PollAuth(ctx, "st1", RealmAny, ""); err != nil {
		t.Fatalf("PollAuth: %v", err)
	}
	if q := lastQuery(t, *recs); strings.Contains(q, "realm") {
		t.Fatalf("RealmAny 不该发 realm，实得 %q", q)
	}

	if _, err := c.Accounts().PollAuthIn(ctx, AuthPollIn{
		State: "st2", Realm: RealmCN, Region: "HK",
	}); err != nil {
		t.Fatalf("PollAuthIn: %v", err)
	}
	if q := lastQuery(t, *recs); q != "realm=cn&region=HK&state=st2" {
		t.Fatalf("显式 realm 要发出去，实得 %q", q)
	}
}

// 价目表里的别名被手改坏（值不是字符串）时，整份配置仍要读得出来。
func TestPricingToleratesDirtyAlias(t *testing.T) {
	srv, _ := newReplay(t, map[string]func(http.ResponseWriter, *http.Request){
		"/api/settings/pricing": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"currency":"CNY","money_unit":"元/百万 tokens",`+
				`"versions":[],`+
				`"aliases":{"glm-5.2":"glm-5.2","broken":123,"also":"ok"},`+
				`"priced_models":["glm-5.2"],"stale_after_days":90}`)
		},
	})
	got, err := NewClient(srv.URL, WithAdminCookie("wb_session=x")).Settings().Pricing(context.Background())
	if err != nil {
		t.Fatalf("Pricing: %v", err)
	}
	if got.Aliases["glm-5.2"] != "glm-5.2" || got.Aliases["also"] != "ok" {
		t.Fatalf("好别名应当保留：%v", got.Aliases)
	}
	if _, ok := got.Aliases["broken"]; ok {
		t.Fatalf("非字符串值应当被跳过：%v", got.Aliases)
	}
}

// 局部更新里「清空」与「不改」是两个动作，必须都能表达。
func TestWatchdogPatchClears(t *testing.T) {
	srv, recs := newReplay(t, nil)
	c := NewClient(srv.URL, WithAdminCookie("wb_session=x"))
	ctx := context.Background()

	// Ptr("") / Ptr([]string{}) 是**清空**：空串和空列表在服务端都有意义。
	if _, err := c.Watchdog().SaveConfig(ctx, WatchdogConfigPatch{
		Mail: &WatchdogMailPatch{Password: Ptr(""), To: Ptr([]string{})},
	}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	b := lastBody(t, *recs)
	if !strings.Contains(b, `"password":""`) {
		t.Fatalf("Ptr(\"\") 应当发出空串：%s", b)
	}
	if !strings.Contains(b, `"to":[]`) {
		t.Fatalf("Ptr([]string{}) 应当发出空数组：%s", b)
	}

	// nil 是**不改**：整个 to / password 都不该出现在请求体里。
	if _, err := c.Watchdog().SaveConfig(ctx, WatchdogConfigPatch{
		Mail: &WatchdogMailPatch{Host: Ptr("smtp.example.com")},
	}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	b = lastBody(t, *recs)
	if strings.Contains(b, `"to"`) || strings.Contains(b, `"password"`) {
		t.Fatalf("nil 字段不该出现：%s", b)
	}

	// 空 targets 本来就会被服务端拒（400），要真的发出去让用户看到报错，
	// 而不是被 omitempty 吞成「什么都没改」。
	if _, err := c.Watchdog().SaveConfig(ctx, WatchdogConfigPatch{
		Targets: Ptr([]WatchdogTarget{}),
	}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if b := lastBody(t, *recs); !strings.Contains(b, `"targets":[]`) {
		t.Fatalf("Ptr([]WatchdogTarget{}) 应当发出空数组：%s", b)
	}
}
