package wbsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AdminAuth 为**管理面**（/api/*）提供会话凭据。
type AdminAuth interface {
	// Cookie 返回 Cookie 请求头的值，形如 "wb_session=<value>"。
	// 实现可在此惰性登录；并发调用应安全。
	Cookie(ctx context.Context) (string, error)

	// Invalidate 使当前凭据失效。下次 Cookie 前会重新获取。
	// 收到 401 时由传输层调用。
	Invalidate()
}

// GatewayKey 为**数据面**（/v1/*）提供网关密钥。
type GatewayKey interface {
	// Bearer 返回网关密钥（如 "wbk_xxx"）；可带或不带 "Bearer " 前缀。
	Bearer(ctx context.Context) (string, error)
}

// StaticSessionAuth 使用一段已经登录获得的会话 Cookie。
//
// 值须为完整的 Cookie 头，例如 "wb_session=eyJ...". 通常从浏览器开发者工具
// 复制，或用 NewPasswordAuth 自动登录后由 PasswordAuth 内部持有。
type StaticSessionAuth string

// Cookie 实现 AdminAuth。
func (s StaticSessionAuth) Cookie(context.Context) (string, error) { return string(s), nil }

// Invalidate 实现 AdminAuth（静态凭据无可失效）。
func (StaticSessionAuth) Invalidate() {}

// StaticKey 是一个固定不变的网关密钥。
type StaticKey string

// Bearer 实现 GatewayKey。
func (k StaticKey) Bearer(context.Context) (string, error) { return string(k), nil }

// PasswordAuth 用「用户名 + 密码」登录管理面并维护会话 Cookie。
//
// 它在需要时 POST {baseURL}/api/login，从 Set-Cookie 取回 wb_session 并缓存；
// 传输层在收到 401 后调用 Invalidate，下次请求即自动重登。
//
// 关于登录风控：服务端对 /api/login 按 IP + 用户名双维度计数，**5 次失败锁 10 分钟**。
// 因此本实现刻意保守——登录是单飞的（并发 401 只触发一次），且失败**不重试**，
// 直接把错误交给调用方。请勿在循环里反复调用导致误锁。
type PasswordAuth struct {
	baseURL  string
	username string
	password string
	hc       *http.Client

	mu     sync.Mutex
	cookie string
}

// AuthOption 配置 PasswordAuth。
type AuthOption func(*passwordAuthOptions)

type passwordAuthOptions struct {
	hc *http.Client
}

// WithLoginClient 指定登录所用的 *http.Client（默认一个 15s 超时的客户端）。
func WithLoginClient(hc *http.Client) AuthOption {
	return func(o *passwordAuthOptions) { o.hc = hc }
}

// NewPasswordAuth 构造基于用户名密码的鉴权。
//
// baseURL 为 workbuddy-manager 的根地址（与 NewClient 一致），用于拼接 /api/login。
// 服务部署在子路径时，baseURL 带上该前缀即可。
func NewPasswordAuth(baseURL, username, password string, opts ...AuthOption) *PasswordAuth {
	o := passwordAuthOptions{hc: &http.Client{Timeout: 15 * time.Second}}
	for _, fn := range opts {
		fn(&o)
	}
	return &PasswordAuth{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		hc:       o.hc,
	}
}

// Cookie 实现 AdminAuth。
func (a *PasswordAuth) Cookie(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cookie != "" {
		return a.cookie, nil
	}
	if err := a.loginLocked(ctx); err != nil {
		return "", err
	}
	return a.cookie, nil
}

// Invalidate 实现 AdminAuth：清空缓存，下次 Cookie 时重登。
func (a *PasswordAuth) Invalidate() {
	a.mu.Lock()
	a.cookie = ""
	a.mu.Unlock()
}

func (a *PasswordAuth) loginLocked(ctx context.Context) error {
	payload, err := json.Marshal(map[string]string{
		"username": a.username,
		"password": a.password,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.baseURL+"/api/login", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.hc.Do(req)
	if err != nil {
		return fmt.Errorf("wbsdk: 登录请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wbsdk: 登录失败: %w", newAPIError(resp.StatusCode, raw))
	}

	cookie := ""
	for _, ck := range resp.Cookies() {
		if ck.Name == sessionCookieName {
			cookie = ck.Name + "=" + ck.Value
			break
		}
	}
	if cookie == "" {
		// 回退：直接从 Set-Cookie 头里截取 name=value。
		if sc := resp.Header.Get("Set-Cookie"); sc != "" {
			if i := strings.IndexByte(sc, ';'); i >= 0 {
				sc = sc[:i]
			}
			cookie = strings.TrimSpace(sc)
		}
	}
	if cookie == "" {
		return errors.New("wbsdk: 登录成功但未收到会话 Cookie")
	}
	a.cookie = cookie
	return nil
}
