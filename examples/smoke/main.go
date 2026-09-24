// Command smoke 对一个真实运行的 workbuddy-manager 实例做端到端冒烟：
// 登录 → 取当前用户 → 拉账号 → 建/列/删密钥。
//
// 用法：
//
//	WB_BASE=http://127.0.0.1:7864 WB_USER=admin WB_PASS=... go run ./examples/smoke
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	wbsdk "github.com/sanguine886/workbuddy-sdk"
)

func main() {
	base := env("WB_BASE", "http://127.0.0.1:7864")
	user := env("WB_USER", "admin")
	pass := env("WB_PASS", "smoketest123")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin := wbsdk.NewClient(base, wbsdk.WithAdminLogin(user, pass))

	me, err := admin.Me(ctx)
	if err != nil {
		log.Fatalf("FAIL me: %v", err)
	}
	fmt.Printf("OK   me: %s (%s)\n", me.Username, me.Role)

	accounts, err := admin.Accounts().List(ctx)
	if err != nil {
		log.Fatalf("FAIL accounts: %v", err)
	}
	fmt.Printf("OK   accounts: total=%d upstream_available=%v\n", accounts.Total, deref(accounts.PoolAvailable))

	created, err := admin.Keys().Create(ctx, wbsdk.KeyIn{
		Name:  "smoke",
		Realm: wbsdk.RealmCN,
		Quota: 1000,
	})
	if err != nil {
		log.Fatalf("FAIL keys.create: %v", err)
	}
	if created.Key == "" {
		log.Fatal("FAIL keys.create: 未返回明文密钥")
	}
	fmt.Printf("OK   keys.create: id=%d prefix=%s realm=%s\n", created.ID, created.Prefix, created.Realm)

	keys, err := admin.Keys().List(ctx)
	if err != nil {
		log.Fatalf("FAIL keys.list: %v", err)
	}
	fmt.Printf("OK   keys.list: %d 把\n", len(keys))

	// 404 → NotFoundError 映射。
	if err := admin.Keys().Delete(ctx, 999999); err == nil {
		log.Fatal("FAIL keys.delete(404): 期望报错")
	} else if !errors.Is(err, wbsdk.ErrNotFound) {
		log.Fatalf("FAIL keys.delete(404): 期望 ErrNotFound，得到 %v", err)
	}
	fmt.Println("OK   keys.delete(不存在) → ErrNotFound")

	// 数据面：用新建的密钥调 /v1/models。无上游时应是 502 upstream_unavailable，
	// 但绝不能是 401（那说明密钥注入没生效）。
	gw := wbsdk.NewClient(base, wbsdk.WithGatewayKey(created.Key))
	if _, err := gw.Models(ctx); err != nil {
		var apiErr *wbsdk.APIError
		if !errors.As(err, &apiErr) {
			log.Fatalf("FAIL /v1/models: 非 APIError: %v", err)
		}
		if apiErr.StatusCode == 401 {
			log.Fatalf("FAIL /v1/models: 401（密钥注入未生效）: %v", err)
		}
		fmt.Printf("OK   /v1/models: 密钥鉴权通过，上游不可用返回 %d (%s)\n", apiErr.StatusCode, apiErr.Code)
	} else {
		fmt.Println("OK   /v1/models: 200")
	}

	if err := admin.Keys().Delete(ctx, created.ID); err != nil {
		log.Fatalf("FAIL keys.delete: %v", err)
	}
	fmt.Println("OK   keys.delete")

	// ── 其余管理面命名空间：逐个打一遍，确认契约与解码都通 ──
	summary, err := admin.Stats().Summary(ctx, wbsdk.RealmCN)
	if err != nil {
		log.Fatalf("FAIL stats.summary: %v", err)
	}
	fmt.Printf("OK   stats.summary: today_requests=%d active_keys=%d\n", summary.TodayRequests, summary.ActiveKeys)

	logs, err := admin.Logs().List(ctx, wbsdk.RequestLogQuery{Size: 5})
	if err != nil {
		log.Fatalf("FAIL logs.list: %v", err)
	}
	fmt.Printf("OK   logs.list: total=%d\n", logs.Total)

	sec, err := admin.Security().Config(ctx)
	if err != nil {
		log.Fatalf("FAIL security.config: %v", err)
	}
	fmt.Printf("OK   security.config: enabled=%v mode=%s\n", sec.Enabled, sec.Mode)

	rules, err := admin.Security().Rules(ctx)
	if err != nil {
		log.Fatalf("FAIL security.rules: %v", err)
	}
	fmt.Printf("OK   security.rules: %d 条\n", len(rules))

	if _, err := admin.Settings().Upstream(ctx); err != nil {
		log.Fatalf("FAIL settings.upstream: %v", err)
	}
	fmt.Println("OK   settings.upstream")

	rs, err := admin.Settings().ReloadState(ctx)
	if err != nil {
		log.Fatalf("FAIL settings.reload-state: %v", err)
	}
	fmt.Printf("OK   settings.reload-state: running=%v\n", rs.Running)

	users, err := admin.Users().List(ctx)
	if err != nil {
		log.Fatalf("FAIL users.list: %v", err)
	}
	fmt.Printf("OK   users.list: %d 个\n", len(users))

	al, err := admin.Users().AuditLogs(ctx, 10, 0)
	if err != nil {
		log.Fatalf("FAIL users.audit-logs: %v", err)
	}
	fmt.Printf("OK   users.audit-logs: total=%d\n", al.Total)

	versions, err := admin.System().Versions(ctx)
	if err != nil {
		log.Fatalf("FAIL system.versions: %v", err)
	}
	fmt.Printf("OK   system.versions: manager=%s\n", versions.Manager)

	if _, err := admin.System().UpdateStatus(ctx); err != nil {
		log.Fatalf("FAIL system.update-status: %v", err)
	}
	fmt.Println("OK   system.update-status")

	sched, err := admin.Tasks().ClaimSchedule(ctx)
	if err != nil {
		log.Fatalf("FAIL tasks.claim-schedule: %v", err)
	}
	fmt.Printf("OK   tasks.claim-schedule: hours=%v\n", sched.Hours)

	st, err := admin.Upstream().Status(ctx)
	if err != nil {
		log.Fatalf("FAIL upstream.status: %v", err)
	}
	fmt.Printf("OK   upstream.status: connected=%v\n", st.Connected)

	cat, err := admin.Catalog().Get(ctx, wbsdk.RealmCN, false)
	if err != nil {
		log.Fatalf("FAIL catalog.get: %v", err)
	}
	fmt.Printf("OK   catalog.get: source=%s models=%d\n", cat.Source, len(cat.Models))

	// ── 访问令牌（上游 v1.0.68+）：会话建令牌 → 令牌鉴权 → 权限边界 ──
	tokRO, err := admin.Tokens().Create(ctx, "smoke-ro", "readonly", nil)
	if err != nil {
		log.Fatalf("FAIL tokens.create(readonly): %v", err)
	}
	if !strings.HasPrefix(tokRO.Token, "wbt_") {
		log.Fatalf("FAIL tokens.create: 明文前缀不对: %q", tokRO.Token)
	}
	fmt.Printf("OK   tokens.create(readonly): id=%d prefix=%s\n", tokRO.ID, tokRO.Prefix)

	ro := wbsdk.NewClient(base, wbsdk.WithAdminToken(tokRO.Token))
	meRO, err := ro.Me(ctx)
	if err != nil {
		log.Fatalf("FAIL 令牌鉴权: %v", err)
	}
	if meRO.Role != "viewer" {
		log.Fatalf("FAIL readonly 令牌角色应为 viewer，得到 %q", meRO.Role)
	}
	fmt.Printf("OK   令牌鉴权: %s role=%s\n", meRO.Username, meRO.Role)

	if res, err := ro.Do(ctx, http.MethodPost, "/api/accounts/checkin-all", nil); err != nil {
		log.Fatalf("FAIL readonly 调写接口: %v", err)
	} else if res.StatusCode != http.StatusForbidden {
		log.Fatalf("FAIL readonly 调写接口应 403，得到 %d", res.StatusCode)
	}
	fmt.Println("OK   readonly 令牌调写接口被拒（403）")

	if res, err := ro.Do(ctx, http.MethodGet, "/api/tokens", nil); err != nil {
		log.Fatalf("FAIL readonly 调令牌管理: %v", err)
	} else if res.StatusCode != http.StatusForbidden {
		log.Fatalf("FAIL readonly 调令牌管理应 403，得到 %d", res.StatusCode)
	}
	fmt.Println("OK   readonly 令牌调令牌管理被拒（403）")

	tokADM, err := admin.Tokens().Create(ctx, "smoke-adm", "admin", nil)
	if err != nil {
		log.Fatalf("FAIL tokens.create(admin): %v", err)
	}
	adm := wbsdk.NewClient(base, wbsdk.WithAdminToken(tokADM.Token))
	meADM, err := adm.Me(ctx)
	if err != nil {
		log.Fatalf("FAIL admin 令牌鉴权: %v", err)
	}
	if meADM.Role != "admin" {
		log.Fatalf("FAIL admin 令牌角色应为 admin，得到 %q", meADM.Role)
	}
	fmt.Println("OK   admin 令牌鉴权（role=admin）")

	// admin 令牌可调**一般**写接口
	k, err := adm.Keys().Create(ctx, wbsdk.KeyIn{Name: "by-token"})
	if err != nil {
		log.Fatalf("FAIL admin 令牌建密钥: %v", err)
	}
	if err := adm.Keys().Delete(ctx, k.ID); err != nil {
		log.Fatalf("FAIL admin 令牌删密钥: %v", err)
	}
	fmt.Println("OK   admin 令牌可调一般写接口（建/删密钥）")

	// 但**会话专属**接口仍被拒（清日志）
	if res, err := adm.Do(ctx, http.MethodPost, "/api/logs/clear", nil); err != nil {
		log.Fatalf("FAIL admin 令牌调会话专属: %v", err)
	} else if res.StatusCode != http.StatusForbidden {
		log.Fatalf("FAIL 会话专属接口应 403，得到 %d", res.StatusCode)
	}
	fmt.Println("OK   admin 令牌调会话专属接口被拒（403）")

	// admin 令牌也不能管令牌
	if res, err := adm.Do(ctx, http.MethodGet, "/api/tokens", nil); err != nil {
		log.Fatalf("FAIL admin 令牌调令牌管理: %v", err)
	} else if res.StatusCode != http.StatusForbidden {
		log.Fatalf("FAIL admin 令牌调令牌管理应 403，得到 %d", res.StatusCode)
	}
	fmt.Println("OK   admin 令牌调令牌管理被拒（403）")

	// 删除后立即失效
	if err := admin.Tokens().Delete(ctx, tokRO.ID); err != nil {
		log.Fatalf("FAIL tokens.delete: %v", err)
	}
	if _, err := ro.Me(ctx); !errors.Is(err, wbsdk.ErrUnauthorized) {
		log.Fatalf("FAIL 删除后应 401，得到 %v", err)
	}
	fmt.Println("OK   令牌删除后立即失效（401）")

	if err := admin.Tokens().Delete(ctx, tokADM.ID); err != nil {
		log.Fatalf("FAIL tokens.delete(admin): %v", err)
	}

	// 会话自动重登：吊销当前用户全部会话（旧 cookie 立即失效），
	// 下一次请求应 401 → 自动重登 → 成功。
	if err := admin.DoJSON(ctx, http.MethodPost, "/api/sessions/revoke", nil, nil); err != nil {
		log.Fatalf("FAIL sessions.revoke: %v", err)
	}
	me2, err := admin.Me(ctx)
	if err != nil {
		log.Fatalf("FAIL me(重登后): %v", err)
	}
	fmt.Printf("OK   session 吊销后自动重登: %s\n", me2.Username)

	fmt.Println("SMOKE PASSED")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func deref(b *bool) any {
	if b == nil {
		return "?"
	}
	return *b
}
