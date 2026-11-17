package wbsdk

import (
	"context"
	"net/http"
)

// ToolsSearch 调用上游的 web 搜索（POST /v1/tools/search）。
//
// 入参（服务端只校验、不改写，未知字段原样放行）：
//
//	query           string   必填
//	type            string   固定 "text2text"
//	max_results     int      1..20
//	allowed_domains []string
//	blocked_domains []string
//	freshness       string   "" | d1 | m1 | y1
//
// 响应是上游 agenttool 网关的信封**原样透传**（服务端不翻译字段，业务码也
// 原样透传），所以这里直接给 map —— 具体字段取决于上游实现。
//
// 鉴权与 chat 同管线：用**同一把网关密钥**既调模型也调搜索（密钥体系一把钥匙
// 走全部数据面，搜索不另立凭证）。
func (c *Client) ToolsSearch(ctx context.Context, in map[string]any) (map[string]any, error) {
	var out map[string]any
	if _, err := c.doJSON(ctx, http.MethodPost, "/v1/tools/search", nil, in, &out); err != nil {
		return nil, err
	}
	return out, nil
}
