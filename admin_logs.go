package wbsdk

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// LogsService 封装请求日志查询（/api/logs）。
type LogsService struct{ c *Client }

// Logs 返回请求日志服务。
func (c *Client) Logs() *LogsService { return &LogsService{c: c} }

// RequestLogQuery 是请求日志的筛选条件。
type RequestLogQuery struct {
	Page   int    // 从 1 起；0 用服务端默认
	Size   int    // 每页条数（服务端上限 200）
	KeyID  string // 密钥 id 的数字字符串，或 "all"
	Model  string // 模型名模糊匹配
	Status string // "ok" | "error"
	IP     string // 来源 IP 模糊匹配
	Days   int    // 时间范围（天）
	Realm  Realm
}

// List 分页查询请求日志。
func (s *LogsService) List(ctx context.Context, q RequestLogQuery) (*Page[RequestLog], error) {
	vals := url.Values{}
	if q.Page > 0 {
		vals.Set("page", strconv.Itoa(q.Page))
	}
	if q.Size > 0 {
		vals.Set("size", strconv.Itoa(q.Size))
	}
	if q.KeyID != "" {
		vals.Set("key_id", q.KeyID)
	}
	if q.Model != "" {
		vals.Set("model", q.Model)
	}
	if q.Status != "" {
		vals.Set("status", q.Status)
	}
	if q.IP != "" {
		vals.Set("ip", q.IP)
	}
	if q.Days > 0 {
		vals.Set("days", strconv.Itoa(q.Days))
	}
	if q.Realm != RealmAny {
		vals.Set("realm", string(q.Realm))
	}
	var out Page[RequestLog]
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/logs", vals, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Clear 清空请求日志。
func (s *LogsService) Clear(ctx context.Context) error {
	_, err := s.c.doJSON(ctx, http.MethodPost, "/api/logs/clear", nil, nil, nil)
	return err
}
