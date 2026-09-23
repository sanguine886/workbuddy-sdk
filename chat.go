package wbsdk

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// newSSEScanner 构造 SSE 行扫描器（单行上限 32 MiB）。
func newSSEScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxRespBytes)
	return sc
}

// ChatMessage 是一条对话消息。Content 可以是 string，也可以是内容分片数组。
type ChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content,omitempty"`
	Name    string `json:"name,omitempty"`
}

// ChatRequest 是 OpenAI Chat Completions 请求体。
//
// Extra 用于承载未建模的字段（如 tools、response_format 等），在序列化时合并
// 进顶层；已建模的字段优先，Extra 只补充缺失的键。
type ChatRequest struct {
	Model           string         `json:"model"`
	Messages        []ChatMessage  `json:"messages"`
	Stream          bool           `json:"stream,omitempty"`
	Temperature     *float64       `json:"temperature,omitempty"`
	TopP            *float64       `json:"top_p,omitempty"`
	MaxTokens       *int           `json:"max_tokens,omitempty"`
	ReasoningEffort string         `json:"reasoning_effort,omitempty"`
	StreamOptions   any            `json:"stream_options,omitempty"`
	Extra           map[string]any `json:"-"`
}

// MarshalJSON 合并 Extra 字段。
func (r ChatRequest) MarshalJSON() ([]byte, error) {
	type alias ChatRequest
	base, err := json.Marshal(alias(r))
	if err != nil {
		return nil, err
	}
	if len(r.Extra) == 0 {
		return base, nil
	}
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil {
		return nil, err
	}
	for k, v := range r.Extra {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	return json.Marshal(m)
}

// Usage 是响应里的用量（网关在末帧附带上游真实扣费 credit）。
type Usage struct {
	PromptTokens     int      `json:"prompt_tokens"`
	CompletionTokens int      `json:"completion_tokens"`
	TotalTokens      int      `json:"total_tokens"`
	Credit           *float64 `json:"credit"` // 上游真实扣费；null = 未返回
}

// ChatChoice 是一个候选回复。
type ChatChoice struct {
	Index        int          `json:"index"`
	Message      *ChatMessage `json:"message,omitempty"`
	Delta        *ChatMessage `json:"delta,omitempty"`
	FinishReason string       `json:"finish_reason"`
}

// ChatResponse 是非流式对话响应。
type ChatResponse struct {
	ID      string       `json:"id"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   *Usage       `json:"usage"`
	Raw     []byte       `json:"-"` // 原始响应体
}

// Chat 发起一次非流式对话。
func (c *Client) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req == nil {
		return nil, errors.New("wbsdk: ChatRequest 不能为 nil")
	}
	body := *req
	body.Stream = false
	payload, err := json.Marshal(&body)
	if err != nil {
		return nil, err
	}
	res, err := c.doRawAccept(ctx, http.MethodPost, "/v1/chat/completions", nil, payload, acceptJSON)
	if err != nil {
		return nil, err
	}
	if !res.OK() {
		return nil, res.Err()
	}
	var out ChatResponse
	if err := res.Decode(&out); err != nil {
		return nil, err
	}
	out.Raw = res.Body
	return &out, nil
}

// ChatStream 发起一次流式对话，逐条交付 SSE data: 载荷（raw 字节）。
// 载荷字节仅在回调执行期间有效。
func (c *Client) ChatStream(ctx context.Context, req *ChatRequest, fn func(raw []byte) error) error {
	if req == nil {
		return errors.New("wbsdk: ChatRequest 不能为 nil")
	}
	body := *req
	body.Stream = true
	payload, err := json.Marshal(&body)
	if err != nil {
		return err
	}
	return c.DoStream(ctx, http.MethodPost, "/v1/chat/completions", payload, fn)
}

// Responses 透传一次 OpenAI Responses 协议请求（原始 payload → 原始响应）。
func (c *Client) Responses(ctx context.Context, payload []byte) (*RawResponse, error) {
	return c.doRawAccept(ctx, http.MethodPost, "/v1/responses", nil, payload, acceptJSON)
}

// ResponsesStream 透传一次 Responses 流式请求。
func (c *Client) ResponsesStream(ctx context.Context, payload []byte, fn func(raw []byte) error) error {
	return c.DoStream(ctx, http.MethodPost, "/v1/responses", payload, fn)
}

// Messages 透传一次 Anthropic Messages 协议请求。
func (c *Client) Messages(ctx context.Context, payload []byte) (*RawResponse, error) {
	return c.doRawAccept(ctx, http.MethodPost, "/v1/messages", nil, payload, acceptJSON)
}

// MessagesStream 透传一次 Anthropic Messages 流式请求。
func (c *Client) MessagesStream(ctx context.Context, payload []byte, fn func(raw []byte) error) error {
	return c.DoStream(ctx, http.MethodPost, "/v1/messages", payload, fn)
}

// ModelInfo 是模型清单里的一条。
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// ModelList 是 GET /v1/models 的返回（OpenAI 形状，已按密钥版本裁剪）。
type ModelList struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

// Models 拉取当前密钥可见的模型列表。
func (c *Client) Models(ctx context.Context) (*ModelList, error) {
	res, err := c.doRawAccept(ctx, http.MethodGet, "/v1/models", nil, nil, acceptJSON)
	if err != nil {
		return nil, err
	}
	if !res.OK() {
		return nil, res.Err()
	}
	var out ModelList
	if err := res.Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
