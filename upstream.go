package wbsdk

import (
	"context"
	"net/http"
	"net/url"
)

// UpstreamService 封装上游状态与模型简表接口（/api/status、/api/models）。
type UpstreamService struct{ c *Client }

// Upstream 返回上游服务。
func (c *Client) Upstream() *UpstreamService { return &UpstreamService{c: c} }

// Status 拉取上游 workbuddy2api 的运行时状态（账号池健康度等）。
// 上游不可达时返回 connected=false（不是错误）。
func (s *UpstreamService) Status(ctx context.Context) (*UpstreamStatus, error) {
	var out UpstreamStatus
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/status", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Models 拉取上游可用模型简表；realm 非空时只返回该版本。
func (s *UpstreamService) Models(ctx context.Context, realm Realm) (*UpstreamModelsResponse, error) {
	q := url.Values{}
	if realm != RealmAny {
		q.Set("realm", string(realm))
	}
	var out UpstreamModelsResponse
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/models", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
