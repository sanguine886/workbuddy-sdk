package wbsdk

import (
	"context"
	"net/http"
	"net/url"
)

// 余额端点：让客户端面板按统一单位显示密钥剩余额度。
//
// 三个端点分工固定：
//
//   - /api/status 是**公开元数据**，只声明数值单位（没有任何额度，无鉴权可达）；
//   - /api/usage/token/ 负责算数值（要网关密钥）；
//   - /v1/credits 是客户端默认的余额路径（同样用网关密钥）。
//
// 之所以三档口径（百分比折算后的积分 / 剩余积分 / 百万 token）统一折算成
// 「UNIT」：它们的量纲不同，没有任何真实货币或量词能同时表达。

// TokenQuota 是余额的数值部分。
type TokenQuota struct {
	Object string `json:"object"`
	// UnlimitedQuota 恒为 false：即便两份额度都不设上限，也要显示账号池的剩余积分。
	UnlimitedQuota bool    `json:"unlimited_quota"`
	TotalGranted   float64 `json:"total_granted"`
	TotalUsed      float64 `json:"total_used"`
	TotalAvailable float64 `json:"total_available"`
	// Scope 是口径：cn | global | merged。客户端不必理解，供面板与排障。
	Scope string `json:"scope"`
}

// TokenUsageResponse 是 GET /api/usage/token/ 的返回。
type TokenUsageResponse struct {
	Code bool       `json:"code"`
	Data TokenQuota `json:"data"`
}

// CreditsBalance 是 /v1/credits 的数值部分。
//
// 同一份数值给了两个键：RikkaHub 读 TotalUsage 当余额，new-api 读
// TotalAvailable——客户端不必改配置。
type CreditsBalance struct {
	Object         string  `json:"object"`
	TotalCredits   float64 `json:"total_credits"`
	TotalUsed      float64 `json:"total_used"`
	TotalAvailable float64 `json:"total_available"`
	TotalUsage     float64 `json:"total_usage"`
	Scope          string  `json:"scope"`
}

// CreditsResponse 是 GET /v1/credits 的返回。
type CreditsResponse struct {
	Code bool           `json:"code"`
	Data CreditsBalance `json:"data"`
}

// BalanceUnit 是公开的余额单位元数据。
type BalanceUnit struct {
	QuotaPerUnit               int     `json:"quota_per_unit"`
	QuotaDisplayType           string  `json:"quota_display_type"`
	CustomCurrencySymbol       string  `json:"custom_currency_symbol"`
	CustomCurrencyExchangeRate float64 `json:"custom_currency_exchange_rate"`
}

// BalanceStatus 是 GET /api/status 的返回（**公开**，无需凭据）。
type BalanceStatus struct {
	Success bool        `json:"success"`
	Data    BalanceUnit `json:"data"`
}

// BalanceScope 是余额报哪个池的口径。
//
// 服务端对非法值**静默回落到 auto**，不报错（调用方是客户端面板）。
type BalanceScope string

const (
	// ScopeAuto 让服务端决定：有上限的密钥报自己，否则按密钥 realm、
	// 再按最近一次实际调用的 realm，最后合并。
	ScopeAuto BalanceScope = ""
	// ScopeCN 只报国内版的池。
	ScopeCN BalanceScope = "cn"
	// ScopeGlobal 只报国际版的池。
	ScopeGlobal BalanceScope = "global"
	// ScopeMerged 两版合并成一个数。
	ScopeMerged BalanceScope = "merged"
)

func (s BalanceScope) query() url.Values {
	if s == ScopeAuto {
		return nil
	}
	return url.Values{"scope": []string{string(s)}}
}

// TokenUsage 查询**当前网关密钥**的剩余额度（GET /api/usage/token/）。
//
// 需要网关密钥（WithGatewayKey）。停用 / 过期 / 用尽都照常给数，不拦——
// 恰恰在调不动的时候最需要知道还剩多少。
//
// 这个调用**有副作用**：服务端会记一行请求日志（0 token）并刷新密钥的
// 「最近使用」与来源 IP。
func (c *Client) TokenUsage(ctx context.Context) (*TokenUsageResponse, error) {
	return c.TokenUsageOf(ctx, ScopeAuto)
}

// TokenUsageOf 同 TokenUsage，但显式指定报哪个池。
//
// 注意 `scope` 只在密钥**没有**额度上限时影响数字：设了上限的密钥回答的是
// 「我这把还能花多少」，服务端会忽略池口径直接报 merged——硬拆会得出
// 「上限 100、池里剩 5000」这种自相矛盾的读数。
func (c *Client) TokenUsageOf(ctx context.Context, scope BalanceScope) (*TokenUsageResponse, error) {
	q := scope.query()
	var out TokenUsageResponse
	if _, err := c.doJSON(ctx, http.MethodGet, "/api/usage/token/", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Credits 查询客户端默认余额（GET /v1/credits）。
//
// RikkaHub 之类客户端默认配置成 {baseUrl}/credits 并把 data.total_usage 当余额显示。
func (c *Client) Credits(ctx context.Context) (*CreditsResponse, error) {
	return c.CreditsOf(ctx, ScopeAuto)
}

// CreditsOf 同 Credits，但显式指定报哪个池。
func (c *Client) CreditsOf(ctx context.Context, scope BalanceScope) (*CreditsResponse, error) {
	q := scope.query()
	var out CreditsResponse
	if _, err := c.doJSON(ctx, http.MethodGet, "/v1/credits", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BalanceStatus 读取余额的单位元数据（GET /api/status）。
//
// 这个端点**公开无鉴权**：它只声明数值单位，不含任何额度。
func (c *Client) BalanceStatus(ctx context.Context) (*BalanceStatus, error) {
	var out BalanceStatus
	if _, err := c.doJSON(ctx, http.MethodGet, "/api/status", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
