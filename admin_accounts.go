package wbsdk

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// AccountsService 封装 /api 下的账号管理与签到接口。
type AccountsService struct{ c *Client }

// Accounts 返回账号服务。
func (c *Client) Accounts() *AccountsService { return &AccountsService{c: c} }

func accountPath(file string) string {
	return "/api/accounts/" + url.PathEscape(file)
}

func (c *Client) effectiveRealm(r Realm) Realm {
	if r != RealmAny {
		return r
	}
	if def := c.Realm(); def != RealmAny {
		return def
	}
	return RealmCN
}

// List 拉取账号列表（本地授权信息 + 上游运行时状态）。
func (s *AccountsService) List(ctx context.Context) (*AccountsResponse, error) {
	var out AccountsResponse
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/accounts", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// StartAuth 发起扫码登录；realm 为空时使用客户端默认版本。
func (s *AccountsService) StartAuth(ctx context.Context, realm Realm) (*AuthStart, error) {
	body := map[string]string{"realm": string(s.c.effectiveRealm(realm))}
	var out AuthStart
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/auth/start", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PollAuth 轮询扫码结果。region 仅国际版需要（新号必须先做地区注册，如 "HK"）。
func (s *AccountsService) PollAuth(ctx context.Context, state string, realm Realm, region string) (*AuthPoll, error) {
	q := url.Values{}
	q.Set("state", state)
	if realm != RealmAny {
		q.Set("realm", string(realm))
	}
	if region != "" {
		q.Set("region", region)
	}
	var out AuthPoll
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/auth/poll", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Checkin 对单个账号执行一次签到。
func (s *AccountsService) Checkin(ctx context.Context, file string) (*CheckinResult, error) {
	var out CheckinResult
	path := accountPath(file) + "/checkin"
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckinAll 对所有可签到账号执行一次签到。
func (s *AccountsService) CheckinAll(ctx context.Context) (*CheckinAllResult, error) {
	var out CheckinAllResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/accounts/checkin-all", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Credits 查询单个账号的实时积分（直接向腾讯查询）。force=false 时命中服务端 60s 缓存。
func (s *AccountsService) Credits(ctx context.Context, file string, force bool) (*Credits, error) {
	q := url.Values{}
	q.Set("force", strconv.FormatBool(force))
	var out Credits
	path := accountPath(file) + "/credits"
	if _, err := s.c.doJSON(ctx, http.MethodGet, path, q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RefreshCredits 并发刷新所有账号的实时积分。
func (s *AccountsService) RefreshCredits(ctx context.Context, force bool) (*CreditsRefresh, error) {
	q := url.Values{}
	q.Set("force", strconv.FormatBool(force))
	var out CreditsRefresh
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/accounts/refresh-credits", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetDisabled 临时停用 / 启用一个账号。
func (s *AccountsService) SetDisabled(ctx context.Context, file string, disabled bool) (*SetDisabledResult, error) {
	var out SetDisabledResult
	path := accountPath(file) + "/disabled"
	body := map[string]bool{"disabled": disabled}
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetNote 给账号写备注（空串 = 清除）。备注按 uid 存本端库，停用不丢。
func (s *AccountsService) SetNote(ctx context.Context, file, note string) (*NoteResult, error) {
	var out NoteResult
	path := accountPath(file) + "/note"
	body := map[string]string{"note": note}
	if _, err := s.c.doJSON(ctx, http.MethodPut, path, nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Test 对账号做连通性探测。
func (s *AccountsService) Test(ctx context.Context, file string) (*ActionResult, error) {
	var out ActionResult
	path := accountPath(file) + "/test"
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Refresh 刷新账号令牌（真的续期并保存）。
func (s *AccountsService) Refresh(ctx context.Context, file string) (*ActionResult, error) {
	var out ActionResult
	path := accountPath(file) + "/refresh"
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ClearCooling 强制清除账号级冷却、熔断/降权与模型级限流（会重启一次上游）。
func (s *AccountsService) ClearCooling(ctx context.Context, file string) (*ActionResult, error) {
	var out ActionResult
	path := accountPath(file) + "/clear-cooling"
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete 删除一个账号（会丢凭证，需重新扫码）。
func (s *AccountsService) Delete(ctx context.Context, file string) (*SuccessResult, error) {
	var out SuccessResult
	if _, err := s.c.doJSON(ctx, http.MethodDelete, accountPath(file), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckinLogQuery 筛选签到记录。
type CheckinLogQuery struct {
	Limit  int
	Offset int
	UID    string
	Days   int
	Realm  Realm
}

// CheckinLogs 分页拉取签到记录。
func (s *AccountsService) CheckinLogs(ctx context.Context, q CheckinLogQuery) (*CheckinLogPage, error) {
	vals := url.Values{}
	if q.Limit > 0 {
		vals.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Offset > 0 {
		vals.Set("offset", strconv.Itoa(q.Offset))
	}
	if q.UID != "" {
		vals.Set("uid", q.UID)
	}
	if q.Days > 0 {
		vals.Set("days", strconv.Itoa(q.Days))
	}
	if q.Realm != RealmAny {
		vals.Set("realm", string(q.Realm))
	}
	var out CheckinLogPage
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/checkin-logs", vals, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ClearCheckinLogs 清空签到记录。
func (s *AccountsService) ClearCheckinLogs(ctx context.Context) error {
	_, err := s.c.doJSON(ctx, http.MethodPost, "/api/checkin-logs/clear", nil, nil, nil)
	return err
}

// TaskLogQuery 筛选自动任务记录。
type TaskLogQuery struct {
	Limit  int
	Offset int
	Kind   string
	UID    string
	Days   int
	Realm  Realm
}

// TaskLogs 分页拉取上游自动任务记录。
func (s *AccountsService) TaskLogs(ctx context.Context, q TaskLogQuery) (*TaskLogResponse, error) {
	vals := url.Values{}
	if q.Limit > 0 {
		vals.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Offset > 0 {
		vals.Set("offset", strconv.Itoa(q.Offset))
	}
	if q.Kind != "" {
		vals.Set("kind", q.Kind)
	}
	if q.UID != "" {
		vals.Set("uid", q.UID)
	}
	if q.Days > 0 {
		vals.Set("days", strconv.Itoa(q.Days))
	}
	if q.Realm != RealmAny {
		vals.Set("realm", string(q.Realm))
	}
	var out TaskLogResponse
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/task-logs", vals, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CollectTaskLogs 立即采集一次上游自动任务日志。
func (s *AccountsService) CollectTaskLogs(ctx context.Context) (*ActionResult, error) {
	var out ActionResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/task-logs/collect", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ClearTaskLogs 清空自动任务记录。
func (s *AccountsService) ClearTaskLogs(ctx context.Context) error {
	_, err := s.c.doJSON(ctx, http.MethodPost, "/api/task-logs/clear", nil, nil, nil)
	return err
}

// UpstreamLogs 读取上游容器日志（与自动任务相关的行）。
func (s *AccountsService) UpstreamLogs(ctx context.Context, limit int) (*UpstreamLogs, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var out UpstreamLogs
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/upstream/logs", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Restart 重启上游（一般无需手动调用，保存配置会自动重载）。
func (s *AccountsService) Restart(ctx context.Context) (*ActionResult, error) {
	var out ActionResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/restart", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
