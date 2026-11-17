package wbsdk

import (
	"context"
	"net/http"
)

// SettingsService 封装上游配置、模型别名映射与重载状态（/api/settings/*、/api/upstream/*）。
type SettingsService struct{ c *Client }

// Settings 返回设置服务。
func (c *Client) Settings() *SettingsService { return &SettingsService{c: c} }

// Upstream 读取上游 config.json（敏感字段已掩码）。读不到时 Available=false 并带原因。
func (s *SettingsService) Upstream(ctx context.Context) (*UpstreamConfig, error) {
	var out UpstreamConfig
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/settings/upstream", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveUpstream 保存上游配置（仅允许改写非敏感段）。
//
// patch 里只放要改的段（如 {"schedule": {...}}），服务端不会用空值覆盖未提交的段。
// 返回体里的 ReloadHint 非空时，说明当前环境无法自动重载上游，需要人工重启。
func (s *SettingsService) SaveUpstream(ctx context.Context, patch map[string]any) (*UpstreamConfig, error) {
	var out UpstreamConfig
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/settings/upstream", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TestUpstash 探测 Upstash 连通性；token 留空表示使用已保存的值。
func (s *SettingsService) TestUpstash(ctx context.Context, url, token string) (*ActionResult, error) {
	body := map[string]any{"url": url}
	if token != "" {
		body["token"] = token
	}
	var out ActionResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/settings/upstash/test", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReloadState 读取上游重载状态（保存配置后自动重启）。
func (s *SettingsService) ReloadState(ctx context.Context) (*ReloadState, error) {
	var out ReloadState
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/upstream/reload-state", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReloadUpstream 立即重启上游（等待结果）。一般无需手动调用。
func (s *SettingsService) ReloadUpstream(ctx context.Context) (*ActionResult, error) {
	var out ActionResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/settings/upstash/reload", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ModelMap 读取模型别名映射（别名 → 真名）。
func (s *SettingsService) ModelMap(ctx context.Context) (map[string]string, error) {
	var out map[string]string
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/settings/model-map", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SaveModelMap 保存模型别名映射（空键 / 空值会被服务端丢弃）。
func (s *SettingsService) SaveModelMap(ctx context.Context, m map[string]string) (map[string]string, error) {
	var out map[string]string
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/settings/model-map", nil, m, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GitHubToken 读取 GitHub Token 的生效状态（**只回掩码**）。
//
// Source 为 "env" 时说明值由部署注入，面板填的那份不生效。
func (s *SettingsService) GitHubToken(ctx context.Context) (*GitHubToken, error) {
	var out GitHubToken
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/settings/github-token", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveGitHubToken 保存面板这份 GitHub Token（空串 = 清除）。不回明文。
func (s *SettingsService) SaveGitHubToken(ctx context.Context, token string) (*GitHubToken, error) {
	var out GitHubToken
	body := map[string]string{"token": token}
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/settings/github-token", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GitHubMirror 读取 GitHub 镜像基址与来源（国内机器直连 GitHub 超时时用）。
func (s *SettingsService) GitHubMirror(ctx context.Context) (*GitHubMirror, error) {
	var out GitHubMirror
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/settings/github-mirror", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveGitHubMirror 保存镜像基址（空串 = 清除，回落到环境变量）。
//
// 只接受「协议 + 主机 + 可选路径」：带查询串或 # 片段会被拒（拼出来的地址必然
// 是错误页）。
func (s *SettingsService) SaveGitHubMirror(ctx context.Context, mirror string) (*GitHubMirror, error) {
	var out GitHubMirror
	body := map[string]string{"mirror": mirror}
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/settings/github-mirror", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── 计费价目（/api/settings/pricing）─────────────────────────

// PricingBand 是一档价（峰值 / 非峰值）。
type PricingBand struct {
	Peak    *float64 `json:"peak,omitempty"`
	OffPeak *float64 `json:"off_peak,omitempty"`
}

// PricingModelTable 是一个模型的价目表。
//
// 服务端要求至少给 cache_miss 与 output 两档。
type PricingModelTable struct {
	CacheHit  *PricingBand `json:"cache_hit,omitempty"`
	CacheMiss *PricingBand `json:"cache_miss,omitempty"`
	Output    *PricingBand `json:"output,omitempty"`
}

// PricingVersion 是一档价目（随版本分发的那份，或管理员的覆盖）。
type PricingVersion struct {
	EffectiveFrom string                       `json:"effective_from"`
	VerifiedAt    string                       `json:"verified_at"`
	Source        string                       `json:"source"`
	SourceNote    string                       `json:"source_note"`
	HolidaySource string                       `json:"holiday_source"`
	Models        map[string]PricingModelTable `json:"models"`
	PeakWindows   []any                        `json:"peak_windows"`
	PeakDays      []any                        `json:"peak_days"`
	HolidayYears  []string                     `json:"holiday_years"`
	// Overridden 表示这一档来自管理员的覆盖，而不是随版本分发的那份。
	Overridden bool `json:"overridden"`
	// AgeDays 是这一档价目距上次核对过了多少天（nil = 没核对过）。
	AgeDays *int `json:"age_days"`
}

// Pricing 是价目表的描述。
type Pricing struct {
	Currency  string           `json:"currency"`
	MoneyUnit string           `json:"money_unit"`
	Versions  []PricingVersion `json:"versions"`
	// Active 是当前生效的那一档。
	Active map[string]any `json:"active"`
	// IsPeakNow 表示当前是否处于峰值时段。
	IsPeakNow bool `json:"is_peak_now"`
	// Override 是当前的覆盖内容（没有覆盖时为 nil）。
	Override    map[string]any `json:"override"`
	HasOverride bool           `json:"has_override"`
	// MissingHolidayYears 是节假日表里缺的年份。
	MissingHolidayYears []int `json:"missing_holiday_years"`
	// StaleAfterDays 是价目表超过多少天没核对就应当复核。
	StaleAfterDays int               `json:"stale_after_days"`
	Aliases        map[string]string `json:"aliases"`
	PricedModels   []string          `json:"priced_models"`
}

// PricingOverride 是价目覆盖（PUT /api/settings/pricing 的请求体）。
//
// 传 nil / 空结构 = 恢复成随版本分发的那份。
type PricingOverride struct {
	Versions     []PricingOverrideVersion `json:"versions,omitempty"`
	ModelAliases map[string]string        `json:"model_aliases,omitempty"`
	Currency     string                   `json:"currency,omitempty"`
}

// PricingOverrideVersion 是一档覆盖价目。
type PricingOverrideVersion struct {
	// EffectiveFrom 必须是 YYYY-MM-DD。
	EffectiveFrom string                       `json:"effective_from"`
	Models        map[string]PricingModelTable `json:"models"`
}

// Pricing 读取价目表 + 当前生效档 + 是否用了覆盖。
func (s *SettingsService) Pricing(ctx context.Context) (*Pricing, error) {
	var out Pricing
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/settings/pricing", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SavePricing 保存（或清除）价目覆盖。
//
// override 为 nil 时请求体是 null —— 服务端据此恢复成随版本分发的那份。
// 覆盖**只影响之后的调用**：已经落库的费用按当时的价目算好，不会被改写。
func (s *SettingsService) SavePricing(ctx context.Context, override *PricingOverride) (*Pricing, error) {
	var body any
	if override != nil {
		body = override
	}
	var out Pricing
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/settings/pricing", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── 网关 CORS（/api/settings/gateway-cors）────────────────────

// GatewayCORS 是**数据面**（/v1、/v2）的跨域设置。
//
// 与管理面 /api 的来源白名单是两套：数据面鉴权走 Bearer，可以允许 "*"。
type GatewayCORS struct {
	Enabled bool     `json:"enabled"`
	Origins []string `json:"origins"`
	// MaxAge 是浏览器侧的预检缓存秒数（上限 7200）。
	MaxAge int `json:"max_age"`
	// AdminOrigins 是管理面的来源白名单（**不可改**，只读展示）。
	AdminOrigins []string `json:"admin_origins"`
	// MaxOrigins 是最多能填多少条来源。
	MaxOrigins int `json:"max_origins"`
	// Prefixes 是这套设置生效的路径前缀。
	Prefixes []string `json:"prefixes"`
}

// GatewayCORSPatch 是网关 CORS 的局部更新。
type GatewayCORSPatch struct {
	Enabled *bool    `json:"enabled,omitempty"`
	Origins []string `json:"origins,omitempty"`
	MaxAge  *int     `json:"max_age,omitempty"`
}

// GatewayCORS 读取网关 CORS 设置。
func (s *SettingsService) GatewayCORS(ctx context.Context) (*GatewayCORS, error) {
	var out GatewayCORS
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/settings/gateway-cors", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveGatewayCORS 保存网关 CORS（改完即时生效，不需要重启）。
func (s *SettingsService) SaveGatewayCORS(ctx context.Context, patch GatewayCORSPatch) (*GatewayCORS, error) {
	var out GatewayCORS
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/settings/gateway-cors", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── 自动更新（/api/settings/auto-update）─────────────────────

// AutoUpdate 是自动更新配置 + 最近一次巡检的结果。
type AutoUpdate struct {
	Enabled bool `json:"enabled"`
	// IntervalHours 是检测间隔（小时）。
	IntervalHours int `json:"interval_hours"`
	// WindowStart / WindowEnd 是动手的时间窗（服务器本地时间，小时）。
	WindowStart int `json:"window_start"`
	WindowEnd   int `json:"window_end"`
	// Target 取值 auto（连上游一起更新）| manager（只更新管理端）。
	Target string          `json:"target"`
	State  AutoUpdateState `json:"state"`
	// DockerOK 表示当前环境能否操作 docker（不能时 target=auto 会退化成只更新管理端）。
	DockerOK bool `json:"docker_ok"`
	// UnsupportedReason 非空时说明本环境不支持一键更新。
	UnsupportedReason string `json:"unsupported_reason"`
	// ServerTZOffsetMinutes 是服务器本地时区相对 UTC 的偏移（分钟）。
	ServerTZOffsetMinutes int `json:"server_tz_offset_minutes"`
}

// AutoUpdateState 是最近一次巡检的结果。
type AutoUpdateState struct {
	LastCheckAt    int64  `json:"last_check_at"`
	LastAction     string `json:"last_action"`
	LastMessage    string `json:"last_message"`
	PendingVersion string `json:"pending_version"`
}

// AutoUpdatePatch 是自动更新配置的局部更新。
type AutoUpdatePatch struct {
	Enabled       *bool  `json:"enabled,omitempty"`
	IntervalHours *int   `json:"interval_hours,omitempty"`
	WindowStart   *int   `json:"window_start,omitempty"`
	WindowEnd     *int   `json:"window_end,omitempty"`
	Target        string `json:"target,omitempty"`
}

// AutoUpdate 读取自动更新配置与最近一次巡检结果。
func (s *SettingsService) AutoUpdate(ctx context.Context) (*AutoUpdate, error) {
	var out AutoUpdate
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/settings/auto-update", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveAutoUpdate 保存自动更新配置。
//
// 开启它等于授权面板在无人值守时自行下载并安装新版本、重启服务——因此服务端
// 要求**会话管理员**（API Token 不行）。
func (s *SettingsService) SaveAutoUpdate(ctx context.Context, patch AutoUpdatePatch) (*AutoUpdate, error) {
	var out AutoUpdate
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/settings/auto-update", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
