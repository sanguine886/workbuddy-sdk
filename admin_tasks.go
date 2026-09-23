package wbsdk

import (
	"context"
	"net/http"
)

// TasksService 封装成长任务一键执行（/api/task-run、/api/task-claim-schedule）。
type TasksService struct{ c *Client }

// Tasks 返回成长任务服务。
func (c *Client) Tasks() *TasksService { return &TasksService{c: c} }

// 执行模式：按风险分级。
const (
	TaskModePreview = "preview" // 只查询（dry-run）
	TaskModeClaim   = "claim"   // 只领已完成任务的奖励（幂等）
	TaskModeFull    = "full"    // 点亮 + 领奖（会伪造活跃上报，有风控风险）
)

// RunStatus 读取当前 / 上次执行状态与输出尾部。
func (s *TasksService) RunStatus(ctx context.Context) (*TaskRunStatus, error) {
	var out TaskRunStatus
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/task-run", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RunStart 启动一次成长任务执行。mode 取 TaskMode* 常量；target 一般为 "ALL"。
// full 模式必须 confirm=true。
func (s *TasksService) RunStart(ctx context.Context, mode, target string, confirm bool) (*ActionResult, error) {
	body := map[string]any{"mode": mode, "target": target, "confirm": confirm}
	var out ActionResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/task-run", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RunStop 停止当前执行。
func (s *TasksService) RunStop(ctx context.Context) (*ActionResult, error) {
	var out ActionResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/task-run/stop", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ClaimSchedule 读取定时领奖配置（仅幂等领奖，不含点亮）。
func (s *TasksService) ClaimSchedule(ctx context.Context) (*TaskRunSchedule, error) {
	var out TaskRunSchedule
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/task-claim-schedule", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveClaimSchedule 保存定时领奖配置（hours 为整点数组，如 []int{9, 21}）。
func (s *TasksService) SaveClaimSchedule(ctx context.Context, enabled bool, hours []int) (*TaskRunSchedule, error) {
	body := map[string]any{"enabled": enabled, "hours": hours}
	var out TaskRunSchedule
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/task-claim-schedule", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
