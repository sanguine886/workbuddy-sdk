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
| 管理面 | `/api` | HMAC 签名会话 Cookie（`wb_session`）或管理面 API Token（`wbt_`） |
| 数据面 | `/v1`、`/v2`、`/responses` | 网关密钥（`Authorization: Bearer wbk_...`） |
| 公开 | `/healthz`、`/api/healthz`、`/api/status`、`/api/claim/*` | 无（红包抽奖端刻意公开，收链接的人本来就没账号；`/api/status` 只声明余额单位，不含额度） |

鉴权按路径前缀自动选择。**两个例外**：

- `/api/usage/token/`（余额端点）挂在 `/api` 前缀下，但鉴权用的是**网关密钥**
  （`wbk_`）——服务端直接读 `Authorization` 头，不认会话 Cookie。
- `/api/status`（余额单位元数据）**公开无鉴权**。

管理面提供四种方式：

- **`WithAdminLogin(user, pass)`** —— 推荐。自动 `POST {baseURL}/api/login`、缓存会话
  Cookie，收到 401 时失效并**重登一次**。登录是单飞的（并发 401 只触发一次），
  且失败不重试，避免撞上服务端「5 次失败锁 10 分钟」的登录风控。
- `WithAdminCredentials(baseURL, user, pass)` —— 同上，但显式指定登录地址；适合把
  同一份鉴权复用到多个 Client。
- `WithAdminCookie("wb_session=...")` —— 直接给一段已登录的 Cookie（不自动续）。
- `WithAdminToken("wbt_...")` —— 用**管理面 API Token**（`Authorization: Bearer`），
  适合脚本 / CI：无需用户名密码与会话往返，优先级高于 Cookie，且不触发 401 重登。
  **需服务端支持作用域化 API Token**（上游 **v1.0.68** 起支持；设计见
  [docs/api-tokens.md](https://github.com/ithtelab/workbuddy-manager/blob/main/docs/api-tokens.md)）。
  用会话（`WithAdminLogin`）建令牌，再用 `WithAdminToken` 全程 headless 调用即可。

> **少数接口只认会话**：`POST /api/system/update`（会重启服务）、
> `GET /api/accounts/export`（批量导出带凭据的账号文件）由服务端强制
> `require_session_admin`，用 `wbt_` 调会得到 403（"该接口不接受 API Token"）。
> 这些操作请用 `WithAdminLogin` 建立的会话客户端。

自定义可自行实现 `AdminAuth` / `GatewayKey` 接口，通过 `WithAdminAuth` /
`WithGatewayKey` 注入。

### 多分组（`WithUpstreamID`）

上游支持多分组（每组装一个独立的 workbuddy2api 实例、各有自己的账号目录）。
**账号级接口几乎全部接受 `upstream_id`，不传就是操作默认分组。**

```go
c := wbsdk.NewClient(base, wbsdk.WithAdminLogin("admin", "***"),
	wbsdk.WithUpstreamID(5)) // 之后所有账号级调用都作用于分组 5

c.Accounts().Checkin(ctx, "workbuddy123.json") // 自动带上 ?upstream_id=5
c.Accounts().Order(ctx, wbsdk.Ptr(9))          // 显式传的覆盖默认值
c.SetUpstreamID(0)                             // 取消，回到默认分组

// 发码 + 轮询：两者要带**同一个**分组（落盘发生在轮询那一步）
st, _ := c.Accounts().StartAuthIn(ctx, wbsdk.AuthStartIn{Realm: wbsdk.RealmGlobal, Region: "HK"})
p, _ := c.Accounts().PollAuthIn(ctx, wbsdk.AuthPollIn{State: st.State, Realm: wbsdk.RealmAny})
```

> 发码的 `Realm` 与轮询的 `Realm` 语义**不同**：前者决定打哪一版端点（必须给
> 确定值，`RealmAny` 用客户端默认），后者是**一致性校验**——服务端拿它与发码
> 时登记的版本比，不一致就回 `realm_mismatch` 且不落盘。所以轮询传 `RealmAny`
> 才是「按登记的那个来」，固定写 `RealmCN` 会让国际版的码一直失败。

单分组部署不用管它。为什么要显式设：错分组的症状极其误导——按文件名清冷却时，
服务端会去默认分组的目录里找那个文件，找不到就回 **404「账号文件不存在」**，
看起来像账号被删了。数据面（`/v1/*`）不受影响：那边用哪个上游由密钥决定。

### 局部更新：`nil` 与零值的区别

有些配置接口是**局部合并**（以服务端当前值为底）：监控 `SaveConfig`、抓包
`SaveConfig`、网关 CORS `SaveGatewayCORS`。它们的字段都是指针，语义固定：

| 写法 | 含义 |
|---|---|
| `nil` | **不改**这一项 |
| `wbsdk.Ptr("")` | 真的把它改成空串（例如清空已存的邮箱地址） |
| `wbsdk.Ptr(false)` | 真的关掉它 |
| `wbsdk.Ptr([]string{})` | 真的清空这个列表（例如清空收件人） |

```go
// 只关掉邮件通知，其余不动
patch := wbsdk.WatchdogConfigPatch{Mail: &wbsdk.WatchdogMailPatch{Enabled: wbsdk.Ptr(false)}}
// 清空已保存的 SMTP 密码
patch = wbsdk.WatchdogConfigPatch{Mail: &wbsdk.WatchdogMailPatch{Password: wbsdk.Ptr("")}}
// 清空收件人（值切片写不出这个动作：omitempty 会把 [] 吃掉）
patch = wbsdk.WatchdogConfigPatch{Mail: &wbsdk.WatchdogMailPatch{To: wbsdk.Ptr([]string{})}}
```

**例外：`Settings().SaveAutoUpdate` 是整体替换**（`AutoUpdateConfig` 用值类型），
`interval_hours` / `window_start` / `window_end` 缺一个就 400。正确用法是先
`AutoUpdate()` 读出来、改想要的字段、再整个传回去。

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
  文件上传（`System().UploadPackage`）同样**不重试、也不做 401 自动重登**：
  请求体是流式的一次性数据，重发需要调用方自己重开 reader。
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
| `Usage` | 当前密钥的用量与额度（`tokens` / `credits`，不限额度为 `null`） |
| `TokenUsage` / `TokenUsageOf` | 当前密钥的剩余额度（`/api/usage/token/`，**用网关密钥鉴权**）；`*Of` 版可指定报哪个池（`ScopeCN` / `ScopeGlobal` / `ScopeMerged`） |
| `Credits` / `CreditsOf` | 客户端默认余额路径（`/v1/credits`，RikkaHub / new-api 两种口径）；`*Of` 版同上 |
| `BalanceStatus` | 余额的**公开**单位元数据（`/api/status`，无鉴权） |
| `ToolsSearch` | 上游 web 搜索（`POST /v1/tools/search`，信封原样透传） |
| `MessagesCountTokens` / `CountTokens` | Anthropic 的 token **估算**（`/v1/messages/count_tokens`，走完整鉴权；估算口径偏高） |

### 管理面（`/api/*`）

| 服务 | 方法 |
|---|---|
| `Accounts()` | `List` `StartAuth` `StartAuthIn`（含地区 / 分组 / 线路）`StartAuthFor`（定向重登）`PollAuth` `PollAuthIn` `Checkin` `CheckinAll` `Credits` `RefreshCredits` `RefreshCredit` `SetDisabled` `SetNote` `SetProxy` `SetWeight` `ForceCooling` `Move` `Test` `Refresh` `ClearCooling` `ClearCoolingDetail` `Delete` `CheckinLogs` `ClearCheckinLogs` `TaskLogs` `CollectTaskLogs` `ClearTaskLogs` `UpstreamLogs` `Restart` `Export`（仅会话）`Upload` `AdminAPIStatus` `CreditsSnapshot` `CreditsSnapshotInactive` `ProbeModels` `RefreshTokens` `RefreshPlan` `SaveRefreshPlan` `RunRefreshPlan` `ClaimCompensation` `DailyPlan` `SaveDailyPlan` `RunDailyTask` `Order` `SaveOrder` `TaskScheduler` `KeepTaskScheduler` |
| `AccountRisk()` | `List` `Acknowledge` `Resume`（账号风险预警；`Resume` 解不掉时服务端回 409） |
| `Keys()` | `List` `Create` `Update` `Delete` `ResetUsage` `CheckModels` `Export` `LocalImportStatus` `DetectLocalClient` `ImportLocal` `Reveal`（取回明文） |
| `Stats()` | `Summary` `Daily` `Hourly` `CreditsFlow` `ByModel` `ByKey` `Upstream` `RepairUsage` `RebuildUsage` `ByAccount` `ByAccountModel`，以及带自定义日期区间的 `DailyRange` `CreditsFlowRange` `ByModelRange` `ByKeyRange` |
| `Logs()` | `List` `Clear` `AccountBackfill`（账号列为何是空的） |
| `Security()` | `Config` `SaveConfig` `Rules` `AddRule` `RemoveRule` `Logs` `ClearLogs` |
| `Settings()` | `Upstream` `SaveUpstream` `TestUpstash` `ReloadState` `ReloadUpstream` `ModelMap` `SaveModelMap` `GitHubToken` `SaveGitHubToken` `GitHubMirror` `SaveGitHubMirror` `Pricing` `SavePricing` `GatewayCORS` `SaveGatewayCORS` `AutoUpdate` `SaveAutoUpdate` |
| `Tap()` | `Config` `SaveConfig` `Backfill` `Prune` `Sessions` `Session` `DeleteSession` `Turn` `Diff` `ExportSession` `Clear`（**抓包**：库里存完整对话内容，服务端要求管理员） |
| `Watchdog()` | `Config` `SaveConfig` `Status` `Run` `ResetBaseline` `CheckMailEgress` `TestMail` `QQBots` `TestQQ` `TestLLM`（上游网页改版监控） |
| `Users()` | `List` `Add` `Update` `Remove` `AuditLogs` |
| `System()` | `Sysinfo` `UpdateStatus` `CheckUpdate` `Versions` `Changelog` `Update` `UpdateFrom`（可选在线 / 离线包）`SetUpstreamRef`（**已废弃**，v2.10 起 404）`Package` `UploadPackage` `UploadPackageFile` `DeletePackage` |
| `Tasks()` | `RunStatus` `RunStart` `RunStop` `ClaimSchedule` `SaveClaimSchedule` |
| `Upstream()` | `Status` `StatusOf` `Models`（**上游运行时**状态与模型简表） |
| `Upstreams()` | `List` `Create` `Update` `Delete` `Probe`（**本端接了几个上游**，即账号池分组） |
| `Proxies()` | `Routes` `CreateRoute` `UpdateRoute` `DeleteRoute` `ImportRoutes` `ProbeRoute` `Bindings` `BindAccounts` `AutoAssign` `AccountRoutes` `SavePool` `Settings` `SaveSettings` `ProbeSettings` |
| `PGSync()` | `Get` `Save` `Test` `Export` `Import` `Status`（PostgreSQL 异地备份） |
| `RedPackets()` | `Create` `List` `Get` `Revoke` `ClaimInfo` `Claim`（后两个**公开**） |
| `Catalog()` | `Get`（模型中心，含显示名与推理档位） |
| `Playground()` | `Models` `Chat` `ChatStream`（内置测试台，仅管理员） |
| 会话 | `Me` `Logout` `RevokeSessions` |
| 逃生舱 | `Do` `DoJSON` `DoStream`（覆盖尚未封装的接口） |
| 存活探测 | `Healthz`（网关 + 上游连通性）`AdminHealthz`（管理端）——**均无需凭据** |

> 两个别名路径（网关同时注册的 `/v2/chat/completions`、以及 baseURL 不含 `/v1`
> 时的 `/responses`）未做成具名方法，需要时用 `Do` / `DoStream` 直接调用即可。

### 错误归一化

非 2xx 一律归一化成 `*APIError`（`StatusCode` / `Code` / `Message` / `Hint` /
`Raw` / **`RetryAfter`**）。`RetryAfter` 是 `Retry-After` 头换算出的秒数（0 =
没有该头）：429 会带它，据此退避比固定间隔准。401 / 403 / 404 / 429 另有
`AuthError` / `PermissionError` / `NotFoundError` / `QuotaError` 包装，
用 `errors.As` 或 `errors.Is(wbsdk.ErrNotFound)` 都能取到。

> **v2.11 的路径迁移**：上游运行时状态从 `/api/status` 搬到了
> `/api/upstream/status`——旧地址让给了客户端余额协议的公开元数据端点。
> `Upstream().Status` 已改打新路径（继续用旧路径会解出一片零值而不报错），
> 按分组取用 `Upstream().StatusOf(ctx, upstreamID)`。

示例：

```go
summary, _ := admin.Stats().Summary(ctx, wbsdk.RealmGlobal)
fmt.Println(summary.TodayRequests, summary.TodayCredit)

logs, _ := admin.Logs().List(ctx, wbsdk.RequestLogQuery{Size: 50, Status: "error"})
fmt.Println(logs.Total, len(logs.Items))

_, _ = admin.Security().AddRule(ctx, "allow", "10.0.0.0/8", "office")
users, _ := admin.Users().List(ctx)
versions, _ := admin.System().Versions(ctx)

// 出口线路：读列表（凭据已脱敏）→ 探一次 → 给两台账号绑上
routes, _ := admin.Proxies().Routes(ctx, nil)
probe, _ := admin.Proxies().ProbeRoute(ctx, routes.Routes[0].Name)
fmt.Println(probe.OK, probe.MS, probe.ExitIP)
_, _ = admin.Proxies().BindAccounts(ctx, routes.Routes[0].Name, []string{"a.json", "b.json"}, true, nil)

// 网关侧：这把密钥自己用了多少
usage, _ := gateway.Usage(ctx)
fmt.Println(usage.Tokens.Used, usage.Credits.Remaining) // Remaining 为 nil = 不限额度
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

核对本库是否跟上了 workbuddy-manager 的最新端点（按路径段比对，非印象）：

```sh
python scripts/audit_endpoints.py /path/to/workbuddy-manager
# 按发布版本核对，不需要切换上游工作区：
python scripts/audit_endpoints.py /path/to/workbuddy-manager --ref v2.11.17
# 退出码 0 = 覆盖齐（或只剩已声明的别名路径）；1 = 出现未声明的未覆盖端点

python -m unittest discover -s scripts -p 'test_*.py'
```

版本号只有一个来源（[`version.go`](version.go)）。发行构建可用 ldflags 覆盖，
避免「发版了但代码里还写着旧版本」：

```sh
go build -ldflags "-X github.com/sanguine886/workbuddy-sdk.version=$(git describe --tags)"
```

## 许可

MIT。见 [LICENSE](LICENSE)。
