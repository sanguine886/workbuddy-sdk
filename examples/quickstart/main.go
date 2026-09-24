// Command quickstart 演示 workbuddy-sdk 的基本用法。
//
// 运行前设置好 workbuddy-manager 地址与管理员凭据：
//
//	WB_BASE=https://wb.example.com WB_USER=admin WB_PASS=secret go run ./examples/quickstart
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	wbsdk "github.com/sanguine886/workbuddy-sdk"
)

func main() {
	base := env("WB_BASE", "http://127.0.0.1:7864")
	user := env("WB_USER", "admin")
	pass := env("WB_PASS", "admin123")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 管理面：用户名密码登录 + 401 自动重登。
	admin := wbsdk.NewClient(base,
		wbsdk.WithAdminLogin(user, pass),
	)
	me, err := admin.Me(ctx)
	if err != nil {
		log.Fatalf("登录失败: %v", err)
	}
	fmt.Printf("已登录: %s (%s)\n", me.Username, me.Role)

	accounts, err := admin.Accounts().List(ctx)
	if err != nil {
		log.Fatalf("拉取账号失败: %v", err)
	}
	fmt.Printf("账号总数: %d（上游可用: %v）\n", accounts.Total, deref(accounts.PoolAvailable))
	for _, a := range accounts.Accounts {
		fmt.Printf("  - %s  余额=%v  冷却=%v\n", a.Nickname, derefF(a.Credits), deref(a.Cooling))
	}

	// 建一把密钥（明文只在创建时返回一次）。
	key, err := admin.Keys().Create(ctx, wbsdk.KeyIn{
		Name:  "quickstart",
		Realm: wbsdk.RealmCN,
		Quota: 1_000_000,
	})
	if err != nil {
		log.Fatalf("创建密钥失败: %v", err)
	}
	fmt.Printf("新密钥: %s （请立即保存，仅此一次）\n", key.Key)

	// 数据面：用网关密钥调模型（原始 SSE 帧）。
	chat := wbsdk.NewClient(base, wbsdk.WithGatewayKey(key.Key))
	payload := []byte(`{"model":"glm-5.2","messages":[{"role":"user","content":"你好"}],"stream":true}`)
	err = chat.DoStream(ctx, http.MethodPost, "/v1/chat/completions", payload, func(raw []byte) error {
		if string(raw) == "[DONE]" {
			return nil
		}
		fmt.Printf("chunk: %s\n", raw)
		return nil
	})
	if err != nil {
		log.Fatalf("流式调用失败: %v", err)
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func deref(b *bool) any {
	if b == nil {
		return "未知"
	}
	return *b
}

func derefF(f *float64) any {
	if f == nil {
		return "—"
	}
	return *f
}
