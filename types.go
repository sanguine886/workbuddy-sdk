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

	InPool *bool `json:"in_pool"`
	// Deprecated: 服务端**不返回**这个字段 —— `poolUnknown` 是前端自己算出来的
	// （web/lib/account-status.ts：本次没取到上游状态时把 in_pool 置空并标 true）。
	// 因此这里永远是 nil。要判断「看不到上游」，看 AccountsResponse.PoolAvailable。
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

	// AddedAt 是本面板记录的添加时间（Unix 秒）；拷贝来的账号可能未知。
	AddedAt *int64 `json:"added_at,omitempty"`
	// RefreshRisk 是刷新保护状态（被限流 / 需人工复核）。
	RefreshRisk *RefreshRisk `json:"refresh_risk,omitempty"`
}

// RefreshRisk 是账号的刷新保护状态。
type RefreshRisk struct {
	// Until 是保护到什么时候（Unix 秒）。
	Until         int64 `json:"until"`
	LastFailureAt int64 `json:"last_failure_at"`
	Failures      int   `json:"failures"`
	NeedsReview   bool  `json:"needs_review"`
	RetryAfterSec int   `json:"retry_after_sec"`
	Paused        bool  `json:"paused"`
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
	// Target 只在**定向重登**（StartAuthFor 传了 uid）时返回：这次要刷的是哪个账号。
	// 此时 Realm 跟随目标账号，调用方传的 realm 被忽略。
	Target *AuthTarget `json:"target,omitempty"`
}

// AuthTarget 是定向重登的目标账号。
type AuthTarget struct {
	UID           string `json:"uid"`
	File          string `json:"file"`
	Nickname      string `json:"nickname"`
	Realm         Realm  `json:"realm"`
	PanelDisabled bool   `json:"panel_disabled"`
}

// AuthPoll 是 GET /api/auth/poll 的返回。
//
// Status 取值：waiting | success | expired | invalid | account_mismatch。
// `account_mismatch` 是**定向重登**扫错了账号：不会落盘，看 ExpectedUID /
// ExpectedName 与 UID / Nickname 的差别。
type AuthPoll struct {
	Status   string `json:"status"`
	UID      string `json:"uid"`
	Nickname string `json:"nickname"`
	Updated  bool   `json:"updated"`
	Realm    Realm  `json:"realm"`
	File     string `json:"file"`
	// RegionNote 是国际版新号的地区注册提示。
	RegionNote string `json:"region_note"`
	// ExpectedUID / ExpectedName 是定向重登里**期望**的账号（account_mismatch 时会出现）。
	ExpectedUID  string `json:"expected_uid"`
	ExpectedName string `json:"expected_name"`
	// Revive 是重登后自动解禁的结果（只有成功且该账号被面板停用过才有）。
	Revive *AccountRevive `json:"revive,omitempty"`

	// Deprecated: 上游从来没有 Expected / Got 这两个键（两版都搜不到），
	// 一直是空串。判断扫错账号请用 Status == "account_mismatch" + ExpectedUID。
	Expected string `json:"expected"`
	Got      string `json:"got"`
}

// AccountRevive 是「重新登录后自动解禁」的结果。
//
// 登录在语义上就是「刷新账号状态」：凭证是全新的，上游那句「需重新登录」的
// 禁用理由已经消失，所以面板会顺手把它放回池里。
type AccountRevive struct {
	// Attempted 表示是否真的尝试了（没被面板停用就不必尝试）。
	Attempted bool `json:"attempted"`
	// OK 表示解禁成功。
	OK bool `json:"ok"`
	// StillDisabled 三态：true 仍禁用 / false 已回池 / nil 读不到上游状态。
	StillDisabled *bool `json:"still_disabled"`
	// Pending 表示上游还没重载完，稍后会自动生效。
	Pending *bool  `json:"pending"`
	Message string `json:"message"`
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
	// DailyQuota 是每日 token 限额；0 = 不限（与 Quota 各自独立，任一超限即拒绝）。
	DailyQuota int64 `json:"daily_quota"`
	// DailyUsedTokens 是今天（服务器本地日期）已用的 token，次日 0 点归零。
	DailyUsedTokens int64 `json:"daily_used_tokens"`
	// UpstreamID 是绑定的上游接入点；nil = 默认上游（默认上游不是库里的一行）。
	UpstreamID *int `json:"upstream_id,omitempty"`
	// PacketID 是来源红包的 id；nil = 手工建的。
	PacketID   *int   `json:"packet_id"`
	CreatedAt  int64  `json:"created_at"`
	LastUsedAt *int64 `json:"last_used_at"`

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
	// DailyQuota 是每日 token 限额（0 = 不限；与 Quota 各自独立，任一超限即拒绝）。
	DailyQuota int   `json:"daily_quota,omitempty"`
	Realm      Realm `json:"realm,omitempty"`
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
	// DailyQuota 是每日 token 限额。**必须用指针**：nil = 不改，0 = 改成不限。
	DailyQuota *int   `json:"daily_quota,omitempty"`
	Realm      *Realm `json:"realm,omitempty"`
}

// WhitelistCheck 是模型白名单校验的结果。
type WhitelistCheck struct {
	// Checked=false 表示当时拿不到模型清单（无法校验），unknown 不可当作「全部正确」。
	Checked bool     `json:"checked"`
	Unknown []string `json:"unknown"`
	Reason  string   `json:"reason,omitempty"`
}

// ClearCoolingResult 是 POST /api/accounts/{file}/clear-cooling 的返回。
//
// 两条路径的明细不同，字段全部可选（见 ClearCoolingDetail 的说明）。
type ClearCoolingResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	UID     string `json:"uid"`

	// ── 在线清除（不停上游）──
	// Online=true 表示走上游管理接口清掉的，没有重启。
	Online *bool `json:"online,omitempty"`
	// Models 是在线清掉的模型级限流名单（原因为 11102 的误标）。
	Models []string `json:"models,omitempty"`
	// Unfrozen 表示是否顺带解除了面板写入的强制冷却。
	Unfrozen *bool `json:"unfrozen,omitempty"`

	// ── 离线清除（停上游 → 改 state.json → 启上游）──
	// StateFile 是改到的上游状态文件；账号还没进 state.json 时为空。
	StateFile string `json:"state_file,omitempty"`
	// Backup 是改动前的备份文件路径（空 = 没备份）。
	Backup string `json:"backup,omitempty"`
	// Before 是被清掉的旧状态（until / reason / model_cooldowns /
	// breaker_until / degrade_until）；账号原本没有持久化状态时为空对象。
	Before map[string]any `json:"before,omitempty"`
}
