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

type recorded struct {
	Method string
	Path   string
	Query  string
	Body   string
}

func ptr[T any](v T) *T { return &v }

func TestOpsServicesContract(t *testing.T) {
	var mu sync.Mutex
	var recs []recorded
	// 响应按路径固定返回；断言只关心请求形状与解码结果。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		recs = append(recs, recorded{r.Method, r.URL.Path, r.URL.RawQuery, string(b)})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		var body string
		switch r.URL.Path {
		case "/api/stats/summary":
			body = `{"today_requests":5,"today_credit":1.25,"active_keys":2,"usage_health":{"ok":true,"logs_today":5},"failures":{"today_4xx":1}}`
		case "/api/stats/daily":
			body = `[{"day":"2026-09-23","requests":3,"credit":0.5,"failed":2}]`
		case "/api/stats/by-key":
			body = `[{"name":"ci","requests":1,"credit":0.1}]`
		case "/api/logs":
			body = `{"total":1,"items":[{"id":9,"status":200,"stream":true,"realm":"cn","cache_hit_tokens":null,"credit":0.02}]}`
		case "/api/security/config":
			body = `{"enabled":true,"mode":"whitelist"}`
		case "/api/security/rules":
			if r.Method == http.MethodPost {
				body = `{"id":3,"kind":"allow","cidr":"10.0.0.0/8","note":"office","created_at":1}`
			} else {
				body = `[{"id":1,"kind":"deny","cidr":"1.2.3.4","note":"","created_at":1}]`
			}
		case "/api/security/logs":
			body = `[{"id":1,"ip":"9.9.9.9","path":"/v1/chat/completions","blocked":true,"reason":"invalid_key"}]`
		case "/api/users":
			body = `[{"username":"admin","role":"admin"}]`
		case "/api/users/bob":
			body = `{"username":"bob","role":"viewer","sessions_revoked":true,"relogin_required":false}`
		case "/api/audit-logs":
			body = `{"items":[{"id":1,"actor":"admin","action":"login","detail":"来源 1.1.1.1"}],"total":1}`
		case "/api/settings/upstream":
			body = `{"available":true,"config_path":"/opt/wb/config.json","pool":{"max_concurrent":4}}`
		case "/api/settings/model-map":
			body = `{"gpt-4o-mini":"glm-5.2"}`
		case "/api/upstream/reload-state":
			// last_at 是 time.time()：可能是小数，用小数锁死这一契约。
			body = `{"running":false,"pending":true,"restart_count":3,"last_at":1727000000.5}`
		case "/api/system/versions":
			body = `{"manager":"1.0.66","upstream_connected":false,"upstream_dir":"/opt/workbuddy2api"}`
		case "/api/system/changelog":
			body = `{"available":true,"versions":[{"version":"1.0.66","date":"2026-09-20","sections":[{"title":"修复","items":[{"level":0,"text":"x"}]}]}]}`
		case "/api/task-run":
			body = `{"running":false,"available":false,"schedule":{"enabled":true,"hours":[9,21]}}`
		case "/api/model-catalog":
			body = `{"models":[{"id":"glm-5.2","name":"GLM-5.2","efforts":["high"]}],"source":"none","summary":{"total":1},"realm":"cn"}`
		case "/api/playground/models":
			body = `{"models":[{"id":"glm-5.2","name":"GLM-5.2","efforts":["high"],"series":"glm"}],"source":"none","realm":"global"}`
		case "/api/status":
			body = `{"connected":false,"realm_totals":{"cn":{"total":0,"healthy":0}}}`
		default:
			body = `{}`
		}
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAdminCookie("wb_session=x"), WithRealm(RealmGlobal))
	ctx := context.Background()

	// 统计：realm 透传。
	sum, err := c.Stats().Summary(ctx, RealmGlobal)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if sum.TodayRequests != 5 || sum.TodayCredit != 1.25 || sum.UsageHealth == nil || !sum.UsageHealth.OK {
		t.Fatalf("Summary decode: %+v", sum)
	}
	if lastQuery(t, recs) != "realm=global" {
		t.Fatalf("summary query = %q", lastQuery(t, recs))
	}

	if _, err := c.Stats().Daily(ctx, 7, RealmCN); err != nil {
		t.Fatalf("Daily: %v", err)
	}
	if q := lastQuery(t, recs); q != "days=7&realm=cn" {
		t.Fatalf("daily query = %q", q)
	}

	// realm=Any（""）不应带 realm 参数。
	if _, err := c.Stats().ByKey(ctx, 30, RealmAny); err != nil {
		t.Fatalf("ByKey: %v", err)
	}
	if q := lastQuery(t, recs); strings.Contains(q, "realm") {
		t.Fatalf("by-key 不应带 realm: %q", q)
	}

	// 日志：分页 + stream 布尔解码。
	page, err := c.Logs().List(ctx, RequestLogQuery{Page: 2, Size: 50, Status: "error", Realm: RealmCN})
	if err != nil {
		t.Fatalf("Logs.List: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || !page.Items[0].Stream || page.Items[0].Realm != RealmCN {
		t.Fatalf("Logs decode: %+v", page)
	}
	if q := lastQuery(t, recs); !strings.Contains(q, "page=2") || !strings.Contains(q, "status=error") || !strings.Contains(q, "realm=cn") {
		t.Fatalf("logs query = %q", q)
	}

	// 安全：POST body 与解码。
	cfg, err := c.Security().SaveConfig(ctx, SecurityConfig{Enabled: true, Mode: "whitelist"})
	if err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if !cfg.Enabled || cfg.Mode != "whitelist" {
		t.Fatalf("SaveConfig decode: %+v", cfg)
	}
	if b := lastBody(t, recs); !strings.Contains(b, `"enabled":true`) || !strings.Contains(b, `"mode":"whitelist"`) {
		t.Fatalf("SaveConfig body = %q", b)
	}

	rule, err := c.Security().AddRule(ctx, "allow", "10.0.0.0/8", "office")
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	if rule.Kind != "allow" || rule.CIDR != "10.0.0.0/8" {
		t.Fatalf("AddRule decode: %+v", rule)
	}

	logs, err := c.Security().Logs(ctx, 100)
	if err != nil {
		t.Fatalf("Security.Logs: %v", err)
	}
	if len(logs) != 1 || !logs[0].Blocked || logs[0].Reason == nil || *logs[0].Reason != "invalid_key" {
		t.Fatalf("Security.Logs decode: %+v", logs)
	}

	// 用户：PATCH 只提交非 nil 字段。
	if _, err := c.Users().Update(ctx, "bob", UserPatch{Role: ptr("viewer")}); err != nil {
		t.Fatalf("Users.Update: %v", err)
	}
	if b := lastBody(t, recs); strings.Contains(b, "password") || !strings.Contains(b, `"role":"viewer"`) {
		t.Fatalf("Users.Update body = %q", b)
	}
	if m := lastMethod(t, recs); m != http.MethodPatch {
		t.Fatalf("Users.Update method = %q", m)
	}
	if p := lastPath(t, recs); p != "/api/users/bob" {
		t.Fatalf("Users.Update path = %q", p)
	}

	if _, err := c.Users().AuditLogs(ctx, 50, 0); err != nil {
		t.Fatalf("AuditLogs: %v", err)
	}

	// 设置：读取上游配置（段用 map 承载）与模型映射。
	uc, err := c.Settings().Upstream(ctx)
	if err != nil {
		t.Fatalf("Settings.Upstream: %v", err)
	}
	if uc.Available == nil || !*uc.Available || uc.Pool["max_concurrent"] != float64(4) {
		t.Fatalf("UpstreamConfig decode: %+v", uc)
	}
	mm, err := c.Settings().ModelMap(ctx)
	if err != nil {
		t.Fatalf("ModelMap: %v", err)
	}
	if mm["gpt-4o-mini"] != "glm-5.2" {
		t.Fatalf("ModelMap decode: %+v", mm)
	}
	rs, err := c.Settings().ReloadState(ctx)
	if err != nil {
		t.Fatalf("ReloadState: %v", err)
	}
	if !rs.Pending || rs.RestartCount != 3 || rs.LastAt != 1727000000.5 {
		t.Fatalf("ReloadState decode: %+v", rs)
	}

	// 系统：版本与更新日志。
	v, err := c.System().Versions(ctx)
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if v.Manager != "1.0.66" {
		t.Fatalf("Versions decode: %+v", v)
	}
	cl, err := c.System().Changelog(ctx)
	if err != nil {
		t.Fatalf("Changelog: %v", err)
	}
	if !cl.Available || len(cl.Versions) != 1 || cl.Versions[0].Sections[0].Title != "修复" {
		t.Fatalf("Changelog decode: %+v", cl)
	}

	// 任务：状态与定时配置。
	tr, err := c.Tasks().RunStatus(ctx)
	if err != nil {
		t.Fatalf("Tasks.RunStatus: %v", err)
	}
	if tr.Available || tr.Schedule.Enabled == nil || !*tr.Schedule.Enabled || len(tr.Schedule.Hours) != 2 {
		t.Fatalf("TaskRunStatus decode: %+v", tr)
	}

	// 模型目录：force + realm 透传。
	mc, err := c.Catalog().Get(ctx, RealmCN, true)
	if err != nil {
		t.Fatalf("Catalog.Get: %v", err)
	}
	if mc.Source != "none" || len(mc.Models) != 1 || mc.Summary.Total != 1 {
		t.Fatalf("ModelCatalog decode: %+v", mc)
	}
	if q := lastQuery(t, recs); !strings.Contains(q, "realm=cn") || !strings.Contains(q, "force=true") {
		t.Fatalf("catalog query = %q", q)
	}

	// 测试台模型（未指定 realm → 用客户端默认 global）。
	if _, err := c.Playground().Models(ctx, RealmAny); err != nil {
		t.Fatalf("Playground.Models: %v", err)
	}
	if q := lastQuery(t, recs); !strings.Contains(q, "realm=global") {
		t.Fatalf("playground query = %q", q)
	}

	// 上游状态。
	st, err := c.Upstream().Status(ctx)
	if err != nil {
		t.Fatalf("Upstream.Status: %v", err)
	}
	if st.Connected || st.RealmTotals["cn"].Total != 0 {
		t.Fatalf("UpstreamStatus decode: %+v", st)
	}
}

func lastQuery(t *testing.T, recs []recorded) string {
	t.Helper()
	if len(recs) == 0 {
		t.Fatal("no request recorded")
	}
	return recs[len(recs)-1].Query
}

func lastBody(t *testing.T, recs []recorded) string {
	t.Helper()
	if len(recs) == 0 {
		t.Fatal("no request recorded")
	}
	return recs[len(recs)-1].Body
}

func lastMethod(t *testing.T, recs []recorded) string {
	t.Helper()
	if len(recs) == 0 {
		t.Fatal("no request recorded")
	}
	return recs[len(recs)-1].Method
}

func lastPath(t *testing.T, recs []recorded) string {
	t.Helper()
	if len(recs) == 0 {
		t.Fatal("no request recorded")
	}
	return recs[len(recs)-1].Path
}
