package wbsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// TapService 封装「抓包」（/api/tap）——把网关收到的每一轮对话请求落库，
// 便于排查「agent 到底发了什么」。
//
// 注意：抓包库里存的是**完整对话内容**（含 system prompt 与工具结果）。
// 读会话 / 读轮次要求管理员，且每次都留审计。
//
// 权限分两档：读（`Config` `Sessions` `Session` `Turn` `Diff` `ExportSession`）
// 接受管理面 API Token（`wbt_`）；**写**（`SaveConfig` `Backfill` `Prune`
// `DeleteSession` `Clear`）服务端只认会话登录（require_session_admin），用
// Token 会拿到 403 —— 调用它们请用 WithAdminLogin 建客户端。
type TapService struct{ c *Client }

// Tap 返回抓包服务。
func (c *Client) Tap() *TapService { return &TapService{c: c} }

// TapConfig 是抓包的开关与容量上限。
type TapConfig struct {
	Enabled bool `json:"enabled"`
	// MaxBody 是单条请求体最多存多少字节（4 KiB ~ 8 MiB）。
	MaxBody int `json:"max_body"`
	// RetentionDays 是保留天数（1 ~ 90）。
	RetentionDays int `json:"retention_days"`
	// MaxSessions 是最多保留多少个会话（10 ~ 5000）。
	MaxSessions int `json:"max_sessions"`
}

// TapStats 是抓包库当前的体量统计。
type TapStats struct {
	Sessions         int    `json:"sessions"`
	Turns            int    `json:"turns"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	FirstAt          *int64 `json:"first_at"`
	LastAt           *int64 `json:"last_at"`
}

// TapConfigResponse 是 GET / PUT /api/tap/config 的返回。
type TapConfigResponse struct {
	Config TapConfig `json:"config"`
	Stats  TapStats  `json:"stats"`
}

// TapSession 是会话列表里的一条。
type TapSession struct {
	ID          int64  `json:"id"`
	Fingerprint string `json:"fingerprint"`
	KeyID       *int64 `json:"key_id"`
	KeyName     string `json:"key_name"`
	// Client 是从 UA 归类出来的客户端名（claude-code / codex / …）。
	Client   string `json:"client"`
	ClientUA string `json:"client_ua"`
	// Kind 是 "main"（主会话）或辅助调用（标题生成等）。
	Kind             string `json:"kind"`
	ParentID         *int64 `json:"parent_id"`
	Protocol         string `json:"protocol"`
	Model            string `json:"model"`
	StartedAt        int64  `json:"started_at"`
	UpdatedAt        int64  `json:"updated_at"`
	Turns            int    `json:"turns"`
	AuxCount         int    `json:"aux_count"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
}

// TapAux 是挂在主会话下的辅助调用。
type TapAux struct {
	ID    int64  `json:"id"`
	Kind  string `json:"kind"`
	TS    int64  `json:"ts"`
	Model string `json:"model"`
}

// TapTurnSummary 是会话详情里的一轮摘要（不含正文）。
type TapTurnSummary struct {
	ID  int64 `json:"id"`
	Seq int   `json:"seq"`
	TS  int64 `json:"ts"`
	// Status 是这一轮返回给客户端的 HTTP 状态码；nil = 没采到（存量行）。
	// 上游那一列是 INTEGER，不是 "ok"/"error" 这类字串。
	Status       *int   `json:"status"`
	Stream       bool   `json:"stream"`
	LatencyMS    *int   `json:"latency_ms"`
	FirstTokenMS *int   `json:"first_token_ms"`
	Model        string `json:"model"`
	MappedModel  string `json:"mapped_model"`
	// 四个 token 计数都是指针：上游那几列可空，NULL 是「没采到」而不是 0
	// ——0 token 是一个真实结果（例如失败请求），两者混起来会让用量统计失真。
	PromptTokens     *int     `json:"prompt_tokens"`
	CompletionTokens *int     `json:"completion_tokens"`
	CacheHitTokens   *int     `json:"cache_hit_tokens"`
	CacheMissTokens  *int     `json:"cache_miss_tokens"`
	Cost             *float64 `json:"cost"`
	Error            string   `json:"error"`
	Truncated        bool     `json:"truncated"`
	Protocol         string   `json:"protocol"`
	RequestBytes     int      `json:"request_bytes"`
	ResponseBytes    int      `json:"response_bytes"`
}

// TapSessionDetail 是单个会话的详情（会话头 + 每轮摘要）。
type TapSessionDetail struct {
	ID               int64            `json:"id"`
	Fingerprint      string           `json:"fingerprint"`
	KeyID            *int64           `json:"key_id"`
	KeyName          string           `json:"key_name"`
	Client           string           `json:"client"`
	ClientUA         string           `json:"client_ua"`
	Kind             string           `json:"kind"`
	ParentID         *int64           `json:"parent_id"`
	Protocol         string           `json:"protocol"`
	Model            string           `json:"model"`
	StartedAt        int64            `json:"started_at"`
	UpdatedAt        int64            `json:"updated_at"`
	Turns            int              `json:"turns"`
	Aux              []TapAux         `json:"aux"`
	PromptTokens     int              `json:"prompt_tokens"`
	CompletionTokens int              `json:"completion_tokens"`
	Items            []TapTurnSummary `json:"items"`
}

// TapSummary 是请求体的结构摘要。
type TapSummary struct {
	Messages    int            `json:"messages"`
	Roles       map[string]int `json:"roles"`
	Tools       int            `json:"tools"`
	SystemChars int            `json:"system_chars"`
	ToolNames   []string       `json:"tool_names"`
}

// TapTurn 是一轮请求的记录。
//
// 正文（Request / Response / Recon / Usage）随协议而变（Chat 透传、
// Anthropic / Responses 各有自己的报文），因此这里保留为 json.RawMessage：
// 要判结构由调用方按协议自己解。RequestOK / ResponseOK 说明正文是否解析成功
// ——截断过的记录必然解析不了，Truncated 为真。
type TapTurn struct {
	ID          int64  `json:"id"`
	SessionID   int64  `json:"session_id"`
	Seq         int    `json:"seq"`
	TS          int64  `json:"ts"`
	Path        string `json:"path"`
	Protocol    string `json:"protocol"`
	Model       string `json:"model"`
	MappedModel string `json:"mapped_model"`
	// Status 同 TapTurnSummary.Status：HTTP 状态码，nil = 没采到。
	Status       *int `json:"status"`
	Stream       bool `json:"stream"`
	LatencyMS    *int `json:"latency_ms"`
	FirstTokenMS *int `json:"first_token_ms"`
	// 四个 token 计数可空（NULL = 没采到，不是 0），见 TapTurnSummary。
	PromptTokens     *int            `json:"prompt_tokens"`
	CompletionTokens *int            `json:"completion_tokens"`
	CacheHitTokens   *int            `json:"cache_hit_tokens"`
	CacheMissTokens  *int            `json:"cache_miss_tokens"`
	Cost             *float64        `json:"cost"`
	Error            string          `json:"error"`
	Truncated        bool            `json:"truncated"`
	Request          json.RawMessage `json:"request"`
	RequestOK        bool            `json:"request_ok"`
	RequestRaw       string          `json:"request_raw"`
	RequestText      string          `json:"request_text"`
	Response         json.RawMessage `json:"response"`
	ResponseOK       bool            `json:"response_ok"`
	ResponseRaw      string          `json:"response_raw"`
	Recon            json.RawMessage `json:"recon"`
	Usage            json.RawMessage `json:"usage"`
	TraceID          string          `json:"trace_id"`
	Summary          *TapSummary     `json:"summary"`

	// ── 两端视角（同一轮在网关两侧的两份报文）──────────────────
	//
	// `Request` / `Response` 是**上游那一跳**的样子；`ClientResponse` 才是
	// 客户端真正收到的那份。对 Chat（透传）两者相同，对 Anthropic / Responses
	// （要翻译）是两份不同的报文——只看 Response 会把翻译层的输出当成客户端
	// 的输入，排障时得出完全相反的结论。
	UpstreamResponse   json.RawMessage `json:"upstream_response"`
	UpstreamResponseOK bool            `json:"upstream_response_ok"`
	UpstreamRecon      json.RawMessage `json:"upstream_recon"`
	// ClientResponse 在存量行（v2.11 之前抓的）上是 null。
	ClientResponse    json.RawMessage `json:"client_response"`
	ClientResponseOK  bool            `json:"client_response_ok"`
	ClientResponseRaw *string         `json:"client_response_raw"`
	ClientRecon       json.RawMessage `json:"client_recon"`
	// Passthrough 表示这一跳没有翻译（Chat 协议，或客户端视角没采集到）。
	Passthrough bool `json:"passthrough"`
	// ClientProtocol 是解读 ClientResponse 用的协议（chat 透传时同 Protocol）。
	ClientProtocol string `json:"client_protocol"`
	// RequestDigest 是请求体的结构摘要（截断前算的），截断场景下的 diff 用它。
	RequestDigest json.RawMessage `json:"request_digest"`
}

// TapDiffParam 是顶层参数的一处差异。
type TapDiffParam struct {
	Key    string  `json:"key"`
	Before *string `json:"before"`
	After  *string `json:"after"`
}

// TapDiffSystem 是 system 段的一处差异。
type TapDiffSystem struct {
	Changed     bool   `json:"changed"`
	BeforeChars int    `json:"before_chars"`
	AfterChars  int    `json:"after_chars"`
	Before      string `json:"before"`
	After       string `json:"after"`
}

// TapDiffMessage 是一条消息的差异。
type TapDiffMessage struct {
	Index   int    `json:"index"`
	Role    string `json:"role"`
	Preview string `json:"preview,omitempty"`
	Before  string `json:"before,omitempty"`
	After   string `json:"after,omitempty"`
	Chars   int    `json:"chars,omitempty"`
}

// TapDiffMessages 是消息段的差异。
type TapDiffMessages struct {
	BeforeTotal int              `json:"before_total"`
	AfterTotal  int              `json:"after_total"`
	Common      int              `json:"common"`
	Added       []TapDiffMessage `json:"added"`
	Removed     []TapDiffMessage `json:"removed"`
	Changed     []TapDiffMessage `json:"changed"`
}

// TapDiffTool 是一处工具定义的差异。
type TapDiffTool struct {
	Name   string `json:"name"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// TapDiffTools 是工具段的差异。
type TapDiffTools struct {
	Added   []string      `json:"added"`
	Removed []string      `json:"removed"`
	Changed []TapDiffTool `json:"changed"`
}

// TapDiffSummary 是差异的计数总览。
type TapDiffSummary struct {
	Params  int `json:"params"`
	System  int `json:"system"`
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Changed int `json:"changed"`
	Tools   int `json:"tools"`
}

// TapDiffRef 是被比较的那一轮的标识。
type TapDiffRef struct {
	ID    int64  `json:"id"`
	Seq   int    `json:"seq"`
	TS    int64  `json:"ts"`
	Model string `json:"model"`
}

// TapDiff 是两轮请求的结构化差异。
type TapDiff struct {
	Before   TapDiffRef      `json:"before"`
	After    TapDiffRef      `json:"after"`
	Params   []TapDiffParam  `json:"params"`
	System   TapDiffSystem   `json:"system"`
	Messages TapDiffMessages `json:"messages"`
	Tools    TapDiffTools    `json:"tools"`
	Summary  TapDiffSummary  `json:"summary"`
}

// TapBackfillResult 是按 UA 重新归类历史会话的结果。
type TapBackfillResult struct {
	// Changed 是被重新归类的会话数；Aux 是补上归属的辅助调用数。
	Changed int      `json:"changed"`
	Aux     int      `json:"aux"`
	Stats   TapStats `json:"stats"`
}

// TapPruneResult 是按保留策略清理的结果。
type TapPruneResult struct {
	Turns int      `json:"turns"`
	Stats TapStats `json:"stats"`
}

// TapClearResult 是清空抓包库的结果。
type TapClearResult struct {
	OK    bool `json:"ok"`
	Turns int  `json:"turns"`
}

// TapExport 是导出的自包含 HTML。
type TapExport struct {
	Filename string
	HTML     []byte
}

// Config 读取抓包开关与容量（任何登录用户都能读）。
func (s *TapService) Config(ctx context.Context) (*TapConfigResponse, error) {
	var out TapConfigResponse
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/tap/config", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveConfig 改抓包开关与上限（写入即生效，不需要重启服务）。
//
// 只提交要改的字段（nil 指针 / 空切片表示「不动它」）。
func (s *TapService) SaveConfig(ctx context.Context, patch TapConfigPatch) (*TapConfigResponse, error) {
	var out TapConfigResponse
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/tap/config", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TapConfigPatch 是抓包配置的局部更新。
type TapConfigPatch struct {
	Enabled       *bool `json:"enabled,omitempty"`
	MaxBody       *int  `json:"max_body,omitempty"`
	RetentionDays *int  `json:"retention_days,omitempty"`
	MaxSessions   *int  `json:"max_sessions,omitempty"`
}

// Backfill 按入库的 UA 重新归类历史会话（会**改历史数据**）。
func (s *TapService) Backfill(ctx context.Context) (*TapBackfillResult, error) {
	var out TapBackfillResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/tap/backfill", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Prune 立刻按保留策略清理一次。
func (s *TapService) Prune(ctx context.Context) (*TapPruneResult, error) {
	var out TapPruneResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/tap/prune", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Sessions 分页列出会话。
//
// IncludeAux 为 true 时把辅助调用（标题生成等）也单列出来。
func (s *TapService) Sessions(ctx context.Context, q TapSessionQuery) (*Page[TapSession], error) {
	vals := url.Values{}
	if q.Page > 0 {
		vals.Set("page", strconv.Itoa(q.Page))
	}
	if q.Size > 0 {
		vals.Set("size", strconv.Itoa(q.Size))
	}
	if q.Days > 0 {
		vals.Set("days", strconv.Itoa(q.Days))
	}
	if q.Client != "" {
		vals.Set("client", q.Client)
	}
	if q.Q != "" {
		vals.Set("q", q.Q)
	}
	if q.IncludeAux {
		vals.Set("include_aux", "true")
	}
	var out Page[TapSession]
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/tap/sessions", vals, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TapSessionQuery 是会话列表的筛选条件。
type TapSessionQuery struct {
	Page int
	Size int
	Days int
	// Client 是客户端名（claude-code / codex / …）；"all" 表示不限。
	Client string
	// Q 在模型名与指纹里做模糊匹配。
	Q string
	// IncludeAux 为 true 时把辅助调用也单列出来。
	IncludeAux bool
}

// Session 读取单个会话的详情（会话头 + 每轮摘要）。
func (s *TapService) Session(ctx context.Context, id int64) (*TapSessionDetail, error) {
	var out TapSessionDetail
	if _, err := s.c.doJSON(ctx, http.MethodGet, tapSessionPath(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSession 删除一个会话及其全部轮次。
func (s *TapService) DeleteSession(ctx context.Context, id int64) error {
	_, err := s.c.doJSON(ctx, http.MethodDelete, tapSessionPath(id), nil, nil, nil)
	return err
}

// Turn 读取一轮请求的完整记录（含正文）。
func (s *TapService) Turn(ctx context.Context, id int64) (*TapTurn, error) {
	var out TapTurn
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/tap/turns/"+strconv.FormatInt(id, 10),
		nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Diff 比较两轮请求的结构化差异（before / after 是**轮次 id**，不是 seq）。
func (s *TapService) Diff(ctx context.Context, before, after int64) (*TapDiff, error) {
	vals := url.Values{}
	vals.Set("before", strconv.FormatInt(before, 10))
	vals.Set("after", strconv.FormatInt(after, 10))
	var out TapDiff
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/tap/diff", vals, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ExportSession 导出会话为**自包含 HTML**（服务端带 attachment 头，浏览器会
// 当成文件下载而不是打开页面）。
//
// 响应体走的是 HTML 而不是 JSON，所以单独发 Accept: text/html。
func (s *TapService) ExportSession(ctx context.Context, id int64) (*TapExport, error) {
	res, err := s.c.doRawAccept(ctx, http.MethodGet, tapSessionPath(id)+"/export",
		nil, nil, acceptHTML)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, wrapStatus(newAPIErrorWithHeader(res.StatusCode, res.Body, res.Header))
	}
	// 响应体只读到 maxRespBytes 为止。正好读满说明**大概率被截断了** ——
	// 一份半截的 HTML 比报错更糟：它会安静地少掉后半段对话，而调用方以为
	// 拿到了完整文件。宁可在这里失败。
	if len(res.Body) >= maxRespBytes {
		return nil, fmt.Errorf(
			"wbsdk: 导出的会话超过 %d MiB 上限（可能已被截断），请分批导出或直接从服务端取",
			maxRespBytes>>20)
	}
	name := "workbuddy-trace.html"
	if cd := res.Header.Get("Content-Disposition"); cd != "" {
		if i := strings.IndexByte(cd, '"'); i >= 0 {
			if j := strings.IndexByte(cd[i+1:], '"'); j >= 0 {
				name = cd[i+1 : i+1+j]
			}
		}
	}
	return &TapExport{Filename: name, HTML: res.Body}, nil
}

// Clear 清空整个抓包库。
func (s *TapService) Clear(ctx context.Context) (*TapClearResult, error) {
	var out TapClearResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/tap/clear", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func tapSessionPath(id int64) string {
	return "/api/tap/sessions/" + strconv.FormatInt(id, 10)
}
