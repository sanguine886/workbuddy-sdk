# Changelog

本文件记录 workbuddy-sdk 的版本变更。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

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
