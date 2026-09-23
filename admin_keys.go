package wbsdk

import (
	"context"
	"net/http"
	"strconv"
)

// KeysService 封装 /api/keys 下的密钥管理接口。
type KeysService struct{ c *Client }

// Keys 返回密钥服务。
func (c *Client) Keys() *KeysService { return &KeysService{c: c} }

func keyPath(id int) string { return "/api/keys/" + strconv.Itoa(id) }

// List 列出全部密钥（不含明文）。
func (s *KeysService) List(ctx context.Context) ([]ApiKey, error) {
	var out []ApiKey
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/keys", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Create 新建密钥。返回体里的 Key 字段是**明文密钥，仅此一次**，请立即保存。
func (s *KeysService) Create(ctx context.Context, in KeyIn) (*ApiKey, error) {
	var out ApiKey
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/keys", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update 部分更新密钥（仅提交非 nil 字段）。
func (s *KeysService) Update(ctx context.Context, id int, patch KeyPatch) (*ApiKey, error) {
	var out ApiKey
	if _, err := s.c.doJSON(ctx, http.MethodPatch, keyPath(id), nil, patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete 删除密钥。
func (s *KeysService) Delete(ctx context.Context, id int) error {
	_, err := s.c.doJSON(ctx, http.MethodDelete, keyPath(id), nil, nil, nil)
	return err
}

// ResetUsage 把该密钥的已用 Token 与已用积分一起归零。
func (s *KeysService) ResetUsage(ctx context.Context, id int) error {
	_, err := s.c.doJSON(ctx, http.MethodPost, keyPath(id)+"/reset-usage", nil, nil, nil)
	return err
}

// CheckModels 检查模型白名单里哪些名字匹配不到已知模型（拼写校验）。
// Checked=false 表示当时拿不到模型清单，不能把空 Unknown 当作「全部正确」。
func (s *KeysService) CheckModels(ctx context.Context, models []string, realm Realm) (*WhitelistCheck, error) {
	body := map[string]any{"models": models, "realm": string(realm)}
	var out WhitelistCheck
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/keys/check-models", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
