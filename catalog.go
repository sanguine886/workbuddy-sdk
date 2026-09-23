package wbsdk

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// CatalogService 封装模型中心（/api/model-catalog）。只读。
type CatalogService struct{ c *Client }

// Catalog 返回模型中心服务。
func (c *Client) Catalog() *CatalogService { return &CatalogService{c: c} }

// Get 拉取指定版本的模型目录。force=true 会绕过 5 分钟缓存直连腾讯——消耗账号池
// 额度且有风控风险，服务端仅在管理员请求时生效（普通用户仍拿缓存）。
func (s *CatalogService) Get(ctx context.Context, realm Realm, force bool) (*ModelCatalog, error) {
	q := url.Values{}
	q.Set("realm", string(s.c.effectiveRealm(realm)))
	q.Set("force", strconv.FormatBool(force))
	var out ModelCatalog
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/model-catalog", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
