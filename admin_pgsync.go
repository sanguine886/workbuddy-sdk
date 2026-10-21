package wbsdk

import (
	"context"
	"net/http"
)

// PGSyncService 封装 PostgreSQL 异地备份：配置、连通性探测、导出 / 恢复与进度
// （/api/settings/pg-sync*）。
//
// 鉴权分两档：读（配置、进度）登录即可；写（存配置、探测、导出、恢复）必须是
// **会话管理员** —— 用 API Token 调用会被服务端拒绝（这两个动作一个会把数据推
// 出去、一个会把数据覆盖回来）。
type PGSyncService struct{ c *Client }

// PGSync 返回备份服务。
func (c *Client) PGSync() *PGSyncService { return &PGSyncService{c: c} }

// pgSyncConfig 是「只回配置」的返回外壳。
type pgSyncConfig struct {
	Config PGSyncConfig `json:"config"`
}

// Get 读取备份配置 + 当前任务进度。
func (s *PGSyncService) Get(ctx context.Context) (*PGSync, error) {
	var out PGSync
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/settings/pg-sync", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Save 保存备份配置（只提交要改的键）。
//
// Password 传空串或掩码 `********` 都表示「沿用原值」—— 这是刻意的：把掩码当新
// 密码存回去，备份就会连不上。
func (s *PGSyncService) Save(ctx context.Context, patch PGSyncConfigPatch) (*PGSyncConfig, error) {
	var out pgSyncConfig
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/settings/pg-sync", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out.Config, nil
}

// Test 探测连通性。表单里刚填的值优先、没填的字段回落到已保存的配置，
// 所以「先测再存」与「先存再测」都走得通。失败不返回 error（看 Ok / Message）。
func (s *PGSyncService) Test(ctx context.Context, patch PGSyncConfigPatch) (*PGSyncTestResult, error) {
	var out PGSyncTestResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/settings/pg-sync/test", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Export 开始把本地数据导出到 PostgreSQL（异步任务，进度看 Status）。
func (s *PGSyncService) Export(ctx context.Context) (*PGSyncJob, error) {
	var out PGSyncJob
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/settings/pg-sync/export", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Import 开始从 PostgreSQL 恢复数据。**会覆盖本地数据** —— 调用前请自行确认。
func (s *PGSyncService) Import(ctx context.Context) (*PGSyncJob, error) {
	var out PGSyncJob
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/settings/pg-sync/import", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Status 只读任务进度（可轮询）。
func (s *PGSyncService) Status(ctx context.Context) (*PGSyncStatus, error) {
	var out PGSyncStatus
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/settings/pg-sync/status", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
