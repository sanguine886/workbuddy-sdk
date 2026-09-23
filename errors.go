package wbsdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// 哨兵错误：配合 errors.Is 使用，便于调用方按类别分支。
var (
	ErrUnauthorized = errors.New("wbsdk: unauthorized")
	ErrForbidden    = errors.New("wbsdk: forbidden")
	ErrNotFound     = errors.New("wbsdk: not found")
	ErrRateLimited  = errors.New("wbsdk: rate limited")
)

// APIError 是服务端返回的非 2xx 响应的归一化形态。
//
// Message 可能是中文散文（管理面）或英文句子（网关面），本库**不解析**它，
// 只原样暴露；Raw 永远保留完整响应体，供调用方自行处理。
type APIError struct {
	StatusCode int    // HTTP 状态码
	Code       string // 机器码：网关面的 error.code；管理面无则为空
	Message    string // 人类可读信息（可能为空）
	Hint       string // 网关面的 error.gateway_hint（可执行建议）
	Raw        []byte // 原始响应体
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	if e.Code != "" {
		return fmt.Sprintf("wbsdk: HTTP %d %s (%s)", e.StatusCode, msg, e.Code)
	}
	return fmt.Sprintf("wbsdk: HTTP %d %s", e.StatusCode, msg)
}

// AuthError 表示鉴权失败（401）。
type AuthError struct{ *APIError }

func (e *AuthError) Unwrap() error { return e.APIError }
func (e *AuthError) Is(target error) bool {
	return target == ErrUnauthorized
}

// PermissionError 表示无权限（403）：如非管理员、密钥被拒、IP 被拦。
type PermissionError struct{ *APIError }

func (e *PermissionError) Unwrap() error { return e.APIError }
func (e *PermissionError) Is(target error) bool {
	return target == ErrForbidden
}

// NotFoundError 表示资源不存在（404）。
type NotFoundError struct{ *APIError }

func (e *NotFoundError) Unwrap() error { return e.APIError }
func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

// QuotaError 表示被限流或配额用尽（429）。
type QuotaError struct{ *APIError }

func (e *QuotaError) Unwrap() error { return e.APIError }
func (e *QuotaError) Is(target error) bool {
	return target == ErrRateLimited
}

// newAPIError 把响应体解析成 *APIError，兼容两种错误形状：
//
//	网关面：{"error":{"message":..,"type":..,"code":..,"gateway_hint":..}}
//	管理面：{"detail":"中文散文"} / {"code":"..","error":".."}
func newAPIError(status int, raw []byte) *APIError {
	e := &APIError{StatusCode: status, Raw: raw}

	var gw struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
			Hint    string `json:"gateway_hint"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &gw) == nil && gw.Error.Message != "" {
		e.Message = gw.Error.Message
		e.Code = gw.Error.Code
		e.Hint = gw.Error.Hint
		return e
	}

	var mg map[string]any
	if json.Unmarshal(raw, &mg) == nil {
		if d, ok := mg["detail"].(string); ok {
			e.Message = d
		}
		if e.Message == "" {
			if s, ok := mg["error"].(string); ok {
				e.Message = s
			}
		}
		if c, ok := mg["code"].(string); ok {
			e.Code = c
		}
	}

	if e.Message == "" && len(raw) > 0 {
		// detail 可能是数组（FastAPI 校验错误）等非字符串：退化为原文截断。
		e.Message = truncate(strings.TrimSpace(string(raw)), 500)
	}
	return e
}

// wrapStatus 把 *APIError 包成更具体的类型（保持 errors.As 可回溯到 *APIError）。
func wrapStatus(err *APIError) error {
	switch err.StatusCode {
	case http.StatusUnauthorized:
		return &AuthError{APIError: err}
	case http.StatusForbidden:
		return &PermissionError{APIError: err}
	case http.StatusNotFound:
		return &NotFoundError{APIError: err}
	case http.StatusTooManyRequests:
		return &QuotaError{APIError: err}
	default:
		return err
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
