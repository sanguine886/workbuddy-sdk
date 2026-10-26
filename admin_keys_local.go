package wbsdk

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// 本文件是「把密钥一键写进本机客户端」一组端点（v2.10 新增）。
//
// 只有在**面板跑在用户自己机器上**时才有意义：它读的是服务端所在机器的文件
// 系统与注册表，去写那台机器上的 cc-switch / ZCode 配置。远程部署时服务端会
// 一律拒绝（403）。三个端点都是管理员接口。

// Export 把一把密钥导出成 cc-switch / ZCode 的配置片段（POST /api/keys/export）。
//
// **只读、无副作用**（除了服务端记一条审计）：它不查库、不接受 key_id，只把
// 调用方给的明文 token 做格式转换 —— 面板拿不回已存密钥的明文。
//
// 与 ImportLocal 共用同一份字段口径，两条路产出的配置完全一致：导出的能粘贴，
// 导入的就能用。要自动写进本机客户端用 ImportLocal；这里只返回片段，由调用方决定。
func (s *KeysService) Export(ctx context.Context, in KeyExportIn) (*KeyExportResult, error) {
	if in.Token == "" {
		return nil, &APIError{StatusCode: 0, Message: "Export 需要密钥明文 token"}
	}
	if in.Client == "" {
		in.Client = LocalClientCCSwitch
	}
	var out KeyExportResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/keys/export", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// LocalImportStatus 读取本机导入的可用状态（只读，开关关着也是 200）。
//
// 远程访问时 Clients 为空：那时不去探测服务器上装了什么，因为那与用户本机无关，
// 报出来只会让人误以为"已安装、可以导入"。
func (s *KeysService) LocalImportStatus(ctx context.Context) (*LocalImportStatus, error) {
	var out LocalImportStatus
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/keys/import-local/status", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DetectLocalClient 自动检测客户端装在哪（GET /api/keys/import-local/detect）。
//
// client 取值 ccswitch / zcode；deep=true 才允许受限扫描磁盘。检测顺序是
// "可信度从高到低"：环境变量 → 正在跑的进程 → 协议处理器（ccswitch:// 指向谁）
// → 上次的结果 → 常见安装位 → 扫描。命中会缓存到面板数据目录，之后一直有效。
// 只读文件系统与注册表，不写用户的任何配置。
func (s *KeysService) DetectLocalClient(ctx context.Context, client string, deep bool) (*LocalClientDetect, error) {
	if client == "" {
		client = LocalClientCCSwitch
	}
	q := url.Values{}
	q.Set("client", client)
	q.Set("deep", strconv.FormatBool(deep))
	var out LocalClientDetect
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/keys/import-local/detect", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ImportLocal 把刚创建的密钥写进本机客户端（POST /api/keys/import-local）。
//
// Token 必须是创建密钥时返回的明文（面板只存哈希）。返回体**不回显密钥**。
//
// 两条路径：deeplink（首选，把配置拼成客户端官方的 ccswitch:// 链接交给操作系统，
// 客户端自己校验入库、不用关它）与 direct（直接改配置/数据库，写入那一刻客户端
// 必须是退出的）。Mode=auto（默认）时能走深链就走深链。
//
// 失败原因由 *APIError 带出：404 = 本机没装（not_installed）；409 = 当前状态不允许
// （client_running / cannot_detect / cannot_stop / deeplink_unavailable /
// cannot_open / bad_format），改完状态可以重试。403 = 面板不在本机。
func (s *KeysService) ImportLocal(ctx context.Context, in LocalImportIn) (*LocalImportResult, error) {
	if in.Token == "" {
		return nil, &APIError{StatusCode: 0, Message: "ImportLocal 需要密钥明文 token"}
	}
	var out LocalImportResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/keys/import-local", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
