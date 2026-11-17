# Changelog

本文件记录 workbuddy-sdk 的版本变更。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [0.4.0] - 2026-11-17

对齐 workbuddy-manager **v2.11.17**：191 个端点里 189 个有具名方法，剩下两个是
**别名路径**（`/v2/chat/completions`、`/responses`），用 `Do` / `DoStream` 直达。
`scripts/audit_endpoints.py --ref v2.11.17` 的退出码为 0。

### 破坏性变更

- **`Upstream().Status` 改打 `/api/upstream/status`**。v2.11 起「上游运行时状态」
  从 `/api/status` 搬走了——那个地址让给了客户端余额协议的**公开元数据**端点
  （`{"success":true,"data":{quota_per_unit,…}}`），必须无鉴权可达。继续用旧路径
  的话，解出来的是一片零值而不报错。新增 `Upstream().StatusOf(ctx, upstreamID)`
  按分组取。

### 新增

- **监控**（`Watchdog()`）：`Config` / `SaveConfig` / `Status` / `Run` /
  `ResetBaseline` / `CheckMailEgress` / `TestMail` / `QQBots` / `TestQQ` /
  `TestLLM`。配置是**局部更新**（服务端以当前配置为底合并），凭据字段只回掩码。
- **抓包**（`Tap()`）：`Config` / `SaveConfig` / `Backfill` / `Prune` /
  `Sessions` / `Session` / `DeleteSession` / `Turn` / `Diff` / `ExportSession`
  （自包含 HTML）/ `Clear`。轮次正文随协议而变，用 `json.RawMessage` 保留原文。
- **账号风险预警**（`AccountRisk()`）：`List` / `Acknowledge` / `Resume`
  （解不掉时服务端回 409）。
- **账号顺序与调度**（`Accounts()`）：`Order` / `SaveOrder`（自定义排序）、
  `TaskScheduler` / `KeepTaskScheduler`（只保留一套调度）。
- **价目 / CORS / 自动更新**（`Settings()`）：`Pricing` / `SavePricing`、
  `GatewayCORS` / `SaveGatewayCORS`、`AutoUpdate` / `SaveAutoUpdate`。
- **按账号统计**（`Stats()`）：`ByAccount` / `ByAccountModel`。
- **其它**：`Keys().Reveal`（取回密钥明文）、`Logs().AccountBackfill`
  （账号列为何是空的）、`Client.ToolsSearch`（`POST /v1/tools/search` web 搜索
  透传）、`Client.TokenUsage` / `Client.Credits` / `Client.BalanceStatus`
  （余额三端点）。
- **账号积分快照**（`Accounts()`）：`CreditsSnapshotInactive`（连停用 / 令牌过期
  的账号一起返回，每行带 `Active`）、`RefreshCredit`（只刷新一个账号文件）。
  原有的 `CreditsSnapshot` / `RefreshCredits` 签名不变。
- **账号定向重登**（`Accounts()`）：`StartAuthFor(ctx, realm, uid)` —— 对已存在的
  账号刷新凭证，版本跟随目标账号（传的 realm 被忽略）。旧 `StartAuth` 不带 uid，
  行为不变。返回值新增 `AuthStart.Target`（这次刷的是谁）。
- **清除冷却的明细**（`Accounts()`）：`ClearCoolingDetail` —— 返回走的是在线清除
  （`Online` / `Models` / `Unfrozen`）还是离线清除（`StateFile` / `Backup` /
  `Before`）。旧 `ClearCooling` 保持不变。
- **统计的自定义日期区间**（`Stats()`）：`DailyRange` / `CreditsFlowRange` /
  `ByModelRange` / `ByKeyRange` + `DateRange`（`YYYY-MM-DD`，**两端都含**）。
  只有 Start / End 都填才生效；只填一个会被服务端忽略并退回 `days` 口径。
- **`APIError.RetryAfter`**：429 的 `Retry-After` 头换算成秒数（0 = 没有该头）。
  服务端会把上游文案里的「将在 … 重置」算成秒数，据此退避比固定间隔准。

### 变更

- `/api/usage/token/` 挂在管理面前缀下，但鉴权用的是**网关密钥**。传输层新增
  `isGatewayKeyPath`：这个路径注入 `wbk_` 密钥而不是会话 Cookie，401 也**不**
  触发会话重登（密钥无效重登没用）。
- `/api/status` 现在是**公开**端点（只声明数值单位，不含额度），因此它不再
  注入凭据，也不因为「没配管理面凭据」而报错。
- 已有类型补齐 v2.11 新增的字段（都是**可选 / 附加上下文**，对旧服务端无影响）：
  `RequestLog.Account/Effort/Channel/Cost`、`UsagePoint.Cost/UnpricedRequests/
  Success`、`UsageBreakdown.Cost/UnpricedRequests/TokensPerSec/
  ThroughputSamples`、`StatsSummary.*Cost/*Unpriced/CostCurrency`、
  `UsageHourPoint.Cost/UnpricedRequests/Success`、`ApiKey.DailyQuota/
  DailyUsedTokens/UpstreamID/PacketID`、`Account.AddedAt/RefreshRisk`、
  `KeyUsage.DailyTokens`、`CreditsSnapshot.ActiveCount/TotalRegistered`、
  `AuthPoll.ExpectedUID/ExpectedName/Revive`、`ProbeModelRow.GatewaySynced/
  GatewaySyncSkipped/GatewaySyncMessage`、`ProxyProbe.Hint`、
  `CreditsFlow.Granularity/Hourly`，以及**请求侧**的 `KeyIn.DailyQuota` /
  `KeyPatch.DailyQuota`。
- `Account.PoolUnknown` 标记 **Deprecated**：它从来不是服务端字段（`poolUnknown`
  是前端 `account-status.ts` 自己算的），所以一直是 nil。判断「看不到上游」请用
  `AccountsResponse.PoolAvailable`。
- `AuthPoll` 的两个旧字段订正：`Status` 的取值里 **`realm_mismatch` 不存在**（两版
  上游都搜不到该字符串），真正新增的是 `account_mismatch`（定向重登扫错账号）；
  `Expected` / `Got` 标记 **Deprecated** —— 上游从来没有这两个键，一直是空串，
  判断扫错账号请用 `Status == "account_mismatch"` + `ExpectedUID`。
- `ProbeModelsIn` 的注释订正：服务端上限是**账号 ≤ 20、模型 ≤ 10、组合 ≤ 60**
  （旧注释写的「账号 ≤ 10、模型 ≤ 30」从 v2.10.2 起就不对）。
- `Accounts().CreditsSnapshot` 的**服务端**语义变了：默认只返回当前还能被调度的
  账号（停用 / 令牌过期的默认排除），已删账号的登记不再下发。要全量用
  `CreditsSnapshotInactive`；对账时用 `TotalRegistered` 对比登记表总行数。

### 说明

- `Tap()` 的接口读取的是**完整对话内容**（含 system prompt 与工具结果），
  服务端要求管理员且留审计。`Tap().ExportSession` 返回自包含 HTML 的字节，
  文件名取自服务端的 `Content-Disposition`。
- 监控的 `TestMail` / `TestQQ` / `TestLLM` 接受**表单当前值**（不落库），
  传 nil 表示用已保存的配置——测试用的是「还没保存也能验」的那份值。
- **上游行为变化（本库不代劳，但要知情）**：
  - 11102 / `service info not found` 从 5xx 改判 **404** `model_not_found`，
    于是会命中 `NotFoundError` 而不是 `*APIError`（按 5xx 重试的调用方要改）。
  - 429 现在带 `Retry-After`（见上）。
  - 未锁定版本的密钥，**裸模型名白名单**从「只匹配国内版」改为「允许同名模型的
    任意版本」；显式 `cn:` / `global:` 仍只放行对应版本。已锁版本的密钥不变。
  - `/v1/models` 的国内版 id 不再带 `cn:` 前缀（`global:` 保留），按前缀做匹配的
    调用方需自行适配。

## [0.3.0] - 2026-10-20

对齐 workbuddy-manager **v2.10.2**：149 个端点里 147 个有具名方法，剩下两个是
**别名路径**（`/v2/chat/completions`、`/responses`），用 `Do` / `DoStream` 直达。
`scripts/audit_endpoints.py` 的退出码为 0 —— 而且这次是**真 0**：审计脚本的匹配
规则被修正了（见下），此前它把嵌套路径误判成已覆盖。

### 新增

- **账号批量操作**（`Accounts()`）：`Export`（把带凭据的账号文件打成 zip 下载，
  仅**会话管理员**；返回原始响应，Body 即 zip 字节）、`Upload`（多文件流式
  multipart，重复导入默认**拒绝**，除非 `overwrite=true`）、`AdminAPIStatus`
  （上游 admin.enabled 三态：`nil` = 非默认分组读不到）、`CreditsSnapshot`
  （**只读本地面板记录**的积分快照，带 `known` 区分账号是否还在）、
  `ProbeModels`（账号 × 模型主动探测，分类 ok/rate/credit/auth/no_model/error）、
  `RefreshTokens`（一键全量续期，末尾只重载一次上游）。
- **计划任务**（`Accounts()`）：`RefreshPlan` / `SaveRefreshPlan` /
  `RunRefreshPlan`（每 N 天全量刷令牌；保存时三项必填），
  `DailyPlan` / `SaveDailyPlan` / `RunDailyTask`（签到 / 领补偿 / 领奖 / 做任务），
  以及 `ClaimCompensation`（领活动补偿，已领过的直接跳过）。
- **密钥导出与本机导入**（`Keys()`）：`Export`（cc-switch / ZCode 配置片段，
  只读无副作用）、`LocalImportStatus`、`DetectLocalClient`、
  `ImportLocal`（深链优先 / 直接写配置；仅面板跑在本机时可用，返回体不回显密钥）。
- **`MessagesCountTokens` / `CountTokens`**：Anthropic 的 token **估算**端
  （`POST /v1/messages/count_tokens`，走网关密钥的完整鉴权）。
- **`System().UpdateFrom`** 与 `UpdateIn` / `UpdateSource*` 常量：一键更新可选
  `source=release|uploaded`（v2.10 起离线升级包与在线发布走同一条流程）。

### 变更

- `System().SetUpstreamRef` 标记 **Deprecated**：v2.10 起上游随管理端发布，
  `/api/system/upstream-ref` 已移除，调用会 404；保留只为兼容 ≤ v2.9 的服务端。
- `scripts/audit_endpoints.py` 第三次修匹配规则：无参数端点要求**整条路径**字面量，
  参数在中间时要求**参数之后的静态段**也有字面量。旧规则只要 SDK 里有
  `/api/accounts` 就认为 `/api/accounts/export`、`/api/accounts/refresh-plan/run`
  都覆盖了 —— v0.2.0 说的「146/148 具名」因此是**高估**，实际有 17 个端点
  （后面这些批量/计划端点）没有具名方法，本版补齐。

### 说明

- 上传类接口（`Upload` / `UploadPackage`）**不参与重试、不做 401 自动重登**：
  请求体是流式的，重发要调用方重新打开文件。
- `Accounts().Export` 与 `System().Update*` 只接受**会话登录**，API Token（`wbt_`）
  会拿到 403；`Upload` 等其余管理端接口用 `wbt_` 即可。

## [0.2.0] - 2026-10-20

对齐 workbuddy-manager **v2.7.0**：148 个端点里 146 个有具名方法，剩下两个是
**别名路径**（`/v2/chat/completions`、`/responses`），用 `Do` / `DoStream` 直达。
`scripts/audit_endpoints.py` 的退出码为 0。

> 勘误（0.3.0 补记）：当时的「146/148」是审计脚本**匹配规则过松**造成的误报，
> 实际漏了 17 个更深的端点（账号批量 / 计划任务 / 本机导入 / count_tokens）。
> 脚本已在 0.3.0 修正。

### 新增

- **`Proxies()`**：出口线路表与绑定（`Routes` / `CreateRoute` / `UpdateRoute` /
  `DeleteRoute` / `ImportRoutes` / `ProbeRoute` / `Bindings` / `BindAccounts` /
  `AutoAssign` / `AccountRoutes`），池配置（`SavePool`），面板自身的出口代理
  （`Settings` / `SaveSettings` / `ProbeSettings`）。多分组部署可带 `upstream_id`。
- **`PGSync()`**：PostgreSQL 异地备份 —— `Get` / `Save` / `Test` / `Export` /
  `Import` / `Status`。密码读取时为掩码，保存时回传掩码表示「不改动」。
- **`RedPackets()`**：红包（批量发放带额度的密钥）—— `Create`（返回含**明文
  密钥，仅此一次**）/ `List` / `Get` / `Revoke`；以及**公开**的抽奖端
  `ClaimInfo` / `Claim`：不需要任何凭据，为此把 `/api/claim/*` 加入客户端的
  「公开路径」白名单，既不注入凭据也不会因「没配凭据」被自己拦下。
- **`Upstreams()`**：多上游（账号池分组）—— `List` / `Create` / `Update` /
  `Delete` / `Probe`。与 `Settings().Upstream()`（上游自己的 config.json）不是
  一回事，注意区分。
- **`Usage()`**：网关侧的 `GET /v1/usage`，查**当前密钥**的用量与额度
  （`tokens` / `credits` 各带 `used` / `limit` / `remaining`，不限额度为 `null`）。
- **`Stats().Hourly` / `Stats().CreditsFlow`**：某天按小时的用量（服务端固定
  返回 24 个桶），以及积分收支（收入 / 支出 / 净额三套口径都在）。
- **`Settings().GitHubToken` / `SaveGitHubToken` / `GitHubMirror` /
  `SaveGitHubMirror`**：GitHub Token 只回掩码，镜像可设可清。
- **`System().Package` / `UploadPackage` / `UploadPackageFile` / `DeletePackage`**：
  离线升级包。上传走**流式 multipart**，不把整包读进内存。
- **`Accounts().SetProxy` / `SetWeight` / `ForceCooling` / `Move`**：账号级线路
  绑定、手写权重（乘数）、强制冷却、跨分组转移。

### 改进

- `scripts/audit_endpoints.py` 现在解析文件里**所有** `APIRouter`（此前只认第一个，
  于是同一文件里第二个 router（红包抽奖端）的端点被静默漏检）。
- 上传接口单独说明：**不参与重试、也不做 401 自动重登** —— 请求体是一次性的，
  重登要重发，而流式体已经被消费。会话过期时会直接看到 401，重登后再传即可。

### 说明

- 没有破坏性变更：既有方法签名一律未动，新能力全部以新方法 / 新服务加入。
- 新增类型集中在 `types_admin.go`；字段仍与官方前端 `web/lib/types.ts` 对齐，
  可空字段用指针区分「缺失 / null」与零值。

## [0.1.5] - 2026-09-24

### 新增

- **`Healthz()` / `AdminHealthz()`**：网关与管理端的**存活探测**，**无需任何凭据**，
  适合监控探针。为此在鉴权注入里加了「已知公开路径」白名单——不再因为「没配凭据」
  而在客户端就报错，也不会把凭据发到公开端点上。
- 与最新上游（**v1.0.68**）做了一次**端点差分**（机械比对，非印象）：87 个端点中
  85 个有 SDK 覆盖；剩余为两个**别名路径**（`/v2/chat/completions`、`/responses`）——
  别名可用 `Do` / `DoStream` 直达。
- **`scripts/audit_endpoints.py`**：把上面这次核对固化成脚本 ——
  `python scripts/audit_endpoints.py /path/to/workbuddy-manager`，
  退出码可直接用于 CI / 发版检查（只有出现「未声明的未覆盖端点」才非 0）。

## [0.1.4] - 2026-09-24

### 新增

- **`Tokens()`**：管理面作用域化 API Token 的管理（`List` / `Create` / `Update` /
  `Delete`），对齐上游 **v1.0.68** 新增的 `/api/tokens`。`Create` 返回的明文
  **仅此一次**；`TokenPatch.ExpiresAt` 传指向 0 的指针表示「改为永不过期」。
- 示例与冒烟新增**令牌全链路**：用会话建令牌 → 用 `WithAdminToken` 鉴权 →
  校验权限边界（readonly 拒写、admin 令牌被「会话专属」接口拒、令牌不能管令牌、
  删除后立即 401）。

### 变更

- README 的 API Token 链接改指已合并的
  [docs/api-tokens.md](https://github.com/ithtelab/workbuddy-manager/blob/main/docs/api-tokens.md)
  （此前指向设计 PR）。

## [0.1.3] - 2026-09-24

### 新增

- **`WithAdminToken("wbt_...")`**：用管理面 API Token（`Authorization: Bearer`）
  访问 `/api/*`，适合脚本 / CI——无需用户名密码与会话往返。优先级高于 Cookie，
  且 401 时**不触发**重登重试。需服务端支持「作用域化 API Token」。
- **`Version()`**：库版本号；默认值见 `version.go`，发行构建可用
  `-ldflags -X github.com/sanguine886/workbuddy-sdk.version=...` 覆盖。
- 回归测试：默认 UA 携带版本号、Token 注入与优先级、401 不重试、
  Token 不串到数据面。

### 变更

- 默认 User-Agent 由 `Version()` 派生，不再写死——修掉「发到 v0.1.2 了、UA 还写着
  0.1」这类不一致。

## [0.1.2] - 2026-09-24

### 新增

- **`WithSSEEventMode(true)`**：流式解析支持「按事件」模式。同一事件内的多条
  `data:` 行按 SSE 规范以 `\n` 拼接后整体交付，注释与 `event:` / `id:` / `retry:`
  字段忽略。默认仍是「按行」模式（零解析，兼容既有行为）。事件模式的载荷是新
  分配的切片，可跨回调保留。
- **可运行的 godoc 示例**：`ExampleNewClient`、`ExampleClient_Chat`、
  `ExampleClient_ChatStream`、`ExampleWithSSEEventMode`、`ExampleClient_Accounts`、
  `ExampleClient_Keys` —— 均带 `// Output`，`go test` 会校验。
- 回归测试：多行 data 拼接、事件分隔与 EOF flush、注释忽略、按行模式默认行为、
  事件模式载荷跨回调有效。

## [0.1.1] - 2026-09-23

自查后的一轮修复与增强。

### 修复

- **自定义 User-Agent 被覆盖**：`WithHeader("User-Agent", ...)` 会被默认 UA 静默覆盖，
  现在仅在调用方未指定时才写入默认值。
- **`Chat` 可能被当流式处理**：若调用方把 `stream` 塞进 `ChatRequest.Extra`，非流式
  请求体会带上 `stream:true`。现在 `Chat` / `ChatStream` 一律由建模字段决定该键。
- **路径前缀判断过宽**：`/v1abc`、`/apifoo` 之类会被误判为网关 / 管理面路径并注入
  鉴权头；改为按**路径段边界**匹配（`/api` 匹配 `/api` 与 `/api/xxx`，不匹配 `/apifoo`）。
- **`SetRealm` 数据竞争**：realm 改用原子读写，与 `Client` 的并发安全声明一致。
- **错误信息可能被截成半个汉字**：`truncate` 改为按 UTF-8 边界回退。
- **`PasswordAuth` 漏配 baseURL** 时会发出非法 URL；现在直接返回可执行的错误提示。

### 新增

- **`WithAdminLogin(user, pass)`**：登录地址自动取自 `NewClient` 的 baseURL，
  免去重复传参（原 `WithAdminCredentials` 保留）。
- **`WithRetry(RetryPolicy)`**：对**幂等请求**（GET / HEAD / OPTIONS）的指数退避重试，
  支持 `Retry-After`、状态码白名单与网络错误开关；非幂等方法永不重试。
  默认关闭。
- 回归测试：SSE 流式解析、重试与不重试、UA 覆盖、路径前缀精度、Extra 泄漏、
  并发 SetRealm、UTF-8 截断等。

### 变更

- CI 升级到 `actions/checkout@v7` / `actions/setup-go@v7`。

## [0.1.0] - 2026-09-23

首个可用版本。

- 数据面 `/v1/*`：`Chat` `ChatStream` `Responses` `ResponsesStream` `Messages`
  `MessagesStream` `Models`。
- 管理面 `/api/*`：`Accounts` `Keys` `Stats` `Logs` `Security` `Settings` `Users`
  `System` `Tasks` `Upstream` `Catalog` `Playground`，以及 `Me` `Logout`
  `RevokeSessions`；另有 `Do` / `DoJSON` / `DoStream` 逃生舱。
- 仅标准库；函数式 Option；`AdminAuth` / `GatewayKey` 接口；按路径前缀自动注入鉴权。

[0.1.1]: https://github.com/sanguine886/workbuddy-sdk/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/sanguine886/workbuddy-sdk/releases/tag/v0.1.0
