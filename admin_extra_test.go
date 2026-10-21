package wbsdk

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// 这一组测试只关心「请求形状 + 解码结果」：服务端返回体按路径固定，
// 断言的是 SDK 有没有把方法 / 路径 / 查询串 / 请求体拼对。
func TestAdminExtraServicesContract(t *testing.T) {
	var mu sync.Mutex
	var recs []recorded
	// 上传接口的 multipart 内容无法从 recorded.Body 里读（那只是字节），
	// 单独留给它几个变量。
	var upName, upField, upExpected, upContent string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/system/update-package" && r.Method == http.MethodPost {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("ParseMultipartForm: %v", err)
				_, _ = io.WriteString(w, `{}`)
				return
			}
			file, hdr, err := r.FormFile("file")
			if err != nil {
				t.Errorf("FormFile: %v", err)
			} else {
				defer file.Close()
				raw, _ := io.ReadAll(file)
				upField, upName, upContent = "file", hdr.Filename, string(raw)
			}
			upExpected = r.FormValue("expected_sha256")
			_, _ = io.WriteString(w, `{"present":true,"name":"wb.tar.gz","size":11,"sha256":"`+
				strings.Repeat("a", 64)+`","version":"2.7.0"}`)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/claim/") {
			// 公开端点：不该带任何凭据（带了就说明 SDK 误判成管理面）。
			if got := r.Header.Get("Authorization"); got != "" {
				t.Errorf("claim 请求带了 Authorization: %q", got)
			}
			if got := r.Header.Get("Cookie"); got != "" {
				t.Errorf("claim 请求带了 Cookie: %q", got)
			}
		}
		if r.URL.Path == "/v1/usage" {
			if got := r.Header.Get("Authorization"); got != "Bearer wbk_test" {
				t.Errorf("/v1/usage 应带网关密钥，实际 %q", got)
			}
		}
		if strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/api/claim") {
			if got := r.Header.Get("Authorization"); got != "Bearer wbt_test" {
				t.Errorf("%s 应带管理面 token，实际 %q", r.URL.Path, got)
			}
		}

		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		recs = append(recs, recorded{r.Method, r.URL.Path, r.URL.RawQuery, string(b)})
		mu.Unlock()

		var body string
		switch r.URL.Path {
		case "/api/proxy-routes":
			if r.Method == http.MethodPost {
				body = `{"name":"hk1","scheme":"http","host":"1.2.3.4","port":8080,"has_auth":true,` +
					`"masked":"http://u:***@1.2.3.4:8080","weight":100,"enabled":true,"bound":0}`
			} else {
				body = `{"routes":[{"name":"hk1","scheme":"http","host":"1.2.3.4","port":8080,` +
					`"has_auth":true,"masked":"http://u:***@1.2.3.4:8080","weight":100,"enabled":true,` +
					`"bound":2,"bound_names":["老王"],"shared_warning":false,` +
					`"probe":{"ok":true,"ms":12,"at":1727000000,"exit_ip":"1.2.3.4"}}],` +
					`"default":"","schemes":["http","socks5"],"bound_total":2,` +
					`"pool":{"enabled":false,"strategy":"priority","interval_seconds":0,` +
					`"strategies":["priority","random"],"routes_total":1,"routes_enabled":1}}`
			}
		case "/api/proxy-routes/hk1":
			body = `{"name":"hk1","scheme":"http","host":"1.2.3.4","port":null,"has_auth":false,` +
				`"masked":"http://1.2.3.4","weight":200,"enabled":false,"bound":2,"bound_updated":2}`
		case "/api/proxy-routes/import":
			body = `{"imported":2,"problems":["第 3 行：认不出格式（应写成 名字=地址）"]}`
		case "/api/proxy-routes/probe":
			body = `{"ok":true,"ms":88,"exit_ip":"5.6.7.8","exit_info":"JP · Tokyo · Example"}`
		case "/api/proxy-routes/bindings":
			body = `{"accounts":[{"file":"a.json","uid":"u1","nickname":"老王","proxy":"hk1","dir":"/auths"}],` +
				`"routes":["hk1"],"pool":{"enabled":true,"strategy":"random","interval_seconds":60}}`
		case "/api/proxy-routes/hk1/accounts":
			body = `{"name":"hk1","bound":true,"updated":2,"skipped":["小李: 账号文件读取失败"]}`
		case "/api/proxy-routes/hk1/auto-assign":
			body = `{"name":"hk1","bound":true,"updated":3,"skipped":[],"requested":3,"candidates":9,"unbound_left":6}`
		case "/api/settings/proxy-pool":
			body = `{"enabled":true,"strategy":"random","interval_seconds":60,"strategies":["priority"],` +
				`"routes_total":1,"routes_enabled":1}`
		case "/api/settings/proxy":
			body = `{"global_proxy_enabled":true,"http_proxy":"http://u:***@p:1","update_proxy":"",` +
				`"http_configured":true,"update_configured":false,"env_http_proxy":"",` +
				`"effective":"http://u:***@p:1","source":"panel","probe_url":"https://ipinfo.io/json"}`
		case "/api/settings/proxy/probe":
			body = `{"ok":false,"error":"当前没有配置出口代理（面板直连）","ms":0}`
		case "/api/proxies":
			body = `{"routes":["hk1","jp2"],"default":""}`
		case "/api/settings/pg-sync":
			if r.Method == http.MethodPost {
				body = `{"config":{"enabled":true,"host":"pg.internal","port":5432,"dbname":"wb",` +
					`"user":"wb","password":"********","sslmode":"require","interval_minutes":60,` +
					`"keep_local_backup":true,"last_export_at":0,"last_import_at":0}}`
			} else {
				body = `{"config":{"enabled":false,"host":"","port":5432,"dbname":"","user":"",` +
					`"password":"","sslmode":"prefer","interval_minutes":0,"keep_local_backup":true,` +
					`"last_export_at":1727000000,"last_import_at":0},` +
					`"status":{"running":false,"kind":"","ok":null,"step":"","percent":0,` +
					`"tables_total":0,"tables_done":0,"rows":0,"logs":[],"started_at":0,"finished_at":0}}`
			}
		case "/api/settings/pg-sync/test":
			body = `{"ok":true,"message":"连接成功：wb / wb","server_version":"PostgreSQL 16.3"}`
		case "/api/settings/pg-sync/export", "/api/settings/pg-sync/import":
			body = `{"ok":true,"message":"已开始","status":{"running":true,"kind":"export","ok":null,` +
				`"percent":0,"tables_total":12,"logs":[],"started_at":1727000000}}`
		case "/api/settings/pg-sync/status":
			body = `{"running":true,"kind":"export","ok":null,"step":"导出 users","percent":42,` +
				`"tables_total":12,"tables_done":5,"rows":1234,` +
				`"logs":[{"ts":1727000000,"level":"info","text":"开始"}],"started_at":1727000000}`
		case "/api/red-packets":
			if r.Method == http.MethodPost {
				body = `{"id":7,"title":"给老王","quota_kind":"credit","total_amount":10,"shares":2,` +
					`"mode":"even","models":[],"code":"abc123","expires_at":1727600000,` +
					`"created_at":1727000000,"keys":[{"id":31,"name":"给老王-1","prefix":"wbk_aaa",` +
					`"key":"wbk_aaa_secret","quota_credit":5,"used_credit":0,"created_at":1727000000}]}`
			} else {
				body = `[{"id":7,"title":"给老王","quota_kind":"credit","total_amount":10,"shares":2,` +
					`"mode":"even","models":[],"code":"abc123","created_by":"admin",` +
					`"created_at":1727000000,"expires_at":1727600000,"claimed":1,"revoked":false}]`
			}
		case "/api/red-packets/7":
			body = `{"id":7,"title":"给老王","quota_kind":"credit","total_amount":10,"shares":2,` +
				`"mode":"even","models":[],"code":"abc123","created_by":"admin",` +
				`"created_at":1727000000,"expires_at":1727600000,"claimed":1,"revoked":false,` +
				`"items":[{"key_id":31,"prefix":"wbk_aaa","amount":5,"enabled":true,` +
				`"used_tokens":0,"used_credit":1.5,"claimed_by_ip":"9.9.9.9","claimed_at":1727100000},` +
				`{"key_id":null,"prefix":"","amount":5,"enabled":false,"used_tokens":0,"used_credit":0,` +
				`"claimed_by_ip":"","claimed_at":null}]}`
		case "/api/red-packets/7/revoke":
			body = `{"revoked":2}`
		case "/api/claim/abc123":
			if r.Method == http.MethodPost {
				body = `{"amount":5,"quota_kind":"credit","models":[],"key":"wbk_bbb_secret","expires_at":1727600000}`
			} else {
				body = `{"title":"给老王","quota_kind":"credit","shares":2,"left":1,"models":[],` +
					`"expires_at":1727600000,"expired":false,"claimed":true,"my_amount":5,` +
					`"my_key":"wbk_bbb_secret"}`
			}
		case "/api/upstreams":
			if r.Method == http.MethodPost {
				body = `{"id":3,"name":"港区","base_url":"http://hk:8080","has_key":true,` +
					`"api_key_masked":"abcd…wxyz","note":"","enabled":true,"auth_dir":"","container":"",` +
					`"created_at":1727000000,"updated_at":1727000000,"is_default":false,"bound_keys":0}`
			} else {
				body = `{"items":[{"id":null,"name":"默认上游","base_url":"http://127.0.0.1:8080",` +
					`"has_key":true,"api_key_masked":"abcd…wxyz","note":"来自环境变量","enabled":true,` +
					`"auth_dir":"/auths","container":"","created_at":null,"updated_at":null,` +
					`"is_default":true,"bound_keys":2}]}`
			}
		case "/api/upstreams/3":
			if r.Method == http.MethodPatch {
				body = `{"id":3,"name":"港区2","base_url":"http://hk:8080","has_key":false,` +
					`"api_key_masked":"","enabled":false,"is_default":false,"bound_keys":1}`
			} else {
				body = `{"ok":true}`
			}
		case "/api/upstreams/3/probe":
			body = `{"ok":false,"message":"dial tcp: connect refused"}`
		case "/api/stats/hourly":
			body = `[{"day":"2026-10-20","hour":0,"requests":1,"prompt_tokens":2,` +
				`"completion_tokens":3,"credit":0.1,"failed":0}]`
		case "/api/stats/credits":
			body = `{"days":7,"from":"2026-10-14","to":"2026-10-20",` +
				`"today":{"income":1,"expense":0.5,"net":0.5},` +
				`"week":{"income":4,"expense":2,"net":2},` +
				`"total":{"income":4,"expense":2,"net":2},` +
				`"daily":[{"day":"2026-10-20","income":1,"expense":0.5,"net":0.5}]}`
		case "/api/settings/github-token":
			body = `{"configured":true,"masked":"ghp_…abcd","source":"panel"}`
		case "/api/settings/github-mirror":
			body = `{"mirror":"https://gh-proxy.com/","stored":"https://gh-proxy.com/",` +
				`"env_mirror":"","source":"panel","presets":["https://gh-proxy.com/"]}`
		case "/api/system/update-package":
			if r.Method == http.MethodGet {
				body = `{"present":true,"name":"wb.tar.gz","size":1024,"sha256":"` +
					strings.Repeat("b", 64) + `","expected_sha256":"","version":"2.7.0",` +
					`"uploaded_at":1727000000,"uploaded_by":"admin"}`
			} else {
				body = `{"ok":true}`
			}
		case "/api/accounts/a.json/proxy":
			body = `{"ok":true,"file":"a.json","proxy":"hk1","reload_triggered":true}`
		case "/api/accounts/a.json/weight":
			body = `{"ok":true,"file":"a.json","weight":2,"reload_triggered":true}`
		case "/api/accounts/a.json/force-cooling":
			body = `{"ok":true,"message":"已强制设定冷却状态，上游已重启生效",` +
				`"until":"2026-10-20T12:00:00+00:00","until_changed":true}`
		case "/api/accounts/a.json/move":
			body = `{"ok":true,"file":"a.json","uid":"u1",` +
				`"from":{"id":null,"name":"默认上游"},"to":{"id":3,"name":"港区"}}`
		case "/v1/usage":
			body = `{"object":"key.usage","key":{"name":"ci","prefix":"wbk_aaa","realm":"cn",` +
				`"enabled":true,"expires_at":0,"expired":false,"models":["glm-5.2"]},` +
				`"tokens":{"used":1000,"limit":null,"remaining":null},` +
				`"credits":{"used":1,"limit":10,"remaining":9},` +
				`"last_used_at":1727000000,"snapshot_at":1727000100,"data_source":"panel-recorded"}`
		default:
			body = `{}`
		}
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	ctx := context.Background()
	admin := NewClient(srv.URL, WithAdminToken("wbt_test"))
	// 公开抽奖端刻意用**没有任何凭据**的客户端：能调通才算真公开。
	anon := NewClient(srv.URL)
	gateway := NewClient(srv.URL, WithGatewayKey("wbk_test"))

	// ── 代理线路 ─────────────────────────────────────
	routes, err := admin.Proxies().Routes(ctx, ptr(3))
	if err != nil {
		t.Fatalf("Proxies().Routes: %v", err)
	}
	if len(routes.Routes) != 1 || routes.Routes[0].Name != "hk1" {
		t.Fatalf("unexpected routes: %+v", routes.Routes)
	}
	if routes.Routes[0].Port == nil || *routes.Routes[0].Port != 8080 {
		t.Fatalf("port 未解码: %+v", routes.Routes[0].Port)
	}
	if routes.Routes[0].Probe == nil || routes.Routes[0].Probe.ExitIP != "1.2.3.4" {
		t.Fatalf("probe 未解码: %+v", routes.Routes[0].Probe)
	}
	if routes.BoundTotal != 2 || routes.Pool.Strategy != "priority" {
		t.Fatalf("unexpected list meta: %+v", routes)
	}

	created, err := admin.Proxies().CreateRoute(ctx, ProxyRouteIn{Name: "hk1", URL: "http://u:p@1.2.3.4:8080"})
	if err != nil {
		t.Fatalf("CreateRoute: %v", err)
	}
	if created.Name != "hk1" || created.Weight != 100 {
		t.Fatalf("unexpected created: %+v", created)
	}

	patch := ProxyRoutePatch{NewName: ptr("hk2"), Weight: ptr(200), Enabled: ptr(false)}
	updated, err := admin.Proxies().UpdateRoute(ctx, "hk1", patch, ptr(3))
	if err != nil {
		t.Fatalf("UpdateRoute: %v", err)
	}
	if updated.Port != nil {
		t.Fatalf("port 应为 null（nil），得到 %+v", *updated.Port)
	}
	if updated.BoundUpdated == nil || *updated.BoundUpdated != 2 {
		t.Fatalf("bound_updated 未解码: %+v", updated.BoundUpdated)
	}

	del, err := admin.Proxies().DeleteRoute(ctx, "hk1", true, ptr(3))
	if err != nil {
		t.Fatalf("DeleteRoute: %v", err)
	}
	if del.BoundCleared != 0 {
		t.Fatalf("unexpected delete: %+v", del)
	}

	imp, err := admin.Proxies().ImportRoutes(ctx, "hk1=http://a:1\n坏行")
	if err != nil {
		t.Fatalf("ImportRoutes: %v", err)
	}
	if imp.Imported != 2 || len(imp.Problems) != 1 {
		t.Fatalf("unexpected import: %+v", imp)
	}

	probe, err := admin.Proxies().ProbeRoute(ctx, "hk1")
	if err != nil {
		t.Fatalf("ProbeRoute: %v", err)
	}
	if !probe.OK || probe.ExitInfo == "" {
		t.Fatalf("unexpected probe: %+v", probe)
	}

	bindings, err := admin.Proxies().Bindings(ctx, nil)
	if err != nil {
		t.Fatalf("Bindings: %v", err)
	}
	if len(bindings.Accounts) != 1 || bindings.Accounts[0].Dir != "/auths" {
		t.Fatalf("unexpected bindings: %+v", bindings)
	}

	bound, err := admin.Proxies().BindAccounts(ctx, "hk1", []string{"a.json"}, true, ptr(3))
	if err != nil {
		t.Fatalf("BindAccounts: %v", err)
	}
	if bound.Updated != 2 || len(bound.Skipped) != 1 {
		t.Fatalf("unexpected bind result: %+v", bound)
	}

	auto, err := admin.Proxies().AutoAssign(ctx, "hk1", 3, false, ptr(3))
	if err != nil {
		t.Fatalf("AutoAssign: %v", err)
	}
	if auto.Requested == nil || *auto.Requested != 3 || auto.UnboundLeft == nil || *auto.UnboundLeft != 6 {
		t.Fatalf("unexpected auto-assign: %+v", auto)
	}

	pool, err := admin.Proxies().SavePool(ctx, ProxyPoolPatch{Enabled: ptr(true), Strategy: "random", IntervalSeconds: ptr(60)})
	if err != nil {
		t.Fatalf("SavePool: %v", err)
	}
	if !pool.Enabled || pool.IntervalSeconds != 60 {
		t.Fatalf("unexpected pool: %+v", pool)
	}

	ps, err := admin.Proxies().Settings(ctx)
	if err != nil {
		t.Fatalf("Proxies().Settings: %v", err)
	}
	if ps.Source != "panel" || !ps.HTTPConfigured {
		t.Fatalf("unexpected proxy settings: %+v", ps)
	}

	if _, err := admin.Proxies().SaveSettings(ctx, ProxySettingsPatch{HTTPProxy: ptr("http://u:***@p:1")}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	probe2, err := admin.Proxies().ProbeSettings(ctx)
	if err != nil {
		t.Fatalf("ProbeSettings: %v", err)
	}
	if probe2.OK || probe2.Error == "" {
		t.Fatalf("unexpected settings probe: %+v", probe2)
	}

	accRoutes, err := admin.Proxies().AccountRoutes(ctx)
	if err != nil {
		t.Fatalf("AccountRoutes: %v", err)
	}
	if len(accRoutes.Routes) != 2 {
		t.Fatalf("unexpected account routes: %+v", accRoutes)
	}

	// ── PostgreSQL 备份 ──────────────────────────────
	pg, err := admin.PGSync().Get(ctx)
	if err != nil {
		t.Fatalf("PGSync().Get: %v", err)
	}
	if pg.Config.SSLMode != "prefer" || pg.Status.OK != nil {
		t.Fatalf("unexpected pgsync: %+v", pg)
	}
	if pg.Config.LastExportAt != 1727000000 {
		t.Fatalf("last_export_at 未解码: %+v", pg.Config)
	}

	saved, err := admin.PGSync().Save(ctx, PGSyncConfigPatch{Enabled: ptr(true), IntervalMinutes: ptr(60)})
	if err != nil {
		t.Fatalf("PGSync().Save: %v", err)
	}
	if !saved.Enabled || saved.IntervalMinutes != 60 || saved.SSLMode != "require" {
		t.Fatalf("unexpected saved config: %+v", saved)
	}

	res, err := admin.PGSync().Test(ctx, PGSyncConfigPatch{Host: ptr("pg.internal")})
	if err != nil {
		t.Fatalf("PGSync().Test: %v", err)
	}
	if !res.OK || res.ServerVersion != "PostgreSQL 16.3" {
		t.Fatalf("unexpected test result: %+v", res)
	}

	exp, err := admin.PGSync().Export(ctx)
	if err != nil {
		t.Fatalf("PGSync().Export: %v", err)
	}
	if !exp.OK || !exp.Status.Running || exp.Status.Kind != "export" {
		t.Fatalf("unexpected export: %+v", exp)
	}
	if _, err := admin.PGSync().Import(ctx); err != nil {
		t.Fatalf("PGSync().Import: %v", err)
	}

	st, err := admin.PGSync().Status(ctx)
	if err != nil {
		t.Fatalf("PGSync().Status: %v", err)
	}
	if st.Percent != 42 || st.Rows != 1234 || len(st.Logs) != 1 {
		t.Fatalf("unexpected status: %+v", st)
	}

	// ── 红包 ─────────────────────────────────────────
	packet, err := admin.RedPackets().Create(ctx, RedPacketIn{Title: "给老王", TotalAmount: 10, Shares: 2, Mode: RedPacketEven, TTLDays: ptr(7)})
	if err != nil {
		t.Fatalf("RedPackets().Create: %v", err)
	}
	if len(packet.Keys) != 1 || packet.Keys[0].Key == "" {
		t.Fatalf("创建结果应含明文密钥: %+v", packet)
	}

	list, err := admin.RedPackets().List(ctx)
	if err != nil {
		t.Fatalf("RedPackets().List: %v", err)
	}
	if len(list) != 1 || list[0].Claimed != 1 {
		t.Fatalf("unexpected packet list: %+v", list)
	}

	detail, err := admin.RedPackets().Get(ctx, 7)
	if err != nil {
		t.Fatalf("RedPackets().Get: %v", err)
	}
	if len(detail.Items) != 2 || detail.Items[0].ClaimedByIP == nil || *detail.Items[0].ClaimedByIP != "9.9.9.9" {
		t.Fatalf("unexpected detail: %+v", detail.Items)
	}
	// 收回后那一段：key_id / claimed_at 都是 null，不能解码炸掉。
	if detail.Items[1].KeyID != nil || detail.Items[1].ClaimedAt != nil {
		t.Fatalf("null 字段应为 nil: %+v", detail.Items[1])
	}

	rev, err := admin.RedPackets().Revoke(ctx, 7)
	if err != nil {
		t.Fatalf("RedPackets().Revoke: %v", err)
	}
	if rev.Revoked != 2 {
		t.Fatalf("unexpected revoke: %+v", rev)
	}

	info, err := anon.RedPackets().ClaimInfo(ctx, "abc123")
	if err != nil {
		t.Fatalf("ClaimInfo（无凭据）: %v", err)
	}
	if !info.Claimed || info.MyKey == nil || *info.MyKey != "wbk_bbb_secret" {
		t.Fatalf("unexpected claim info: %+v", info)
	}

	claim, err := anon.RedPackets().Claim(ctx, "abc123")
	if err != nil {
		t.Fatalf("Claim（无凭据）: %v", err)
	}
	if claim.Key != "wbk_bbb_secret" || claim.Amount != 5 {
		t.Fatalf("unexpected claim: %+v", claim)
	}

	// ── 多上游 ───────────────────────────────────────
	ups, err := admin.Upstreams().List(ctx)
	if err != nil {
		t.Fatalf("Upstreams().List: %v", err)
	}
	if len(ups.Items) != 1 || !ups.Items[0].IsDefault || ups.Items[0].ID != nil {
		t.Fatalf("unexpected upstream list: %+v", ups.Items)
	}
	if ups.Items[0].BoundKeys != 2 {
		t.Fatalf("bound_keys 未解码: %+v", ups.Items[0])
	}

	up, err := admin.Upstreams().Create(ctx, UpstreamIn{Name: "港区", BaseURL: "http://hk:8080", APIKey: "k"})
	if err != nil {
		t.Fatalf("Upstreams().Create: %v", err)
	}
	if up.ID == nil || *up.ID != 3 || !up.HasKey {
		t.Fatalf("unexpected upstream: %+v", up)
	}

	if _, err := admin.Upstreams().Update(ctx, 3, UpstreamPatch{Name: ptr("港区2"), Enabled: ptr(false)}); err != nil {
		t.Fatalf("Upstreams().Update: %v", err)
	}

	upProbe, err := admin.Upstreams().Probe(ctx, 3)
	if err != nil {
		t.Fatalf("Upstreams().Probe: %v", err)
	}
	if upProbe.OK || upProbe.Message == "" {
		t.Fatalf("unexpected upstream probe: %+v", upProbe)
	}

	if err := admin.Upstreams().Delete(ctx, 3); err != nil {
		t.Fatalf("Upstreams().Delete: %v", err)
	}

	// ── 统计补充 ─────────────────────────────────────
	hourly, err := admin.Stats().Hourly(ctx, "2026-10-20", RealmCN)
	if err != nil {
		t.Fatalf("Stats().Hourly: %v", err)
	}
	if len(hourly) != 1 || hourly[0].Hour != 0 {
		t.Fatalf("unexpected hourly: %+v", hourly)
	}

	flow, err := admin.Stats().CreditsFlow(ctx, 7, RealmCN)
	if err != nil {
		t.Fatalf("Stats().CreditsFlow: %v", err)
	}
	if flow.Days != 7 || flow.Total.Net != 2 || len(flow.Daily) != 1 {
		t.Fatalf("unexpected credits flow: %+v", flow)
	}

	// ── GitHub 凭据 / 镜像 ───────────────────────────
	tok, err := admin.Settings().GitHubToken(ctx)
	if err != nil {
		t.Fatalf("Settings().GitHubToken: %v", err)
	}
	if !tok.Configured || tok.Source != "panel" {
		t.Fatalf("unexpected token: %+v", tok)
	}
	if _, err := admin.Settings().SaveGitHubToken(ctx, "ghp_x"); err != nil {
		t.Fatalf("SaveGitHubToken: %v", err)
	}

	mirror, err := admin.Settings().GitHubMirror(ctx)
	if err != nil {
		t.Fatalf("Settings().GitHubMirror: %v", err)
	}
	if mirror.Mirror != "https://gh-proxy.com/" || len(mirror.Presets) != 1 {
		t.Fatalf("unexpected mirror: %+v", mirror)
	}
	if _, err := admin.Settings().SaveGitHubMirror(ctx, "https://gh-proxy.com/"); err != nil {
		t.Fatalf("SaveGitHubMirror: %v", err)
	}

	// ── 离线升级包 ───────────────────────────────────
	pkg, err := admin.System().Package(ctx)
	if err != nil {
		t.Fatalf("System().Package: %v", err)
	}
	if !pkg.Present || pkg.Version != "2.7.0" || pkg.Size != 1024 {
		t.Fatalf("unexpected package: %+v", pkg)
	}

	uploaded, err := admin.System().UploadPackage(ctx, "wb.tar.gz", strings.NewReader("hello world"), strings.Repeat("c", 64))
	if err != nil {
		t.Fatalf("UploadPackage: %v", err)
	}
	if !uploaded.Present {
		t.Fatalf("unexpected upload result: %+v", uploaded)
	}
	if upField != "file" || upName != "wb.tar.gz" || upContent != "hello world" {
		t.Fatalf("multipart 内容不对: field=%q name=%q content=%q", upField, upName, upContent)
	}
	if upExpected != strings.Repeat("c", 64) {
		t.Fatalf("expected_sha256 未提交: %q", upExpected)
	}

	if err := admin.System().DeletePackage(ctx); err != nil {
		t.Fatalf("DeletePackage: %v", err)
	}

	// ── 账号级：线路 / 权重 / 冷却 / 转会 ────────────
	ap, err := admin.Accounts().SetProxy(ctx, "a.json", "hk1", ptr(3))
	if err != nil {
		t.Fatalf("Accounts().SetProxy: %v", err)
	}
	if !ap.ReloadTriggered || ap.Proxy != "hk1" {
		t.Fatalf("unexpected set proxy: %+v", ap)
	}

	aw, err := admin.Accounts().SetWeight(ctx, "a.json", 2, nil)
	if err != nil {
		t.Fatalf("Accounts().SetWeight: %v", err)
	}
	if aw.Weight != 2 {
		t.Fatalf("unexpected weight: %+v", aw)
	}

	fc, err := admin.Accounts().ForceCooling(ctx, "a.json", ForceCoolingIn{DurationSeconds: ptr(600)}, nil)
	if err != nil {
		t.Fatalf("Accounts().ForceCooling: %v", err)
	}
	if fc["ok"] != true || fc["until"] == nil {
		t.Fatalf("unexpected force cooling: %+v", fc)
	}

	moved, err := admin.Accounts().Move(ctx, "a.json", 3, nil)
	if err != nil {
		t.Fatalf("Accounts().Move: %v", err)
	}
	if moved.To.ID == nil || *moved.To.ID != 3 || moved.From.ID != nil {
		t.Fatalf("unexpected move: %+v", moved)
	}

	// ── 网关密钥用量 ─────────────────────────────────
	usage, err := gateway.Usage(ctx)
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if usage.Key.Prefix != "wbk_aaa" || usage.Tokens.Limit != nil {
		t.Fatalf("不限额应为 nil: %+v", usage.Tokens)
	}
	if usage.Credits.Remaining == nil || *usage.Credits.Remaining != 9 {
		t.Fatalf("unexpected credits: %+v", usage.Credits)
	}

	// ── 请求形状断言 ─────────────────────────────────
	check := func(method, path, queryContains, bodyContains string) {
		t.Helper()
		for _, r := range recs {
			if r.Method != method || r.Path != path {
				continue
			}
			if queryContains != "" && !strings.Contains(r.Query, queryContains) {
				t.Errorf("%s %s 查询串 = %q，应含 %q", method, path, r.Query, queryContains)
			}
			if bodyContains != "" && !strings.Contains(r.Body, bodyContains) {
				t.Errorf("%s %s 请求体 = %q，应含 %q", method, path, r.Body, bodyContains)
			}
			return
		}
		t.Errorf("没有发出 %s %s", method, path)
	}
	check(http.MethodGet, "/api/proxy-routes", "upstream_id=3", "")
	check(http.MethodPut, "/api/proxy-routes/hk1", "upstream_id=3", `"new_name":"hk2"`)
	check(http.MethodDelete, "/api/proxy-routes/hk1", "force=true", "")
	check(http.MethodPost, "/api/proxy-routes/hk1/accounts", "upstream_id=3", `"bound":true`)
	check(http.MethodPost, "/api/proxy-routes/hk1/auto-assign", "upstream_id=3", `"count":3`)
	check(http.MethodPut, "/api/settings/proxy-pool", "", `"strategy":"random"`)
	check(http.MethodGet, "/api/settings/pg-sync", "", "")
	check(http.MethodPost, "/api/settings/pg-sync/test", "", `"host":"pg.internal"`)
	check(http.MethodGet, "/api/settings/pg-sync/status", "", "")
	check(http.MethodPost, "/api/red-packets", "", `"total_amount":10`)
	check(http.MethodPost, "/api/claim/abc123", "", "")
	check(http.MethodGet, "/api/upstreams", "", "")
	check(http.MethodPatch, "/api/upstreams/3", "", `"name":"港区2"`)
	check(http.MethodGet, "/api/stats/hourly", "day=2026-10-20", "")
	check(http.MethodGet, "/api/stats/credits", "days=7", "")
	check(http.MethodPut, "/api/settings/github-token", "", `"token":"ghp_x"`)
	check(http.MethodDelete, "/api/system/update-package", "", "")
	check(http.MethodPut, "/api/accounts/a.json/proxy", "upstream_id=3", `"proxy":"hk1"`)
	check(http.MethodPost, "/api/accounts/a.json/force-cooling", "", `"duration_seconds":600`)
	check(http.MethodPost, "/api/accounts/a.json/move", "", `"to_upstream_id":3`)
	check(http.MethodGet, "/v1/usage", "", "")
}
