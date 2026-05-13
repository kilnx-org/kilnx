package runtime

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// flushRecorder is defined in stream_test.go (shared across LLM and SSE tests).

func TestHyperstreamWriter_DeltaAndFinal(t *testing.T) {
	rec := newFlushRecorder()
	hw := &hyperstreamWriter{
		w:        rec,
		flusher:  rec,
		target:   "#chat",
		swap:     "append",
		suspense: "msg-1",
		channel:  "conv-42",
	}

	if err := hw.writeDelta("Hello "); err != nil {
		t.Fatalf("delta1: %v", err)
	}
	if err := hw.writeDelta("<world>"); err != nil {
		t.Fatalf("delta2: %v", err)
	}
	if err := hw.writeFinal(); err != nil {
		t.Fatalf("final: %v", err)
	}

	out := rec.Body.String()

	// Each envelope is on its own SSE message (`data: ...\n\n`).
	parts := strings.Split(strings.TrimRight(out, "\n"), "\n\n")
	if len(parts) != 3 {
		t.Fatalf("expected 3 SSE messages, got %d:\n%s", len(parts), out)
	}

	for i, p := range parts {
		if !strings.Contains(p, "data: <hs-partial ") {
			t.Errorf("part %d missing data prefix: %q", i, p)
		}
	}

	if !strings.Contains(parts[0], `target="#chat"`) ||
		!strings.Contains(parts[0], `swap="append"`) ||
		!strings.Contains(parts[0], `suspense-id="msg-1"`) ||
		!strings.Contains(parts[0], `channel="conv-42"`) {
		t.Errorf("part0 attrs unexpected: %q", parts[0])
	}
	if !strings.Contains(parts[0], `>Hello </hs-partial>`) {
		t.Errorf("part0 missing literal text: %q", parts[0])
	}
	// Model output containing HTML must be escaped.
	if !strings.Contains(parts[1], `&lt;world&gt;`) {
		t.Errorf("part1 should HTML-escape angle brackets: %q", parts[1])
	}
	if !strings.Contains(parts[2], `final="true"`) {
		t.Errorf("final envelope missing final attr: %q", parts[2])
	}
	// seq must be monotonic.
	if !strings.Contains(parts[0], `seq="1"`) ||
		!strings.Contains(parts[1], `seq="2"`) ||
		!strings.Contains(parts[2], `seq="3"`) {
		t.Errorf("seq not monotonic across parts: %s", out)
	}
}

func TestHyperstreamWriter_DeltaSkipsEmpty(t *testing.T) {
	rec := newFlushRecorder()
	hw := &hyperstreamWriter{w: rec, flusher: rec, target: "#x", swap: "inner", suspense: "s", channel: "c"}
	if err := hw.writeDelta(""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("empty delta should not write, got %q", rec.Body.String())
	}
}

func TestExecuteLLMStream_MissingAPIKey(t *testing.T) {
	os.Unsetenv("ANTHROPIC_API_KEY")
	rec := newFlushRecorder()
	node := parser.Node{Type: parser.NodeLLM, LLMStreamTarget: "#x"}
	err := executeLLMStream(context.Background(), rec, node, newMockExecutor(), nil)
	if err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY not set") {
		t.Fatalf("expected api-key error, got %v", err)
	}
}

func TestExecuteLLMStream_NonFlusherFails(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")
	type plainWriter struct{ http.ResponseWriter }
	w := &plainWriter{ResponseWriter: nil}
	node := parser.Node{Type: parser.NodeLLM, LLMStreamTarget: "#x"}
	err := executeLLMStream(context.Background(), w, node, newMockExecutor(), nil)
	if err == nil || !strings.Contains(err.Error(), "flushing") {
		t.Fatalf("expected flusher error, got %v", err)
	}
}

func TestExecuteLLMStream_EmptyHistoryWritesGreeting(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	rec := newFlushRecorder()
	node := parser.Node{Type: parser.NodeLLM, LLMStreamTarget: "#chat"}
	if err := executeLLMStream(context.Background(), rec, node, newMockExecutor(), nil); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	out := rec.Body.String()
	if !strings.Contains(out, "Como posso ajudar") {
		t.Errorf("expected greeting, got %q", out)
	}
	if !strings.Contains(out, `final="true"`) {
		t.Errorf("expected final envelope, got %q", out)
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected SSE content-type, got %q", rec.Header().Get("Content-Type"))
	}
}

func TestExecuteLLMStream_EndToEnd(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher, _ := w.(http.Flusher)
		writeEvent := func(event string, data string) {
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
			flusher.Flush()
		}
		writeEvent("message_start", `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-test","content":[],"stop_reason":null,"usage":{"input_tokens":1,"output_tokens":0}}}`)
		writeEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		writeEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello "}}`)
		writeEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world"}}`)
		writeEvent("content_block_stop", `{"type":"content_block_stop","index":0}`)
		writeEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`)
		writeEvent("message_stop", `{"type":"message_stop"}`)
	}))
	defer srv.Close()
	defer withTestLLMClient(srv.URL)()

	db := newMockExecutor()
	db.queryRowsWithParamsResults["SELECT papel, conteudo FROM messages"] = []database.Row{
		{"papel": "user", "conteudo": "oi"},
	}

	rec := newFlushRecorder()
	node := parser.Node{
		Type:            parser.NodeLLM,
		LLMHistorySQL:   "SELECT papel, conteudo FROM messages",
		LLMStreamTarget: "#chat-msgs",
		LLMStreamSwap:   "append",
	}
	params := map[string]string{"_stream_suspense": "msg-42", "_stream_channel": "conv-7"}

	if err := executeLLMStream(context.Background(), rec, node, db, params); err != nil {
		t.Fatalf("executeLLMStream failed: %v", err)
	}

	out := rec.Body.String()
	parts := strings.Split(strings.TrimRight(out, "\n"), "\n\n")
	// Expect: 2 deltas + 1 final = 3 envelopes.
	if len(parts) != 3 {
		t.Fatalf("expected 3 envelopes, got %d:\n%s", len(parts), out)
	}
	if !strings.Contains(parts[0], `>Hello </hs-partial>`) {
		t.Errorf("first delta unexpected: %q", parts[0])
	}
	if !strings.Contains(parts[1], `>world</hs-partial>`) {
		t.Errorf("second delta unexpected: %q", parts[1])
	}
	if !strings.Contains(parts[2], `final="true"`) {
		t.Errorf("final envelope missing: %q", parts[2])
	}
	for _, p := range parts {
		if !strings.Contains(p, `target="#chat-msgs"`) {
			t.Errorf("missing target attr: %q", p)
		}
		if !strings.Contains(p, `suspense-id="msg-42"`) {
			t.Errorf("missing suspense id: %q", p)
		}
		if !strings.Contains(p, `channel="conv-7"`) {
			t.Errorf("missing channel: %q", p)
		}
	}

	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected SSE content-type, got %q", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("X-Accel-Buffering") != "no" {
		t.Errorf("expected proxy-buffering disabled, got %q", rec.Header().Get("X-Accel-Buffering"))
	}
}

func TestExecuteLLMStream_HistoryError(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	db := newMockExecutor()
	db.queryRowsWithParamsErr["SELECT papel, conteudo FROM messages"] = fmt.Errorf("db down")

	rec := newFlushRecorder()
	node := parser.Node{
		Type:            parser.NodeLLM,
		LLMHistorySQL:   "SELECT papel, conteudo FROM messages",
		LLMStreamTarget: "#chat",
	}
	err := executeLLMStream(context.Background(), rec, node, db, nil)
	if err == nil || !strings.Contains(err.Error(), "llm history query") {
		t.Fatalf("expected history error, got %v", err)
	}
}
