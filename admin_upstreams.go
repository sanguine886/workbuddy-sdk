package wbsdk

import (
	"context"
	"net/http"
	"strconv"
)

// UpstreamsService 封装「本端接了几个上游」的管理：地址、密钥、分组账号目录
// 与连通性探测（/api/upstreams*）。
//
// 与 Settings().Upstream() 不是一回事：那个读的是**上游 workbuddy2api 自己的
// config.json**；这里管的是本端接的上游清单 —— 密钥绑定到哪个上游，它的请求
// 就只走那个上游的账号池。
//
// 鉴权：列表登录即可；增删改要**会话管理员**（凭据级配置，不接受 API Token）；
// 探测要管理员。
type UpstreamsService struct{ c *Client }

// Upstreams 返回多上游管理服务。
func (c *Client) Upstreams() *UpstreamsService { return &UpstreamsService{c: c} }

func managedUpstreamPath(id int) string {
	return "/api/upstreams/" + strconv.Itoa(id)
}

// List 拉取全部上游：默认上游排在最前（ID 为 nil、不可编辑删除），其余按创建顺序。
func (s *UpstreamsService) List(ctx context.Context) (*UpstreamList, error) {
	var out UpstreamList
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/upstreams", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create 新建上游。返回里的 APIKeyMasked 是脱敏值（明文不回传）。
func (s *UpstreamsService) Create(ctx context.Context, in UpstreamIn) (*ManagedUpstream, error) {
	var out ManagedUpstream
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/upstreams", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update 局部更新：只改提交的字段。APIKey 留空表示不修改。
func (s *UpstreamsService) Update(ctx context.Context, id int, patch UpstreamPatch) (*ManagedUpstream, error) {
	var out ManagedUpstream
	if _, err := s.c.doJSON(ctx, http.MethodPatch, managedUpstreamPath(id), nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete 删除上游记录（不删账号目录）。分组里还有账号、或还有密钥绑着它时，
// 服务端回 409 并说明先处理什么。
func (s *UpstreamsService) Delete(ctx context.Context, id int) error {
	_, err := s.c.doJSON(ctx, http.MethodDelete, managedUpstreamPath(id), nil, nil, nil)
	return err
}

// Probe 探测上游是否可达（走它的 /healthz），失败原因原样返回。
func (s *UpstreamsService) Probe(ctx context.Context, id int) (*UpstreamProbe, error) {
	var out UpstreamProbe
	path := managedUpstreamPath(id) + "/probe"
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
