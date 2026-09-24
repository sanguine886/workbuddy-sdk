package wbsdk

import (
	"bytes"
	"io"
)

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

// splitSSEField 按 SSE 规范拆分一行 "field: value"：在**第一个**冒号处切分，
// 并去掉 value 的单个前导空格。没有冒号时整行视为字段名、值为空。
func splitSSEField(line []byte) (field string, value []byte) {
	if i := bytes.IndexByte(line, ':'); i >= 0 {
		return string(line[:i]), bytes.TrimPrefix(line[i+1:], []byte(" "))
	}
	return string(line), nil
}

// parseSSEEvents 按 SSE 规范把流切分为**事件**交付：
//
//   - 同一事件内的多条 `data:` 行以 "\n" 拼接后整体交付；
//   - 注释行（以 ':' 开头）与 event / id / retry 字段忽略；
//   - 事件以空行分隔；流结束时 flush 最后一个未闭合的事件；
//   - 完全没有 data 的事件（如只有注释或 event: 行）不触发回调。
//
// 交付的字节是新分配的切片，**跨回调有效**（与按行模式不同）。
func parseSSEEvents(r io.Reader, fn func(raw []byte) error) error {
	sc := newSSEScanner(r)
	var (
		buf      []byte
		haveData bool
	)
	flush := func() error {
		if !haveData {
			return nil
		}
		payload := buf
		buf, haveData = nil, false
		return fn(payload)
	}

	for sc.Scan() {
		line := bytes.TrimRight(sc.Bytes(), "\r")
		switch {
		case len(line) == 0: // 事件分隔
			if err := flush(); err != nil {
				return err
			}
		case line[0] == ':': // 注释
			continue
		default:
			field, value := splitSSEField(line)
			if field != "data" {
				continue
			}
			if haveData {
				buf = append(buf, '\n')
			}
			buf = append(buf, value...)
			haveData = true
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return flush()
}
