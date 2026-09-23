package wbsdk

// 本文件覆盖日志 / 统计 / 安全 / 设置 / 用户 / 系统 / 模型 / 测试台 / 上游 /
// 任务等管理面类型，字段与官方前端 web/lib/types.ts 对齐。

// Page 是分页返回的通用形状。
type Page[T any] struct {
	Total int `json:"total"`
	Items []T `json:"items"`
}

// ── 请求日志 ────────────────────────────────────────────

// RequestLog 是一次网关调用日志。
type RequestLog struct {
	ID               int      `json:"id"`
	TS               int64    `json:"ts"`
	KeyID            *int     `json:"key_id"`
	KeyName          *string  `json:"key_name"`
	IP               string   `json:"ip"`
	Model            *string  `json:"model"`
	MappedModel      *string  `json:"mapped_model"`
	Status           int      `json:"status"`
	Realm            Realm    `json:"realm"`
	PromptTokens     int      `json:"prompt_tokens"`
	CompletionTokens int      `json:"completion_tokens"`
	LatencyMS        int      `json:"latency_ms"`
	FirstTokenMS     *int     `json:"first_token_ms"`
	CacheHitTokens   *int     `json:"cache_hit_tokens"`
	CacheMissTokens  *int     `json:"cache_miss_tokens"`
	CacheWriteTokens *int     `json:"cache_write_tokens"`
	UA               *string  `json:"ua"`
	Error            *string  `json:"error"`
	Stream           bool     `json:"stream"`
	Credit           *float64 `json:"credit"`
}

// ── 用量统计 ────────────────────────────────────────────

// UsagePoint 是按天聚合的一个数据点。
type UsagePoint struct {
	Day              string  `json:"day"`
	Requests         int     `json:"requests"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	Credit           float64 `json:"credit"`
	Failed           *int    `json:"failed"`
}

// UsageBreakdown 是按模型 / 按密钥聚合的一行。
type UsageBreakdown struct {
	Name             string  `json:"name"`
	Requests         int     `json:"requests"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	Credit           float64 `json:"credit"`
}

// UsageHealth 暴露「统计没有在累计」这种静默故障。
type UsageHealth struct {
	OK        bool   `json:"ok"`
	Detail    string `json:"detail"`
	LogsToday int    `json:"logs_today"`
}

// FailureStats 是失败请求数（4xx / 5xx 分档）。
type FailureStats struct {
	Today4xx int `json:"today_4xx"`
	Today5xx int `json:"today_5xx"`
	Week4xx  int `json:"week_4xx"`
	Week5xx  int `json:"week_5xx"`
}

// StatsSummary 是总览。
type StatsSummary struct {
	TodayRequests int           `json:"today_requests"`
	TodayTokens   int           `json:"today_tokens"`
	TodayCredit   float64       `json:"today_credit"`
	WeekCredit    float64       `json:"week_credit"`
	TotalCredit   float64       `json:"total_credit"`
	WeekRequests  int           `json:"week_requests"`
	WeekTokens    int           `json:"week_tokens"`
	TotalRequests int           `json:"total_requests"`
	TotalTokens   int           `json:"total_tokens"`
	ActiveKeys    int           `json:"active_keys"`
	TopModel      *string       `json:"top_model"`
	UsageHealth   *UsageHealth  `json:"usage_health"`
	Failures      *FailureStats `json:"failures"`
}

// ── 安全 / IP 管控 ──────────────────────────────────────

// SecurityConfig 是入站 IP 管控的全局开关与模式。
type SecurityConfig struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode"` // whitelist | blacklist
}

// IpRule 是一条 IP 规则。
type IpRule struct {
	ID        int    `json:"id"`
	Kind      string `json:"kind"` // allow | deny
	CIDR      string `json:"cidr"`
	Note      string `json:"note"`
	CreatedAt int64  `json:"created_at"`
}

// IpAccessLog 是一条入站访问审计记录。
type IpAccessLog struct {
	ID      int     `json:"id"`
	TS      int64   `json:"ts"`
	IP      string  `json:"ip"`
	Path    string  `json:"path"`
	Blocked bool    `json:"blocked"`
	UA      *string `json:"ua"`
	Reason  *string `json:"reason"` // 拦截原因短码；null = 未记录
}

// ── 用户 / 审计 ─────────────────────────────────────────

// UserItem 是管理端用户。
type UserItem struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

// UserPatch 是更新用户的请求体（仅提交非 nil 字段）。
type UserPatch struct {
	Password *string `json:"password,omitempty"`
	Role     *string `json:"role,omitempty"`
}

// UserUpdateResult 是更新用户的返回。
type UserUpdateResult struct {
	Username        string `json:"username"`
	Role            string `json:"role"`
	SessionsRevoked bool   `json:"sessions_revoked"`
	ReloginRequired bool   `json:"relogin_required"`
}

// AuditLog 是一条管理端审计记录。
type AuditLog struct {
	ID     int    `json:"id"`
	TS     int64  `json:"ts"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
	Target string `json:"target"`
	Detail string `json:"detail"`
	IP     string `json:"ip"`
}

// AuditLogPage 是审计日志分页返回。
type AuditLogPage struct {
	Items []AuditLog `json:"items"`
	Total int        `json:"total"`
}

// ── 上游配置 ────────────────────────────────────────────

// UpstashConfig 是 Upstash 连接信息（token 不回传）。
type UpstashConfig struct {
	URL         string `json:"url"`
	HasToken    bool   `json:"has_token"`
	TokenMasked string `json:"token_masked"`
}

// UpstreamConfig 是上游 workbuddy2api 的 config.json 视图（敏感字段已掩码）。
//
// 各配置段用 map 承载：它们的字段随上游版本演进，做成强类型只会不断漂移。
type UpstreamConfig struct {
	Available       *bool          `json:"available"`
	ReloadHint      string         `json:"reload_hint"`
	ReloadScheduled *bool          `json:"reload_scheduled"`
	ConfigPath      string         `json:"config_path"`
	Error           string         `json:"error"`
	Listen          string         `json:"listen"`
	APIKeyMasked    string         `json:"api_key_masked"`
	AuthDir         string         `json:"auth_dir"`
	UpstreamAuthDir string         `json:"upstream_auth_dir"`
	Schedule        map[string]any `json:"schedule"`
	Pool            map[string]any `json:"pool"`
	Cooldown        map[string]any `json:"cooldown"`
	Features        map[string]any `json:"features"`
	SessionSticky   map[string]any `json:"session_sticky"`
	Prompt          map[string]any `json:"prompt"`
	Server          map[string]any `json:"server"`
	Upstream        map[string]any `json:"upstream"`
	Global          map[string]any `json:"global"`
	Admin           map[string]any `json:"admin"`
	Upstash         *UpstashConfig `json:"upstash"`
}

// ReloadState 是上游重载状态。
type ReloadState struct {
	Running bool `json:"running"`
	Pending bool `json:"pending"`
	// LastAt 是上次重载完成的时刻。服务端取 time.time()，**可能是小数**，
	// 故用 float64（Unix 秒）。
	LastAt       float64 `json:"last_at"`
	LastOK       *bool   `json:"last_ok"`
	LastMessage  string  `json:"last_message"`
	RestartCount int     `json:"restart_count"`
}

// ── 系统更新 ────────────────────────────────────────────

// UpdateLogLine 是更新过程的一行日志。
type UpdateLogLine struct {
	TS    int64  `json:"ts"`
	Level string `json:"level"`
	Text  string `json:"text"`
}

// UpdateSignature 是发布包签名校验结果。
type UpdateSignature struct {
	Status string `json:"status"` // verified | skipped | none
	Detail string `json:"detail"`
}

// UpdateStatus 是更新进度与当前版本。
type UpdateStatus struct {
	Available         *bool            `json:"available"`
	InContainer       *bool            `json:"in_container"`
	CanUpdateUpstream *bool            `json:"can_update_upstream"`
	Running           bool             `json:"running"`
	OK                *bool            `json:"ok"`
	Step              string           `json:"step"`
	Logs              []UpdateLogLine  `json:"logs"`
	Version           string           `json:"version"`
	UpdaterFound      bool             `json:"updater_found"`
	UpstreamDir       string           `json:"upstream_dir"`
	UpstreamRef       string           `json:"upstream_ref"`
	StartedAt         *int64           `json:"started_at"`
	FinishedAt        *int64           `json:"finished_at"`
	Duration          *float64         `json:"duration"`
	PID               *int             `json:"pid"`
	LogTail           string           `json:"log_tail"`
	Signature         *UpdateSignature `json:"signature"`
}

// VersionSide 是单个组件的版本对比。
type VersionSide struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	HasUpdate bool   `json:"has_update"`
	Error     string `json:"error"`
}

// ManagerVersion 是管理端的版本对比。
type ManagerVersion struct {
	VersionSide
	URL  string `json:"url"`
	Repo string `json:"repo"`
}

// UpstreamChange 是上游两个版本之间的单条提交。
type UpstreamChange struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
	Date    string `json:"date"`
}

// UpstreamVersion 是上游的版本对比。
type UpstreamVersion struct {
	VersionSide
	Date      string           `json:"date"`
	Subject   string           `json:"subject"`
	Ahead     *int             `json:"ahead"`
	Total     *int             `json:"total"`
	Changes   []UpstreamChange `json:"changes"`
	Truncated *bool            `json:"truncated"`
	Repo      string           `json:"repo"`
}

// UpdateCheck 是检测更新的结果。
type UpdateCheck struct {
	CheckedAt int64           `json:"checked_at"`
	Cached    bool            `json:"cached"`
	Manager   ManagerVersion  `json:"manager"`
	Upstream  UpstreamVersion `json:"upstream"`
	HasAny    bool            `json:"has_any"`
}

// Versions 是当前部署版本信息。
type Versions struct {
	Manager           string `json:"manager"`
	UpstreamConnected bool   `json:"upstream_connected"`
	UpstreamAccounts  *int   `json:"upstream_accounts"`
	UpstreamDir       string `json:"upstream_dir"`
}

// ChangelogItem 是更新日志里的一条。
type ChangelogItem struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}

// ChangelogSection 是更新日志里的一个分类。
type ChangelogSection struct {
	Title string          `json:"title"`
	Items []ChangelogItem `json:"items"`
}

// ChangelogVersion 是一个版本的更新日志。
type ChangelogVersion struct {
	Version    string             `json:"version"`
	Date       string             `json:"date"`
	Unreleased bool               `json:"unreleased"`
	Sections   []ChangelogSection `json:"sections"`
}

// Changelog 是解析后的更新日志。
type Changelog struct {
	Available bool               `json:"available"`
	Error     string             `json:"error"`
	Path      string             `json:"path"`
	Total     *int               `json:"total"`
	Versions  []ChangelogVersion `json:"versions"`
	Truncated *bool              `json:"truncated"`
	Current   string             `json:"current"`
}

// ── 上游状态与模型 ──────────────────────────────────────

// PoolCounts 是上游为一组账号给出的计数。
type PoolCounts struct {
	Total        int `json:"total"`
	Healthy      int `json:"healthy"`
	Cooling      int `json:"cooling"`
	Disabled     int `json:"disabled"`
	InFlightFull int `json:"in_flight_full"`
}

// UpstreamStatus 是上游 workbuddy2api 的 /status。
type UpstreamStatus struct {
	Connected      bool                  `json:"connected"`
	Accounts       []map[string]any      `json:"accounts"`
	Cooling        *int                  `json:"cooling"`
	Disabled       *int                  `json:"disabled"`
	Healthy        *int                  `json:"healthy"`
	Total          *int                  `json:"total"`
	InFlightFull   *int                  `json:"in_flight_full"`
	RealmTotals    map[string]PoolCounts `json:"realm_totals"`
	RedisMode      string                `json:"redis_mode"`
	StickySessions *int                  `json:"sticky_sessions"`
	Error          string                `json:"error"`
}

// UpstreamModel 是上游模型简表里的一条。
type UpstreamModel struct {
	ID              string `json:"id"`
	OwnedBy         string `json:"owned_by"`
	ContextLength   *int   `json:"context_length"`
	ContextWindow   *int   `json:"context_window"`
	MaxOutputTokens *int   `json:"max_output_tokens"`
}

// UpstreamModelsResponse 是 GET /api/models 的返回。
type UpstreamModelsResponse struct {
	Models []UpstreamModel `json:"models"`
	Source string          `json:"source"` // dynamic | static | unknown
	Count  int             `json:"count"`
}

// ── 模型目录 ────────────────────────────────────────────

// CatalogModel 是模型目录里的一条（比上游简表多显示名与推理档位）。
type CatalogModel struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	ContextLength     int      `json:"context_length"`
	MaxOutputTokens   int      `json:"max_output_tokens"`
	Efforts           []string `json:"efforts"`
	Description       string   `json:"description"`
	Credits           string   `json:"credits"`
	Vendor            string   `json:"vendor"`
	Tags              []string `json:"tags"`
	IsDefault         bool     `json:"is_default"`
	SupportsReasoning bool     `json:"supports_reasoning"`
	SupportsToolCall  bool     `json:"supports_tool_call"`
	OnlyReasoning     bool     `json:"only_reasoning"`
	ReasoningSummary  string   `json:"reasoning_summary"`
	DefaultEffort     string   `json:"default_effort"`
	SupportsImages    bool     `json:"supports_images"`
	Series            string   `json:"series"`
}

// CatalogSummary 是模型目录的统计。
type CatalogSummary struct {
	Total        int      `json:"total"`
	Reasoning    int      `json:"reasoning"`
	LargeContext int      `json:"large_context"`
	MaxContext   int      `json:"max_context"`
	Series       []string `json:"series"`
	UniqueIDs    int      `json:"unique_ids"`
}

// ModelCatalog 是 GET /api/model-catalog 的返回。
type ModelCatalog struct {
	Models      []CatalogModel `json:"models"`
	Source      string         `json:"source"` // tencent | upstream | none
	SourceLabel string         `json:"source_label"`
	Via         string         `json:"via"`
	Errors      []string       `json:"errors"`
	Cached      bool           `json:"cached"`
	CacheAge    int64          `json:"cache_age"`
	Summary     CatalogSummary `json:"summary"`
	Realm       Realm          `json:"realm"`
}

// ── 聊天测试台 ──────────────────────────────────────────

// PlaygroundModel 是测试台的模型下拉项。
type PlaygroundModel struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Efforts []string `json:"efforts"`
	Series  string   `json:"series"`
}

// PlaygroundModels 是 GET /api/playground/models 的返回。
type PlaygroundModels struct {
	Models      []PlaygroundModel `json:"models"`
	Source      string            `json:"source"`
	SourceLabel string            `json:"source_label"`
	Realm       Realm             `json:"realm"`
}

// PlaygroundChatIn 是测试台对话请求。
type PlaygroundChatIn struct {
	Model           string        `json:"model"`
	Messages        []ChatMessage `json:"messages"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
	Stream          bool          `json:"stream"`
	Temperature     *float64      `json:"temperature,omitempty"`
	MaxTokens       *int          `json:"max_tokens,omitempty"`
	Realm           Realm         `json:"realm,omitempty"`
}

// ── 成长任务 ────────────────────────────────────────────

// TaskRunSchedule 是定时领奖配置。
type TaskRunSchedule struct {
	Enabled *bool `json:"enabled"`
	Hours   []int `json:"hours"`
}

// TaskRunStatus 是成长任务一键执行的状态。
type TaskRunStatus struct {
	Running           bool            `json:"running"`
	Mode              string          `json:"mode"`
	Target            string          `json:"target"`
	StartedAt         int64           `json:"started_at"`
	FinishedAt        int64           `json:"finished_at"`
	ExitCode          *int            `json:"exit_code"`
	TimedOut          bool            `json:"timed_out"`
	Error             string          `json:"error"`
	Lines             []string        `json:"lines"`
	Available         bool            `json:"available"`
	UnavailableReason string          `json:"unavailable_reason"`
	Schedule          TaskRunSchedule `json:"schedule"`
}

// ── 上游自带的统计 ──────────────────────────────────────

// UpstreamStatRow 是上游 /v1/stats 的一行（字段名照上游 JSON）。
type UpstreamStatRow struct {
	Model            *string  `json:"model"`
	Requests         *int     `json:"requests"`
	Success          *int     `json:"success"`
	Failed           *int     `json:"failed"`
	TotalTokens      *int     `json:"total_tokens"`
	PromptTokens     *int     `json:"prompt_tokens"`
	CompletionTokens *int     `json:"completion_tokens"`
	Credit           *float64 `json:"credit"`
	CacheHitRate     *float64 `json:"cache_hit_rate"`
}

// UpstreamStats 是上游自己那份统计。口径与本端不同（含直连上游的调用）。
type UpstreamStats struct {
	Available bool              `json:"available"`
	Error     string            `json:"error"`
	Enabled   *bool             `json:"enabled"`
	Message   string            `json:"message"`
	Since     string            `json:"since"`
	UptimeSec *int              `json:"uptime_sec"`
	Total     *UpstreamStatRow  `json:"total"`
	Models    []UpstreamStatRow `json:"models"`
}
