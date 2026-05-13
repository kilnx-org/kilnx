package analyzer

import (
	"fmt"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// checkLLMAgentRequired walks every LLM node in agent mode and flags
// blocks that miss `max-budget-usd`. The spec registry marks this attr
// `Required: true` but the registry does not enforce it; analysis does.
func checkLLMAgentRequired(app *parser.App) []Diagnostic {
	var diags []Diagnostic
	walkLLMNodes(app, func(node parser.Node, where string) {
		if node.LLMMode != "agent" {
			return
		}
		if node.LLMAgentBudget <= 0 {
			diags = append(diags, Diagnostic{
				Level:   "error",
				Message: "llm agent requires max-budget-usd (declared as Required in spec)",
				Context: where,
			})
		}
	})
	return diags
}

// checkLLMAgentReservedAttrs warns about reserved attributes that the
// runtime currently ignores, and about the dangerous bypassPermissions
// mode.
func checkLLMAgentReservedAttrs(app *parser.App) []Diagnostic {
	var diags []Diagnostic
	walkLLMNodes(app, func(node parser.Node, where string) {
		if node.LLMMode != "agent" {
			return
		}
		if node.LLMAgentPool > 0 {
			diags = append(diags, Diagnostic{
				Level:   "warning",
				Message: "llm agent pool is reserved syntax; runtime always spawns per request in v0.2.x",
				Context: where,
			})
		}
		if node.LLMAgentPoolIdleTTL != "" {
			diags = append(diags, Diagnostic{
				Level:   "warning",
				Message: "llm agent pool-idle-ttl is reserved syntax; runtime ignores it in v0.2.x",
				Context: where,
			})
		}
		if node.LLMAgentPermissionMode == "bypassPermissions" {
			diags = append(diags, Diagnostic{
				Level:   "warning",
				Message: "llm agent permission-mode: bypassPermissions skips ALL safety prompts; review before shipping",
				Context: where,
			})
		}
	})
	return diags
}

// checkLLMAgentWorkspaceRoot ensures `config workspace-root` is set when
// any agent block declares an explicit cwd, OR when any agent block is
// declared at all (tmp dirs need a root to live under).
func checkLLMAgentWorkspaceRoot(app *parser.App) []Diagnostic {
	hasAgent := false
	walkLLMNodes(app, func(node parser.Node, where string) {
		if node.LLMMode == "agent" {
			hasAgent = true
		}
	})
	if !hasAgent {
		return nil
	}
	if app.Config != nil && app.Config.WorkspaceRoot != "" {
		return nil
	}
	return []Diagnostic{{
		Level:   "error",
		Message: "llm agent requires `config workspace-root` to be set",
		Context: "config",
	}}
}

// checkLLMAgentMCPRefs validates that every MCP name referenced by an
// agent block resolves to a top-level `mcp <name>` declaration.
func checkLLMAgentMCPRefs(app *parser.App) []Diagnostic {
	declared := make(map[string]bool, len(app.MCPServers))
	for _, s := range app.MCPServers {
		declared[s.Name] = true
	}
	var diags []Diagnostic
	walkLLMNodes(app, func(node parser.Node, where string) {
		if node.LLMMode != "agent" {
			return
		}
		for _, name := range node.LLMAgentMCP {
			if !declared[name] {
				diags = append(diags, Diagnostic{
					Level:   "error",
					Message: fmt.Sprintf("llm agent mcp references %q but no top-level `mcp %s` block is declared", name, name),
					Context: where,
				})
			}
		}
	})
	return diags
}

// walkLLMNodes invokes fn for every NodeLLM in pages, actions, fragments,
// APIs, jobs, schedules, and webhooks.
func walkLLMNodes(app *parser.App, fn func(node parser.Node, where string)) {
	visit := func(nodes []parser.Node, where string) {
		var rec func([]parser.Node)
		rec = func(ns []parser.Node) {
			for _, n := range ns {
				if n.Type == parser.NodeLLM {
					fn(n, where)
				}
				if len(n.Children) > 0 {
					rec(n.Children)
				}
			}
		}
		rec(nodes)
	}
	for _, p := range app.Pages {
		visit(p.Body, "page "+p.Path)
	}
	for _, a := range app.Actions {
		visit(a.Body, "action "+a.Path)
	}
	for _, f := range app.Fragments {
		visit(f.Body, "fragment "+f.Path)
	}
	for _, a := range app.APIs {
		visit(a.Body, "api "+a.Path)
	}
	for _, j := range app.Jobs {
		visit(j.Body, "job "+j.Name)
	}
	for _, s := range app.Schedules {
		visit(s.Body, "schedule "+s.Name)
	}
	for _, wh := range app.Webhooks {
		for _, ev := range wh.Events {
			visit(ev.Body, "webhook "+wh.Path)
		}
	}
}

// HasLLMAgentBlock reports whether the app declares any `llm ... agent`
// block. The runtime startup uses this to decide whether to require the
// `claude` CLI on PATH.
func HasLLMAgentBlock(app *parser.App) bool {
	found := false
	walkLLMNodes(app, func(node parser.Node, _ string) {
		if node.LLMMode == "agent" {
			found = true
		}
	})
	return found
}
