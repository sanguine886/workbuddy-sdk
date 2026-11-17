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

// ── 按账号聚合（/api/stats/by-account*）──────────────────────

// AccountStatsQuery 是「按账号」聚合的筛选条件。
//
// Start / End 都是 YYYY-MM-DD，且**两端都含**；给了它们就忽略 Days。
type AccountStatsQuery struct {
	Days  int
	Realm Realm
	Start string
	End   string
	Limit int
}

// AccountUsagePoint 是一条「日期 × 账号」的用量。
type AccountUsagePoint struct {
	Day      string `json:"day"`
	Account  string `json:"account"`
	Nickname string `json:"nickname"`
	// UID8 是账号列 `昵称(uid8)` 里拆出来的短 uid（拆不出时为空串）。
	UID8     string  `json:"uid8"`
	Requests int     `json:"requests"`
	Tokens   int     `json:"tokens"`
	Credit   float64 `json:"credit"`
	// Cost 是费用（元）；nil = 这个模型没收录价目。
	Cost *float64 `json:"cost"`
}

// AccountUsageList 是 GET /api/stats/by-account 的返回。
type AccountUsageList struct {
	Items []AccountUsagePoint `json:"items"`
	Total int                 `json:"total"`
	// Truncated 表示行数超过 limit，被截断了。
	Truncated bool   `json:"truncated"`
	From      string `json:"from"`
	To        string `json:"to"`
}

// AccountModelUsage 是一条「账号 × 模型」的用量。
type AccountModelUsage struct {
	Account  string   `json:"account"`
	Nickname string   `json:"nickname"`
	UID8     string   `json:"uid8"`
	Model    string   `json:"model"`
	Requests int      `json:"requests"`
	Tokens   int      `json:"tokens"`
	Credit   float64  `json:"credit"`
	Cost     *float64 `json:"cost"`
}

// AccountModelUsageList 是 GET /api/stats/by-account-model 的返回。
type AccountModelUsageList struct {
	Items     []AccountModelUsage `json:"items"`
	Total     int                 `json:"total"`
	Truncated bool                `json:"truncated"`
	From      string              `json:"from"`
	To        string              `json:"to"`
}

func accountStatsQuery(q AccountStatsQuery) url.Values {
	vals := url.Values{}
	if q.Days > 0 {
		vals.Set("days", strconv.Itoa(q.Days))
	}
	if q.Realm != RealmAny {
		vals.Set("realm", string(q.Realm))
	}
	if q.Start != "" {
		vals.Set("start", q.Start)
	}
	if q.End != "" {
		vals.Set("end", q.End)
	}
	if q.Limit > 0 {
		vals.Set("limit", strconv.Itoa(q.Limit))
	}
	return vals
}

// ByAccount 按「日期 × 账号」聚合请求数、token 与积分。
//
// 数据源是请求日志：账号列是**异步回填**的，没回填的行不计入（宁可少一行，
// 也不把「不知道是谁」摊到某个号上）。
func (s *StatsService) ByAccount(ctx context.Context, q AccountStatsQuery) (*AccountUsageList, error) {
	var out AccountUsageList
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/stats/by-account",
		accountStatsQuery(q), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ByAccountModel 按「账号 × 模型」聚合（回答「哪个号在跑哪些模型、各花了多少」）。
func (s *StatsService) ByAccountModel(ctx context.Context, q AccountStatsQuery) (*AccountModelUsageList, error) {
	var out AccountModelUsageList
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/stats/by-account-model",
		accountStatsQuery(q), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
