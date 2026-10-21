package wbsdk

// 本文件覆盖后加的几块管理面能力：代理线路 / PostgreSQL 异地备份 / 红包 /
// 多上游 / 按小时用量 / 积分收支 / GitHub 凭据与镜像 / 离线升级包，
// 以及网关侧的密钥用量（/v1/usage）。
// 字段与官方前端 web/lib/types.ts 对齐；服务端新增字段会被 encoding/json 忽略。

// ── 代理线路（出口）──────────────────────────────────────

// ProxyRoute 是一条命名出口线路。**凭据已脱敏**：masked 里的密码是 `***`。
type ProxyRoute struct {
	Name   string `json:"name"`
	Scheme string `json:"scheme"`
	Host   string `json:"host"`
	// Port 为 nil 表示地址里没写端口（不是 0）。
	Port    *int   `json:"port"`
	HasAuth bool   `json:"has_auth"`
	Masked  string `json:"masked"`
	// Local 表示出口落在内网 / 本机（这类线路多半是自建中转）。
	Local *bool `json:"local"`
	// Weight 是池里按权重挑选时的相对份量（1~1000）。
	Weight  int  `json:"weight"`
	Enabled bool `json:"enabled"`

	// Bound 是绑定到这条线路的账号数；BoundNames 只给前几个昵称。
	Bound      int      `json:"bound"`
	BoundNames []string `json:"bound_names"`
	// SharedWarning 表示绑了 ≥3 台账号（同一出口 IP 上的号容易被上游按 IP 聚类）。
	SharedWarning *bool `json:"shared_warning"`
	// Probe 是面板记着的上次探测结果；没探过为 nil。
	Probe *ProxyProbeRecord `json:"probe"`
	// BoundUpdated 只在「改名」的返回里出现：跟着改过去的账号数。
	BoundUpdated *int `json:"bound_updated"`
}

// ProxyProbeRecord 是面板记录的上次探测结果（列表刷新后仍能看到）。
type ProxyProbeRecord struct {
	OK     bool   `json:"ok"`
	MS     int    `json:"ms"`
	At     int64  `json:"at"`
	ExitIP string `json:"exit_ip"`
	Error  string `json:"error"`
}

// ProxyProbe 是「现在探一次」的结果。失败原因原样返回，便于排查。
type ProxyProbe struct {
	OK       bool   `json:"ok"`
	MS       int    `json:"ms"`
	ExitIP   string `json:"exit_ip"`
	ExitInfo string `json:"exit_info"`
	Error    string `json:"error"`
}

// ProxyPool 是代理池的挑选配置与一览。
type ProxyPool struct {
	Enabled         bool     `json:"enabled"`
	Strategy        string   `json:"strategy"` // priority | round_robin | random | least_connections
	IntervalSeconds int      `json:"interval_seconds"`
	Strategies      []string `json:"strategies"`
	RoutesTotal     int      `json:"routes_total"`
	RoutesEnabled   int      `json:"routes_enabled"`
}

// ProxyRouteList 是 GET /api/proxy-routes 的返回。
type ProxyRouteList struct {
	Routes     []ProxyRoute `json:"routes"`
	Default    string       `json:"default"`
	Schemes    []string     `json:"schemes"`
	BoundTotal int          `json:"bound_total"`
	Pool       ProxyPool    `json:"pool"`
}

// ProxyAccountRoutes 是 GET /api/proxies 的返回（账号页选线路用的轻量列表）。
type ProxyAccountRoutes struct {
	Routes  []string `json:"routes"`
	Default string   `json:"default"`
}

// RouteAccount 是一个账号的线路绑定（Proxy 为空 = 跟随池策略）。
type RouteAccount struct {
	File     string `json:"file"`
	UID      string `json:"uid"`
	Nickname string `json:"nickname"`
	Proxy    string `json:"proxy"`
	// Dir 是该账号所在的账号目录（多分组时用来定位文件）。
	Dir string `json:"dir"`
}

// ProxyBindings 是 GET /api/proxy-routes/bindings 的返回。
type ProxyBindings struct {
	Accounts []RouteAccount `json:"accounts"`
	Routes   []string       `json:"routes"`
	Pool     ProxyPool      `json:"pool"`
}

// ProxyImportResult 是批量导入的结果：能导的先导进去，逐行报问题。
type ProxyImportResult struct {
	Imported int      `json:"imported"`
	Problems []string `json:"problems"`
}

// ProxyDeleteResult 是删除线路的返回。
type ProxyDeleteResult struct {
	Name string `json:"name"`
	// BoundCleared 是被一并解除绑定的账号数（Force=false 时恒为 0）。
	BoundCleared int `json:"bound_cleared"`
}

// ProxyBindResult 是批量绑定 / 解绑的返回。
type ProxyBindResult struct {
	Name    string   `json:"name"`
	Bound   bool     `json:"bound"`
	Updated int      `json:"updated"`
	Skipped []string `json:"skipped"`
	// 以下几个字段只有「自动分配」的返回才有。
	Requested   *int `json:"requested"`
	Candidates  *int `json:"candidates"`
	UnboundLeft *int `json:"unbound_left"`
}

// ProxySettings 是面板自身出口代理的配置（地址已脱敏）。
type ProxySettings struct {
	GlobalProxyEnabled bool   `json:"global_proxy_enabled"`
	HTTPProxy          string `json:"http_proxy"`
	UpdateProxy        string `json:"update_proxy"`
	HTTPConfigured     bool   `json:"http_configured"`
	UpdateConfigured   bool   `json:"update_configured"`
	EnvHTTPProxy       string `json:"env_http_proxy"`
	EnvUpdateProxy     string `json:"env_update_proxy"`
	Effective          string `json:"effective"`
	// Source: panel | env | none | off（off = 全局开关关掉了出口代理）。
	Source   string `json:"source"`
	ProbeURL string `json:"probe_url"`
}

// ProxyRouteIn 是新建线路的请求体。
type ProxyRouteIn struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ProxyRoutePatch 是修改线路的请求体（只提交非 nil 的字段）。
type ProxyRoutePatch struct {
	// NewName 非空时执行「改名」：绑定它的账号会一并改过去。
	NewName *string `json:"new_name,omitempty"`
	URL     *string `json:"url,omitempty"`
	Weight  *int    `json:"weight,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
}

// ProxyPoolPatch 是保存池配置的请求体。
type ProxyPoolPatch struct {
	Enabled         *bool  `json:"enabled,omitempty"`
	Strategy        string `json:"strategy,omitempty"`
	IntervalSeconds *int   `json:"interval_seconds,omitempty"`
}

// ProxySettingsPatch 是保存出口代理的请求体。
//
// 地址里密码回显为 `***` 时服务端会沿用已存的密码；传空串表示清除。
type ProxySettingsPatch struct {
	GlobalProxyEnabled *bool   `json:"global_proxy_enabled,omitempty"`
	HTTPProxy          *string `json:"http_proxy,omitempty"`
	UpdateProxy        *string `json:"update_proxy,omitempty"`
}

// ── PostgreSQL 异地备份 ─────────────────────────────────

// PGSyncConfig 是 PostgreSQL 备份的连接配置。
//
// 读取时 Password 恒为掩码 `********`；保存时原样回传表示「不改动」。
type PGSyncConfig struct {
	Enabled         bool   `json:"enabled"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	DBName          string `json:"dbname"`
	User            string `json:"user"`
	Password        string `json:"password"`
	SSLMode         string `json:"sslmode"` // disable | allow | prefer | require | verify-ca | verify-full
	IntervalMinutes int    `json:"interval_minutes"`
	KeepLocalBackup bool   `json:"keep_local_backup"`
	LastExportAt    int64  `json:"last_export_at"`
	LastImportAt    int64  `json:"last_import_at"`
}

// PGSyncLogLine 是备份 / 恢复过程的一行日志。
type PGSyncLogLine struct {
	TS    int64  `json:"ts"`
	Level string `json:"level"`
	Text  string `json:"text"`
}

// PGSyncStatus 是任务进度（与一键更新的状态同构）。
type PGSyncStatus struct {
	Running bool   `json:"running"`
	Kind    string `json:"kind"` // "" | export | import
	// OK 为 nil 表示从未跑过（与「跑失败了」区分）。
	OK          *bool           `json:"ok"`
	Step        string          `json:"step"`
	Percent     int             `json:"percent"`
	TablesTotal int             `json:"tables_total"`
	TablesDone  int             `json:"tables_done"`
	Rows        int64           `json:"rows"`
	Logs        []PGSyncLogLine `json:"logs"`
	StartedAt   int64           `json:"started_at"`
	FinishedAt  int64           `json:"finished_at"`
}

// PGSync 是 GET /api/settings/pg-sync 的返回：配置 + 当前进度。
type PGSync struct {
	Config PGSyncConfig `json:"config"`
	Status PGSyncStatus `json:"status"`
}

// PGSyncTestResult 是连通性探测结果（失败也不报错，看 OK / Message）。
type PGSyncTestResult struct {
	OK            bool   `json:"ok"`
	Message       string `json:"message"`
	ServerVersion string `json:"server_version,omitempty"`
}

// PGSyncJob 是「开始导出 / 开始恢复」的返回。
type PGSyncJob struct {
	OK      bool         `json:"ok"`
	Message string       `json:"message"`
	Status  PGSyncStatus `json:"status"`
}

// PGSyncConfigPatch 是保存备份配置的请求体（只提交要改的键）。
type PGSyncConfigPatch struct {
	Enabled         *bool   `json:"enabled,omitempty"`
	Host            *string `json:"host,omitempty"`
	Port            *int    `json:"port,omitempty"`
	DBName          *string `json:"dbname,omitempty"`
	User            *string `json:"user,omitempty"`
	Password        *string `json:"password,omitempty"`
	SSLMode         *string `json:"sslmode,omitempty"`
	IntervalMinutes *int    `json:"interval_minutes,omitempty"`
	KeepLocalBackup *bool   `json:"keep_local_backup,omitempty"`
}

// ── 红包（批量发放的带额度密钥）─────────────────────────

// 红包额度类型。
const (
	RedPacketCredit = "credit" // 积分红包：总额按份拆、不限定模型
	RedPacketToken  = "token"  // token 红包：按份拆 token、必须限定模型
)

// 红包分配方式。
const (
	RedPacketLucky = "lucky" // 拼手气：随机，有人多有人少
	RedPacketEven  = "even"  // 均分：每份一样
)

// RedPacket 是一个红包（不含明文密钥 —— 库里只存哈希）。
type RedPacket struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	QuotaKind   string  `json:"quota_kind"`
	TotalAmount float64 `json:"total_amount"`
	Shares      int     `json:"shares"`
	Mode        string  `json:"mode"`
	// Models：token 红包非空、积分红包恒为空（两类规则相反）。
	Models []string `json:"models"`
	// Code 是抽奖码：拼分享链接用，**它是凭据**，别贴到公开场合。
	Code      string `json:"code"`
	CreatedBy string `json:"created_by"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
	// Claimed 是已被抽走的份数。
	Claimed int `json:"claimed"`
	// Revoked 表示整批已收回（密钥被删）。
	Revoked bool `json:"revoked"`
}

// RedPacketItem 是红包里的一份（= 一把密钥）。
type RedPacketItem struct {
	// KeyID 为 nil 表示这把密钥已被收回删除（此时 Prefix 为空串）。
	KeyID      *int    `json:"key_id"`
	Prefix     string  `json:"prefix"`
	Amount     float64 `json:"amount"`
	Enabled    bool    `json:"enabled"`
	UsedTokens int64   `json:"used_tokens"`
	UsedCredit float64 `json:"used_credit"`
	// ClaimedByIP / ClaimedAt 为 nil 表示还没被领。
	ClaimedByIP *string `json:"claimed_by_ip"`
	ClaimedAt   *int64  `json:"claimed_at"`
}

// RedPacketDetail 是红包详情。
type RedPacketDetail struct {
	RedPacket
	Items []RedPacketItem `json:"items"`
}

// RedPacketCreated 是创建红包的返回：**含明文密钥，且仅此一次**。
type RedPacketCreated struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	QuotaKind   string   `json:"quota_kind"`
	TotalAmount float64  `json:"total_amount"`
	Shares      int      `json:"shares"`
	Mode        string   `json:"mode"`
	Models      []string `json:"models"`
	Code        string   `json:"code"`
	ExpiresAt   int64    `json:"expires_at"`
	CreatedAt   int64    `json:"created_at"`
	// Keys 里的 ApiKey.Key 是明文密钥 —— 离开这次返回就再也拿不到了。
	Keys []ApiKey `json:"keys"`
}

// RedPacketIn 是创建红包的请求体。
type RedPacketIn struct {
	Title       string  `json:"title,omitempty"`
	QuotaKind   string  `json:"quota_kind,omitempty"` // 空 = credit
	TotalAmount float64 `json:"total_amount"`
	Shares      int     `json:"shares"`
	Mode        string  `json:"mode,omitempty"` // 空 = lucky
	// TTLDays 为 nil 时按服务端默认（7 天）。
	TTLDays *int     `json:"ttl_days,omitempty"`
	Models  []string `json:"models,omitempty"`
}

// RedPacketRevoked 是收回整批的返回。
type RedPacketRevoked struct {
	Revoked int `json:"revoked"`
}

// ClaimInfo 是抽奖页的元信息。**不含密钥** —— 没点「开启」之前拿不到。
//
// 已经领过的 IP 会连带拿到自己那一份的明文（MyKey），刷新页面就能找回来。
type ClaimInfo struct {
	Title     string   `json:"title"`
	QuotaKind string   `json:"quota_kind"`
	Shares    int      `json:"shares"`
	Left      int      `json:"left"`
	Models    []string `json:"models"`
	ExpiresAt int64    `json:"expires_at"`
	Expired   bool     `json:"expired"`
	Claimed   bool     `json:"claimed"`
	MyAmount  *float64 `json:"my_amount"`
	MyKey     *string  `json:"my_key"`
}

// ClaimResult 是抽到的那一份（含明文密钥）。
type ClaimResult struct {
	Amount    float64  `json:"amount"`
	QuotaKind string   `json:"quota_kind"`
	Models    []string `json:"models"`
	Key       string   `json:"key"`
	ExpiresAt int64    `json:"expires_at"`
}

// ── 多上游（账号池分组）─────────────────────────────────

// ManagedUpstream 是面板接入的一个上游。**api_key 恒为掩码**。
type ManagedUpstream struct {
	// ID 为 nil 表示默认上游（来自环境变量 / 上游 config.json，不可编辑删除）。
	ID           *int   `json:"id"`
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	HasKey       bool   `json:"has_key"`
	APIKeyMasked string `json:"api_key_masked"`
	Note         string `json:"note"`
	Enabled      bool   `json:"enabled"`
	AuthDir      string `json:"auth_dir"`
	Container    string `json:"container"`
	CreatedAt    *int64 `json:"created_at"`
	UpdatedAt    *int64 `json:"updated_at"`
	IsDefault    bool   `json:"is_default"`
	// BoundKeys 是绑定到它上面的密钥数（默认上游统计的是「没绑定的」那些）。
	BoundKeys int `json:"bound_keys"`
}

// UpstreamList 是 GET /api/upstreams 的返回。
type UpstreamList struct {
	Items []ManagedUpstream `json:"items"`
}

// UpstreamIn 是新建上游的请求体。
type UpstreamIn struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	// APIKey 可为空（自建实例可能不开鉴权，或稍后再填）。
	APIKey    string `json:"api_key,omitempty"`
	Note      string `json:"note,omitempty"`
	Enabled   *bool  `json:"enabled,omitempty"`
	AuthDir   string `json:"auth_dir,omitempty"`
	Container string `json:"container,omitempty"`
}

// UpstreamPatch 是修改上游的请求体（PATCH 语义：只提交非 nil 的字段）。
//
// 服务端的三态：键**省略** = 不改；键为 **null** = 显式改（api_key / note /
// auth_dir / container 被清空，enabled 变停用）；键有值 = 改成它。Go 的指针 +
// omitempty 只能表达前两种「改成空」的等价写法 —— 想清空就传指向空串的指针
// （`api_key: ""` 等价于 null）。
type UpstreamPatch struct {
	Name      *string `json:"name,omitempty"`
	BaseURL   *string `json:"base_url,omitempty"`
	APIKey    *string `json:"api_key,omitempty"`
	Note      *string `json:"note,omitempty"`
	Enabled   *bool   `json:"enabled,omitempty"`
	AuthDir   *string `json:"auth_dir,omitempty"`
	Container *string `json:"container,omitempty"`
}

// UpstreamProbe 是上游连通性探测结果（走它的 /healthz）。
type UpstreamProbe struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// ── 按小时用量与积分收支 ────────────────────────────────

// UsageHourPoint 是某一天里一个小时的数据点。
//
// 服务端**固定返回 24 个桶**（没数据的补 0），X 轴不随当前时刻抖动。
type UsageHourPoint struct {
	Day              string  `json:"day"`
	Hour             int     `json:"hour"`
	Requests         int     `json:"requests"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	Credit           float64 `json:"credit"`
	Failed           int     `json:"failed"`
}

// CreditsFlowPoint 是一天的积分收入 / 支出。
type CreditsFlowPoint struct {
	Day     string  `json:"day"`
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
	Net     float64 `json:"net"`
}

// CreditsFlowSum 是一段时间的积分收支合计。
type CreditsFlowSum struct {
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
	Net     float64 `json:"net"`
}

// CreditsFlow 是积分收支统计。
//
// **收入与支出是两套口径**：收入来自余额增加的流水（近似值，按查询时刻归日），
// 支出来自每次调用的真实扣费。所以三项都列出来，不要只拿 Net 下结论。
type CreditsFlow struct {
	Days  int                `json:"days"`
	From  string             `json:"from"`
	To    string             `json:"to"`
	Today CreditsFlowSum     `json:"today"`
	Week  CreditsFlowSum     `json:"week"`
	Total CreditsFlowSum     `json:"total"`
	Daily []CreditsFlowPoint `json:"daily"`
}

// ── GitHub 凭据 / 镜像 ──────────────────────────────────

// GitHubToken 是 GitHub Token 的生效状态（**只回掩码**）。
type GitHubToken struct {
	Configured bool   `json:"configured"`
	Masked     string `json:"masked"`
	// Source: env | panel | ""（未配置）。env 时面板填的那份不生效。
	Source string `json:"source"`
}

// GitHubMirror 是 GitHub 镜像基址与来源。
type GitHubMirror struct {
	Mirror    string   `json:"mirror"`
	Stored    string   `json:"stored"`
	EnvMirror string   `json:"env_mirror"`
	Source    string   `json:"source"` // panel | env | ""
	Presets   []string `json:"presets"`
}

// ── 离线升级包 ──────────────────────────────────────────

// UpdatePackage 是已上传的离线升级包（内网机器用它代替「从 GitHub 拉最新」）。
type UpdatePackage struct {
	// Present 为 false 表示记录还在但文件被清掉了。
	Present        bool   `json:"present"`
	Name           string `json:"name,omitempty"`
	Size           int64  `json:"size,omitempty"`
	SHA256         string `json:"sha256,omitempty"`
	ExpectedSHA256 string `json:"expected_sha256,omitempty"`
	Version        string `json:"version,omitempty"`
	UploadedAt     int64  `json:"uploaded_at,omitempty"`
	UploadedBy     string `json:"uploaded_by,omitempty"`
}

// ── 网关侧：密钥用量 ────────────────────────────────────

// KeyUsage 是 GET /v1/usage 的返回：面板**已记录**的用量快照。
//
// 这些数字不是向上游现问的（DataSource 恒为 "panel-recorded"），
// 读取时刻即 SnapshotAt。
type KeyUsage struct {
	Object     string         `json:"object"`
	Key        UsageKey       `json:"key"`
	Tokens     UsageAllowance `json:"tokens"`
	Credits    UsageAllowance `json:"credits"`
	LastUsedAt int64          `json:"last_used_at"`
	SnapshotAt int64          `json:"snapshot_at"`
	DataSource string         `json:"data_source"`
}

// UsageKey 是这把密钥的元信息（不含明文）。
type UsageKey struct {
	Name      string   `json:"name"`
	Prefix    string   `json:"prefix"`
	Realm     Realm    `json:"realm"` // 空 = 不限制版本
	Enabled   bool     `json:"enabled"`
	ExpiresAt int64    `json:"expires_at"`
	Expired   bool     `json:"expired"`
	Models    []string `json:"models"`
}

// UsageAllowance 是一项额度：Limit / Remaining 为 nil 表示不限制。
type UsageAllowance struct {
	Used      int64  `json:"used"`
	Limit     *int64 `json:"limit"`
	Remaining *int64 `json:"remaining"`
}

// ── 账号级线路 / 权重 / 冷却 / 分组转移 ─────────────────

// AccountProxyResult 是 PUT /api/accounts/{file}/proxy 的返回。
type AccountProxyResult struct {
	OK              bool   `json:"ok"`
	File            string `json:"file"`
	Proxy           string `json:"proxy"`
	ReloadTriggered bool   `json:"reload_triggered"`
}

// AccountWeightResult 是 PUT /api/accounts/{file}/weight 的返回（Weight 是乘数）。
type AccountWeightResult struct {
	OK              bool    `json:"ok"`
	File            string  `json:"file"`
	Weight          float64 `json:"weight"`
	ReloadTriggered bool    `json:"reload_triggered"`
}

// ForceCoolingIn 是强制设定账号冷却的请求体。
//
// 三选一：DurationSeconds（相对秒数）、Until（ISO 8601 / RFC 3339 截止时刻，
// 优先于 DurationSeconds）、Permanent（永久，按 2099-12-31 记）。
type ForceCoolingIn struct {
	DurationSeconds *int   `json:"duration_seconds,omitempty"`
	Until           string `json:"until,omitempty"`
	Permanent       bool   `json:"permanent,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// GroupRef 是一个上游分组的引用（ID 为 nil = 默认分组）。
type GroupRef struct {
	ID   *int   `json:"id"`
	Name string `json:"name"`
}

// AccountMoveResult 是把账号转移到另一个分组的返回。
type AccountMoveResult struct {
	OK   bool     `json:"ok"`
	File string   `json:"file"`
	UID  string   `json:"uid"`
	From GroupRef `json:"from"`
	To   GroupRef `json:"to"`
}
