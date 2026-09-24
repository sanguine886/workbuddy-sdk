package wbsdk

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const sessionCookieName = "wb_session"

// Client 是 workbuddy-manager 客户端。
//
// 并发安全：底层 *http.Client 惰性构建且并发复用；realm 用原子读写；
// 鉴权实现（PasswordAuth 等）各自内部同步。NewClient 不做任何网络或资源分配。
type Client struct {
	baseURL   string
	userAgent string
	headers   http.Header
	admin     AdminAuth
	key       GatewayKey
	log       *slog.Logger

	realm        atomic.Value // string
	retry        *RetryPolicy
	sseEventMode bool

	hcOnce    sync.Once
	hc        *http.Client
	provided  *http.Client
	transport http.RoundTripper
	timeout   time.Duration
}

// NewClient 创建客户端。baseURL 形如 "https://wb.example.com"；
// 子路径部署时带上前缀（如 "https://example.com/workbuddy-manager"）。
func NewClient(baseURL string, opts ...Option) *Client {
	o := defaultOptions()
	for _, fn := range opts {
		fn(&o)
	}
	base := strings.TrimRight(baseURL, "/")

	admin := o.admin
	if o.loginSet {
		// 登录地址取自客户端 baseURL，调用方无需重复传。
		admin = NewPasswordAuth(base, o.loginUser, o.loginPass)
	}

	c := &Client{
		baseURL:      base,
		userAgent:    o.userAgent,
		headers:      o.headers,
		admin:        admin,
		key:          o.key,
		log:          o.log,
		retry:        o.retry,
		sseEventMode: o.sseEventMode,
		provided:     o.hc,
		transport:    o.transport,
		timeout:      o.timeout,
	}
	c.realm.Store(string(o.realm))
	return c
}

func (c *Client) httpClient() *http.Client {
	c.hcOnce.Do(func() {
		if c.provided != nil {
			c.hc = c.provided
			return
		}
		tr := c.transport
		if tr == nil {
			tr = http.DefaultTransport
		}
		c.hc = &http.Client{Timeout: c.timeout, Transport: tr}
	})
	return c.hc
}

// CloseIdleConnections 关闭底层空闲连接。若底层客户端尚未构建，则不新建。
func (c *Client) CloseIdleConnections() {
	if c.hc != nil {
		c.hc.CloseIdleConnections()
	}
}

// BaseURL 返回客户端配置的根地址。
func (c *Client) BaseURL() string { return c.baseURL }

// Realm 返回默认版本。
func (c *Client) Realm() Realm {
	if v, ok := c.realm.Load().(string); ok {
		return Realm(v)
	}
	return RealmCN
}

// SetRealm 修改默认版本（并发安全，可在运行期调用）。
func (c *Client) SetRealm(r Realm) { c.realm.Store(string(r)) }
