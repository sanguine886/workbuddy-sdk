package wbsdk

import (
	"context"
	"net/http"
)

// SettingsService 封装上游配置、模型别名映射与重载状态（/api/settings/*、/api/upstream/*）。
type SettingsService struct{ c *Client }

// Settings 返回设置服务。
func (c *Client) Settings() *SettingsService { return &SettingsService{c: c} }

// Upstream 读取上游 config.json（敏感字段已掩码）。读不到时 Available=false 并带原因。
func (s *SettingsService) Upstream(ctx context.Context) (*UpstreamConfig, error) {
	var out UpstreamConfig
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/settings/upstream", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveUpstream 保存上游配置（仅允许改写非敏感段）。
//
// patch 里只放要改的段（如 {"schedule": {...}}），服务端不会用空值覆盖未提交的段。
// 返回体里的 ReloadHint 非空时，说明当前环境无法自动重载上游，需要人工重启。
func (s *SettingsService) SaveUpstream(ctx context.Context, patch map[string]any) (*UpstreamConfig, error) {
	var out UpstreamConfig
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/settings/upstream", nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TestUpstash 探测 Upstash 连通性；token 留空表示使用已保存的值。
func (s *SettingsService) TestUpstash(ctx context.Context, url, token string) (*ActionResult, error) {
	body := map[string]any{"url": url}
	if token != "" {
		body["token"] = token
	}
	var out ActionResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/settings/upstash/test", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReloadState 读取上游重载状态（保存配置后自动重启）。
func (s *SettingsService) ReloadState(ctx context.Context) (*ReloadState, error) {
	var out ReloadState
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/upstream/reload-state", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReloadUpstream 立即重启上游（等待结果）。一般无需手动调用。
func (s *SettingsService) ReloadUpstream(ctx context.Context) (*ActionResult, error) {
	var out ActionResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/settings/upstash/reload", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ModelMap 读取模型别名映射（别名 → 真名）。
func (s *SettingsService) ModelMap(ctx context.Context) (map[string]string, error) {
	var out map[string]string
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/settings/model-map", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SaveModelMap 保存模型别名映射（空键 / 空值会被服务端丢弃）。
func (s *SettingsService) SaveModelMap(ctx context.Context, m map[string]string) (map[string]string, error) {
	var out map[string]string
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/settings/model-map", nil, m, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GitHubToken 读取 GitHub Token 的生效状态（**只回掩码**）。
//
// Source 为 "env" 时说明值由部署注入，面板填的那份不生效。
func (s *SettingsService) GitHubToken(ctx context.Context) (*GitHubToken, error) {
	var out GitHubToken
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/settings/github-token", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveGitHubToken 保存面板这份 GitHub Token（空串 = 清除）。不回明文。
func (s *SettingsService) SaveGitHubToken(ctx context.Context, token string) (*GitHubToken, error) {
	var out GitHubToken
	body := map[string]string{"token": token}
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/settings/github-token", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GitHubMirror 读取 GitHub 镜像基址与来源（国内机器直连 GitHub 超时时用）。
func (s *SettingsService) GitHubMirror(ctx context.Context) (*GitHubMirror, error) {
	var out GitHubMirror
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/settings/github-mirror", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveGitHubMirror 保存镜像基址（空串 = 清除，回落到环境变量）。
//
// 只接受「协议 + 主机 + 可选路径」：带查询串或 # 片段会被拒（拼出来的地址必然
// 是错误页）。
func (s *SettingsService) SaveGitHubMirror(ctx context.Context, mirror string) (*GitHubMirror, error) {
	var out GitHubMirror
	body := map[string]string{"mirror": mirror}
	if _, err := s.c.doJSON(ctx, http.MethodPut, "/api/settings/github-mirror", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
