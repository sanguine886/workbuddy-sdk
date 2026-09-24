package wbsdk

import (
	"context"
	"net/http"
)

// GatewayHealth 是网关存活探测（`GET /healthz`，**无需任何凭据**）。
//
// 服务端刻意只回布尔：不复述上游账号池的规模（那是未认证访问者不该知道的）。
type GatewayHealth struct {
	Service    string `json:"service"`
	UpstreamOK bool   `json:"upstream_ok"`
}

// Healthz 探测网关存活与上游连通性。**不需要任何凭据**——适合监控探针。
func (c *Client) Healthz(ctx context.Context) (*GatewayHealth, error) {
	var out GatewayHealth
	if _, err := c.doJSON(ctx, http.MethodGet, "/healthz", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AdminHealth 是管理端存活探测（`GET /api/healthz`，**无需任何凭据**）。
type AdminHealth struct {
	OK      bool   `json:"ok"`
	Service string `json:"service"`
}

// AdminHealthz 探测管理端存活。**不需要任何凭据**，比 `Sysinfo()`（需要登录）
// 更适合做监控探针。
func (c *Client) AdminHealthz(ctx context.Context) (*AdminHealth, error) {
	var out AdminHealth
	if _, err := c.doJSON(ctx, http.MethodGet, "/api/healthz", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
