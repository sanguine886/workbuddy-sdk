package wbsdk

import (
	"context"
	"net/http"
	"net/url"
)

// AccountRiskService 封装账号风险预警（/api/account-risk）。
//
// 预警来自「上游刷新令牌被限流 / 需要人工复核」这类事件；面板把它展示出来，
// 并允许确认（acknowledge）或尝试解除保护暂停（resume）。
type AccountRiskService struct{ c *Client }

// AccountRisk 返回账号风险预警服务。
func (c *Client) AccountRisk() *AccountRiskService { return &AccountRiskService{c: c} }

// AccountRiskQuery 是风险预警列表的筛选条件。
type AccountRiskQuery struct {
	// UpstreamID 指定分组；nil = 默认分组。
	UpstreamID *int
	// AllGroups 跨全部分组查（MultiGroup 部署时才需要）。
	AllGroups bool
	Realm     Realm
	// IncludeAcknowledged 为 true 时把已确认的预警也一并返回。
	IncludeAcknowledged bool
}

// AccountRiskWarning 是一条账号风险预警。
type AccountRiskWarning struct {
	ID          string `json:"id"`
	UID         string `json:"uid"`
	Nickname    string `json:"nickname"`
	Realm       Realm  `json:"realm"`
	Operation   string `json:"operation"`
	HTTPStatus  int    `json:"http_status"`
	TS          int64  `json:"ts"`
	ActionError string `json:"action_error"`
	// Paused 表示该账号当前确实处于刷新保护暂停中；nil = 上游状态未知。
	Paused       *bool  `json:"paused"`
	PauseUntil   int64  `json:"pause_until"`
	NeedsReview  bool   `json:"needs_review"`
	Failures     int    `json:"failures"`
	Group        string `json:"group"`
	Historical   bool   `json:"historical"`
	Acknowledged bool   `json:"acknowledged"`
}

// AccountRiskList 是 GET /api/account-risk 的返回。
type AccountRiskList struct {
	Items        []AccountRiskWarning `json:"items"`
	FailedGroups []string             `json:"failed_groups"`
	Total        int                  `json:"total"`
}

// List 列出账号风险预警。
func (s *AccountRiskService) List(ctx context.Context, q AccountRiskQuery) (*AccountRiskList, error) {
	vals := s.c.groupQ(q.UpstreamID)
	if q.AllGroups {
		vals.Set("all_groups", "true")
	}
	if q.Realm != RealmAny {
		vals.Set("realm", string(q.Realm))
	}
	if q.IncludeAcknowledged {
		vals.Set("include_acknowledged", "true")
	}
	var out AccountRiskList
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/account-risk", vals, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Acknowledge 确认一条预警（它不再出现在默认列表里）。
func (s *AccountRiskService) Acknowledge(ctx context.Context, ident string) error {
	_, err := s.c.doJSON(ctx, http.MethodPost,
		"/api/account-risk/"+url.PathEscape(ident)+"/acknowledge", nil, nil, nil)
	return err
}

// Resume 解除账号的刷新保护暂停。
//
// 服务端解不掉时返回 **409**（暂停尚未解除）；用 errors.As 取 *APIError 看
// StatusCode 与 Message 即可分辨。
func (s *AccountRiskService) Resume(ctx context.Context, ident string) error {
	_, err := s.c.doJSON(ctx, http.MethodPost,
		"/api/account-risk/"+url.PathEscape(ident)+"/resume", nil, nil, nil)
	return err
}
