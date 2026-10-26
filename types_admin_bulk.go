package wbsdk

import (
	"encoding/json"
	"io"
)

// ── 账号批量操作（v2.8 ~ v2.10 新增）────────────────────────────────────
//
// 这一组端点解决的是「账号一多就得逐个点」的问题：批量导入/导出、批量探测
// 模型可用性、批量刷新令牌、以及对账用的积分快照。老版本管理端没有这些路由，
// 调用会得到 404 —— 调用方应把它当作"服务端太旧"，而不是自己参数写错了。

// AdminAPIStatus 是 GET /api/accounts/admin-api-status 的返回。
//
// 上游的「账号管理接口」（admin.enabled）是 revive / enable / disable 这组
// 解封动作的前提：开关没开时，重新登录只换令牌、清不掉「禁用」位。面板据此
// 把提示从「重新登录解封」换成「先在设置里开启管理接口」。
type AdminAPIStatus struct {
	// Enabled 为 nil 表示**未知**：非默认分组跑在另一套容器/主机上，面板读不到
	// 它的 config.json，于是不敢拿默认分组的开关去误导它。
	Enabled *bool `json:"enabled"`
}

// CreditsSnapshotAccount 是快照里的一行（按 uid 登记，跨全部分组）。
type CreditsSnapshotAccount struct {
	UID          string  `json:"uid"`
	Credits      float64 `json:"credits"`       // 登记时刻的余额，不是实时值
	RegisteredAt int64   `json:"registered_at"` // 登记时刻（epoch 秒）
	// Known 表示这个 uid 现在**还**对应本机的一个账号。账号删掉后历史快照仍在，
	// 对账脚本可据 known=false 把它排除，或反过来当作"曾经存在"的凭据。
	Known bool `json:"known"`
}

// CreditsSnapshot 是 GET /api/accounts/credits-snapshot 的返回。
//
// **只读本地记录，不触发任何上游查询**：数字是面板上次记下来的，可能是几小时前
// 的。要实时余额请用 Accounts.RefreshCredits（那会真的打腾讯接口，且要管理员）。
type CreditsSnapshot struct {
	SnapshotAt int64                    `json:"snapshot_at"` // 最新一条的登记时刻
	Count      int                      `json:"count"`
	KnownCount int                      `json:"known_count"`
	Accounts   []CreditsSnapshotAccount `json:"accounts"`
}

// AccountUploadFile 是批量导入时的一个待上传文件。
//
// Reader 只被读一次（流式转发），所以上传失败要重试就必须重新打开文件。
type AccountUploadFile struct {
	Filename string
	Reader   io.Reader
}

// AccountUploadHit 是批量导入里**成功写入**的一个账号文件。
type AccountUploadHit struct {
	File string `json:"file"`
	UID  string `json:"uid"`
	// Updated 为 true 表示覆盖了已存在的文件（仅在 Added / Overwritten 里出现）。
	Updated *bool `json:"updated,omitempty"`
}

// AccountUploadMiss 是批量导入里被拒绝或失败的一条。
//
// Rejected 是"服务端主动不写"（账号已存在且没开覆盖、线路无效、条目无效），
// Failed 是"读不懂或写不进去"（超 2 MB、不是合法 JSON、IO 失败）——
// 两者都不影响同一批里其它文件，逐条看 Message 即可。
type AccountUploadMiss struct {
	File    string `json:"file"`
	UID     string `json:"uid,omitempty"`
	Message string `json:"message"`
}

// AccountUploadResult 是 POST /api/accounts/upload 的返回。
//
// 没有真正的"跳过"：**重复导入是拒绝而不是覆盖**，除非请求带 overwrite=true。
type AccountUploadResult struct {
	OK          bool                `json:"ok"` // 无 rejected 且无 failed
	Uploaded    []AccountUploadHit  `json:"uploaded"`
	Added       []AccountUploadHit  `json:"added"`
	Overwritten []AccountUploadHit  `json:"overwritten"`
	Rejected    []AccountUploadMiss `json:"rejected"`
	Failed      []AccountUploadMiss `json:"failed"`
}

// ProbeModelsIn 是 POST /api/accounts/probe-models 的请求体。
//
// Accounts 传账号文件名；Models 传模型名。服务端限制：账号 ≤ 10、模型 ≤ 30、
// 组合 ≤ 60（超了 400）。同账号内串行且每条间隔 600ms，不同账号最多 4 路并发，
// 因此这个调用**可能耗时数十秒**，ctx 超时要留够。
type ProbeModelsIn struct {
	Accounts []string `json:"accounts"`
	Models   []string `json:"models"`
}

// ProbeModelRow 是一条「账号 × 模型」的探测结果。
type ProbeModelRow struct {
	File     string `json:"file"`
	UID      string `json:"uid"`
	Nickname string `json:"nickname"`
	Realm    Realm  `json:"realm"`
	Model    string `json:"model"`
	// Status 取值：ok / rate（6004 模型限流）/ credit（402 或积分文案）/
	// auth（401 / 12153）/ no_model（11102，**不算错误**）/ error。
	Status    string `json:"status"`
	Code      *int   `json:"code"` // 上游业务码；null = 没拿到
	Message   string `json:"message"`
	LatencyMS int    `json:"latency_ms"`
}

// ProbeModelsSummary 是按 Status 分类的计数。
type ProbeModelsSummary struct {
	OK      int `json:"ok"`
	Rate    int `json:"rate"`
	Credit  int `json:"credit"`
	Auth    int `json:"auth"`
	NoModel int `json:"no_model"`
	Error   int `json:"error"`
}

// ProbeModelsResult 是 POST /api/accounts/probe-models 的返回。
//
// Results 已按「账号顺序 → 模型顺序」排好，Requests == len(Results)。
type ProbeModelsResult struct {
	Results  []ProbeModelRow    `json:"results"`
	Summary  ProbeModelsSummary `json:"summary"`
	Requests int                `json:"requests"`
}

// TokenRefreshRow 是「一键刷新令牌」里一个账号的结果。
type TokenRefreshRow struct {
	File     string `json:"file"`
	Nickname string `json:"nickname"`
	UID      string `json:"uid"`
	OK       bool   `json:"ok"`
	Message  string `json:"message"`
}

// RefreshTokensResult 是 POST /api/accounts/refresh-tokens 的返回。
//
// Failed 是给人看的一行摘要（"昵称: 原因"），Results 是逐条结构化结果 ——
// 两者都保留：界面贴摘要，脚本读 Results。
type RefreshTokensResult struct {
	Total           int               `json:"total"`
	Succeeded       int               `json:"succeeded"`
	Failed          []string          `json:"failed"`
	Results         []TokenRefreshRow `json:"results"`
	ReloadTriggered bool              `json:"reload_triggered"`
	// ReloadError 只在「刷新成功但重载失败」时非空：凭证已经写回，
	// 但运行中的上游还没加载到它。一个都没成功时服务端不会重载，这里也是空。
	ReloadError string `json:"reload_error"`
}

// RefreshPlan 是令牌刷新计划（GET /api/accounts/refresh-plan）。
//
// 与手动「一键刷新」的区别：那个是点一次；这是**定时**每 N 天全量刷一次。
type RefreshPlan struct {
	Enabled      bool  `json:"enabled"`
	IntervalDays int   `json:"interval_days"` // 1-365
	Hour         int   `json:"hour"`          // 0-23，按整点对齐
	LastRun      int64 `json:"last_run"`      // epoch 秒；0 = 从未跑过
	// NextRun 为 nil 表示计划未启用（服务端返回 null）。
	NextRun *int64 `json:"next_run"`
}

// RefreshPlanIn 是 PUT /api/accounts/refresh-plan 的请求体。
//
// 三个字段都**必须给**：服务端逐项校验类型与范围（enabled 必须是布尔、
// interval_days 1-365、hour 0-23），缺一个就是 400，不做"省略即不改"。
// 要改一项就先 GET 出来、改完整体提交。
type RefreshPlanIn struct {
	Enabled      bool `json:"enabled"`
	IntervalDays int  `json:"interval_days"`
	Hour         int  `json:"hour"`
}

// RefreshPlanRun 是 POST /api/accounts/refresh-plan/run 的返回。
//
// 与 RefreshTokensResult 不是同一套数字：这里是**串行巡检**的统计 ——
// Checked 是真正进了续期流程的账号数（force=true 时即全部），Renewed 成功，
// Failed 失败，Skipped 是面板停用的账号（不续期）。
type RefreshPlanRun struct {
	Checked int    `json:"checked"`
	Renewed int    `json:"renewed"`
	Failed  int    `json:"failed"`
	Skipped int    `json:"skipped"`
	LastRun int64  `json:"last_run"`
	NextRun *int64 `json:"next_run"`
}

// CompensationRow 是「领活动补偿」里一个账号的结果。
type CompensationRow struct {
	Nickname string  `json:"nickname"`
	Status   string  `json:"status"` // claimed / nothing / already / skipped / failed
	Credit   float64 `json:"credit"`
	Message  string  `json:"message"`
}

// CompensationResult 是 POST /api/accounts/claim-compensation 的返回。
//
// Total 只算**真的发了请求**的账号（claimed / nothing / failed）；
// already / skipped 既不是成功也不是失败，算进分母会让界面显示成"5/6 成功"。
type CompensationResult struct {
	OK      bool              `json:"ok"`
	Total   int               `json:"total"`
	Claimed int               `json:"claimed"`
	Credit  float64           `json:"credit"`
	Already int               `json:"already"`
	Skipped int               `json:"skipped"`
	Nothing int               `json:"nothing"`
	Failed  int               `json:"failed"`
	Results []CompensationRow `json:"results"`
}

// DailyTaskPlan 是一个每日任务的计划项。
type DailyTaskPlan struct {
	Enabled bool `json:"enabled"`
	Hour    int  `json:"hour"`
	// LastRun 形如 "2026-10-15-09"（哪一天的哪个整点跑过），空串 = 没跑过。
	// 它是"同一小时只跑一次"的去重键，所以是字符串而不是时间戳。
	LastRun string `json:"last_run"`
}

// DailyPlan 是 GET /api/accounts/daily-plan 的返回：四类任务的开关与执行整点。
//
// 四个任务里 full 会**伪造活跃上报**，属写风险动作，服务端默认全关；
// compensation 默认也关（上游调度器的活跃上报本来就每天领一次）。
type DailyPlan struct {
	Checkin      DailyTaskPlan `json:"checkin"`
	Compensation DailyTaskPlan `json:"compensation"`
	Claim        DailyTaskPlan `json:"claim"`
	Full         DailyTaskPlan `json:"full"`
}

// DailyTaskIn 是提交计划时的单个任务项。
//
// 与 DailyTaskPlan 的区别：这里**没有** last_run —— 服务端只接受 enabled/hour，
// 传了 last_run 也会被忽略（改时刻不该把"这一小时已经跑过"抹掉）。
type DailyTaskIn struct {
	Enabled bool `json:"enabled"`
	Hour    int  `json:"hour"`
}

// DailyPlanIn 是 PUT /api/accounts/daily-plan 的请求体：四类任务都要给。
type DailyPlanIn struct {
	Checkin      DailyTaskIn `json:"checkin"`
	Compensation DailyTaskIn `json:"compensation"`
	Claim        DailyTaskIn `json:"claim"`
	Full         DailyTaskIn `json:"full"`
}

// DailyTaskRun 是 POST /api/accounts/daily-plan/run 的返回：本次执行结果 +
// 执行后的整份计划（该任务已被标记为"本小时已跑"）。
type DailyTaskRun struct {
	OK      bool      `json:"ok"`
	Message string    `json:"message"`
	Task    string    `json:"task"`
	Plan    DailyPlan `json:"plan"`
}

// ── 本机客户端导入（/api/keys/import-local/*，v2.10 新增）──────────────
//
// 这组端点只在**面板跑在用户自己机器上**时才有意义：它读的是服务端所在机器的
// 文件系统/注册表，去写那台机器上的 cc-switch / ZCode 配置。远程部署时服务端
// 一律拒绝（403，见 LocalImportStatus.LocalCaller）。

// 本机导入支持的客户端与导入方式（服务端只认这几个值，别的会 400）。
const (
	LocalClientCCSwitch = "ccswitch"
	LocalClientZCode    = "zcode"

	LocalImportAuto     = "auto"     // 能走官方深链就走深链（默认）
	LocalImportDeeplink = "deeplink" // 强制深链；不支持时 409
	LocalImportDirect   = "direct"   // 直接改客户端配置；要求客户端已退出
)

// LocalClientStatus 是某一款客户端在本机的可导入状态（**只读、无副作用**）。
type LocalClientStatus struct {
	Client    string `json:"client"` // ccswitch / zcode
	Installed bool   `json:"installed"`
	// Running 三态：nil = 探测不了（不确定是否在跑）。
	Running    *bool `json:"running"`
	Importable bool  `json:"importable"`
	// Reason 为空表示可导入；否则是 not_installed / client_running / cannot_detect。
	Reason    string  `json:"reason"`
	Target    string  `json:"target"` // 会被写的配置文件路径
	Exe       *string `json:"exe"`    // 面板认的是哪个可执行文件；nil = 没认出来
	ExeSource string  `json:"exe_source"`
	// NeedsClose 表示"点下去会先替你关掉客户端再写"，界面要提前说清。
	NeedsClose bool `json:"needs_close"`
	// Deeplink 表示这台机器能走客户端官方的 ccswitch:// 深链（不用关客户端）。
	Deeplink bool `json:"deeplink"`
}

// LocalImportStatus 是 GET /api/keys/import-local/status 的返回。
//
// 开关关着时也是 200（Enabled=false）而不是 403：界面要据此隐藏这一块，
// 报错会让人以为是故障。
type LocalImportStatus struct {
	Enabled     bool                `json:"enabled"`
	LocalCaller bool                `json:"local_caller"`
	Clients     []LocalClientStatus `json:"clients"`
}

// LocalClientDetect 是 GET /api/keys/import-local/detect 的返回。
//
// 检测顺序"可信度从高到低"：环境变量 → 正在跑的进程 → 注册表里的协议处理器 →
// 上次的结果 → 常见安装位 →（Deep=true 时）受限扫描。命中会缓存到面板数据目录。
type LocalClientDetect struct {
	Client string  `json:"client"`
	Found  bool    `json:"found"`
	Exe    *string `json:"exe"`
	// Source 取值如 env / process / registry / cache / known / scan；空 = 没找到。
	Source string `json:"source"`
}

// KeyExportIn 是 POST /api/keys/export 与 /api/keys/import-local 的共用参数。
//
// Token 必须是**创建密钥时返回的明文**：面板只存哈希，拿不回明文；所以这里
// 不接受 key_id —— 调用方（前端/脚本）要在创建后自己留住那一次响应。
type KeyExportIn struct {
	Client       string   `json:"client"` // ccswitch / zcode（见 LocalClient* 常量）
	Token        string   `json:"token"`
	App          string   `json:"app,omitempty"` // cc-switch 要指定 app；ZCode 忽略
	BaseURL      string   `json:"base_url,omitempty"`
	ProviderName string   `json:"provider_name,omitempty"`
	Models       []string `json:"models,omitempty"`
	DefaultModel string   `json:"default_model,omitempty"`
}

// KeyExportResult 是 POST /api/keys/export 的返回：客户端可直接粘贴的配置片段。
//
// 两种客户端形状不同：cc-switch 给 AppType + SettingsConfig，ZCode 给 Provider。
// 未用到的那一侧为 nil。返回体**不含密钥**（片段里已内嵌，见服务端说明）。
type KeyExportResult struct {
	Client         string   `json:"client"`
	BaseURL        string   `json:"base_url"`
	Realm          Realm    `json:"realm"`
	Models         []string `json:"models"`
	Name           string   `json:"name"`
	AppType        string   `json:"app_type"`
	SettingsConfig any      `json:"settings_config"` // cc-switch 的 settings_config
	Provider       any      `json:"provider"`        // ZCode 的 provider 片段
}

// LocalImportIn 是 POST /api/keys/import-local 的请求体：导出参数 + 三个开关。
type LocalImportIn struct {
	KeyExportIn
	SetCurrent bool `json:"set_current,omitempty"`
	// Mode 取值 auto（默认，能走深链就走）/ deeplink / direct。
	// direct 需要客户端当时是退出的，除非 CloseRunning=true。
	Mode         string `json:"mode,omitempty"`
	CloseRunning bool   `json:"close_running,omitempty"`
}

// LocalImportResult 是 POST /api/keys/import-local 的返回。
//
// **不回显密钥**：明文只在"创建密钥"那一次响应里出现。
// 失败原因是 404（not_installed）/ 409（client_running、cannot_detect、
// cannot_stop、deeplink_unavailable、cannot_open、bad_format…），由 APIError 带出。
type LocalImportResult struct {
	Client     string `json:"client"`
	App        string `json:"app"`
	Realm      Realm  `json:"realm"`
	BaseURL    string `json:"base_url"`
	ModelCount int    `json:"model_count"`
	Method     string `json:"method"` // deeplink / direct，实际走的那条
	// Lifecycle 是 direct 路径下"关掉客户端—写完—拉起来"的过程记录；
	// 走深链时为 nil。
	Lifecycle map[string]any `json:"lifecycle"`
	// Action 是各客户端实现的写入动作：created / updated / appended /
	// handed-off（深链已交给操作系统）等。
	Action     string `json:"action"`
	ProviderID string `json:"provider_id"`
	AppType    string `json:"app_type"`
	Name       string `json:"name"`
	Target     string `json:"target"` // 被写的文件；深链为空
	Backup     string `json:"backup"` // 写前备份路径；深链为空
	// Handler 是深链实际交给了谁（Windows 上是协议处理器的命令）。
	Handler string `json:"handler"`
	// ProviderName / Models 只出现在 ZCode 的结果里。
	ProviderName string `json:"provider_name"`
	Models       int    `json:"models"`
}

// ── Anthropic token 估算（POST /v1/messages/count_tokens）───────────────

// CountTokensRequest 是 /v1/messages/count_tokens 的请求体。
//
// 服务端只读 model / system / messages[].content / tools 四处的文本，其余字段
// 一律不看；所以这里用 any 承载消息与工具，免得把 Anthropic 的多模态结构整套
// 建模（客户端本来就能给出完整 body，用 MessagesCountTokens 直接透传更省事）。
type CountTokensRequest struct {
	Model    string         `json:"model,omitempty"`
	System   any            `json:"system,omitempty"`
	Messages []any          `json:"messages,omitempty"`
	Tools    []any          `json:"tools,omitempty"`
	Extra    map[string]any `json:"-"`
}

// MarshalJSON 合并 Extra 字段（已建模的键优先）。
func (r CountTokensRequest) MarshalJSON() ([]byte, error) {
	type alias CountTokensRequest
	base, err := json.Marshal(alias(r))
	if err != nil {
		return nil, err
	}
	if len(r.Extra) == 0 {
		return base, nil
	}
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil {
		return nil, err
	}
	for k, v := range r.Extra {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	return json.Marshal(m)
}

// CountTokensResult 是 /v1/messages/count_tokens 的返回。
//
// InputTokens 是**估算值**（CJK 约 1 字符 1 token、其余约 4 字符 1 token，
// 每个 block 再加 4 的结构开销）：网关拿不到上游分词器，口径刻意偏高 ——
// 高估会让客户端少塞一点但请求仍能过，低估会让真实请求被上游以"上下文过长"拒。
type CountTokensResult struct {
	InputTokens int `json:"input_tokens"`
}
