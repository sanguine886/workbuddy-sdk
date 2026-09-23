// Package wbsdk 是 workbuddy-manager 的 Go 客户端库。
//
// workbuddy-manager（https://github.com/ithtelab/workbuddy-manager）是腾讯
// CodeBuddy 账号池的管理控制台 + OpenAI 兼容反代网关。它对外暴露**两个 API 面**，
// 本库两者都覆盖，共享同一套 Option / 鉴权 / 错误模型：
//
//   - 数据面（Data Plane）：/v1/*、/v2/chat/completions、/responses、
//     /v1/messages。鉴权用网关密钥（Authorization: Bearer wbk_...）。
//     Chat / ChatStream / Responses / Messages / Models。
//   - 管理面（Control Plane）：/api/*。鉴权用 HMAC 签名会话 Cookie（wb_session）。
//     Accounts / Keys / Stats / Logs / Security / Settings / Users / System /
//     Tasks / Upstream / Catalog / Playground，以及会话方法 Me / Logout /
//     RevokeSessions。
//
// # 边界
//
// 本库只负责**传输 / 鉴权 / 类型映射**，不承载任何业务语义：
//
//   - 数据面保留「raw 字节通道」哲学（对齐 is7Qin/codex-sdk 的定位）：流式只做
//     SSE data: 行提取，把完整帧字节交给调用方，事件语义、计费、编排留在网关侧。
//     非流式的结构化封装（Chat）是可选的便利层。
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
