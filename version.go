package wbsdk

// version 是库的版本号，默认值与最近一次发布保持一致。
//
// 发行构建可用 -ldflags 覆盖，避免「发版了但代码里还写着旧版本」：
//
//	go build -ldflags "-X github.com/sanguine886/workbuddy-sdk.version=v0.1.3"
var version = "0.1.3"

// Version 返回本库的版本号。
func Version() string { return version }

// defaultUserAgent 由版本号派生，是 UA 的**唯一来源**。
func defaultUserAgent() string {
	return "workbuddy-sdk/" + version + " (+https://github.com/sanguine886/workbuddy-sdk)"
}
