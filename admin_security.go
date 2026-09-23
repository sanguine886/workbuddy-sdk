package wbsdk

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// SecurityService 封装入站 IP 管控与访问审计（/api/security）。
type SecurityService struct{ c *Client }

// Security 返回安全服务。
func (c *Client) Security() *SecurityService { return &SecurityService{c: c} }

// Config 读取全局 IP 管控开关与模式。
func (s *SecurityService) Config(ctx context.Context) (*SecurityConfig, error) {
	var out SecurityConfig
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/security/config", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveConfig 保存全局 IP 管控开关与模式。
func (s *SecurityService) SaveConfig(ctx context.Context, cfg SecurityConfig) (*SecurityConfig, error) {
	var out SecurityConfig
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/security/config", nil, cfg, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Rules 列出全部 IP 规则。
func (s *SecurityService) Rules(ctx context.Context) ([]IpRule, error) {
	var out []IpRule
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/security/rules", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddRule 新增一条 IP 规则（kind 为 allow / deny；cidr 支持单个 IP 或 CIDR）。
func (s *SecurityService) AddRule(ctx context.Context, kind, cidr, note string) (*IpRule, error) {
	body := map[string]string{"kind": kind, "cidr": cidr, "note": note}
	var out IpRule
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/security/rules", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RemoveRule 删除一条 IP 规则。
func (s *SecurityService) RemoveRule(ctx context.Context, id int) error {
	path := "/api/security/rules/" + strconv.Itoa(id)
	_, err := s.c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil)
	return err
}

// Logs 读取入站访问审计（默认只记拦截）。
func (s *SecurityService) Logs(ctx context.Context, limit int) ([]IpAccessLog, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var out []IpAccessLog
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/security/logs", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ClearLogs 清空入站访问审计。
func (s *SecurityService) ClearLogs(ctx context.Context) error {
	_, err := s.c.doJSON(ctx, http.MethodPost, "/api/security/logs/clear", nil, nil, nil)
	return err
}
