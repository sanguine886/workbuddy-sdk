package wbsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxRespBytes 单次响应体上限（防止异常响应把内存吃光）。
const maxRespBytes = 32 << 20 // 32 MiB

const acceptJSON = "application/json"
const acceptSSE = "text/event-stream"

// RawResponse 是未经解析的响应。
type RawResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// OK 报告状态码是否 < 400。
func (r *RawResponse) OK() bool { return r.StatusCode < 400 }

// Err 在非 2xx 时返回包装后的 *APIError，否则返回 nil。
func (r *RawResponse) Err() error {
	if r.OK() {
		return nil
	}
	return wrapStatus(newAPIError(r.StatusCode, r.Body))
}

// Decode 把响应体按 JSON 解码到 v（仅应在 OK 时调用）。
func (r *RawResponse) Decode(v any) error {
	if len(r.Body) == 0 {
		return nil
	}
	return json.Unmarshal(r.Body, v)
}

// Do 是通用逃生舱：对任意路径发原始请求，返回原始响应（不解析、不判错）。
//
// 用途：服务端新增接口时无需等待 SDK 更新即可调用。鉴权按 path 前缀自动选择
// （/api → 管理面 Cookie；/v1、/v2、/responses → 网关密钥）。非 2xx 不返回
// error——用 res.Err() 自行检查。
func (c *Client) Do(ctx context.Context, method, path string, body []byte) (*RawResponse, error) {
	return c.doRawAccept(ctx, method, path, nil, body, acceptJSON)
}

// DoJSON 是 Do 的结构化版本：in 序列化为 JSON 请求体，out 反序列化响应体；
// 非 2xx 返回包装后的 *APIError（可用 errors.As 回溯）。
func (c *Client) DoJSON(ctx context.Context, method, path string, in, out any) error {
	_, err := c.doJSON(ctx, method, path, nil, in, out)
	return err
}

// DoStream 对任意路径发起流式请求，逐条交付 SSE data: 载荷（raw 字节）。
//
// 载荷字节**引用内部扫描缓冲，仅在 fn 执行期间有效**；需跨回调保留请自行拷贝。
// fn 返回错误会立即终止读取并原样返回该错误。非 2xx 时返回包装后的 *APIError。
func (c *Client) DoStream(ctx context.Context, method, path string, payload []byte, fn func(raw []byte) error) error {
	resp, err := c.send(ctx, method, path, nil, payload, acceptSSE)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
		return wrapStatus(newAPIError(resp.StatusCode, raw))
	}
	return c.streamSSE(resp.Body, fn)
}

// streamSSE 按当前模式解析 SSE 流并交付载荷（模式见 WithSSEEventMode）。
func (c *Client) streamSSE(body io.Reader, fn func(raw []byte) error) error {
	if c.sseEventMode {
		return parseSSEEvents(body, fn)
	}
	sc := newSSEScanner(body)
	for sc.Scan() {
		payload, ok := extractSSEDataLine(sc.Bytes())
		if !ok {
			continue
		}
		if err := fn(payload); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("wbsdk: 读取事件流: %w", err)
	}
	return nil
}

func normalizePath(p string) string {
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

// hasPathPrefix 判断 path 是否落在某个**路径段前缀**下：
// "/api" 匹配 "/api" 与 "/api/xxx"，但**不匹配** "/apifoo"。
func hasPathPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func isAdminPath(path string) bool { return hasPathPrefix(path, "/api") }

func isGatewayPath(path string) bool {
	return hasPathPrefix(path, "/v1") ||
		hasPathPrefix(path, "/v2") ||
		hasPathPrefix(path, "/responses")
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body []byte, accept string) (*http.Request, error) {
	path = normalizePath(path)
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, rd)
	if err != nil {
		return nil, err
	}
	for k, vs := range c.headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	// 仅在调用方没有通过 WithHeader("User-Agent", ...) 指定时才用默认值，
	// 否则自定义 UA 会被这里的 Set 静默覆盖。
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if body != nil {
		req.Header.Set("Content-Type", acceptJSON)
	}
	if err := c.injectAuth(ctx, path, req.Header); err != nil {
		return nil, err
	}
	return req, nil
}

// injectAuth 按路径前缀注入鉴权头。
func (c *Client) injectAuth(ctx context.Context, path string, h http.Header) error {
	switch {
	case isAdminPath(path):
		if c.admin == nil {
			return errors.New("wbsdk: 该接口需要管理面鉴权，请用 WithAdminAuth / WithAdminCredentials / WithAdminCookie 配置")
		}
		cookie, err := c.admin.Cookie(ctx)
		if err != nil {
			return err
		}
		if cookie != "" {
			h.Set("Cookie", cookie)
		}
	case isGatewayPath(path):
		if c.key == nil {
			return errors.New("wbsdk: 该接口需要网关密钥，请用 WithGatewayKey 配置")
		}
		token, err := c.key.Bearer(ctx)
		if err != nil {
			return err
		}
		if token != "" {
			h.Set("Authorization", "Bearer "+strings.TrimPrefix(token, "Bearer "))
		}
	}
	return nil
}

// send 执行请求，并在 RetryPolicy 允许时对**幂等请求**做退避重试。
func (c *Client) send(ctx context.Context, method, path string, query url.Values, body []byte, accept string) (*http.Response, error) {
	if c.retry == nil {
		return c.doOnce(ctx, method, path, query, body, accept)
	}
	policy := c.retry.normalized()
	for attempt := 0; ; attempt++ {
		resp, err := c.doOnce(ctx, method, path, query, body, accept)
		if attempt >= policy.MaxRetries || !retryable(method, policy, resp, err) {
			return resp, err
		}
		delay := policy.backoff(attempt, resp)
		if resp != nil {
			drainAndClose(resp.Body)
		}
		if serr := sleepCtx(ctx, delay); serr != nil {
			return nil, serr
		}
	}
}

// retryable 判断这次结果是否值得重试。**只重试幂等方法**（GET / HEAD / OPTIONS），
// 写操作即使命中策略也绝不重试。
func retryable(method string, p RetryPolicy, resp *http.Response, err error) bool {
	if !idempotentMethod(method) {
		return false
	}
	if err != nil {
		return p.retryTransportErrors()
	}
	return resp != nil && p.statusRetryable(resp.StatusCode)
}

// doOnce 执行一次请求；管理面收到 401 时失效凭据并**重试一次**（自动重登）。
func (c *Client) doOnce(ctx context.Context, method, path string, query url.Values, body []byte, accept string) (*http.Response, error) {
	req, err := c.newRequest(ctx, method, path, query, body, accept)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized && isAdminPath(path) && c.admin != nil {
		drainAndClose(resp.Body)
		c.admin.Invalidate()
		req2, err2 := c.newRequest(ctx, method, path, query, body, accept)
		if err2 != nil {
			return nil, err2
		}
		return c.httpClient().Do(req2)
	}
	return resp, nil
}

func drainAndClose(rc io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(rc, 1<<16))
	_ = rc.Close()
}

func (c *Client) doRawAccept(ctx context.Context, method, path string, query url.Values, body []byte, accept string) (*RawResponse, error) {
	resp, err := c.send(ctx, method, path, query, body, accept)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return nil, fmt.Errorf("wbsdk: 读取响应失败: %w", err)
	}
	return &RawResponse{StatusCode: resp.StatusCode, Header: resp.Header, Body: raw}, nil
}

func (c *Client) doRaw(ctx context.Context, method, path string, query url.Values, body []byte) (*RawResponse, error) {
	return c.doRawAccept(ctx, method, path, query, body, acceptJSON)
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, in, out any) (*RawResponse, error) {
	var payload []byte
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("wbsdk: 编码请求体: %w", err)
		}
		payload = b
	}
	res, err := c.doRaw(ctx, method, path, query, payload)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return res, wrapStatus(newAPIError(res.StatusCode, res.Body))
	}
	if out != nil && len(res.Body) > 0 {
		if err := json.Unmarshal(res.Body, out); err != nil {
			return res, fmt.Errorf("wbsdk: 解析响应 (%s %s): %w", method, path, err)
		}
	}
	return res, nil
}
