// Package wbsdk 是 workbuddy-manager 的 Go 客户端库。
//
// workbuddy-manager（https://github.com/ithtelab/workbuddy-manager）是腾讯
// CodeBuddy 账号池的管理控制台 + OpenAI 兼容反代网关。它对外暴露**两个 API 面**，
// 本库两者都覆盖，共享同一套 Option / 鉴权 / 错误模型：
//
//   - 数据面（Data Plane）：/v1/*、/v2/chat/completions、/responses、
//     /v1/messages。鉴权用网关密钥（Authorization: Bearer wbk_...）。
//     Chat / ChatStream / Responses / Messages / MessagesCountTokens / Models /
//     Usage / TokenUsage / Credits / ToolsSearch。
//   - 管理面（Control Plane）：/api/*。鉴权用 HMAC 签名会话 Cookie（wb_session）
//     或管理面 API Token（wbt_）。Accounts / AccountRisk / Keys / Stats / Logs /
//     Security / Settings / Tap / Watchdog / Users / System / Tasks / Upstream /
//     Upstreams / Proxies / PGSync / RedPackets / Catalog / Playground，
//     以及会话方法 Me / Logout / RevokeSessions。
//
// 例外一：红包抽奖端（/api/claim/*）由服务端刻意公开（收到分享链接的人本来就
// 没有账号），本库对它**不注入任何凭据**，也不要求配置凭据。余额单位元数据
// /api/status 同样公开（它只声明数值单位，不含任何额度）。
//
// 例外二：少数管理端接口服务端只接受**会话登录**（require_session_admin）——
// 一键更新（会重启服务）与批量导出账号（带凭据的 zip），API Token 会得到 403。
// 调用它们请用 WithAdminLogin 建客户端。
//
// 例外三：/api/usage/token/（余额端点）挂在管理面前缀下，但鉴权用的是**网关
// 密钥**——服务端直接读 Authorization 头，不认会话 Cookie。
//
// # 分组（多上游）
//
// 上游支持多分组：每组装一个独立的 workbuddy2api 实例、各有自己的账号目录。
// 账号级接口几乎全部接受 upstream_id，**不传就是操作默认分组**。单分组部署不必
// 关心；多分组部署用 WithUpstreamID 设默认值（或在个别调用里显式传参），
// 否则会静默作用到错误的实例上——而报错常常是「账号文件不存在」这种极具误导
// 性的 404。数据面（/v1/*）不受影响：用哪个上游由密钥决定。
//
// # 局部更新与 Ptr
//
// 部分配置接口是**局部合并**（以服务端当前值为底）：监控 SaveConfig、抓包
// SaveConfig、网关 SaveGatewayCORS。它们的字段都是指针：nil = 不改这一项，
// 指向零值 = 真的把它改成零值（例如清空已存的密码）。后者用 Ptr 构造：
//
//	patch := wbsdk.WatchdogMailPatch{Password: wbsdk.Ptr("")}
//
// 反面例子是 Settings().SaveAutoUpdate：那个接口是**整体替换**，所以请求体用
// 值类型（AutoUpdateConfig），缺字段直接 400 而不是保持原值。
//
// # 边界
//
// 本库只负责**传输 / 鉴权 / 类型映射**，不承载任何业务语义：
//
//   - 数据面保留「raw 字节通道」哲学（对齐 is7Qin/codex-sdk 的定位）：流式只做
//     SSE frame 提取，把载荷字节交给调用方，事件语义、计费、编排留在网关侧。
//     默认「按行」交付（每条 data: 回调一次，零解析）；WithSSEEventMode(true)
//     可切换为「按事件」交付（多行 data: 以 \n 拼接）。非流式的结构化封装
//     （Chat）是可选的便利层。
//   - 管理面天然是结构化 JSON，因此提供强类型方法。字段与官方前端
//     web/lib/types.ts 对齐。
//   - **不解析错误文案**：管理面的错误体是中文散文，网关面是 OpenAI 形状；
//     本库统一归一化为 *APIError（StatusCode / Code / Message / Hint / Raw），
//     多语言与展示留给调用方，避免把文案当契约。
//
// # 惰性构建与并发
//
// NewClient 零网络、零开销；底层 *http.Client 在首次请求时才创建（连接池复用）。
// Client 可并发使用。PasswordAuth 的自动重登是**单飞**的（并发 401 只触发一次
// 登录），且失败不重试——避免撞上服务端「5 次失败锁 10 分钟」的登录风控。
//
// # 依赖
//
// 仅标准库。
package wbsdk
