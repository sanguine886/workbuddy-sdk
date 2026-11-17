package wbsdk

import "encoding/json"

// StringMap 是「字符串键 → 字符串值」的 JSON 对象，解码时**跳过非字符串的值**。
//
// 为什么不用 map[string]string：这类字段多半是用户在面板上手工填的，服务端往往
// 只校验「是个对象」而不校验值的类型。一个被改成数字的键会让整个响应解码失败
// ——为了一个脏别名，把整份配置都读不出来，代价完全不对等。
//
// 编码行为与普通 map 一致。
type StringMap map[string]string

// UnmarshalJSON 逐键取值：字符串照收，其余（数字 / 布尔 / 对象 / null）丢弃。
func (m *StringMap) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := make(StringMap, len(raw))
	for k, v := range raw {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			out[k] = s
		}
	}
	*m = out
	return nil
}
