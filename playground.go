package wbsdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// PlaygroundService 封装内置聊天测试台（/api/playground）。仅管理员可用。
//
// 测试台用管理端登录态直接试调上游，**不经过**密钥/IP/配额校验，只用于调试。
type PlaygroundService struct{ c *Client }

// Playground 返回测试台服务。
func (c *Client) Playground() *PlaygroundService { return &PlaygroundService{c: c} }

// Models 拉取测试台的模型下拉（含各自支持的推理档位）。
func (s *PlaygroundService) Models(ctx context.Context, realm Realm) (*PlaygroundModels, error) {
	q := url.Values{}
	q.Set("realm", string(s.c.effectiveRealm(realm)))
	var out PlaygroundModels
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/playground/models", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Chat 发起一次非流式测试对话，返回原始响应（OpenAI 形状）。
func (s *PlaygroundService) Chat(ctx context.Context, in PlaygroundChatIn) (*RawResponse, error) {
	in.Stream = false
	if in.Realm == RealmAny {
		in.Realm = s.c.effectiveRealm(in.Realm)
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	return s.c.doRawAccept(ctx, http.MethodPost, "/api/playground/chat", nil, payload, acceptJSON)
}

// ChatStream 发起一次流式测试对话，逐条交付 SSE data: 载荷（raw 字节）。
func (s *PlaygroundService) ChatStream(ctx context.Context, in PlaygroundChatIn, fn func(raw []byte) error) error {
	in.Stream = true
	if in.Realm == RealmAny {
		in.Realm = s.c.effectiveRealm(in.Realm)
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return s.c.DoStream(ctx, http.MethodPost, "/api/playground/chat", payload, fn)
}
