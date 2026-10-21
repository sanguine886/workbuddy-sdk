package wbsdk

import (
	"context"
	"net/http"
)

// Usage 查询**当前网关密钥**自己的用量与额度（GET /v1/usage）。
//
// 需要网关密钥（WithGatewayKey）。返回的是面板**已记录**的数值，不是向上游
// 现问的：DataSource 恒为 "panel-recorded"，SnapshotAt 是读取时刻。
//
// 两个容易踩的点：
//
//   - 这次调用本身有**副作用** —— 服务端会记一行请求日志（0 token）并刷新
//     密钥的「最近使用」与来源 IP；所以监控脚本轮询会出现在请求日志里。
//   - LastUsedAt 是**本次调用之前**的值（touch 发生在取快照之后）；
//     Credits 里的数字是 int() 截断过的整数，与面板的浮点展示可能差 1。
//
// 与 Stats() 的分工：这里的口径是「这把密钥用了多少」（做配额看板 / 自检），
// Stats() 是管理面的全局用量统计。
func (c *Client) Usage(ctx context.Context) (*KeyUsage, error) {
	var out KeyUsage
	if _, err := c.doJSON(ctx, http.MethodGet, "/v1/usage", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
