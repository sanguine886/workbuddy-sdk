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

// RevealedKey 是一把密钥的明文。
type RevealedKey struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	// Prefix 是密钥前缀（便于核对是哪一把）。
	Prefix string `json:"prefix"`
	// Key 是**明文密钥**——它能用完额度，拿到就当密码保管。
	Key string `json:"key"`
}

// Reveal 取回一把已创建密钥的明文（「再复制一次」）。
//
// 列表接口只读账号（viewer）也能调，所以取明文单独要求**管理员**，且每次留
// 审计（只记哪一把，绝不记明文）。存量密钥（面板保存明文之前建的）库里只有
// 哈希，取不回来时服务端回 **409** 并说明该怎么办——不要把它当成
// 「密钥不存在」（404）。
func (s *KeysService) Reveal(ctx context.Context, id int) (*RevealedKey, error) {
	var out RevealedKey
	if _, err := s.c.doJSON(ctx, http.MethodPost, keyPath(id)+"/reveal", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
