package wbsdk

import (
	"context"
	"errors"
	"net/http"
	"strconv"
)

// 本文件是「账号批量操作」一组端点（v2.8 ~ v2.10 新增）。
//
// 这些路由在旧版管理端上不存在，调用会 404。它们是**管理员**接口；其中
// Export 额外要求**会话登录**（`require_session_admin`）：它把带凭据的账号文件
// 打包下载，服务端刻意不允许 API Token 触发批量导出（`wbt_` 会拿到 403）。
// Upload 走 multipart，同样不参与重试。

// Export 把选中分组的账号文件打成 zip 下载（GET /api/accounts/export）。
//
// 返回的是**未解析的原始响应**：Body 就是 zip 字节，Header 里有
// `Content-Disposition: attachment; filename="accounts.zip"`。包里是账号凭据，
// 服务端带了 `Cache-Control: no-store` —— 调用方也别把它落盘到共享目录。
//
// 仅会话管理员可调（API Token 会得到 403）。
func (s *AccountsService) Export(ctx context.Context, upstreamID *int) (*RawResponse, error) {
	res, err := s.c.doRaw(ctx, http.MethodGet, "/api/accounts/export", groupQuery(upstreamID), nil)
	if err != nil {
		return nil, err
	}
	if !res.OK() {
		// 非 2xx 时 Body 是 JSON 错误体（例如 403「该接口不接受 API Token」），
		// 统一归一化成 *APIError，调用方不必自己判状态码。
		return res, res.Err()
	}
	return res, nil
}

// Upload 批量导入账号文件（POST /api/accounts/upload）。
//
// overwrite=false（默认）时已存在的账号会被**拒绝**并出现在 Rejected 里，而
// 不是覆盖；单个文件失败不影响同批其它文件，逐条看 Rejected / Failed 即可。
// 服务端限制单个文件 ≤ 2 MB。
//
// 上传不重试、也不做 401 自动重登（请求体是流式的，重发需要调用方重新打开文件）。
func (s *AccountsService) Upload(ctx context.Context, files []AccountUploadFile, upstreamID *int, overwrite bool) (*AccountUploadResult, error) {
	if len(files) == 0 {
		return nil, errors.New("wbsdk: Upload 至少要一个文件")
	}
	q := groupQuery(upstreamID)
	if overwrite {
		q.Set("overwrite", strconv.FormatBool(overwrite))
	}
	parts := make([]UploadPart, 0, len(files))
	for _, f := range files {
		name := f.Filename
		if name == "" {
			name = "workbuddy.json"
		}
		parts = append(parts, UploadPart{Field: "files", Filename: name, Reader: f.Reader})
	}
	var out AccountUploadResult
	if _, err := s.c.doMultipart(ctx, "/api/accounts/upload", q, parts, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AdminAPIStatus 读取上游「账号管理接口」（admin.enabled）开关状态。
//
// 非默认分组读不到它的 config.json，服务端返回 `enabled: null`（未知）。
func (s *AccountsService) AdminAPIStatus(ctx context.Context, upstreamID *int) (*AdminAPIStatus, error) {
	var out AdminAPIStatus
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/accounts/admin-api-status", groupQuery(upstreamID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreditsSnapshot 读取面板已登记的积分快照（只读，不触发任何上游查询）。
//
// 快照跨全部分组、按 uid 登记；账号删掉后历史记录仍在，用 Known 区分。
// 数字是「已登记」的，不是实时余额 —— 要实时值用 RefreshCredits。
func (s *AccountsService) CreditsSnapshot(ctx context.Context) (*CreditsSnapshot, error) {
	var out CreditsSnapshot
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/accounts/credits-snapshot", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProbeModels 主动探测「账号 × 模型」能否用（POST /api/accounts/probe-models）。
//
// 会**真的发上游请求**：同账号内串行、每条间隔 600ms，不同账号最多 4 路并发，
// 一次最多 60 条组合；看到 6004 也不重试（探测不能变成新的限流来源）。
// 因此这个调用可能耗时数十秒，ctx 超时要留够。
//
// 结果分类：ok / rate（6004 限流）/ credit（402 或积分文案）/ auth（401 / 12153）/
// no_model（11102，**不算错误**）/ error。
func (s *AccountsService) ProbeModels(ctx context.Context, in ProbeModelsIn, upstreamID *int) (*ProbeModelsResult, error) {
	var out ProbeModelsResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/accounts/probe-models", groupQuery(upstreamID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RefreshTokens 一键刷新全部账号的 accessToken（POST /api/accounts/refresh-tokens）。
//
// 并发上限默认 3（腾讯风控），全部跑完后**只重载一次上游**。成功 0 个时不重载；
// 「刷新成功但重载失败」时 ReloadError 非空（凭证已写回，只是在跑的上游还没加载）。
// 失败通常是 refreshToken 已失效，需要重新扫码。
func (s *AccountsService) RefreshTokens(ctx context.Context, upstreamID *int) (*RefreshTokensResult, error) {
	var out RefreshTokensResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/accounts/refresh-tokens", groupQuery(upstreamID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RefreshPlan 读取令牌刷新计划（只读，任何已登录用户可看）。
func (s *AccountsService) RefreshPlan(ctx context.Context) (*RefreshPlan, error) {
	var out RefreshPlan
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/accounts/refresh-plan", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveRefreshPlan 保存令牌刷新计划（管理员）。
//
// 三个字段都必须给（服务端逐项校验，缺一个就是 400）；要改一项先 RefreshPlan
// 取出来、改完整体提交。last_run 不受影响 —— 改间隔不该把"上次跑过"抹掉。
func (s *AccountsService) SaveRefreshPlan(ctx context.Context, in RefreshPlanIn) (*RefreshPlan, error) {
	var out RefreshPlan
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/accounts/refresh-plan", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RunRefreshPlan 立即执行一次全量刷新（管理员），并把它算作本轮。
//
// 与 RefreshTokens 的区别：这里**串行**（与后台巡检同一套，防腾讯风控），
// 且**不重载上游**（写回的凭证由上游 auths 热加载自行收录）。执行后重置计划计时，
// 避免刚手动跑完到点又跑一次。
func (s *AccountsService) RunRefreshPlan(ctx context.Context) (*RefreshPlanRun, error) {
	var out RefreshPlanRun
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/accounts/refresh-plan/run", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ClaimCompensation 给所有账号领一次「活动补偿」（有则领，没有就跳过）。
//
// 这类补偿带截止日期，漏了就没了 —— 管理端自己留了一条不依赖上游"活跃上报"
// 开关的路。已领过的账号直接跳过（Status=already），不发注定失败的写请求。
func (s *AccountsService) ClaimCompensation(ctx context.Context, upstreamID *int) (*CompensationResult, error) {
	var out CompensationResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/accounts/claim-compensation", groupQuery(upstreamID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DailyPlan 读取每日任务计划（签到 / 领补偿 / 领奖 / 做任务）。
//
// 只读，任何已登录用户可看；改计划要管理员。
func (s *AccountsService) DailyPlan(ctx context.Context) (*DailyPlan, error) {
	var out DailyPlan
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/accounts/daily-plan", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveDailyPlan 保存每日任务计划（管理员）。四类任务都要给。
func (s *AccountsService) SaveDailyPlan(ctx context.Context, in DailyPlanIn) (*DailyPlan, error) {
	var out DailyPlan
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/accounts/daily-plan", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RunDailyTask 立即执行一次某个每日任务（管理员），不等定时。
//
// task 只能是 checkin / compensation / claim / full（服务端校验，别的值 400）。
// 执行后该任务被标记为「本小时已跑」，避免刚手动跑完到点又跑一次。
// 注意 full 会伪造活跃上报，属写风险动作。
func (s *AccountsService) RunDailyTask(ctx context.Context, task string) (*DailyTaskRun, error) {
	if task == "" {
		return nil, errors.New("wbsdk: RunDailyTask 需要 task")
	}
	body := map[string]string{"task": task}
	var out DailyTaskRun
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/accounts/daily-plan/run", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
