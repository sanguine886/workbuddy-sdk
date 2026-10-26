package wbsdk

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// 这一组覆盖 v2.8 ~ v2.10 新增的批量/计划/本机导入端点：只关心请求形状与解码。
func TestAdminBulkEndpointsContract(t *testing.T) {
	var mu sync.Mutex
	recs := map[string]recorded{}
	var uploadFiles int
	var uploadQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/accounts/export" {
			if got := r.Header.Get("Authorization"); got != "Bearer wbt_test" {
				t.Errorf("export 应带管理面 token，实际 %q", got)
			}
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", `attachment; filename="accounts.zip"`)
			_, _ = w.Write([]byte("PK\x03\x04zip"))
			return
		}
		if r.URL.Path == "/api/accounts/upload" {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("ParseMultipartForm: %v", err)
			} else {
				uploadFiles = len(r.MultipartForm.File["files"])
				uploadQuery = r.URL.RawQuery
			}
			_, _ = io.WriteString(w, `{"ok":false,"uploaded":[{"file":"a.json","uid":"u1","updated":false}],`+
				`"added":[{"file":"a.json","uid":"u1","updated":false}],"overwritten":[],`+
				`"rejected":[{"file":"b.json","uid":"u2","message":"账号已存在，未覆盖"}],`+
				`"failed":[{"file":"c.json","message":"文件格式无效"}]}`)
			return
		}
		if r.URL.Path == "/v1/messages/count_tokens" {
			if got := r.Header.Get("Authorization"); got != "Bearer wbk_test" {
				t.Errorf("count_tokens 应带网关密钥，实际 %q", got)
			}
		}

		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		recs[r.Method+" "+r.URL.Path] = recorded{r.Method, r.URL.Path, r.URL.RawQuery, string(b)}
		mu.Unlock()

		var body string
		switch r.URL.Path {
		case "/api/accounts/admin-api-status":
			body = `{"enabled":null}`
		case "/api/accounts/credits-snapshot":
			body = `{"snapshot_at":1727000000,"count":1,"known_count":1,"accounts":[` +
				`{"uid":"u1","credits":12.5,"registered_at":1727000000,"known":true}]}`
		case "/api/accounts/probe-models":
			body = `{"results":[{"file":"a.json","uid":"u1","nickname":"老王","realm":"cn",` +
				`"model":"glm-5.2","status":"no_model","code":11102,"message":"无此模型","latency_ms":120}],` +
				`"summary":{"ok":0,"rate":0,"credit":0,"auth":0,"no_model":1,"error":0},"requests":1}`
		case "/api/accounts/refresh-tokens":
			body = `{"total":1,"succeeded":1,"failed":[],` +
				`"results":[{"file":"a.json","nickname":"老王","uid":"u1","ok":true,"message":"ok"}],` +
				`"reload_triggered":true,"reload_error":""}`
		case "/api/accounts/refresh-plan":
			body = `{"enabled":true,"interval_days":5,"hour":3,"last_run":0,"next_run":1727200000}`
		case "/api/accounts/refresh-plan/run":
			body = `{"checked":3,"renewed":2,"failed":1,"skipped":0,"last_run":1727100000,"next_run":null}`
		case "/api/accounts/claim-compensation":
			body = `{"ok":true,"total":2,"claimed":1,"credit":5,"already":1,"skipped":0,` +
				`"nothing":1,"failed":0,"results":[{"nickname":"老王","status":"claimed",` +
				`"credit":5,"message":"领到 5 积分"}]}`
		case "/api/accounts/daily-plan":
			body = `{"checkin":{"enabled":true,"hour":9,"last_run":"2026-10-15-09"},` +
				`"compensation":{"enabled":false,"hour":10,"last_run":""},` +
				`"claim":{"enabled":false,"hour":10,"last_run":""},` +
				`"full":{"enabled":false,"hour":10,"last_run":""}}`
		case "/api/accounts/daily-plan/run":
			body = `{"ok":true,"message":"签到完成","task":"checkin","plan":{` +
				`"checkin":{"enabled":true,"hour":9,"last_run":"2026-10-20-09"},` +
				`"compensation":{"enabled":false,"hour":10,"last_run":""},` +
				`"claim":{"enabled":false,"hour":10,"last_run":""},` +
				`"full":{"enabled":false,"hour":10,"last_run":""}}}`
		case "/api/keys/export":
			body = `{"client":"ccswitch","base_url":"https://wb.example.com/v1","realm":"cn",` +
				`"models":["glm-5.2"],"name":"WorkBuddy ci","app_type":"claude",` +
				`"settings_config":{"env":{"ANTHROPIC_BASE_URL":"https://wb.example.com"}}}`
		case "/api/keys/import-local/status":
			body = `{"enabled":true,"local_caller":false,"clients":[]}`
		case "/api/keys/import-local/detect":
			body = `{"client":"ccswitch","found":true,"exe":"C:\\cc\\cc-switch.exe","source":"registry"}`
		case "/api/keys/import-local":
			body = `{"client":"ccswitch","app":"claude","realm":"cn",` +
				`"base_url":"https://wb.example.com/v1","model_count":1,"method":"deeplink",` +
				`"lifecycle":null,"action":"handed-off","provider_id":"","app_type":"claude",` +
				`"name":"WorkBuddy ci","target":"","backup":"","handler":"cc-switch.exe %1"}`
		case "/v1/messages/count_tokens":
			body = `{"input_tokens":42}`
		case "/api/system/update":
			body = `{"ok":true,"message":"已开始更新"}`
		default:
			body = `{}`
		}
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	ctx := context.Background()
	admin := NewClient(srv.URL, WithAdminToken("wbt_test"))
	gateway := NewClient(srv.URL, WithGatewayKey("wbk_test"))

	// ── 账号批量 ─────────────────────────────────────
	zip, err := admin.Accounts().Export(ctx, ptr(3))
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !bytes.HasPrefix(zip.Body, []byte("PK")) {
		t.Fatalf("export 应返回 zip 字节，得到 %q", zip.Body)
	}
	if cd := zip.Header.Get("Content-Disposition"); !strings.Contains(cd, "accounts.zip") {
		t.Fatalf("unexpected Content-Disposition: %q", cd)
	}

	up, err := admin.Accounts().Upload(ctx, []AccountUploadFile{
		{Filename: "a.json", Reader: strings.NewReader(`{"account":{"uid":"u1"}}`)},
		{Filename: "b.json", Reader: strings.NewReader(`{"account":{"uid":"u2"}}`)},
	}, ptr(3), true)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if uploadFiles != 2 {
		t.Fatalf("multipart 字段 files 应有 2 个文件，得到 %d", uploadFiles)
	}
	if !strings.Contains(uploadQuery, "overwrite=true") || !strings.Contains(uploadQuery, "upstream_id=3") {
		t.Fatalf("unexpected upload query: %q", uploadQuery)
	}
	if up.OK || len(up.Added) != 1 || len(up.Rejected) != 1 || len(up.Failed) != 1 {
		t.Fatalf("unexpected upload result: %+v", up)
	}
	if up.Added[0].Updated == nil || *up.Added[0].Updated {
		t.Fatalf("updated=false 未解码: %+v", up.Added[0].Updated)
	}

	status, err := admin.Accounts().AdminAPIStatus(ctx, ptr(3))
	if err != nil {
		t.Fatalf("AdminAPIStatus: %v", err)
	}
	if status.Enabled != nil {
		t.Fatalf("enabled=null 应解成 nil，得到 %+v", *status.Enabled)
	}

	snap, err := admin.Accounts().CreditsSnapshot(ctx)
	if err != nil {
		t.Fatalf("CreditsSnapshot: %v", err)
	}
	if snap.KnownCount != 1 || snap.Accounts[0].Credits != 12.5 || !snap.Accounts[0].Known {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}

	probe, err := admin.Accounts().ProbeModels(ctx, ProbeModelsIn{Accounts: []string{"a.json"}, Models: []string{"glm-5.2"}}, ptr(3))
	if err != nil {
		t.Fatalf("ProbeModels: %v", err)
	}
	if probe.Requests != 1 || probe.Summary.NoModel != 1 || probe.Results[0].Code == nil || *probe.Results[0].Code != 11102 {
		t.Fatalf("unexpected probe: %+v", probe)
	}
	rec := recs["POST /api/accounts/probe-models"]
	if !strings.Contains(rec.Query, "upstream_id=3") {
		t.Fatalf("probe 应带 upstream_id，得到 %q", rec.Query)
	}
	if !strings.Contains(rec.Body, `"accounts":["a.json"]`) || !strings.Contains(rec.Body, `"models":["glm-5.2"]`) {
		t.Fatalf("unexpected probe body: %s", rec.Body)
	}

	rt, err := admin.Accounts().RefreshTokens(ctx, nil)
	if err != nil {
		t.Fatalf("RefreshTokens: %v", err)
	}
	if rt.Succeeded != 1 || !rt.ReloadTriggered || rt.Results[0].Nickname != "老王" {
		t.Fatalf("unexpected refresh tokens: %+v", rt)
	}

	plan, err := admin.Accounts().RefreshPlan(ctx)
	if err != nil {
		t.Fatalf("RefreshPlan: %v", err)
	}
	if !plan.Enabled || plan.IntervalDays != 5 || plan.NextRun == nil || *plan.NextRun != 1727200000 {
		t.Fatalf("unexpected refresh plan: %+v", plan)
	}

	if _, err := admin.Accounts().SaveRefreshPlan(ctx, RefreshPlanIn{Enabled: false, IntervalDays: 7, Hour: 4}); err != nil {
		t.Fatalf("SaveRefreshPlan: %v", err)
	}
	saved := recs["PUT /api/accounts/refresh-plan"]
	if !strings.Contains(saved.Body, `"enabled":false`) || !strings.Contains(saved.Body, `"interval_days":7`) ||
		!strings.Contains(saved.Body, `"hour":4`) {
		t.Fatalf("unexpected plan body: %s", saved.Body)
	}

	ran, err := admin.Accounts().RunRefreshPlan(ctx)
	if err != nil {
		t.Fatalf("RunRefreshPlan: %v", err)
	}
	if ran.Checked != 3 || ran.Renewed != 2 || ran.Failed != 1 || ran.NextRun != nil {
		t.Fatalf("unexpected refresh run: %+v", ran)
	}

	comp, err := admin.Accounts().ClaimCompensation(ctx, ptr(3))
	if err != nil {
		t.Fatalf("ClaimCompensation: %v", err)
	}
	if comp.Total != 2 || comp.Claimed != 1 || comp.Credit != 5 || comp.Already != 1 {
		t.Fatalf("unexpected compensation: %+v", comp)
	}

	daily, err := admin.Accounts().DailyPlan(ctx)
	if err != nil {
		t.Fatalf("DailyPlan: %v", err)
	}
	if !daily.Checkin.Enabled || daily.Checkin.LastRun != "2026-10-15-09" || daily.Full.Enabled {
		t.Fatalf("unexpected daily plan: %+v", daily)
	}

	if _, err := admin.Accounts().SaveDailyPlan(ctx, DailyPlanIn{
		Checkin: DailyTaskIn{Enabled: true, Hour: 9},
		Full:    DailyTaskIn{Enabled: false, Hour: 11},
	}); err != nil {
		t.Fatalf("SaveDailyPlan: %v", err)
	}
	dsaved := recs["PUT /api/accounts/daily-plan"]
	// 四类任务必须都在：服务端对缺项会 400，SDK 不能靠 omitempty 省掉。
	for _, key := range []string{`"checkin"`, `"compensation"`, `"claim"`, `"full"`} {
		if !strings.Contains(dsaved.Body, key) {
			t.Fatalf("daily-plan body 缺 %s: %s", key, dsaved.Body)
		}
	}

	taskRun, err := admin.Accounts().RunDailyTask(ctx, "checkin")
	if err != nil {
		t.Fatalf("RunDailyTask: %v", err)
	}
	if !taskRun.OK || taskRun.Task != "checkin" || taskRun.Plan.Checkin.LastRun != "2026-10-20-09" {
		t.Fatalf("unexpected daily run: %+v", taskRun)
	}
	if got := recs["POST /api/accounts/daily-plan/run"].Body; got != `{"task":"checkin"}` {
		t.Fatalf("unexpected run body: %s", got)
	}

	// ── 密钥导出 / 本机导入 ───────────────────────────
	exp, err := admin.Keys().Export(ctx, KeyExportIn{Client: LocalClientCCSwitch, Token: "wbk_plain", App: "claude"})
	if err != nil {
		t.Fatalf("Keys().Export: %v", err)
	}
	if exp.AppType != "claude" || exp.SettingsConfig == nil {
		t.Fatalf("unexpected key export: %+v", exp)
	}

	st, err := admin.Keys().LocalImportStatus(ctx)
	if err != nil {
		t.Fatalf("LocalImportStatus: %v", err)
	}
	if !st.Enabled || st.LocalCaller {
		t.Fatalf("unexpected local status: %+v", st)
	}

	det, err := admin.Keys().DetectLocalClient(ctx, "", false)
	if err != nil {
		t.Fatalf("DetectLocalClient: %v", err)
	}
	if !det.Found || det.Exe == nil || *det.Exe != `C:\cc\cc-switch.exe` {
		t.Fatalf("unexpected detect: %+v", det)
	}
	dq := recs["GET /api/keys/import-local/detect"].Query
	if !strings.Contains(dq, "client=ccswitch") || !strings.Contains(dq, "deep=false") {
		t.Fatalf("unexpected detect query: %q", dq)
	}

	imp, err := admin.Keys().ImportLocal(ctx, LocalImportIn{
		KeyExportIn:  KeyExportIn{Client: LocalClientCCSwitch, Token: "wbk_plain", App: "claude"},
		SetCurrent:   true,
		Mode:         LocalImportDirect,
		CloseRunning: true,
	})
	if err != nil {
		t.Fatalf("ImportLocal: %v", err)
	}
	if imp.Method != "deeplink" || imp.Action != "handed-off" || imp.Lifecycle != nil {
		t.Fatalf("unexpected import: %+v", imp)
	}
	ib := recs["POST /api/keys/import-local"].Body
	if !strings.Contains(ib, `"token":"wbk_plain"`) || !strings.Contains(ib, `"set_current":true`) ||
		!strings.Contains(ib, `"mode":"direct"`) || !strings.Contains(ib, `"close_running":true`) {
		t.Fatalf("unexpected import body: %s", ib)
	}

	// ── token 估算 ──────────────────────────────────
	res, err := gateway.MessagesCountTokens(ctx, []byte(`{"messages":[{"role":"user","content":"你好"}]}`))
	if err != nil {
		t.Fatalf("MessagesCountTokens: %v", err)
	}
	if !res.OK() {
		t.Fatalf("count_tokens 失败: %s", res.Body)
	}
	ct, err := gateway.CountTokens(ctx, CountTokensRequest{
		Model:    "glm-5.2",
		System:   "你是助手",
		Messages: []any{map[string]any{"role": "user", "content": "你好"}},
		Extra:    map[string]any{"thinking": map[string]any{"type": "enabled"}},
	})
	if err != nil {
		t.Fatalf("CountTokens: %v", err)
	}
	if ct.InputTokens != 42 {
		t.Fatalf("unexpected input_tokens: %+v", ct)
	}
	cb := recs["POST /v1/messages/count_tokens"].Body
	if !strings.Contains(cb, `"thinking"`) || !strings.Contains(cb, `"system":"你是助手"`) {
		t.Fatalf("count_tokens body 未合并 Extra/System: %s", cb)
	}

	// ── 离线升级包来源 ───────────────────────────────
	if _, err := admin.System().UpdateFrom(ctx, UpdateIn{Target: UpdateTargetBoth, Source: UpdateSourceUploaded}); err != nil {
		t.Fatalf("UpdateFrom: %v", err)
	}
	if got := recs["POST /api/system/update"].Body; !strings.Contains(got, `"source":"uploaded"`) {
		t.Fatalf("update body 缺 source: %s", got)
	}
	if _, err := admin.System().Update(ctx, UpdateTargetManager); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := recs["POST /api/system/update"].Body; strings.Contains(got, `"source"`) {
		t.Fatalf("Update 不该带 source（omitempty 失效）: %s", got)
	}
}
