package wbsdk

import (
	"context"
	"net/http"
	"strconv"
)

// AdminToken 是管理面**作用域化 API Token**（上游 v1.0.68 起支持）。
//
// 与 ApiKey（网关密钥）是**两套东西**：ApiKey 只授权模型调用；AdminToken 授权的是
// 管理接口 `/api/*`，供脚本 / CI 免登录调用。明文形如 `wbt_...`，**只在创建时返回
// 一次**，库中只存哈希。
type AdminToken struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Prefix  string `json:"prefix"`
	Scope   string `json:"scope"` // readonly | admin
	Enabled bool   `json:"enabled"`
	// ExpiresAt 是到期时刻（Unix 秒）；nil = 永不过期。
	ExpiresAt  *int64  `json:"expires_at"`
	CreatedAt  int64   `json:"created_at"`
	CreatedBy  string  `json:"created_by"`
	LastUsedAt *int64  `json:"last_used_at"`
	LastUsedIP *string `json:"last_used_ip"`
}

// CreatedAdminToken 是创建令牌的返回：比 AdminToken 多一个**仅此一次**的明文字段。
type CreatedAdminToken struct {
	AdminToken
	Token string `json:"token"`
}

// TokenPatch 是更新令牌的请求体（PATCH 语义：仅提交非 nil 字段）。
//
// 想把令牌改为「永不过期」，传一个指向 0 的指针（`ExpiresAt: &zero`）——
// 服务端把 0 视为「清除到期时间」。
type TokenPatch struct {
	Name      *string `json:"name,omitempty"`
	Scope     *string `json:"scope,omitempty"`
	Enabled   *bool   `json:"enabled,omitempty"`
	ExpiresAt *int64  `json:"expires_at,omitempty"`
}

// TokensService 封装 /api/tokens（作用域化 API Token 的管理接口）。
//
// **这些接口本身只接受会话鉴权**：用令牌去调「管理令牌」的接口一律 403——
// 否则一个泄露的令牌就能创建 / 吊销 / 提权另一个令牌（自助持久化 + 提权）。
type TokensService struct{ c *Client }

// Tokens 返回访问令牌服务。
func (c *Client) Tokens() *TokensService { return &TokensService{c: c} }

func tokenPath(id int) string { return "/api/tokens/" + strconv.Itoa(id) }

// List 列出全部令牌（**不含明文**，含前缀、scope 与最近使用）。
func (s *TokensService) List(ctx context.Context) ([]AdminToken, error) {
	var out []AdminToken
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/tokens", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Create 创建一个令牌。scope 取 "readonly" 或 "admin"（其它值由服务端降级为只读）。
// expiresAt 为 nil 表示永不过期。
//
// 返回体里的 Token 是**明文，仅此一次**，请立即保存。
func (s *TokensService) Create(ctx context.Context, name, scope string, expiresAt *int64) (*CreatedAdminToken, error) {
	body := map[string]any{"name": name, "scope": scope, "expires_at": expiresAt}
	var out CreatedAdminToken
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/tokens", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update 部分更新令牌（仅提交非 nil 字段）。
func (s *TokensService) Update(ctx context.Context, id int, patch TokenPatch) (*AdminToken, error) {
	var out AdminToken
	if _, err := s.c.doJSON(ctx, http.MethodPatch, tokenPath(id), nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete 删除令牌。删除后使用该令牌的调用方会立即失效（401）。
func (s *TokensService) Delete(ctx context.Context, id int) error {
	_, err := s.c.doJSON(ctx, http.MethodDelete, tokenPath(id), nil, nil, nil)
	return err
}
