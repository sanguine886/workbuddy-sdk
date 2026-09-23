package wbsdk

import "bytes"

// extractSSEDataLine 从一行 SSE 文本中提取 data: 载荷（零拷贝，返回的字节
// 引用传入的 line 切片）。
//
// 兼容 "data: value" 与 "data:value"；行尾 \r 会被剔除以支持 CRLF 流。
// 非 data 行（注释、event:、id:、空行）返回 (nil, false)。
// 这是**传输层 framing**，不是协议解析——载荷内容原样交给调用方。
func extractSSEDataLine(line []byte) ([]byte, bool) {
	line = bytes.TrimRight(line, "\r")
	if len(line) == 0 || line[0] == ':' {
		return nil, false
	}
	const prefix = "data:"
	if !bytes.HasPrefix(line, []byte(prefix)) {
		return nil, false
	}
	payload := line[len(prefix):]
	// 规范允许 "data: value"（一个可选空格）或 "data:value"。
	payload = bytes.TrimPrefix(payload, []byte(" "))
	return payload, true
}
