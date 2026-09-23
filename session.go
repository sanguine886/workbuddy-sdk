package wbsdk

import (
	"context"
	"net/http"
)

// Me 返回当前登录用户（管理面）。
func (c *Client) Me(ctx context.Context) (*Me, error) {
	var out Me
	if _, err := c.doJSON(ctx, http.MethodGet, "/api/me", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RevokeResult 是吊销会话的返回。
type RevokeResult struct {
	OK              bool `json:"ok"`
	ReloginRequired bool `json:"relogin_required"`
}

// RevokeSessions 吊销**当前用户**的全部会话（含本机这次）。
//
// 服务端递增该用户的会话版本，其所有既有 cookie 立即失效。若当前客户端用的是
// PasswordAuth，下一次请求会自动重新登录；用静态 Cookie 的则需更换凭据。
func (c *Client) RevokeSessions(ctx context.Context) (*RevokeResult, error) {
	var out RevokeResult
	if _, err := c.doJSON(ctx, http.MethodPost, "/api/sessions/revoke", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Logout 登出当前会话（清除服务端下发的 cookie；对静态凭据无实质作用）。
func (c *Client) Logout(ctx context.Context) error {
	_, err := c.doJSON(ctx, http.MethodPost, "/api/logout", nil, nil, nil)
	return err
}
