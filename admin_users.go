package wbsdk

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// UsersService 封装管理端用户与审计日志（/api/users、/api/audit-logs）。
type UsersService struct{ c *Client }

// Users 返回用户服务。
func (c *Client) Users() *UsersService { return &UsersService{c: c} }

// List 列出管理端用户。
func (s *UsersService) List(ctx context.Context) ([]UserItem, error) {
	var out []UserItem
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/users", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Add 新建管理用户（role 为 admin / viewer；密码至少 8 位）。
func (s *UsersService) Add(ctx context.Context, username, password, role string) (*UserItem, error) {
	body := map[string]string{"username": username, "password": password, "role": role}
	var out UserItem
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/users", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update 改密码 / 改角色。两者都会吊销该用户的既有会话。
func (s *UsersService) Update(ctx context.Context, username string, patch UserPatch) (*UserUpdateResult, error) {
	path := "/api/users/" + url.PathEscape(username)
	var out UserUpdateResult
	if _, err := s.c.doJSON(ctx, http.MethodPatch, path, nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Remove 删除管理用户。
func (s *UsersService) Remove(ctx context.Context, username string) error {
	path := "/api/users/" + url.PathEscape(username)
	_, err := s.c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil)
	return err
}

// AuditLogs 读取管理端审计日志（仅管理员）。
func (s *UsersService) AuditLogs(ctx context.Context, limit, offset int) (*AuditLogPage, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	var out AuditLogPage
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/audit-logs", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
