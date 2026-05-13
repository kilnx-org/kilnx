package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// claudeBin can be overridden in tests via t.Setenv("PATH", ...) or by
// setting KILNX_CLAUDE_BIN explicitly. Default is the binary on PATH.
func claudeBin() string {
	if v := os.Getenv("KILNX_CLAUDE_BIN"); v != "" {
		return v
	}
	return "claude"
}

// agentResult is the projection of the `result` event exposed to the DSL
// as `:<name>.text`, `:<name>.session_id`, `:<name>.cost_usd`,
// `:<name>.duration_ms`, `:<name>.stop_reason`.
type agentResult struct {
	Text       string
	SessionID  string
	StopReason string
	CostUSD    float64
	DurationMS int64
}

// streamJSONEvent is the subset of fields we care about across all event
// types emitted by `claude -p --output-format stream-json`.
type streamJSONEvent struct {
	Type         string          `json:"type"`
	Subtype      string          `json:"subtype,omitempty"`
	SessionID    string          `json:"session_id,omitempty"`
	Message      json.RawMessage `json:"message,omitempty"`
	Event        json.RawMessage `json:"event,omitempty"`
	Result       string          `json:"result,omitempty"`
	IsError      bool            `json:"is_error,omitempty"`
	StopReason   string          `json:"stop_reason,omitempty"`
	TotalCostUSD float64         `json:"total_cost_usd,omitempty"`
	DurationMS   int64           `json:"duration_ms,omitempty"`
}

// streamInnerEvent is the nested envelope inside `stream_event.event`.
type streamInnerEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
	ContentBlock struct {
		Type  string          `json:"type"`
		Name  string          `json:"name,omitempty"`
		ID    string          `json:"id,omitempty"`
		Input json.RawMessage `json:"input,omitempty"`
	} `json:"content_block,omitempty"`
}

// assistantMessage covers the shape of `assistant.message` for tool-use
// detection when show-tools is on.
type assistantMessage struct {
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text,omitempty"`
		Name  string          `json:"name,omitempty"`
		ID    string          `json:"id,omitempty"`
		Input json.RawMessage `json:"input,omitempty"`
	} `json:"content"`
}

var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// executeLLMAgent spawns `claude -p` and consumes the stream-json output.
// When node.LLMStreamTarget is set the response writer must be a
// http.Flusher; assistant text deltas are emitted as hyperstream
// envelopes in real time. The final agentResult exposes cost, duration,
// session id, stop reason and the full text.
func executeLLMAgent(ctx context.Context, node parser.Node, app *parser.App, params map[string]string, w http.ResponseWriter) (*agentResult, error) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	cwd, cleanup, err := resolveAgentCwd(node, app, params)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// MCP config materialisation.
	mcpConfigPath, mcpCleanup, err := writeMCPConfig(node, app)
	if err != nil {
		return nil, err
	}
	defer mcpCleanup()

	args, err := buildAgentArgs(node, params, mcpConfigPath)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, claudeBin(), args...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("agent stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("agent stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("agent stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("agent spawn: %w", err)
	}

	// Send user prompt to stdin then close. claude reads the entire stdin
	// before producing the first turn.
	prompt := params["prompt"]
	if prompt == "" {
		prompt = params["message"]
	}
	if prompt != "" {
		_, _ = io.WriteString(stdin, prompt)
	}
	_ = stdin.Close()

	// Streaming setup if requested.
	var textHW, toolHW *hyperstreamWriter
	if node.LLMStreamTarget != "" {
		flusher, ok := w.(http.Flusher)
		if !ok {
			_ = cmd.Process.Kill()
			return nil, fmt.Errorf("agent stream: response writer does not support flushing")
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
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		textHW = &hyperstreamWriter{
			w: w, flusher: flusher,
			target: node.LLMStreamTarget, swap: swap,
			suspense: suspense, channel: channel,
		}
		if node.LLMAgentShowTools {
			toolHW = &hyperstreamWriter{
				w: w, flusher: flusher,
				target: node.LLMStreamTarget, swap: swap,
				suspense: suspense, channel: channel + "-tools",
			}
		}
	}

	// Drain stderr in the background so the process doesn't block on a
	// full pipe. Error output is forwarded to the kilnx log.
	stderrBuf := &strings.Builder{}
	go func() {
		s := bufio.NewScanner(stderr)
		s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for s.Scan() {
			stderrBuf.WriteString(s.Text())
			stderrBuf.WriteByte('\n')
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	res := &agentResult{}
	textBuf := &strings.Builder{}
	turns := 0
	maxTurns := node.LLMAgentMaxTurns

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var evt streamJSONEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			fmt.Printf("  llm-agent: skipping unparseable line: %s\n", line)
			continue
		}

		switch evt.Type {
		case "system":
			if evt.SessionID != "" {
				res.SessionID = evt.SessionID
			}
		case "stream_event":
			if len(evt.Event) == 0 {
				continue
			}
			var inner streamInnerEvent
			if err := json.Unmarshal(evt.Event, &inner); err != nil {
				continue
			}
			switch inner.Type {
			case "content_block_delta":
				if inner.Delta.Type == "text_delta" && inner.Delta.Text != "" {
					textBuf.WriteString(inner.Delta.Text)
					if textHW != nil {
						_ = textHW.writeDelta(inner.Delta.Text)
					}
				}
			case "content_block_start":
				if inner.ContentBlock.Type == "tool_use" && toolHW != nil {
					payload := fmt.Sprintf("tool_use:%s", inner.ContentBlock.Name)
					_ = toolHW.writeDelta(payload)
				}
			}
		case "assistant":
			turns++
			if maxTurns > 0 && turns > maxTurns {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				return nil, fmt.Errorf("agent: max turns (%d) exceeded", maxTurns)
			}
			// If show-tools is on, surface tool_use blocks even when not
			// streaming partial messages. Tool results arrive as `user`
			// events later.
			if toolHW != nil && len(evt.Message) > 0 {
				var msg assistantMessage
				if err := json.Unmarshal(evt.Message, &msg); err == nil {
					for _, b := range msg.Content {
						if b.Type == "tool_use" {
							_ = toolHW.writeDelta(fmt.Sprintf("tool_use:%s", b.Name))
						}
					}
				}
			}
		case "user":
			if toolHW != nil && len(evt.Message) > 0 {
				_ = toolHW.writeDelta("tool_result")
			}
		case "result":
			if evt.SessionID != "" {
				res.SessionID = evt.SessionID
			}
			res.CostUSD = evt.TotalCostUSD
			res.DurationMS = evt.DurationMS
			res.StopReason = evt.StopReason
			if evt.Subtype != "" && res.StopReason == "" {
				res.StopReason = evt.Subtype
			}
			if evt.Result != "" && textBuf.Len() == 0 {
				textBuf.WriteString(evt.Result)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		_ = cmd.Wait()
		return nil, fmt.Errorf("agent stream scan: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		errText := strings.TrimSpace(stderrBuf.String())
		if errText == "" {
			return nil, fmt.Errorf("agent wait: %w", err)
		}
		return nil, fmt.Errorf("agent wait: %w (stderr: %s)", err, errText)
	}

	res.Text = textBuf.String()
	if textHW != nil {
		_ = textHW.writeFinal()
	}
	if toolHW != nil {
		_ = toolHW.writeFinal()
	}
	fmt.Printf("  llm-agent -> session=%s cost=$%.6f duration=%dms stop=%s\n",
		res.SessionID, res.CostUSD, res.DurationMS, res.StopReason)
	return res, nil
}

// buildAgentArgs returns the argv passed to `claude`. The user prompt is
// piped through stdin; `-p` puts the CLI in non-interactive mode.
func buildAgentArgs(node parser.Node, params map[string]string, mcpConfigPath string) ([]string, error) {
	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--verbose",
	}

	if node.LLMModel != "" {
		args = append(args, "--model", node.LLMModel)
	}
	if node.LLMSystem != "" {
		args = append(args, "--system-prompt", substituteParams(node.LLMSystem, params))
	}

	mode := node.LLMAgentPermissionMode
	if mode == "" {
		mode = "plan"
	}
	args = append(args, "--permission-mode", mode)

	if len(node.LLMAgentTools) > 0 {
		args = append(args, "--allowed-tools", strings.Join(node.LLMAgentTools, ","))
	}

	if node.LLMAgentBudget > 0 {
		args = append(args, "--max-budget-usd", strconv.FormatFloat(node.LLMAgentBudget, 'f', -1, 64))
	}

	if mcpConfigPath != "" {
		args = append(args, "--mcp-config", mcpConfigPath)
	}

	if node.LLMAgentResume != "" {
		resume := substituteParams(node.LLMAgentResume, params)
		if !uuidRegex.MatchString(resume) {
			return nil, fmt.Errorf("agent resume: %q is not a valid UUID", resume)
		}
		args = append(args, "--resume", resume)
	}

	return args, nil
}

// writeMCPConfig materialises a `--mcp-config` JSON file for the agent
// subset declared by node.LLMAgentMCP. Returns "" and a no-op cleanup
// when no servers are referenced.
func writeMCPConfig(node parser.Node, app *parser.App) (string, func(), error) {
	if len(node.LLMAgentMCP) == 0 {
		return "", func() {}, nil
	}
	if app == nil {
		return "", func() {}, fmt.Errorf("agent mcp: app not available")
	}

	servers := map[string]any{}
	for _, name := range node.LLMAgentMCP {
		var srv *parser.MCPServer
		for i := range app.MCPServers {
			if app.MCPServers[i].Name == name {
				srv = &app.MCPServers[i]
				break
			}
		}
		if srv == nil {
			return "", func() {}, fmt.Errorf("agent mcp: server %q not declared (top-level `mcp %s`)", name, name)
		}
		entry := map[string]any{}
		switch srv.Transport {
		case "sse", "http":
			entry["url"] = srv.URL
			entry["type"] = srv.Transport
		default:
			entry["command"] = srv.Command
			if len(srv.Args) > 0 {
				entry["args"] = srv.Args
			}
			if len(srv.Env) > 0 {
				entry["env"] = srv.Env
			}
		}
		servers[name] = entry
	}

	doc := map[string]any{"mcpServers": servers}
	buf, err := json.Marshal(doc)
	if err != nil {
		return "", func() {}, fmt.Errorf("agent mcp marshal: %w", err)
	}
	f, err := os.CreateTemp("", "kilnx-mcp-*.json")
	if err != nil {
		return "", func() {}, fmt.Errorf("agent mcp tmp: %w", err)
	}
	if _, err := f.Write(buf); err != nil {
		f.Close()
		_ = os.Remove(f.Name())
		return "", func() {}, fmt.Errorf("agent mcp write: %w", err)
	}
	_ = f.Close()
	path := f.Name()
	return path, func() { _ = os.Remove(path) }, nil
}
