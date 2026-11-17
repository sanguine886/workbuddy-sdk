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
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/accounts", s.c.groupQ(nil), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AuthStartIn 是发起扫码登录的全部参数。
//
// region / upstream_id / proxy 看着像「高级选项」，但漏掉任何一个都会让流程
// **静默走到错的地方**：漏 region 是国际版新号落盘成功、随后聊天报 14017；
// 漏 upstream_id 是账号写进了默认分组而不是你以为的那个；漏 proxy 是走默认
// 线路（有些网络只有某条线路能连腾讯）。
type AuthStartIn struct {
	// Realm 决定国内版 / 国际版端点；RealmAny 时用客户端默认版本。
	// 传了 UID（定向重登）时**被服务端忽略**：版本跟随目标账号。
	Realm Realm
	// Region 是国际版的地区代码（如 "HK"）。仅国际版新号需要，国内版留空。
	Region string
	// UID 非空 = 定向重登这个账号（见 StartAuthFor）。
	UID string
	// UpstreamID 指定分组；nil = 默认分组。
	UpstreamID *int
	// Proxy 指定出站线路；nil = **不改**上游当前的默认线路。
	//
	// 陷阱：这是写进上游 config 的全局设置，所以 Ptr("") 是「显式切回默认
	// 线路」而不是「不改」—— 要保留现状就留 nil。
	Proxy *string
}

// StartAuth 发起扫码登录；realm 为空时使用客户端默认版本。
func (s *AccountsService) StartAuth(ctx context.Context, realm Realm) (*AuthStart, error) {
	return s.StartAuthIn(ctx, AuthStartIn{Realm: realm})
}

// StartAuthFor 对一个**已存在的账号**发起定向重登（uid 为空时等价于 StartAuth）。
//
// 传了 uid 就是「刷新这个账号的凭证」，此时 **realm 被忽略**：版本跟随目标账号，
// 而不是界面上切到的那个。返回体里的 Target 会告诉你是哪个账号。
// uid 找不到时服务端回 404。
func (s *AccountsService) StartAuthFor(ctx context.Context, realm Realm, uid string) (*AuthStart, error) {
	return s.StartAuthIn(ctx, AuthStartIn{Realm: realm, UID: uid})
}

// StartAuthIn 是带全部参数的发码入口（国际版地区、分组、出站线路）。
func (s *AccountsService) StartAuthIn(ctx context.Context, in AuthStartIn) (*AuthStart, error) {
	// 用 any 而不是 map[string]string：proxy 要能区分「没给」与「给了空串」。
	body := map[string]any{"realm": string(s.c.effectiveRealm(in.Realm))}
	if in.Region != "" {
		body["region"] = in.Region
	}
	if in.UID != "" {
		body["uid"] = in.UID
	}
	if in.Proxy != nil {
		body["proxy"] = *in.Proxy
	}
	var out AuthStart
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/auth/start",
		s.c.groupQ(in.UpstreamID), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AuthPollIn 是轮询扫码结果的参数。
type AuthPollIn struct {
	// State 是 StartAuth 返回的那串一次性单据，必填。
	State string
	// Realm 决定去哪一版问；RealmAny 时用客户端默认版本。
	Realm Realm
	// Region 是国际版地区代码。可以比 StartAuth 更晚到（用户在弹窗里才选），
	// 但不能只靠这里——后台轮询也要用它做地区注册。
	Region string
	// UpstreamID 必须与 StartAuthIn 传的**同一个**：落盘发生在 poll 这一步，
	// 分组对不上就写不进你那个分组。
	UpstreamID *int
}

// PollAuth 轮询扫码结果。region 仅国际版需要（新号必须先做地区注册，如 "HK"）。
func (s *AccountsService) PollAuth(ctx context.Context, state string, realm Realm, region string) (*AuthPoll, error) {
	return s.PollAuthIn(ctx, AuthPollIn{State: state, Realm: realm, Region: region})
}

// PollAuthIn 是带全部分组参数的轮询入口（与 StartAuthIn 配对使用）。
func (s *AccountsService) PollAuthIn(ctx context.Context, in AuthPollIn) (*AuthPoll, error) {
	q := s.c.groupQ(in.UpstreamID)
	q.Set("state", in.State)
	// 这里**不能**套 effectiveRealm：轮询的 realm 是**一致性校验**，不是路由。
	// 服务端拿它与发码时登记的版本比，不一致就回 realm_mismatch（不落盘）。
	// RealmAny 的含义是「我不知道、按登记的那个来」——折成 cn 会让国际版的码
	// 恒判不一致，调用方一直空转到码过期。发码侧（StartAuthIn）相反：那里的
	// realm 真的决定打哪一版端点，必须给出确定值。
	if in.Realm != RealmAny {
		q.Set("realm", string(in.Realm))
	}
	if in.Region != "" {
		q.Set("region", in.Region)
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
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, s.c.groupQ(nil), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckinAll 对所有可签到账号执行一次签到。
func (s *AccountsService) CheckinAll(ctx context.Context) (*CheckinAllResult, error) {
	var out CheckinAllResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/accounts/checkin-all", s.c.groupQ(nil), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Credits 查询单个账号的实时积分（直接向腾讯查询）。force=false 时命中服务端 60s 缓存。
func (s *AccountsService) Credits(ctx context.Context, file string, force bool) (*Credits, error) {
	q := s.c.groupQ(nil)
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
	return s.refreshCredits(ctx, force, "")
}

// RefreshCredit 只刷新**一个账号文件**的积分（多分组 / 单号排障时用）。
func (s *AccountsService) RefreshCredit(ctx context.Context, file string, force bool) (*CreditsRefresh, error) {
	return s.refreshCredits(ctx, force, file)
}

func (s *AccountsService) refreshCredits(ctx context.Context, force bool, file string) (*CreditsRefresh, error) {
	q := s.c.groupQ(nil)
	q.Set("force", strconv.FormatBool(force))
	if file != "" {
		q.Set("file", file)
	}
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
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, s.c.groupQ(nil), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetNote 给账号写备注（空串 = 清除）。备注按 uid 存本端库，停用不丢。
func (s *AccountsService) SetNote(ctx context.Context, file, note string) (*NoteResult, error) {
	var out NoteResult
	path := accountPath(file) + "/note"
	body := map[string]string{"note": note}
	if _, err := s.c.doJSON(ctx, http.MethodPut, path, s.c.groupQ(nil), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Test 对账号做连通性探测。
func (s *AccountsService) Test(ctx context.Context, file string) (*ActionResult, error) {
	var out ActionResult
	path := accountPath(file) + "/test"
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, s.c.groupQ(nil), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Refresh 刷新账号令牌（真的续期并保存）。
func (s *AccountsService) Refresh(ctx context.Context, file string) (*ActionResult, error) {
	var out ActionResult
	path := accountPath(file) + "/refresh"
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, s.c.groupQ(nil), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ClearCooling 强制清除账号级冷却、熔断/降权与模型级限流。
//
// 只取 ok / message；要看**走的是哪条路**（在线清除还是停上游改 state.json）
// 用 ClearCoolingDetail。
func (s *AccountsService) ClearCooling(ctx context.Context, file string) (*ActionResult, error) {
	var out ActionResult
	path := accountPath(file) + "/clear-cooling"
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, s.c.groupQ(nil), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ClearCoolingDetail 同 ClearCooling，但把成功明细也返回。
//
// 两条路径的明细**不一样**，所以字段都是可选的：
//   - 在线清除（不停上游，v2.11 起优先走这条）：Online=true + Models + Unfrozen；
//   - 离线清除（停上游 → 改 state.json → 启上游）：StateFile + Backup + Before。
//
// 别假定离线那三个字段一定存在 —— v2.10 起在线分支就不再返回它们了。
func (s *AccountsService) ClearCoolingDetail(ctx context.Context, file string) (*ClearCoolingResult, error) {
	var out ClearCoolingResult
	path := accountPath(file) + "/clear-cooling"
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, s.c.groupQ(nil), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete 删除一个账号（会丢凭证，需重新扫码）。
func (s *AccountsService) Delete(ctx context.Context, file string) (*SuccessResult, error) {
	var out SuccessResult
	if _, err := s.c.doJSON(ctx, http.MethodDelete, accountPath(file), s.c.groupQ(nil), nil, &out); err != nil {
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
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/restart", s.c.groupQ(nil), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetProxy 给账号绑定出口线路（空串 = 跟随池策略）。
//
// 改完服务端会重载上游：线路在上游是**加载时**解析进内存的，不重载不生效。
// upstreamID 为 nil 表示默认分组。
func (s *AccountsService) SetProxy(ctx context.Context, file, proxy string, upstreamID *int) (*AccountProxyResult, error) {
	var out AccountProxyResult
	body := map[string]string{"proxy": proxy}
	path := accountPath(file) + "/proxy"
	if _, err := s.c.doJSON(ctx, http.MethodPut, path, s.c.groupQ(upstreamID), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetWeight 设置账号手写权重（乘数，不是优先级）：2 ≈ 同等条件下被抽中的机会翻倍。
//
// weight 传 0 = 恢复默认（等价 1.0，并从账号文件里删掉该键）。
func (s *AccountsService) SetWeight(ctx context.Context, file string, weight float64, upstreamID *int) (*AccountWeightResult, error) {
	var out AccountWeightResult
	body := map[string]float64{"weight": weight}
	path := accountPath(file) + "/weight"
	if _, err := s.c.doJSON(ctx, http.MethodPut, path, s.c.groupQ(upstreamID), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ForceCooling 强制设定账号级冷却（会重启一次上游来生效）。
//
// 返回原始结果字典：服务端除了 ok / message / until，还会带回「改了哪些状态位」。
// 注意该接口只支持默认分组（其它分组没有对应的 state.json 路径，服务端回 409）。
func (s *AccountsService) ForceCooling(ctx context.Context, file string, in ForceCoolingIn, upstreamID *int) (map[string]any, error) {
	var out map[string]any
	path := accountPath(file) + "/force-cooling"
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, s.c.groupQ(upstreamID), in, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Move 把账号**转移**到另一个分组（移动的是账号文件本身，凭证不改）。
//
// toUpstreamID 传 0 表示默认分组；源分组由 upstreamID 指定（nil = 默认分组）。
// 目标分组已有同名文件、或两侧没有登记账号目录时服务端回 409。
func (s *AccountsService) Move(ctx context.Context, file string, toUpstreamID int, upstreamID *int) (*AccountMoveResult, error) {
	var out AccountMoveResult
	body := map[string]int{"to_upstream_id": toUpstreamID}
	path := accountPath(file) + "/move"
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, s.c.groupQ(upstreamID), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── 账号顺序（/api/accounts/order）───────────────────────────

// AccountOrder 是分组的账号自定义顺序（uid 数组）。
//
// 顺序里没提到的账号不会被删改，只是排到末尾——所以可以只提交当前可见的那几条。
type AccountOrder struct {
	Order []string `json:"order"`
}

// Order 读取分组的账号自定义顺序（只读不设管理员门槛：顺序是共享视图）。
func (s *AccountsService) Order(ctx context.Context, upstreamID *int) (*AccountOrder, error) {
	var out AccountOrder
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/accounts/order", s.c.groupQ(upstreamID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveOrder 保存自定义顺序（限管理员：它改变的是所有人看到的列表顺序）。
func (s *AccountsService) SaveOrder(ctx context.Context, order []string, upstreamID *int) (*AccountOrder, error) {
	body := AccountOrder{Order: order}
	var out AccountOrder
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/accounts/order", s.c.groupQ(upstreamID), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── 两套调度的重叠（/api/accounts/task-scheduler）──────────────

// TaskScheduler 是上游调度器与面板每日计划的重叠状态。
type TaskScheduler struct {
	Upstream TaskSchedulerUpstream `json:"upstream"`
	// Panel 是面板每日计划各项任务的开关（checkin / compensation / …）。
	Panel map[string]bool `json:"panel"`
	// Conflicts 是两套同时在跑的任务名。
	Conflicts []string `json:"conflicts"`
}

// TaskSchedulerUpstream 是上游调度器的开关。
type TaskSchedulerUpstream struct {
	// Available 为 false 表示上游配置读不到（此时下面的开关没有意义）。
	Available bool `json:"available"`
	Checkin   bool `json:"checkin"`
	Activity  bool `json:"activity"`
}

// TaskSchedulerKeepResult 是「只保留一套调度」的结果。
type TaskSchedulerKeepResult struct {
	// Plan 是改完之后的每日计划。
	Plan map[string]any `json:"plan"`
	// Scheduler 是改完之后的重叠状态。
	Scheduler TaskScheduler `json:"scheduler"`
	// ReloadScheduled 表示是否已排上上游热重载（需要人工重启时为 false）。
	ReloadScheduled *bool `json:"reload_scheduled"`
	// ReloadHint 非空时说明配置已写入但当前环境无法自动重启上游。
	ReloadHint string `json:"reload_hint"`
}

// TaskScheduler 读取两套调度的重叠状态。
func (s *AccountsService) TaskScheduler(ctx context.Context) (*TaskScheduler, error) {
	var out TaskScheduler
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/accounts/task-scheduler", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// KeepTaskScheduler 只保留一套调度（限管理员）。
//
// keep 取值 "upstream"（关掉面板的签到 / 领补偿）或 "panel"（反过来）。它会写
// 上游 config 并触发热重载，改完可能要等容器重启才生效。
func (s *AccountsService) KeepTaskScheduler(ctx context.Context, keep string) (*TaskSchedulerKeepResult, error) {
	body := map[string]string{"keep": keep}
	var out TaskSchedulerKeepResult
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/accounts/task-scheduler", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
