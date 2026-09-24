package wbsdk

import (
	"log/slog"
	"net/http"
	"time"
)

type options struct {
	hc           *http.Client
	transport    http.RoundTripper
	timeout      time.Duration
	headers      http.Header
	admin        AdminAuth
	adminToken   string
	key          GatewayKey
	realm        Realm
	retry        *RetryPolicy
	sseEventMode bool
	userAgent    string
	log          *slog.Logger

	// 便捷登录：由 Client 用自身 baseURL 构造 PasswordAuth。
	loginSet  bool
	loginUser string
	loginPass string
}

func defaultOptions() options {
	return options{
		realm:   RealmCN,
		headers: make(http.Header),
		// userAgent 留空，由 NewClient 用 defaultUserAgent() 填充——
		// 这样版本号只有一个来源（version.go / ldflags）。
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

// WithTimeout 设置 HTTP 客户端总超时。
//
// 默认 0 = 不设限——流式请求（模型可能思考很久才吐第一个字）需要长时间连接，
// 用固定超时会误杀。约束应由调用方通过 context 控制。
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

// WithAdminAuth 设置管理面鉴权（自定义实现或内置实现）。
func WithAdminAuth(a AdminAuth) Option {
	return func(o *options) { o.admin = a }
}

// WithAdminLogin 用「用户名 + 密码」登录管理面，自动登录并在 401 时重登。
//
// 登录地址取自 NewClient 的 baseURL，因此无需重复传——这是推荐用法。
func WithAdminLogin(username, password string) Option {
	return func(o *options) {
		o.loginSet = true
		o.loginUser = username
		o.loginPass = password
	}
}

// WithAdminCredentials 是 WithAdminLogin 的显式 baseURL 版本，
// 适用于把同一份鉴权复用到多个指向同一服务的 Client。
// baseURL 应与 NewClient 的地址一致（子路径部署时带前缀）。
func WithAdminCredentials(baseURL, username, password string) Option {
	return func(o *options) { o.admin = NewPasswordAuth(baseURL, username, password) }
}

// WithAdminToken 用**管理面 API Token**（`Authorization: Bearer`）访问 /api/*。
//
// 它是给脚本 / CI 用的长期凭据，免去「用户名密码 + 会话 Cookie」的往返；
// 一旦设置，管理面请求就**不再**走 Cookie（也不会触发 401 自动重登）。
//
// 需要服务端支持「作用域化 API Token」（见上游设计文档）；服务端尚未支持时，
// 用 WithAdminLogin / WithAdminCookie 即可。
func WithAdminToken(token string) Option {
	return func(o *options) { o.adminToken = token }
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

// WithRetry 开启对**幂等请求**（GET / HEAD / OPTIONS）的自动重试。
//
// 默认关闭；非幂等方法（POST/PATCH/DELETE）永不重试，避免副作用重复。
// 传入零值 RetryPolicy 也可（等价 DefaultRetryPolicy）。
func WithRetry(p RetryPolicy) Option {
	return func(o *options) { o.retry = &p }
}

// WithSSEEventMode 切换流式解析模式。
//
//   - false（默认，"按行"）：每条 `data:` 行各交付一次。OpenAI / Anthropic 风格
//     都是「一条事件一行 data」，这也是 codex-sdk 的口径，够用且零解析。
//   - true（"按事件"）：按 SSE 规范切分事件，同一事件内**多条 data: 行以 "\n"
//     拼接**后整体交付；注释行（`:` 开头）与 event / id / retry 字段忽略。
//     对接严格实现 SSE 的服务端或多行 data 的流时用它。
func WithSSEEventMode(enabled bool) Option {
	return func(o *options) { o.sseEventMode = enabled }
}

// WithLogger 注入一个 slog 记录器（当前仅用于调试输出，可为空）。
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.log = l }
}
