package wbsdk_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"

	wbsdk "github.com/sanguine886/workbuddy-sdk"
)

// 构造客户端。NewClient 不做网络请求，只保存配置。
func ExampleNewClient() {
	c := wbsdk.NewClient("https://wb.example.com",
		wbsdk.WithGatewayKey("wbk_xxx"),
		wbsdk.WithRealm(wbsdk.RealmGlobal),
	)
	fmt.Println(c.BaseURL(), c.Realm())
	// Output: https://wb.example.com global
}

// 数据面：非流式对话。
func ExampleClient_Chat() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id": "1", "model": "glm-5.2",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "你好"}}],
			"usage": {"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5, "credit": 0.05}
		}`)
	}))
	defer srv.Close()

	c := wbsdk.NewClient(srv.URL, wbsdk.WithGatewayKey("wbk_xxx"))
	resp, err := c.Chat(context.Background(), &wbsdk.ChatRequest{
		Model:    "glm-5.2",
		Messages: []wbsdk.ChatMessage{{Role: "user", Content: "你好"}},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.Choices[0].Message.Content)
	fmt.Printf("扣费 %.2f\n", *resp.Usage.Credit)
	// Output:
	// 你好
	// 扣费 0.05
}

// 数据面：流式对话，逐条交付原始 SSE 载荷（事件语义由调用方决定）。
func ExampleClient_ChatStream() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"delta\":\"你\"}\n\n")
		_, _ = io.WriteString(w, "data: {\"delta\":\"好\"}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := wbsdk.NewClient(srv.URL, wbsdk.WithGatewayKey("wbk_xxx"))
	err := c.ChatStream(context.Background(), &wbsdk.ChatRequest{
		Model:    "glm-5.2",
		Messages: []wbsdk.ChatMessage{{Role: "user", Content: "你好"}},
	}, func(raw []byte) error {
		if string(raw) == "[DONE]" {
			fmt.Println("结束")
			return nil
		}
		fmt.Println(string(raw))
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	// Output:
	// {"delta":"你"}
	// {"delta":"好"}
	// 结束
}

// 事件模式：同一事件内的多条 data: 行按规范以换行拼接后整体交付。
func ExampleWithSSEEventMode() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, ": keep-alive\n\n")
		_, _ = io.WriteString(w, "data: 第一行\ndata: 第二行\n\n")
	}))
	defer srv.Close()

	c := wbsdk.NewClient(srv.URL, wbsdk.WithGatewayKey("wbk_xxx"), wbsdk.WithSSEEventMode(true))
	err := c.DoStream(context.Background(), http.MethodPost, "/v1/chat/completions", []byte(`{}`),
		func(raw []byte) error {
			fmt.Println(string(raw))
			return nil
		})
	if err != nil {
		log.Fatal(err)
	}
	// Output:
	// 第一行
	// 第二行
}

// 管理面：用户名密码登录（自动携带会话 Cookie），列出账号并读备注与积分。
func ExampleClient_Accounts() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			http.SetCookie(w, &http.Cookie{Name: "wb_session", Value: "s", Path: "/"})
			_, _ = io.WriteString(w, `{"ok":true}`)
		case "/api/accounts":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"total":2,"accounts":[
				{"file":"a.json","uid":"u1","nickname":"甲","realm":"cn","credits":120,"note":"主力","cooling":false},
				{"file":"b.json","uid":"u2","nickname":"乙","realm":"cn","credits":80,"note":"","cooling":true}
			]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := wbsdk.NewClient(srv.URL, wbsdk.WithAdminLogin("admin", "pw"))
	accounts, err := c.Accounts().List(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, a := range accounts.Accounts {
		fmt.Printf("%s 备注=%q 积分=%.0f 冷却=%v\n", a.Nickname, a.Note, *a.Credits, *a.Cooling)
	}
	// Output:
	// 甲 备注="主力" 积分=120 冷却=false
	// 乙 备注="" 积分=80 冷却=true
}

// 管理面：创建密钥（明文仅在创建时返回一次）。
func ExampleClient_Keys() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id": 7, "name": "ci", "prefix": "wbk_abcd", "enabled": true,
			"realm": "cn", "quota_credit": 50, "quota": 1000000,
			"used_tokens": 0, "used_credit": 0, "key": "wbk_secret_shown_once"
		}`)
	}))
	defer srv.Close()

	c := wbsdk.NewClient(srv.URL, wbsdk.WithAdminCookie("wb_session=x"))
	key, err := c.Keys().Create(context.Background(), wbsdk.KeyIn{
		Name:  "ci",
		Realm: wbsdk.RealmCN,
		Quota: 1_000_000,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(key.ID, key.Key)
	// Output: 7 wbk_secret_shown_once
}
