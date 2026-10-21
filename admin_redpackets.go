package wbsdk

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// RedPacketService 封装红包：批量发放带额度的密钥，以及公开的抽奖端
// （/api/red-packets*、/api/claim/{code}）。
//
// 权限：创建与收回要**管理员**（发的是额度，消耗账号池的积分 / token）；
// 列表与详情登录即可；抽奖两端**公开** —— 收到链接的人不需要账号。
type RedPacketService struct{ c *Client }

// RedPackets 返回红包服务。
func (c *Client) RedPackets() *RedPacketService { return &RedPacketService{c: c} }

func packetPath(id int) string {
	return "/api/red-packets/" + strconv.Itoa(id)
}

// Create 创建红包。返回里含**明文密钥**（Keys[].Key），且仅此一次。
func (s *RedPacketService) Create(ctx context.Context, in RedPacketIn) (*RedPacketCreated, error) {
	var out RedPacketCreated
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/red-packets", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List 拉取红包列表（不含明文密钥 —— 库里也没有）。
func (s *RedPacketService) List(ctx context.Context) ([]RedPacket, error) {
	var out []RedPacket
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/red-packets", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get 读取红包详情：每一份的密钥前缀与用量（仍不含明文）。
func (s *RedPacketService) Get(ctx context.Context, id int) (*RedPacketDetail, error) {
	var out RedPacketDetail
	if _, err := s.c.doJSON(ctx, http.MethodGet, packetPath(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Revoke 收回整批（停用并删除这批密钥，可逆性由服务端保证：只删密钥不删红包记录）。
func (s *RedPacketService) Revoke(ctx context.Context, id int) (*RedPacketRevoked, error) {
	var out RedPacketRevoked
	if _, err := s.c.doJSON(ctx, http.MethodPost, packetPath(id)+"/revoke", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ClaimInfo 读取抽奖页元信息。**公开接口**，不需要任何凭据。
//
// 已经领过的来源 IP 会连带拿到自己那一份的明文（MyKey），刷新即可找回。
func (s *RedPacketService) ClaimInfo(ctx context.Context, code string) (*ClaimInfo, error) {
	var out ClaimInfo
	path := "/api/claim/" + url.PathEscape(code)
	if _, err := s.c.doJSON(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Claim 抽一份，返回明文密钥与这一份的额度。**公开接口**，不需要任何凭据。
//
// 同一来源 IP 对同一个红包只能抽一次（服务端回 409）；链接失效 / 过期 / 领完
// 都归为 404「来晚了」。
func (s *RedPacketService) Claim(ctx context.Context, code string) (*ClaimResult, error) {
	var out ClaimResult
	path := "/api/claim/" + url.PathEscape(code)
	if _, err := s.c.doJSON(ctx, http.MethodPost, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
