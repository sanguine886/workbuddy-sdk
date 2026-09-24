package wbsdk

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 事件模式：同一事件内的多条 data: 行以 "\n" 拼接；注释与 event/id/retry 忽略。
func TestParseSSEEventsMultiLine(t *testing.T) {
	stream := strings.Join([]string{
		": 这是注释，忽略",
		"event: message",
		"id: 7",
		"retry: 3000",
		"data: 第一行",
		"data: 第二行",
		"data:第三行", // 无空格也认
		"",
		"data: 独立事件",
		"",
		": 只有一个注释的事件，不该触发回调",
		"",
		"data: 无结尾换行的最后一条", // EOF 时 flush
	}, "\n")

	var got []string
	if err := parseSSEEvents(strings.NewReader(stream), func(raw []byte) error {
		got = append(got, string(raw))
		return nil
	}); err != nil {
		t.Fatalf("parseSSEEvents: %v", err)
	}

	want := []string{"第一行\n第二行\n第三行", "独立事件", "无结尾换行的最后一条"}
	if len(got) != len(want) {
		t.Fatalf("events = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// 默认（按行）模式：每条 data: 行各交付一次，保持与 codex-sdk 一致。
func TestParseSSEByLineDefault(t *testing.T) {
	stream := "data: a\ndata: b\n\ndata: c\n"
	var got []string
	sc := newSSEScanner(strings.NewReader(stream))
	for sc.Scan() {
		if p, ok := extractSSEDataLine(sc.Bytes()); ok {
			got = append(got, string(p))
		}
	}
	want := []string{"a", "b", "c"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

// WithSSEEventMode 应让 ChatStream 交付合并后的事件载荷。
func TestChatStreamEventMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, ": ping\n\n")
		_, _ = io.WriteString(w, "data: {\"part\":1,\ndata: \"part\":2}\n\n") // 一个事件拆成两行
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithGatewayKey("k"), WithSSEEventMode(true))
	var got []string
	err := c.ChatStream(context.Background(), &ChatRequest{
		Model:    "m",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	}, func(raw []byte) error {
		got = append(got, string(raw))
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("events = %#v", got)
	}
	if got[0] != "{\"part\":1,\n\"part\":2}" {
		t.Fatalf("multi-line data not joined: %q", got[0])
	}
	if got[1] != "[DONE]" {
		t.Fatalf("last event = %q", got[1])
	}
}

// 事件模式下，回调里保留的字节必须跨回调有效（与按行模式的临时缓冲不同）。
func TestSSEEventModePayloadSurvivesCallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, s := range []string{"one", "two", "three"} {
			_, _ = io.WriteString(w, "data: "+s+"\n\n")
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithGatewayKey("k"), WithSSEEventMode(true))
	var kept []string
	err := c.DoStream(context.Background(), http.MethodPost, "/v1/chat/completions", []byte(`{}`),
		func(raw []byte) error {
			kept = append(kept, string(raw)) // 直接保留引用，事件模式下是安全的
			return nil
		})
	if err != nil {
		t.Fatalf("DoStream: %v", err)
	}
	if strings.Join(kept, "|") != "one|two|three" {
		t.Fatalf("kept = %#v", kept)
	}
}
