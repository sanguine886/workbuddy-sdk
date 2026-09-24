# workbuddy-sdk

[workbuddy-manager](https://github.com/ithtelab/workbuddy-manager) 的 Go 客户端库。
覆盖它的**两个 API 面**（数据面 `/v1/*` 与管理面 `/api/*`），共享同一套
Option、鉴权与错误模型。

> 设计取向对齐 [is7Qin/codex-sdk](https://github.com/is7Qin/codex-sdk)：
> Go、近零依赖（仅标准库）、函数式 Option、`Auth` 接口、惰性构建；
> 数据面保留「raw 字节通道」哲学——流式只做 SSE 帧提取，业务语义留给调用方。

## 安装

```sh
go get github.com/sanguine886/workbuddy-sdk
```

要求 Go 1.22+，无第三方依赖。

## 快速开始

### 管理面（用户名密码登录 + 401 自动重登）

```go
admin := wbsdk.NewClient("https://wb.example.com",
	wbsdk.WithAdminLogin("admin", "密码"), // 登录地址自动取自 baseURL，无需重复传
)

me, _ := admin.Me(ctx)
fmt.Println(me.Username, me.Role)

accounts, _ := admin.Accounts().List(ctx)
for _, a := range accounts.Accounts {
	fmt.Println(a.Nickname, a.Note, a.Credits)
}

key, _ := admin.Keys().Create(ctx, wbsdk.KeyIn{
	Name:  "my-app",
	Realm: wbsdk.RealmCN,
	Quota: 1_000_000,
})
fmt.Println("明文密钥（仅此一次）:", key.Key)
```

### 数据面（网关密钥调用模型）

```go
chat := wbsdk.NewClient("https://wb.example.com", wbsdk.WithGatewayKey("wbk_xxx"))

// 非流式
resp, _ := chat.Chat(ctx, &wbsdk.ChatRequest{
	Model:    "glm-5.2",
	Messages: []wbsdk.ChatMessage{{Role: "user", Content: "你好"}},
})
fmt.Println(resp.Choices[0].Message.Content, resp.Usage)

// 流式：拿到的是原始 SSE data: 帧，自行解析（事件语义留给调用方）
chat.ChatStream(ctx, &wbsdk.ChatRequest{
	Model:    "glm-5.2",
	Messages: []wbsdk.ChatMessage{{Role: "user", Content: "你好"}},
}, func(raw []byte) error {
	fmt.Println(string(raw))
	return nil
})
```

### 逃生舱（服务端新增接口时无需等 SDK 更新）

```go
// 原始字节
res, _ := admin.Do(ctx, http.MethodGet, "/api/some/new/endpoint", nil)
if err := res.Err(); err != nil { /* *APIError */ }

// 结构化
var out map[string]any
_ = admin.DoJSON(ctx, http.MethodGet, "/api/some/new/endpoint", nil, &out)
```

## 鉴权

| 面 | 前缀 | 凭据 |
|---|---|---|
| 管理面 | `/api` | HMAC 签名会话 Cookie（`wb_session`） |
| 数据面 | `/v1`、`/v2`、`/responses` | 网关密钥（`Authorization: Bearer wbk_...`） |

鉴权按路径前缀自动选择。管理面提供三种方式：

- **`WithAdminLogin(user, pass)`** —— 推荐。自动 `POST {baseURL}/api/login`、缓存会话
  Cookie，收到 401 时失效并**重登一次**。登录是单飞的（并发 401 只触发一次），
  且失败不重试，避免撞上服务端「5 次失败锁 10 分钟」的登录风控。
- `WithAdminCredentials(baseURL, user, pass)` —— 同上，但显式指定登录地址；适合把
  同一份鉴权复用到多个 Client。
- `WithAdminCookie("wb_session=...")` —— 直接给一段已登录的 Cookie（不自动续）。

自定义可自行实现 `AdminAuth` / `GatewayKey` 接口，通过 `WithAdminAuth` /
`WithGatewayKey` 注入。

## 错误处理

所有非 2xx 响应统一归一化为 `*APIError`（`StatusCode` / `Code` / `Message` /
`Hint` / `Raw`），并按状态码包成更具体的类型：

```go
_, err := admin.Me(ctx)
switch {
case errors.Is(err, wbsdk.ErrUnauthorized): // 401
case errors.Is(err, wbsdk.ErrForbidden):    // 403
case errors.Is(err, wbsdk.ErrNotFound):     // 404
case errors.Is(err, wbsdk.ErrRateLimited):  // 429（配额用尽 / 限流）
}

var apiErr *wbsdk.APIError
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.StatusCode, apiErr.Code, apiErr.Message, apiErr.Hint)
}
```

`Message` 可能是中文散文（管理面）或英文句子（网关面），本库**不解析文案**，
原样暴露；`Raw` 保留完整响应体。

## 重试与超时

- **重试默认关闭**（本库主要面向管理面，写操作多，盲目重试会有副作用）。需要时：

  ```go
  c := wbsdk.NewClient(base, wbsdk.WithAdminLogin("admin", "pw"),
      wbsdk.WithRetry(wbsdk.DefaultRetryPolicy())) // 或自定义 RetryPolicy
  ```

  开启后**只重试幂等方法**（GET / HEAD / OPTIONS），遇到 429 / 5xx 或网络错误时
  按指数退避 + 抖动重试，并优先尊重 `Retry-After`。POST / PATCH / DELETE
  **永不重试**，避免签名、签到这类操作被重复执行。
- **超时默认不设限**（`WithTimeout` 可选）。流式调用（模型可能思考很久才吐第一个字）
  需要长连接，固定超时会误杀；请用 `context` 控制单次调用的时限。

## 流式解析

默认**按行**交付：每条 `data:` 行回调一次。这是零解析的原始通道，与 OpenAI /
Anthropic 的「一条事件一行 data」约定一致，也与 codex-sdk 的口径相同。

对接严格实现 SSE 的服务端（或 data 跨多行）时，用 `WithSSEEventMode(true)`
切换为**按事件**交付：同一事件内的多条 `data:` 行按规范以 `\n` 拼接后整体回调，
注释行与 `event:` / `id:` / `retry:` 字段忽略，事件以空行分隔。

```go
c := wbsdk.NewClient(base,
	wbsdk.WithGatewayKey("wbk_xxx"),
	wbsdk.WithSSEEventMode(true),
)
```

> **两种模式的字节有效期不同**：按行模式的载荷引用内部扫描缓冲，**仅在回调执行
> 期间有效**；按事件模式的载荷是新分配的切片，**可跨回调保留**。

## 覆盖的接口

### 数据面（`/v1/*`）

| 方法 | 说明 |
|---|---|
| `Chat` / `ChatStream` | OpenAI Chat Completions（非流式 / 流式） |
| `Responses` / `ResponsesStream` | OpenAI Responses 协议透传 |
| `Messages` / `MessagesStream` | Anthropic Messages 协议透传 |
| `Models` | 当前密钥可见的模型列表（已按版本与白名单裁剪） |

### 管理面（`/api/*`）

| 服务 | 方法 |
|---|---|
| `Accounts()` | `List` `StartAuth` `PollAuth` `Checkin` `CheckinAll` `Credits` `RefreshCredits` `SetDisabled` `SetNote` `Test` `Refresh` `ClearCooling` `Delete` `CheckinLogs` `ClearCheckinLogs` `TaskLogs` `CollectTaskLogs` `ClearTaskLogs` `UpstreamLogs` `Restart` |
| `Keys()` | `List` `Create` `Update` `Delete` `ResetUsage` `CheckModels` |
| `Stats()` | `Summary` `Daily` `ByModel` `ByKey` `Upstream` `RepairUsage` `RebuildUsage` |
| `Logs()` | `List` `Clear` |
| `Security()` | `Config` `SaveConfig` `Rules` `AddRule` `RemoveRule` `Logs` `ClearLogs` |
| `Settings()` | `Upstream` `SaveUpstream` `TestUpstash` `ReloadState` `ReloadUpstream` `ModelMap` `SaveModelMap` |
| `Users()` | `List` `Add` `Update` `Remove` `AuditLogs` |
| `System()` | `Sysinfo` `UpdateStatus` `CheckUpdate` `Versions` `Changelog` `Update` `SetUpstreamRef` |
| `Tasks()` | `RunStatus` `RunStart` `RunStop` `ClaimSchedule` `SaveClaimSchedule` |
| `Upstream()` | `Status` `Models` |
| `Catalog()` | `Get`（模型中心，含显示名与推理档位） |
| `Playground()` | `Models` `Chat` `ChatStream`（内置测试台，仅管理员） |
| 会话 | `Me` `Logout` `RevokeSessions` |
| 逃生舱 | `Do` `DoJSON` `DoStream`（覆盖尚未封装的接口） |

示例：

```go
summary, _ := admin.Stats().Summary(ctx, wbsdk.RealmGlobal)
fmt.Println(summary.TodayRequests, summary.TodayCredit)

logs, _ := admin.Logs().List(ctx, wbsdk.RequestLogQuery{Size: 50, Status: "error"})
fmt.Println(logs.Total, len(logs.Items))

_, _ = admin.Security().AddRule(ctx, "allow", "10.0.0.0/8", "office")
users, _ := admin.Users().List(ctx)
versions, _ := admin.System().Versions(ctx)
```

> 字段类型与官方前端 `web/lib/types.ts` 对齐；服务端新增字段会被
> `encoding/json` 自动忽略，对上游小版本演进具备容忍度。
> 时间戳字段按服务端的**实际**类型建模（例如重载状态的 `last_at` 是浮点
> Unix 秒——这是靠真机端到端测试逮出来的，不是照抄文档）。

## 版本与兼容

本库直接对齐 workbuddy-manager 的 HTTP 契约，该契约**尚未版本化**（见项目
`docs/`）。服务端字段随版本演进时有变动，建议：

- 用 `admin.Me()` / `admin.DoJSON(ctx, "GET", "/api/sysinfo", nil, &v)` 做连通性探测；
- 对可选字段用指针类型判空，不要假设一定存在。

## 开发

```sh
make vet test     # 编译检查 + 单元测试
gofmt -l -w .     # 格式化
```

## 许可

MIT。见 [LICENSE](LICENSE)。
