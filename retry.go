package wbsdk

import (
	"context"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// defaultRetryStatuses 是默认触发重试的状态码。
var defaultRetryStatuses = []int{
	http.StatusTooManyRequests,     // 429
	http.StatusInternalServerError, // 500
	http.StatusBadGateway,          // 502
	http.StatusServiceUnavailable,  // 503
	http.StatusGatewayTimeout,      // 504
}

// RetryPolicy 控制对**幂等请求**（GET / HEAD / OPTIONS）的自动重试。
//
// 默认**关闭**：本库主要面向管理面，写操作（POST/PATCH/DELETE）多，
// 盲目重试可能造成副作用，因此需要显式用 WithRetry 开启。
//
// 开启后，**只重试幂等方法**；非幂等方法一律不重试，即使命中 RetryPolicy。
type RetryPolicy struct {
	// MaxRetries 是最大重试次数（不含首次尝试）。<= 0 时按默认 2 处理。
	// 想彻底关闭重试，就不要调用 WithRetry。
	MaxRetries int
	// BaseDelay 是首次退避时长，随尝试次数指数增长。<= 0 时按 200ms 处理。
	BaseDelay time.Duration
	// MaxDelay 是单次退避上限。<= 0 时按 5s 处理。
	MaxDelay time.Duration
	// Statuses 是触发重试的状态码；为 nil 时用 429 / 500 / 502 / 503 / 504。
	Statuses []int
	// RetryTransportErrors 为 true 时，网络层错误（连接失败、超时）也重试。
	// nil 视为 true。
	RetryTransportErrors *bool
}

// DefaultRetryPolicy 返回一份保守的默认策略（最多重试 2 次）。
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxRetries: 2}
}

func (p RetryPolicy) normalized() RetryPolicy {
	if p.MaxRetries <= 0 {
		p.MaxRetries = 2
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = 200 * time.Millisecond
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 5 * time.Second
	}
	if p.Statuses == nil {
		p.Statuses = defaultRetryStatuses
	}
	return p
}

func (p RetryPolicy) retryTransportErrors() bool {
	return p.RetryTransportErrors == nil || *p.RetryTransportErrors
}

func (p RetryPolicy) statusRetryable(code int) bool {
	for _, s := range p.Statuses {
		if code == s {
			return true
		}
	}
	return false
}

// backoff 计算第 attempt 次重试前的等待时长（含抖动），Retry-After 优先。
func (p RetryPolicy) backoff(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if d, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
			if d > p.MaxDelay {
				d = p.MaxDelay
			}
			return d
		}
	}
	d := p.BaseDelay
	for i := 0; i < attempt && d < p.MaxDelay; i++ {
		d *= 2
	}
	if d > p.MaxDelay || d <= 0 {
		d = p.MaxDelay
	}
	// 一半固定 + 一半随机抖动，避免同一时刻的重试同时打过去。
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}

// parseRetryAfter 解析 Retry-After（秒数或 HTTP 日期）。
func parseRetryAfter(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}

// idempotentMethod 报告该方法是否可安全重试。
func idempotentMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// sleepCtx 睡眠 d，期间 ctx 结束则提前返回其错误。
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
