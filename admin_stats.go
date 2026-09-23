package wbsdk

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// StatsService 封装用量统计（/api/stats）。
type StatsService struct{ c *Client }

// Stats 返回统计服务。
func (c *Client) Stats() *StatsService { return &StatsService{c: c} }

func realmQuery(realm Realm) url.Values {
	q := url.Values{}
	if realm != RealmAny {
		q.Set("realm", string(realm))
	}
	return q
}

// Summary 拉取总览。
func (s *StatsService) Summary(ctx context.Context, realm Realm) (*StatsSummary, error) {
	var out StatsSummary
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/stats/summary", realmQuery(realm), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Daily 按天聚合（近 N 天，含今天）。
func (s *StatsService) Daily(ctx context.Context, days int, realm Realm) ([]UsagePoint, error) {
	q := realmQuery(realm)
	if days > 0 {
		q.Set("days", strconv.Itoa(days))
	}
	var out []UsagePoint
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/stats/daily", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ByModel 按模型聚合。
func (s *StatsService) ByModel(ctx context.Context, days int, realm Realm) ([]UsageBreakdown, error) {
	q := realmQuery(realm)
	if days > 0 {
		q.Set("days", strconv.Itoa(days))
	}
	var out []UsageBreakdown
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/stats/by-model", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ByKey 按密钥聚合。
func (s *StatsService) ByKey(ctx context.Context, days int, realm Realm) ([]UsageBreakdown, error) {
	q := realmQuery(realm)
	if days > 0 {
		q.Set("days", strconv.Itoa(days))
	}
	var out []UsageBreakdown
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/stats/by-key", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Upstream 拉取上游自己那份统计（口径与本端不同：含直连上游的调用）。
func (s *StatsService) Upstream(ctx context.Context) (*UpstreamStats, error) {
	var out UpstreamStats
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/stats/upstream", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RepairUsage 按请求日志回填用量统计的缺口（幂等）。返回原始结果字典。
func (s *StatsService) RepairUsage(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/stats/repair-usage", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RebuildUsage 以请求日志为准重建用量统计（会替换 usage_daily 内容）。
func (s *StatsService) RebuildUsage(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/stats/rebuild-usage", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
