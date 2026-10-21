# Changelog

本文件记录 workbuddy-sdk 的版本变更。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [0.2.0] - 2026-10-20

对齐 workbuddy-manager **v2.7.0**：148 个端点里 146 个有具名方法，剩下两个是
**别名路径**（`/v2/chat/completions`、`/responses`），用 `Do` / `DoStream` 直达。
`scripts/audit_endpoints.py` 的退出码为 0。

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
