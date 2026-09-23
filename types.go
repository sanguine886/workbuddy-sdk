package wbsdk

// 本文件的管理面类型字段与官方前端 web/lib/types.ts 对齐。
// 说明：可空字段用指针以区分「缺失 / null」与零值；服务端新增字段会被 Go
// 的 encoding/json 自动忽略，因此对上游小版本演进具备一定容忍度。

// Me 是当前登录用户。
type Me struct {
	Username string `json:"username"`
	Role     string `json:"role"` // admin | viewer
}

// RateLimitedModel 是被限流的单个模型条目（上游限额台账）。
type RateLimitedModel struct {
	Model   string `json:"model"`
	Until   string `json:"until,omitempty"`
	ResetAt string `json:"reset_at,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// Account 是一个账号在面板上的完整视图：本地授权信息 + 上游运行时状态。
type Account struct {
	File           string   `json:"file"`
	UID            string   `json:"uid"`
	Nickname       string   `json:"nickname"`
	EnterpriseID   string   `json:"enterprise_id"`
	ExpiresAt      int64    `json:"expires_at"`
	IsExpired      bool     `json:"is_expired"`
	RemainSeconds  int64    `json:"remain_seconds"`
	TTLSeconds     *int64   `json:"ttl_seconds"`
	IssuedAt       *int64   `json:"issued_at"`
	Credits        *float64 `json:"credits"`
	Healthy        *bool    `json:"healthy"`
	Disabled       *bool    `json:"disabled"`
	DisabledReason *string  `json:"disabled_reason"`
	InFlight       *int     `json:"in_flight"`
	Cooling        *bool    `json:"cooling"`
	CoolRemaining  *int64   `json:"cool_remaining_sec"`

	RateLimitedModels []RateLimitedModel `json:"rate_limited_models"`

	// 本端给账号写的备注（按 uid 存本端库）。
	Note string `json:"note"`

	SuccessCount     *int    `json:"success_count"`
	ErrTotal         *int    `json:"err_total"`
	BreakerFails     *int    `json:"breaker_fails"`
	DegradeUntil     *string `json:"degrade_until"`
	ConsecutiveFails *int    `json:"consecutive_fails"`
	LastSuccess      *string `json:"last_success"`
	LastUsed         *int64  `json:"last_used"`

	// Source: "file" 来自 auths/ 目录；"pool" 来自上游 /status。
	Source string `json:"source"`

	InPool        *bool  `json:"in_pool"`
	PoolUnknown   *bool  `json:"poolUnknown"`
	InvalidReason string `json:"invalid_reason"`

	// DisabledByPanel：面板通过改文件名（.disabled 后缀）临时停用。
	DisabledByPanel bool `json:"disabled_by_panel"`
	// ManualDisabled：上游的 manual_disabled 状态位（更轻量，推荐）。
	ManualDisabled *bool  `json:"manual_disabled"`
	ManualReason   string `json:"manual_reason"`

	Realm            Realm  `json:"realm"`
	CheckinSupported *bool  `json:"checkin_supported"`
	Domain           string `json:"domain"`
}

// AccountsResponse 是 GET /api/accounts 的返回。
type AccountsResponse struct {
	Total         int       `json:"total"`
	Accounts      []Account `json:"accounts"`
	PoolSynced    *int      `json:"pool_synced"`
	PoolAvailable *bool     `json:"pool_available"`
}

// AuthStart 是 POST /api/auth/start 的返回（扫码登录发起）。
type AuthStart struct {
	State   string `json:"state"`
	AuthURL string `json:"authUrl"`
	Realm   Realm  `json:"realm"`
}

// AuthPoll 是 GET /api/auth/poll 的返回。
//
// Status 取值：waiting | success | expired | invalid | realm_mismatch。
type AuthPoll struct {
	Status     string `json:"status"`
	UID        string `json:"uid"`
	Nickname   string `json:"nickname"`
	Updated    bool   `json:"updated"`
	Realm      Realm  `json:"realm"`
	File       string `json:"file"`
	RegionNote string `json:"region_note"`
	Expected   string `json:"expected"`
	Got        string `json:"got"`
}

// CheckinResult 是单个账号签到（POST .../checkin）的返回。
type CheckinResult struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Credits *float64 `json:"credits"`
}

// CreditExpiry 是某个套餐批次的到期时刻与可用额度。
type CreditExpiry struct {
	At     int64   `json:"at"`     // 到期时刻（epoch 秒）
	Amount float64 `json:"amount"` // 该批次的可用额度
}

// Credits 是 GET /api/accounts/{file}/credits 的返回。
type Credits struct {
	OK       bool           `json:"ok"`
	Credits  *float64       `json:"credits"`
	Message  string         `json:"message"`
	Cached   bool           `json:"cached"`
	CacheAge *int64         `json:"cache_age"`
	Expiries []CreditExpiry `json:"expiries"`
}

// CreditsMeta 描述某个账号积分值的来源（实时 / 缓存）。
type CreditsMeta struct {
	Cached   bool           `json:"cached"`
	CacheAge *int64         `json:"cache_age"`
	Message  string         `json:"message,omitempty"`
	Expiries []CreditExpiry `json:"expiries,omitempty"`
}

// CreditsRefresh 是 POST /api/accounts/refresh-credits 的返回。
type CreditsRefresh struct {
	Total     int                    `json:"total"`
	Succeeded int                    `json:"succeeded"`
	Credits   map[string]*float64    `json:"credits"`
	Meta      map[string]CreditsMeta `json:"meta"`
	Failed    []string               `json:"failed"`
}

// CheckinAllResult 是 POST /api/accounts/checkin-all 的返回。
type CheckinAllResult struct {
	Total     int             `json:"total"`
	Succeeded int             `json:"succeeded"`
	Skipped   int             `json:"skipped"`
	Results   []CheckinAllRow `json:"results"`
}

// CheckinAllRow 是批量签到里单个账号的结果。
type CheckinAllRow struct {
	Nickname string `json:"nickname"`
	OK       bool   `json:"ok"`
	Message  string `json:"message"`
	Code     int    `json:"code,omitempty"`
	Skipped  bool   `json:"skipped,omitempty"`
}

// SetDisabledResult 是 POST /api/accounts/{file}/disabled 的返回。
type SetDisabledResult struct {
	OK              bool   `json:"ok"`
	File            string `json:"file"`
	Disabled        bool   `json:"disabled"`
	Changed         bool   `json:"changed"`
	ReloadTriggered bool   `json:"reload_triggered"`
	Message         string `json:"message"`
}

// NoteResult 是 PUT /api/accounts/{file}/note 的返回。
type NoteResult struct {
	OK   bool   `json:"ok"`
	UID  string `json:"uid"`
	Note string `json:"note"`
}

// ActionResult 是多数「执行型」接口的通用返回（ok + message）。
type ActionResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

// SuccessResult 是删除类接口的返回。
type SuccessResult struct {
	Success bool `json:"success"`
}

// CheckinLog 是一条签到记录。
type CheckinLog struct {
	ID       int    `json:"id"`
	TS       int64  `json:"ts"`
	UID      string `json:"uid"`
	Nickname string `json:"nickname"`
	Source   string `json:"source"` // manual | manual-batch | add | auto
	Kind     string `json:"kind"`   // checkin | keepalive
	Success  bool   `json:"success"`
	Code     *int   `json:"code"`
	Message  string `json:"message"`
	Auto     bool   `json:"auto"`
}

// CheckinLogPage 是签到记录的分页返回。
type CheckinLogPage struct {
	Items      []CheckinLog `json:"items"`
	Total      int          `json:"total"`
	LocalTotal *int         `json:"local_total"`
	AutoTotal  *int         `json:"auto_total"`
}

// TaskLog 是上游自动任务留痕（旅行 / 活跃上报 / 签到 / 保活 / 积分）。
type TaskLog struct {
	ID        int     `json:"id"`
	TS        int64   `json:"ts"`
	UID       string  `json:"uid"`
	Kind      string  `json:"kind"`
	Level     string  `json:"level"` // credit | ok | info | warn | error
	Credits   float64 `json:"credits"`
	Message   string  `json:"message"`    // 上游英文原文
	MessageCN string  `json:"message_cn"` // 中文展示文案
	Nickname  string  `json:"nickname"`
}

// TaskLogStats 是任务记录的聚合统计。
type TaskLogStats struct {
	ByKind       map[string]TaskLogKindStat `json:"by_kind"`
	Total        int                        `json:"total"`
	TotalCredits float64                    `json:"total_credits"`
}

// TaskLogKindStat 是单个类别的计数与积分。
type TaskLogKindStat struct {
	Count   int     `json:"count"`
	Credits float64 `json:"credits"`
}

// TaskLogCollector 是采集器的状态。
type TaskLogCollector struct {
	At     *int64 `json:"at"`
	Parsed *int   `json:"parsed"`
	Added  *int   `json:"added"`
	Error  string `json:"error,omitempty"`
}

// TaskLogResponse 是 GET /api/task-logs 的返回。
type TaskLogResponse struct {
	Logs      []TaskLog         `json:"logs"`
	Total     int               `json:"total"`
	Stats     TaskLogStats      `json:"stats"`
	Kinds     map[string]string `json:"kinds"`
	Collector TaskLogCollector  `json:"collector"`
}

// UpstreamLogs 是 GET /api/upstream/logs 的返回。
type UpstreamLogs struct {
	Available bool     `json:"available"`
	Lines     []string `json:"lines"`
	Total     int      `json:"total"`
}

// ── 密钥 ────────────────────────────────────────────────

// ApiKey 是一把网关密钥（明文仅在创建时返回一次）。
type ApiKey struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Prefix      string   `json:"prefix"`
	Enabled     bool     `json:"enabled"`
	ExpiresAt   *int64   `json:"expires_at"`
	MaxIPs      int      `json:"max_ips"`
	IPAllowlist []string `json:"ip_allowlist"`
	Models      []string `json:"models"`
	Realm       Realm    `json:"realm"` // cn | global | ""（不限制）
	Quota       *int     `json:"quota"` // token 配额；nil/0 = 不限
	UsedTokens  int64    `json:"used_tokens"`
	QuotaCredit float64  `json:"quota_credit"` // 积分配额；0 = 不限
	UsedCredit  float64  `json:"used_credit"`
	CreatedAt   int64    `json:"created_at"`
	LastUsedAt  *int64   `json:"last_used_at"`

	// Key 是明文密钥，**仅在创建时返回一次**。
	Key string `json:"key,omitempty"`
}

// KeyIn 是创建密钥的请求体。
type KeyIn struct {
	Name        string   `json:"name"`
	ExpiresAt   *int64   `json:"expires_at,omitempty"`
	MaxIPs      int      `json:"max_ips,omitempty"`
	IPAllowlist []string `json:"ip_allowlist,omitempty"`
	Models      []string `json:"models,omitempty"`
	Quota       int      `json:"quota,omitempty"`
	QuotaCredit float64  `json:"quota_credit,omitempty"`
	Realm       Realm    `json:"realm,omitempty"`
}

// KeyPatch 是更新密钥的请求体（PATCH 语义：仅提交非 nil 的字段）。
type KeyPatch struct {
	Name        *string   `json:"name,omitempty"`
	Enabled     *bool     `json:"enabled,omitempty"`
	ExpiresAt   *int64    `json:"expires_at,omitempty"`
	MaxIPs      *int      `json:"max_ips,omitempty"`
	IPAllowlist *[]string `json:"ip_allowlist,omitempty"`
	Models      *[]string `json:"models,omitempty"`
	Quota       *int      `json:"quota,omitempty"`
	QuotaCredit *float64  `json:"quota_credit,omitempty"`
	Realm       *Realm    `json:"realm,omitempty"`
}

// WhitelistCheck 是模型白名单校验的结果。
type WhitelistCheck struct {
	// Checked=false 表示当时拿不到模型清单（无法校验），unknown 不可当作「全部正确」。
	Checked bool     `json:"checked"`
	Unknown []string `json:"unknown"`
	Reason  string   `json:"reason,omitempty"`
}
