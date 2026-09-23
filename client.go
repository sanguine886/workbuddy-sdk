package wbsdk

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

const sessionCookieName = "wb_session"

// Client 是 workbuddy-manager 客户端。并发安全。
//
// NewClient 不做任何网络或资源分配；底层 *http.Client 在首次请求时才创建。
type Client struct {
	baseURL   string
	realm     Realm
	userAgent string
	headers   http.Header
	admin     AdminAuth
	key       GatewayKey
	log       *slog.Logger

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
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		realm:     o.realm,
		userAgent: o.userAgent,
		headers:   o.headers,
		admin:     o.admin,
		key:       o.key,
		log:       o.log,
		provided:  o.hc,
		transport: o.transport,
		timeout:   o.timeout,
	}
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
func (c *Client) Realm() Realm { return c.realm }

// SetRealm 修改默认版本（并发使用时慎用）。
func (c *Client) SetRealm(r Realm) { c.realm = r }
