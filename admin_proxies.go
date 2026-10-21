package wbsdk

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// ProxyService 封装出口线路表、账号绑定与全局出口代理：
// /api/proxy-routes*、/api/settings/proxy*、/api/proxies。
//
// 鉴权分两档：读账号页那份轻量列表（AccountRoutes）登录即可（管理面 API Token
// 也行）；其余全部是**会话管理员**专属 —— 用 wbt_ Token 调用会被 403
// 「该接口不接受 API Token」。多分组部署里线路的「绑定计数」按分组统计：带
// upstreamID 时只看那个分组，nil = 默认分组。
type ProxyService struct{ c *Client }

// Proxies 返回代理服务。
func (c *Client) Proxies() *ProxyService { return &ProxyService{c: c} }

func proxyRoutePath(name string) string {
	return "/api/proxy-routes/" + url.PathEscape(name)
}

// groupQuery 组装分组过滤参数（nil = 不带，即默认分组）。
func groupQuery(upstreamID *int) url.Values {
	q := url.Values{}
	if upstreamID != nil {
		q.Set("upstream_id", strconv.Itoa(*upstreamID))
	}
	return q
}

// Routes 读取线路列表（凭据已脱敏）+ 池配置一览。
func (s *ProxyService) Routes(ctx context.Context, upstreamID *int) (*ProxyRouteList, error) {
	var out ProxyRouteList
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/proxy-routes", groupQuery(upstreamID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateRoute 新建一条线路。名字已存在时服务端回 409。
func (s *ProxyService) CreateRoute(ctx context.Context, in ProxyRouteIn) (*ProxyRoute, error) {
	var out ProxyRoute
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/proxy-routes", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateRoute 修改线路：提交谁的字段就改谁。
//
// patch.NewName 非空时执行改名，绑着它的账号会一并改过去（返回里的
// BoundUpdated 是跟着改的账号数）。地址里的密码写 `***` 表示沿用已存的那份。
func (s *ProxyService) UpdateRoute(ctx context.Context, name string, patch ProxyRoutePatch, upstreamID *int) (*ProxyRoute, error) {
	var out ProxyRoute
	path := proxyRoutePath(name)
	if _, err := s.c.doJSON(ctx, http.MethodPut, path, groupQuery(upstreamID), patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteRoute 删除线路。force=false 时若仍有账号绑着，服务端回 409
// （删掉会让那些账号的出口悄悄换掉）。
func (s *ProxyService) DeleteRoute(ctx context.Context, name string, force bool, upstreamID *int) (*ProxyDeleteResult, error) {
	q := groupQuery(upstreamID)
	if force {
		q.Set("force", "true")
	}
	var out ProxyDeleteResult
	path := proxyRoutePath(name)
	if _, err := s.c.doJSON(ctx, http.MethodDelete, path, q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ImportRoutes 批量导入：每行 `名字=地址`，逐行报问题，能导的先导进去。
func (s *ProxyService) ImportRoutes(ctx context.Context, text string) (*ProxyImportResult, error) {
	var out ProxyImportResult
	body := map[string]string{"text": text}
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/proxy-routes/import", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProbeRoute 经指定线路发一次轻请求，回报耗时与出口 IP。
func (s *ProxyService) ProbeRoute(ctx context.Context, name string) (*ProxyProbe, error) {
	var out ProxyProbe
	body := map[string]string{"name": name}
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/proxy-routes/probe", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Bindings 读取「全部账号 → 各自绑定的线路」（空 = 跟随池策略）。
func (s *ProxyService) Bindings(ctx context.Context, upstreamID *int) (*ProxyBindings, error) {
	var out ProxyBindings
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/proxy-routes/bindings", groupQuery(upstreamID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BindAccounts 把一批账号绑定到线路；bound=false 表示解除绑定（改为跟随池策略）。
func (s *ProxyService) BindAccounts(ctx context.Context, name string, files []string, bound bool, upstreamID *int) (*ProxyBindResult, error) {
	var out ProxyBindResult
	body := map[string]any{"files": files, "bound": bound}
	path := proxyRoutePath(name) + "/accounts"
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, groupQuery(upstreamID), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AutoAssign 把 count 台账号自动绑到线路（没绑线路的优先，其次按昵称稳定排序）。
func (s *ProxyService) AutoAssign(ctx context.Context, name string, count int, includeBound bool, upstreamID *int) (*ProxyBindResult, error) {
	var out ProxyBindResult
	body := map[string]any{"count": count, "include_bound": includeBound}
	path := proxyRoutePath(name) + "/auto-assign"
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, groupQuery(upstreamID), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AccountRoutes 读取账号页选线路用的轻量列表（只有线路名与默认线路）。
func (s *ProxyService) AccountRoutes(ctx context.Context) (*ProxyAccountRoutes, error) {
	var out ProxyAccountRoutes
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/proxies", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SavePool 保存池配置（开关 / 策略 / 间隔）。
func (s *ProxyService) SavePool(ctx context.Context, patch ProxyPoolPatch) (*ProxyPool, error) {
	var out ProxyPool
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/settings/proxy-pool", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Settings 读取面板自身的出口代理 / 更新代理配置（已脱敏）+ 生效来源。
func (s *ProxyService) Settings(ctx context.Context) (*ProxySettings, error) {
	var out ProxySettings
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/settings/proxy", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveSettings 保存出口代理。地址里密码回显为 `***` 时服务端沿用已存的密码；
// 传空串表示清除（回落到环境变量）。
func (s *ProxyService) SaveSettings(ctx context.Context, patch ProxySettingsPatch) (*ProxySettings, error) {
	var out ProxySettings
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/settings/proxy", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProbeSettings 测当前生效的出口代理；没配时返回 Ok=false 并说明原因。
func (s *ProxyService) ProbeSettings(ctx context.Context) (*ProxyProbe, error) {
	var out ProxyProbe
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/settings/proxy/probe", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
