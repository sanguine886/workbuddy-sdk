package wbsdk

import "strings"

// Realm 是账号 / 模型 / 密钥的版本归属。
//
// 上游单实例双版本共存、共用账号池：国内版（cn）与国际版（global）。
// 账号、模型、密钥、日志、统计都按版本分区，绝大多数接口都带这个维度。
type Realm string

const (
	// RealmCN 国内版（copilot.tencent.com）。
	RealmCN Realm = "cn"
	// RealmGlobal 国际版（workbuddy.ai）。
	RealmGlobal Realm = "global"
	// RealmAny 不限制（密钥的存量形态，两版都能调）。
	RealmAny Realm = ""
)

// Valid 报告 r 是否是本库认识的值。
func (r Realm) Valid() bool {
	switch r {
	case RealmCN, RealmGlobal, RealmAny:
		return true
	default:
		return false
	}
}

// NormRealm 把任意输入归一化为 Realm：仅 "global"（大小写不敏感）视为国际版，
// 其余一律国内版——与后端 keysvc._norm_realm / 路由层的口径一致。
func NormRealm(v string) Realm {
	if strings.EqualFold(strings.TrimSpace(v), string(RealmGlobal)) {
		return RealmGlobal
	}
	return RealmCN
}
