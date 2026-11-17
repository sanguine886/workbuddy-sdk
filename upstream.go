package wbsdk

import (
	"context"
	"net/http"
	"net/url"
)

// UpstreamService 封装上游状态与模型简表接口（/api/upstream/status、/api/models）。
type UpstreamService struct{ c *Client }

// Upstream 返回上游服务。
func (c *Client) Upstream() *UpstreamService { return &UpstreamService{c: c} }

// Status 拉取上游 workbuddy2api 的运行时状态（账号池健康度等）。
// 上游不可达时返回 connected=false（不是错误）。
//
// 路径说明：v2.11 起这个接口从 /api/status 搬到了 /api/upstream/status ——
// 旧的 /api/status 让给了客户端余额协议的**公开元数据**端点（它必须无鉴权可达）。
func (s *UpstreamService) Status(ctx context.Context) (*UpstreamStatus, error) {
	return s.StatusOf(ctx, nil)
}

// StatusOf 拉取指定分组的上游运行时状态（upstreamID 为 nil = 默认分组）。
func (s *UpstreamService) StatusOf(ctx context.Context, upstreamID *int) (*UpstreamStatus, error) {
	var out UpstreamStatus
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/upstream/status",
		s.c.groupQ(upstreamID), nil, &out); err != nil {
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
