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

// Hourly 按小时聚合某一天（day 形如 "2026-09-23"，空 = 今天）。
//
// 「今日」趋势图用它：范围只有一天时按天聚合只会得到一根柱子。服务端**固定
// 返回 24 个桶**（没数据的补 0），与 Daily 同一套口径（含 Failed 的算法）。
func (s *StatsService) Hourly(ctx context.Context, day string, realm Realm) ([]UsageHourPoint, error) {
	q := realmQuery(realm)
	if day != "" {
		q.Set("day", day)
	}
	var out []UsageHourPoint
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/stats/hourly", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreditsFlow 拉取积分收支（近 N 天，含今天）。
//
// 收入与支出是**两套口径**（收入来自余额增加的流水，按查询时刻归日，是近似值；
// 支出来自每次调用的真实扣费），所以三个合计都给，别只拿 Net 下结论。
func (s *StatsService) CreditsFlow(ctx context.Context, days int, realm Realm) (*CreditsFlow, error) {
	q := realmQuery(realm)
	if days > 0 {
		q.Set("days", strconv.Itoa(days))
	}
	var out CreditsFlow
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/stats/credits", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
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
