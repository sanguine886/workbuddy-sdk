package wbsdk

import (
	"log/slog"
	"net/http"
	"time"
)

type options struct {
	hc        *http.Client
	transport http.RoundTripper
	timeout   time.Duration
	headers   http.Header
	admin     AdminAuth
	key       GatewayKey
	realm     Realm
	userAgent string
	log       *slog.Logger
}

func defaultOptions() options {
	return options{
		realm:     RealmCN,
		userAgent: "workbuddy-sdk/0.1 (+https://github.com/sanguine886/workbuddy-sdk)",
		headers:   make(http.Header),
	}
}

// Option 配置 Client。
type Option func(*options)

// WithHTTPClient 使用完全自定义的 *http.Client（忽略 WithTransport / WithTimeout）。
func WithHTTPClient(hc *http.Client) Option {
	return func(o *options) { o.hc = hc }
}

// WithTransport 自定义传输层（如走代理、注入中间件）；默认 http.DefaultTransport。
func WithTransport(rt http.RoundTripper) Option {
	return func(o *options) { o.transport = rt }
}

// WithTimeout 设置 HTTP 客户端总超时（默认 0，不设限）。
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

// WithHeader 注入附加请求头（同名多次调用为追加）。
func WithHeader(key, value string) Option {
	return func(o *options) { o.headers.Add(key, value) }
}

// WithUserAgent 覆盖默认 User-Agent。
func WithUserAgent(ua string) Option {
	return func(o *options) { o.userAgent = ua }
}

// WithAdminAuth 设置管理面鉴权。
func WithAdminAuth(a AdminAuth) Option {
	return func(o *options) { o.admin = a }
}

// WithAdminCredentials 便捷设置「用户名 + 密码」鉴权（自动登录 + 401 重登）。
// baseURL 应与 NewClient 的地址一致（子路径部署时带前缀）。
func WithAdminCredentials(baseURL, username, password string) Option {
	return func(o *options) { o.admin = NewPasswordAuth(baseURL, username, password) }
}

// WithAdminCookie 便捷设置一段静态会话 Cookie。
func WithAdminCookie(cookie string) Option {
	return func(o *options) { o.admin = StaticSessionAuth(cookie) }
}

// WithGatewayKey 设置数据面网关密钥。
func WithGatewayKey(key string) Option {
	return func(o *options) { o.key = StaticKey(key) }
}

// WithRealm 设置默认版本（cn / global）；数据面与管理面按需覆盖。
func WithRealm(r Realm) Option {
	return func(o *options) { o.realm = r }
}

// WithLogger 注入一个 slog 记录器（当前仅用于调试输出，可为空）。
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.log = l }
}
