package parser

import "github.com/kilnx-org/kilnx/internal/spec"

// Spec entries for the `llm` body keyword and its discriminated body
// children (`response` for single-shot / chat / streaming calls via the
// Anthropic Messages API, `agent` for agentic loops spawned as `claude`
// CLI subprocesses).
//
// Mutually-exclusive children (`response` vs `agent`) are not expressed
// in the spec entity; the analyzer enforces exclusivity at check time,
// mirroring how `validate` does not express its bare-vs-block form in
// the registry.

func init() {
	llmParents := []string{"page", "action", "fragment", "api", "schedule", "job", "webhook"}

	spec.Register(spec.Entity{
		Name:    "llm",
		Kind:    spec.KindKeyword,
		Summary: "Call an LLM. Choose `response` (single-shot / chat) or `agent` (subprocess loop with tools).",
		Description: "Body-level keyword. Bind the result to `<name>` for use by subsequent " +
			"nodes. Common settings (`model`, `system`, `temperature`, `max-tokens`) live " +
			"directly under `llm`; mode-specific children (`response` or `agent`) define " +
			"how the call runs. Exactly one of `response` or `agent` must be present.",
		Syntax:      "llm <name>",
		Args:        []spec.Arg{{Name: "name", Type: "identifier", Required: true}},
		ParentScope: llmParents,
		Children: []string{
			"system", "temperature", "max-tokens",
			"response", "agent",
		},
		Since: "0.2.0",
		Examples: []spec.Example{
			{
				Title: "Streaming chat (response mode)",
				Code: `action /chat method POST
  llm reply
    model: claude-sonnet-4-6
    system: You are a helpful assistant
    response
      history: SELECT papel, conteudo FROM mensagem WHERE conversa_id = :id ORDER BY criada
      stream: #chat-msgs
      stream-swap: append`,
			},
			{
				Title: "Agent loop (subprocess claude CLI)",
				Code: `job process-doc
  llm task
    model: claude-sonnet-4-6
    system: Code assistant
    agent
      cwd: /workspaces/jobs/:doc_id
      tools: read, write, bash
      max-turns: 10
      max-budget-usd: 0.50
      permission-mode: plan`,
			},
		},
		SeeAlso: []string{"response", "agent", "fetch"},
	})

	// ---- common llm child attributes ----
	//
	// NOTE: `model:` is not registered as a spec entity because the name
	// collides with the top-level `model` Keyword (table declaration), and
	// the current spec.Register panics on cross-kind duplicates. The parser
	// still accepts `model: <model-id>` inside `llm`; LSP/MCP autocomplete
	// for it is deferred until spec gains parent-scoped name resolution.
	// Same reason applies to `stream:` inside `response`.

	spec.Register(spec.Entity{
		Name: "system", Kind: spec.KindAttribute,
		Summary:     "System prompt for this `llm` call.",
		Description: "Free-form text. May contain `:param` substitutions resolved against the surrounding action's params.",
		Syntax:      "system: <text>",
		Args:        []spec.Arg{{Name: "text", Type: "string", Required: true}},
		ParentScope: []string{"llm"},
		Since:       "0.2.0",
	})

	spec.Register(spec.Entity{
		Name: "temperature", Kind: spec.KindAttribute,
		Summary:     "Sampling temperature (0.0-1.0).",
		Syntax:      "temperature: <float>",
		Args:        []spec.Arg{{Name: "value", Type: "float", Required: true}},
		ParentScope: []string{"llm"},
		Default:     "1.0",
		Since:       "0.2.0",
	})

	spec.Register(spec.Entity{
		Name: "max-tokens", Kind: spec.KindAttribute,
		Summary:     "Maximum tokens to generate in the response.",
		Syntax:      "max-tokens: <int>",
		Args:        []spec.Arg{{Name: "value", Type: "int", Required: true}},
		ParentScope: []string{"llm"},
		Default:     "1024",
		Since:       "0.2.0",
	})

	// ---- response mode ----

	spec.Register(spec.Entity{
		Name:    "response",
		Kind:    spec.KindKeyword,
		Summary: "Single-shot / chat / streaming Messages API call.",
		Description: "Block-form attribute inside `llm`. Children: `history` (SQL that " +
			"yields message rows), `stream` (CSS selector to enable streaming via " +
			"hyperstream envelopes), `stream-swap` (hyperstream swap style).",
		Syntax:      "response",
		ParentScope: []string{"llm"},
		Children:    []string{"history", "stream-swap"},
		Since:       "0.2.0",
	})

	spec.Register(spec.Entity{
		Name: "history", Kind: spec.KindAttribute,
		Summary:     "SQL query whose rows become the message history.",
		Description: "Each row must expose `papel` (`user`, `assistente`, or `sistema`) and `conteudo` columns.",
		Syntax:      "history: <SQL>",
		Args:        []spec.Arg{{Name: "sql", Type: "string", Required: true}},
		ParentScope: []string{"response"},
		Since:       "0.2.0",
	})

	// NOTE: `stream:` (CSS selector inside `response`) is not registered as
	// a spec entity because the name collides with the top-level `stream`
	// Keyword (SSE endpoint). Parser handles it; LSP/MCP autocomplete
	// deferred until spec supports parent-scoped name resolution.

	spec.Register(spec.Entity{
		Name: "stream-swap", Kind: spec.KindAttribute,
		Summary:     "Hyperstream swap style (`append`, `inner`, `outer`, ...).",
		Description: "Default `append`. Follows hyperstream's swap dispatcher whitelist.",
		Syntax:      "stream-swap: <style>",
		Args:        []spec.Arg{{Name: "style", Type: "enum", Required: true}},
		ParentScope: []string{"response"},
		Default:     "append",
		Since:       "0.2.0",
	})

	// ---- agent mode ----

	spec.Register(spec.Entity{
		Name:    "agent",
		Kind:    spec.KindKeyword,
		Summary: "Agentic loop running as a `claude` CLI subprocess.",
		Description: "Block-form attribute inside `llm`. Spawns the official `claude` CLI " +
			"(min v2.0.0) per request and streams `stream-json` envelopes back. Requires " +
			"`max-budget-usd`. The `claude` CLI must be on PATH at runtime; `kilnx run` " +
			"validates this at startup when any `agent` block is declared.",
		Syntax:      "agent",
		ParentScope: []string{"llm"},
		Children: []string{
			"cwd", "tools", "max-turns", "max-budget-usd", "permission-mode",
			"mcp", "pool", "pool-idle-ttl",
		},
		Since: "0.2.0",
	})

	spec.Register(spec.Entity{
		Name: "cwd", Kind: spec.KindAttribute,
		Summary:     "Working directory for the agent subprocess.",
		Description: "Resolved relative to `config workspace-root`. Must stay within `workspace-root` after symlink resolution. `:param` substitution is supported (e.g. `/workspaces/:tenant_id/:user_id`). If omitted, a temporary directory is created per request and removed on exit.",
		Syntax:      "cwd: <path>",
		Args:        []spec.Arg{{Name: "path", Type: "path", Required: true}},
		ParentScope: []string{"agent"},
		Since:       "0.2.0",
	})

	spec.Register(spec.Entity{
		Name: "tools", Kind: spec.KindAttribute,
		Summary:     "Comma-separated list of tools the agent may use.",
		Description: "Mapped to the `claude` CLI `--allowedTools` flag. Common values: `read`, `write`, `bash`, `web_search`, `glob`, `grep`. An empty list disables all tools (planning-only).",
		Syntax:      "tools: <name>, <name>, ...",
		Args:        []spec.Arg{{Name: "names", Type: "list", Required: false, Variadic: true}},
		ParentScope: []string{"agent"},
		Since:       "0.2.0",
	})

	spec.Register(spec.Entity{
		Name: "max-turns", Kind: spec.KindAttribute,
		Summary:     "Maximum agentic turns before forced stop.",
		Syntax:      "max-turns: <int>",
		Args:        []spec.Arg{{Name: "value", Type: "int", Required: true}},
		ParentScope: []string{"agent"},
		Default:     "10",
		Since:       "0.2.0",
	})

	spec.Register(spec.Entity{
		Name: "max-budget-usd", Kind: spec.KindAttribute,
		Summary:     "Hard cost cap in USD per agent invocation.",
		Description: "Required by the analyzer. The subprocess is killed once total token cost crosses this threshold.",
		Syntax:      "max-budget-usd: <float>",
		Args:        []spec.Arg{{Name: "value", Type: "float", Required: true}},
		ParentScope: []string{"agent"},
		Required:    true,
		Since:       "0.2.0",
	})

	spec.Register(spec.Entity{
		Name: "permission-mode", Kind: spec.KindAttribute,
		Summary:     "Tool-use permission policy: `plan`, `acceptEdits`, or `bypassPermissions`.",
		Description: "Default `plan` (no side effects until user confirms). `acceptEdits` auto-approves file edits inside `cwd`. `bypassPermissions` skips all prompts (dangerous; analyzer warns).",
		Syntax:      "permission-mode: <mode>",
		Args:        []spec.Arg{{Name: "mode", Type: "enum", Required: true}},
		ParentScope: []string{"agent"},
		Default:     "plan",
		Since:       "0.2.0",
	})

	spec.Register(spec.Entity{
		Name: "mcp", Kind: spec.KindAttribute,
		Summary:     "Comma-separated names of MCP servers declared in `config mcp <name>`.",
		Description: "Each name must resolve to an MCP server block in the top-level `config`. The matching servers are mounted into the agent via `--mcp-config`.",
		Syntax:      "mcp: <name>, <name>, ...",
		Args:        []spec.Arg{{Name: "names", Type: "list", Required: true, Variadic: true}},
		ParentScope: []string{"agent"},
		Since:       "0.2.0",
	})

	spec.Register(spec.Entity{
		Name: "pool", Kind: spec.KindAttribute,
		Summary:     "Reserved: number of warm `claude` subprocesses to keep alive (not yet implemented).",
		Description: "Reserved sintaxe in v0.2.0. Analyzer emits a `not yet implemented` warning; runtime ignores the value and always spawns per request.",
		Syntax:      "pool: <int>",
		Args:        []spec.Arg{{Name: "value", Type: "int", Required: false}},
		ParentScope: []string{"agent"},
		Since:       "0.2.0",
	})

	spec.Register(spec.Entity{
		Name: "pool-idle-ttl", Kind: spec.KindAttribute,
		Summary:     "Reserved: idle TTL before pooled subprocesses are killed (not yet implemented).",
		Syntax:      "pool-idle-ttl: <duration>",
		Args:        []spec.Arg{{Name: "duration", Type: "string", Required: false}},
		ParentScope: []string{"agent"},
		Since:       "0.2.0",
	})
}
