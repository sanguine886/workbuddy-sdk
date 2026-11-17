package wbsdk

import (
	"context"
	"net/http"
)

// WatchdogService 封装「上游网页改版监控」（/api/watchdog）。
//
// 监控定期抓取几个公开页面，结构变了就让 LLM 判读，再走邮件 / QQ 通知。
// 全部接口都要求管理员：状态里有账号昵称与收件地址，配置里有凭据（已打码）。
type WatchdogService struct{ c *Client }

// Watchdog 返回监控服务。
func (c *Client) Watchdog() *WatchdogService { return &WatchdogService{c: c} }

// WatchdogConfig 是监控配置。
//
// Mail.Password / LLM.APIKey / QQ.APIKey 只回掩码（"********"）。保存时传回
// 掩码值表示「沿用已存的值」，传空串表示清除。
type WatchdogConfig struct {
	Enabled         bool             `json:"enabled"`
	IntervalMinutes int              `json:"interval_minutes"`
	Targets         []WatchdogTarget `json:"targets"`
	TimeoutSeconds  int              `json:"timeout_seconds"`
	// AssetLimit 是 kind=assets 时最多抓几个静态资源（0 = 只比页面骨架）。
	AssetLimit int `json:"asset_limit"`
	// NotifyOn 取值 change | claimable | anomaly。
	NotifyOn            string       `json:"notify_on"`
	ScanCompensation    bool         `json:"scan_compensation"`
	AutoClaim           bool         `json:"auto_claim"`
	AllowPrivateTargets bool         `json:"allow_private_targets"`
	LLM                 WatchdogLLM  `json:"llm"`
	Mail                WatchdogMail `json:"mail"`
	QQ                  WatchdogQQ   `json:"qq"`
}

// WatchdogTarget 是一个监控目标。
type WatchdogTarget struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	// Kind 取值 html | js | text | assets（assets = 连页面骨架一起比静态资源）。
	Kind    string `json:"kind"`
	Enabled bool   `json:"enabled"`
}

// WatchdogLLM 是判读用的 LLM 配置（OpenAI 兼容接口）。
type WatchdogLLM struct {
	Enabled        bool   `json:"enabled"`
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	Model          string `json:"model"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// WatchdogMail 是邮件通知配置。
type WatchdogMail struct {
	Enabled       bool     `json:"enabled"`
	Host          string   `json:"host"`
	Port          int      `json:"port"`
	Security      string   `json:"security"` // ssl | starttls | none
	Username      string   `json:"username"`
	Password      string   `json:"password"`
	From          string   `json:"from"`
	To            []string `json:"to"`
	SubjectPrefix string   `json:"subject_prefix"`
}

// WatchdogQQ 是 QQ 通知配置（走 AstrBot 的 OpenAPI）。
type WatchdogQQ struct {
	Enabled bool `json:"enabled"`
	// BaseURL 是 AstrBot 的根地址（如 https://astrbot.example.com）。
	BaseURL string `json:"base_url"`
	// APIKey 是 AstrBot「设置 → OpenAPI」创建的 abk_ 密钥。
	APIKey string `json:"api_key"`
	// UMO 形如 platform:消息类型:会话ID，如 qq_official:GroupMessage:<群openid>。
	UMO            string `json:"umo"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// WatchdogConfigPatch 是监控配置的**局部**更新（服务端以当前配置为底合并）。
type WatchdogConfigPatch struct {
	Enabled         *bool `json:"enabled,omitempty"`
	IntervalMinutes *int  `json:"interval_minutes,omitempty"`
	// Targets 传的就是**完整新列表**（服务端整体替换，不是按名字合并）。
	// 注意它必须有至少一个目标，传空切片会被服务端拒（400）；nil = 不改。
	Targets             []WatchdogTarget   `json:"targets,omitempty"`
	TimeoutSeconds      *int               `json:"timeout_seconds,omitempty"`
	AssetLimit          *int               `json:"asset_limit,omitempty"`
	NotifyOn            *string            `json:"notify_on,omitempty"`
	ScanCompensation    *bool              `json:"scan_compensation,omitempty"`
	AutoClaim           *bool              `json:"auto_claim,omitempty"`
	AllowPrivateTargets *bool              `json:"allow_private_targets,omitempty"`
	LLM                 *WatchdogLLMPatch  `json:"llm,omitempty"`
	Mail                *WatchdogMailPatch `json:"mail,omitempty"`
	QQ                  *WatchdogQQPatch   `json:"qq,omitempty"`
}

// WatchdogLLMPatch 是 LLM 段的局部更新。
//
// 字符串字段全是指针：nil = 不改，`Ptr("")` = **清空**（服务端对空串是有
// 意义的，它会把该字段设成空串而不是保持原值）。
type WatchdogLLMPatch struct {
	Enabled        *bool   `json:"enabled,omitempty"`
	BaseURL        *string `json:"base_url,omitempty"`
	APIKey         *string `json:"api_key,omitempty"`
	Model          *string `json:"model,omitempty"`
	TimeoutSeconds *int    `json:"timeout_seconds,omitempty"`
}

// WatchdogMailPatch 是邮件段的局部更新。
//
// 字符串字段全是指针：nil = 不改，`Ptr("")` = 清空。密码尤其要注意 ——
// 「清空密码」和「不改密码」是两件不同的事，用值类型分不出来。
// Security 不是指针：它只接受 ssl / starttls / none，空串没有意义（服务端忽略）。
type WatchdogMailPatch struct {
	Enabled       *bool    `json:"enabled,omitempty"`
	Host          *string  `json:"host,omitempty"`
	Port          *int     `json:"port,omitempty"`
	Security      string   `json:"security,omitempty"`
	Username      *string  `json:"username,omitempty"`
	Password      *string  `json:"password,omitempty"`
	From          *string  `json:"from,omitempty"`
	To            []string `json:"to,omitempty"`
	SubjectPrefix *string  `json:"subject_prefix,omitempty"`
}

// WatchdogQQPatch 是 QQ 段的局部更新。
//
// 字符串字段全是指针：nil = 不改，`Ptr("")` = 清空。
type WatchdogQQPatch struct {
	Enabled        *bool   `json:"enabled,omitempty"`
	BaseURL        *string `json:"base_url,omitempty"`
	APIKey         *string `json:"api_key,omitempty"`
	UMO            *string `json:"umo,omitempty"`
	TimeoutSeconds *int    `json:"timeout_seconds,omitempty"`
}

// WatchdogConfigEnvelope 是 GET /api/watchdog/config 的返回。
type WatchdogConfigEnvelope struct {
	Config WatchdogConfig `json:"config"`
}

// WatchdogSaveResult 是 PUT /api/watchdog/config 的返回。
type WatchdogSaveResult struct {
	Config WatchdogConfig `json:"config"`
	// MailEgress 是开启邮件时顺带做的 SMTP 出口探测结果。
	MailEgress *WatchdogEgress `json:"mail_egress,omitempty"`
	// Warning 非空时说明配置被服务端**改过**（例如邮件出口不通，enabled 被强制关闭）。
	Warning string `json:"warning,omitempty"`
}

// WatchdogState 是落库的运行状态。
type WatchdogState struct {
	LastRunAt  int64           `json:"last_run_at"`
	LastError  string          `json:"last_error"`
	Egress     *WatchdogEgress `json:"egress"`
	LastResult *WatchdogRun    `json:"last_result"`
}

// WatchdogStatus 是 GET /api/watchdog/status 的返回。
type WatchdogStatus struct {
	Config WatchdogConfig `json:"config"`
	State  WatchdogState  `json:"state"`
	// Due 表示按间隔已经到了该跑的时候。
	Due bool `json:"due"`
	// NextRunAt 是预计的下次运行时刻（没跑过时为 nil）。
	NextRunAt *int64 `json:"next_run_at"`
}

// WatchdogVerdict 是 LLM 对「页面变了没有」的判读。
type WatchdogVerdict struct {
	LayoutChanged bool     `json:"layout_changed"`
	Claimable     bool     `json:"claimable"`
	Anomaly       bool     `json:"anomaly"`
	Confidence    float64  `json:"confidence"`
	Summary       string   `json:"summary"`
	Evidence      []string `json:"evidence"`
	Actions       []string `json:"actions"`
	// Source 取值 llm | heuristic。
	Source string `json:"source"`
	// LLMError 非空表示 LLM 判读失败、这份结论是规则判读的退化结果。
	LLMError string `json:"llm_error"`
}

// WatchdogDiff 是结构差异的计数（不含原文）。
type WatchdogDiff struct {
	Added     int  `json:"added"`
	Removed   int  `json:"removed"`
	Truncated bool `json:"truncated"`
}

// WatchdogTargetResult 是单个目标的检测结果。
type WatchdogTargetResult struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Status string `json:"status"` // baseline_created | unchanged | changed
	// Digest 是本次抓到的内容指纹（截断到 16 位）。
	Digest string `json:"digest"`
	// PreviousDigest 是上一次的指纹；只有 Status == "changed" 才有。
	PreviousDigest string           `json:"previous_digest"`
	Lines          int              `json:"lines"`
	Diff           *WatchdogDiff    `json:"diff"`
	Verdict        *WatchdogVerdict `json:"verdict"`
}

// WatchdogError 是某个目标抓取失败的说明。
type WatchdogError struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

// WatchdogSend 是一个通知通道的发送结果。
type WatchdogSend struct {
	OK      bool   `json:"ok"`
	Skipped string `json:"skipped"`
	Error   string `json:"error"`
	SentAt  *int64 `json:"sent_at"`
	// UMO 是 QQ 通道实际发到的会话。
	UMO string `json:"umo"`
	// To 是邮件通道的收件人。
	To []string `json:"to"`
	// Status 是上游 HTTP 状态码（QQ 通道）。
	Status int `json:"status"`
	// Response 是上游响应摘要。
	Response string `json:"response"`
	// Egress 是邮件通道这次用的 SMTP 出口探测（10 分钟内复用上次结果）。
	// 出口不通时 Skipped == "SMTP 出口不通"，原因看这里。
	Egress *WatchdogEgress `json:"egress"`
}

// WatchdogRun 是一轮检测的结果。
//
// 注意：`Status().State.LastResult` 里存的是**同一份结果但少了 OK**（落库时
// 只存 state_result），所以从状态里读出来的 OK 恒为 false，别拿它判断成败——
// 那里看 Errors 是否为空即可。
type WatchdogRun struct {
	OK         bool                   `json:"ok"`
	Trigger    string                 `json:"trigger"` // manual | schedule
	StartedAt  int64                  `json:"started_at"`
	FinishedAt int64                  `json:"finished_at"`
	Targets    []WatchdogTargetResult `json:"targets"`
	// Changed 是发生变化的目标名。
	Changed []string        `json:"changed"`
	Errors  []WatchdogError `json:"errors"`
	// Scan 是活动补偿扫描的结果（没跑则为 nil）。
	Scan map[string]any `json:"scan"`
	// Claim 是自动领取补偿的结果。
	Claim map[string]any `json:"claim"`
	Mail  *WatchdogSend  `json:"mail"`
	QQ    *WatchdogSend  `json:"qq"`
	// Skipped 非空表示这轮**没跑**（已有一次在跑 / SMTP 出口不通），此时
	// 其余字段大多是零值——别把它当成「跑过了，什么都没变」。
	Skipped string `json:"skipped"`
	// Egress 是 Skipped == "SMTP 出口不通" 时的探测明细。
	Egress *WatchdogEgress `json:"egress"`
}

// WatchdogBaselineResult 是清除基线的返回。
type WatchdogBaselineResult struct {
	OK      bool   `json:"ok"`
	Cleared int    `json:"cleared"`
	Message string `json:"message"`
}

// WatchdogEgressPort 是某个 SMTP 端口的探测结果。
type WatchdogEgressPort struct {
	Port   int    `json:"port"`
	OK     bool   `json:"ok"`
	Banner string `json:"banner"`
	Error  string `json:"error"`
}

// WatchdogEgress 是部署环境到 SMTP 的出口探测结果。
type WatchdogEgress struct {
	OK        bool                 `json:"ok"`
	Host      string               `json:"host"`
	Port      int                  `json:"port"`
	Banner    string               `json:"banner"`
	Error     string               `json:"error"`
	Ports     []WatchdogEgressPort `json:"ports"`
	CheckedAt int64                `json:"checked_at"`
}

// WatchdogMailTest 是「发送测试邮件」的结果。
type WatchdogMailTest struct {
	OK      bool            `json:"ok"`
	Skipped string          `json:"skipped"`
	Error   string          `json:"error"`
	SentAt  *int64          `json:"sent_at"`
	To      []string        `json:"to"`
	Message string          `json:"message"`
	Egress  *WatchdogEgress `json:"egress"`
}

// WatchdogQQBots 是「检测 AstrBot 地址 / 密钥 / scope」的结果。
type WatchdogQQBots struct {
	OK     bool   `json:"ok"`
	Error  string `json:"error"`
	Status int    `json:"status"`
	// Bots 是可用的平台适配器（用来拼 UMO）；条目形状由 AstrBot 决定。
	Bots []map[string]any `json:"bots"`
}

// WatchdogQQTest 是「发送测试消息」的结果。
type WatchdogQQTest struct {
	OK       bool             `json:"ok"`
	Skipped  string           `json:"skipped"`
	Error    string           `json:"error"`
	Stage    string           `json:"stage"` // auth | send
	SentAt   *int64           `json:"sent_at"`
	UMO      string           `json:"umo"`
	Status   int              `json:"status"`
	Response string           `json:"response"`
	Bots     []map[string]any `json:"bots"`
}

// WatchdogLLMTest 是「测试 LLM 判读」的结果。
type WatchdogLLMTest struct {
	OK      bool             `json:"ok"`
	Verdict *WatchdogVerdict `json:"verdict"`
}

// Config 读取监控配置。
func (s *WatchdogService) Config(ctx context.Context) (*WatchdogConfigEnvelope, error) {
	var out WatchdogConfigEnvelope
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/watchdog/config", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveConfig 保存监控配置（局部更新：以当前配置为底合并）。
//
// 邮件开启时服务端会先探 SMTP 出口，探不通就把 enabled 强制改成 false 并在
// Warning 里说明——所以「邮件通知已开启」这句话在保存后始终成立。
func (s *WatchdogService) SaveConfig(ctx context.Context, patch WatchdogConfigPatch) (*WatchdogSaveResult, error) {
	var out WatchdogSaveResult
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/watchdog/config", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Status 读取监控状态（配置 + 上次运行 + 是否到期）。
func (s *WatchdogService) Status(ctx context.Context) (*WatchdogStatus, error) {
	var out WatchdogStatus
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/watchdog/status", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Run 立刻跑一轮检测（同步返回结果，通常几秒内结束）。
func (s *WatchdogService) Run(ctx context.Context) (*WatchdogRun, error) {
	var out WatchdogRun
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/watchdog/run", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ResetBaseline 清除全部基线快照（下次检测重新建立基线且**不告警**）。
func (s *WatchdogService) ResetBaseline(ctx context.Context) (*WatchdogBaselineResult, error) {
	var out WatchdogBaselineResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/watchdog/baseline", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckMailEgress 探测部署环境到 SMTP 的出口。
//
// patch 传当前表单值即可（不必先保存）；nil = 用已保存的配置。
func (s *WatchdogService) CheckMailEgress(ctx context.Context, patch *WatchdogMailPatch) (*WatchdogEgress, error) {
	var out WatchdogEgress
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/watchdog/mail/egress", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TestMail 发一封测试邮件（先探出口再真发）。
//
// patch 传当前表单值即可；nil = 用已保存的配置。测试**不看 enabled**：正是在
// 还没开启通知时才需要验证主机与收件人。
func (s *WatchdogService) TestMail(ctx context.Context, patch *WatchdogMailPatch) (*WatchdogMailTest, error) {
	var out WatchdogMailTest
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/watchdog/mail/test", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// QQBots 检测 AstrBot 地址 / 密钥 / scope，并列出可用平台适配器。
//
// 单独一个端点是为了把「密钥没 im 权限」与「UMO 填错」两类失败分得开。
func (s *WatchdogService) QQBots(ctx context.Context, patch *WatchdogQQPatch) (*WatchdogQQBots, error) {
	var out WatchdogQQBots
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/watchdog/qq/bots", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TestQQ 发一条测试消息（先验密钥 / scope，再真发）。
//
// Stage 为 "auth" 表示密钥或 scope 有问题，"send" 表示是发消息这一步失败。
func (s *WatchdogService) TestQQ(ctx context.Context, patch *WatchdogQQPatch) (*WatchdogQQTest, error) {
	var out WatchdogQQTest
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/watchdog/qq/test", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TestLLM 用一段样例差异测试 LLM 判读。
func (s *WatchdogService) TestLLM(ctx context.Context, patch *WatchdogLLMPatch) (*WatchdogLLMTest, error) {
	var out WatchdogLLMTest
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/watchdog/llm/test", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
