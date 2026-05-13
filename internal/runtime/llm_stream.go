package runtime

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// hyperstreamSpecVersion is the wire-protocol version emitted in the `v=`
// attribute of every envelope. Bump when hyperstream/spec breaks compatibility.
const hyperstreamSpecVersion = 0

// hyperstreamWriter serializes hyperstream <hs-partial> envelopes onto a
// Server-Sent Events stream. It is the kilnx server-side counterpart of
// the hyperstream JS client (https://github.com/andreahlert/hyperstream).
type hyperstreamWriter struct {
	w        http.ResponseWriter
	flusher  http.Flusher
	target   string
	swap     string
	suspense string
	channel  string
	seq      int64
}

// writeDelta emits a non-final partial carrying a token chunk.
func (h *hyperstreamWriter) writeDelta(text string) error {
	if text == "" {
		return nil
	}
	h.seq++
	env := h.envelope(text, false)
	return h.writeSSE(env)
}

// writeFinal closes the partial; clients use this to drop suspense placeholders.
func (h *hyperstreamWriter) writeFinal() error {
	h.seq++
	return h.writeSSE(h.envelope("", true))
}

func (h *hyperstreamWriter) envelope(body string, final bool) string {
	finalAttr := ""
	if final {
		finalAttr = ` final="true"`
	}
	// Single-line HTML envelope so SSE `data:` carries it as one line.
	// Body is HTML-escaped to neutralise model-generated markup that could
	// otherwise terminate the wrapper or inject foreign tags.
	return fmt.Sprintf(
		`<hs-partial v="%d" channel=%q seq="%d" ts="%d" target=%q swap=%q suspense-id=%q%s>%s</hs-partial>`,
		hyperstreamSpecVersion,
		h.channel,
		h.seq,
		time.Now().UnixMilli(),
		h.target,
		h.swap,
		h.suspense,
		finalAttr,
		html.EscapeString(body),
	)
}

func (h *hyperstreamWriter) writeSSE(envelope string) error {
	if _, err := fmt.Fprintf(h.w, "id: %d\ndata: %s\n\n", h.seq, envelope); err != nil {
		return err
	}
	h.flusher.Flush()
	return nil
}

// executeLLMStream runs a streaming Messages.New call and writes hyperstream
// envelopes onto w. Caller is responsible for setting any auth/CSRF before
// invocation; this function takes over the response (writes SSE headers).
func executeLLMStream(ctx context.Context, w http.ResponseWriter, node parser.Node, db rowQuerier, params map[string]string) error {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY not set")
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("response writer does not support flushing")
	}

	model := node.LLMModel
	if model == "" {
		model = string(anthropic.ModelClaudeHaiku4_5)
	}
	swap := strings.TrimSpace(node.LLMStreamSwap)
	if swap == "" {
		swap = "append"
	}
	suspense := params["_stream_suspense"]
	if suspense == "" {
		suspense = fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	channel := params["_stream_channel"]
	if channel == "" {
		channel = suspense
	}

	msgs, err := buildLLMMessages(node, db, params)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Disable proxy buffering (nginx/Cloudflare). Hyperstream needs flush per chunk.
	w.Header().Set("X-Accel-Buffering", "no")

	hw := &hyperstreamWriter{
		w:        w,
		flusher:  flusher,
		target:   node.LLMStreamTarget,
		swap:     swap,
		suspense: suspense,
		channel:  channel,
	}

	if len(msgs) == 0 {
		if err := hw.writeDelta("Olá! Como posso ajudar?"); err != nil {
			return err
		}
		return hw.writeFinal()
	}

	req := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 1024,
		Messages:  msgs,
	}
	if node.LLMSystem != "" {
		req.System = []anthropic.TextBlockParam{{Text: node.LLMSystem}}
	}

	stream := llmClient.Messages.NewStreaming(ctx, req)
	defer func() { _ = stream.Close() }()

	chunks := 0
	for stream.Next() {
		evt := stream.Current()
		variant, ok := evt.AsAny().(anthropic.ContentBlockDeltaEvent)
		if !ok {
			continue
		}
		text, ok := variant.Delta.AsAny().(anthropic.TextDelta)
		if !ok {
			continue
		}
		if err := hw.writeDelta(text.Text); err != nil {
			return fmt.Errorf("write delta: %w", err)
		}
		chunks++
	}
	if err := stream.Err(); err != nil {
		return fmt.Errorf("anthropic stream: %w", err)
	}

	if err := hw.writeFinal(); err != nil {
		return err
	}
	fmt.Printf("  llm-stream %s -> %d chunks (channel=%s)\n", model, chunks, channel)
	return nil
}
