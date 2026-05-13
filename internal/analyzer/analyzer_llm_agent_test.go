package analyzer

import (
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func diagMatch(diags []Diagnostic, level, substr string) bool {
	for _, d := range diags {
		if d.Level == level && strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}

func TestCheckLLMAgentRequired_MissingBudget(t *testing.T) {
	app := &parser.App{
		Config: &parser.AppConfig{WorkspaceRoot: "/tmp"},
		Actions: []parser.Page{{
			Path: "/x",
			Body: []parser.Node{{Type: parser.NodeLLM, LLMMode: "agent"}},
		}},
	}
	d := checkLLMAgentRequired(app)
	if !diagMatch(d, "error", "max-budget-usd") {
		t.Errorf("expected required error, got %#v", d)
	}
}

func TestCheckLLMAgentWorkspaceRoot_Missing(t *testing.T) {
	app := &parser.App{
		Actions: []parser.Page{{
			Path: "/x",
			Body: []parser.Node{{Type: parser.NodeLLM, LLMMode: "agent", LLMAgentBudget: 0.1}},
		}},
	}
	d := checkLLMAgentWorkspaceRoot(app)
	if !diagMatch(d, "error", "workspace-root") {
		t.Errorf("expected workspace-root error, got %#v", d)
	}
}

func TestCheckLLMAgentWorkspaceRoot_SilentWhenNoAgent(t *testing.T) {
	app := &parser.App{
		Actions: []parser.Page{{
			Path: "/x",
			Body: []parser.Node{{Type: parser.NodeLLM, LLMMode: "response"}},
		}},
	}
	if d := checkLLMAgentWorkspaceRoot(app); len(d) != 0 {
		t.Errorf("expected no diagnostics, got %#v", d)
	}
}

func TestCheckLLMAgentReservedAttrs(t *testing.T) {
	app := &parser.App{
		Config: &parser.AppConfig{WorkspaceRoot: "/tmp"},
		Actions: []parser.Page{{
			Path: "/x",
			Body: []parser.Node{{
				Type:                   parser.NodeLLM,
				LLMMode:                "agent",
				LLMAgentBudget:         0.1,
				LLMAgentPool:           5,
				LLMAgentPoolIdleTTL:    "10s",
				LLMAgentPermissionMode: "bypassPermissions",
			}},
		}},
	}
	d := checkLLMAgentReservedAttrs(app)
	if !diagMatch(d, "warning", "pool is reserved") {
		t.Errorf("missing pool warning: %#v", d)
	}
	if !diagMatch(d, "warning", "pool-idle-ttl") {
		t.Errorf("missing pool-idle-ttl warning: %#v", d)
	}
	if !diagMatch(d, "warning", "bypassPermissions") {
		t.Errorf("missing bypass warning: %#v", d)
	}
}

func TestCheckLLMAgentMCPRefs_Unknown(t *testing.T) {
	app := &parser.App{
		Config: &parser.AppConfig{WorkspaceRoot: "/tmp"},
		Actions: []parser.Page{{
			Path: "/x",
			Body: []parser.Node{{
				Type:           parser.NodeLLM,
				LLMMode:        "agent",
				LLMAgentBudget: 0.1,
				LLMAgentMCP:    []string{"ghost"},
			}},
		}},
	}
	d := checkLLMAgentMCPRefs(app)
	if !diagMatch(d, "error", `"ghost"`) {
		t.Errorf("expected unknown mcp error, got %#v", d)
	}
}

func TestCheckLLMAgentMCPRefs_Resolved(t *testing.T) {
	app := &parser.App{
		Config:     &parser.AppConfig{WorkspaceRoot: "/tmp"},
		MCPServers: []parser.MCPServer{{Name: "filesystem", Command: "npx"}},
		Actions: []parser.Page{{
			Path: "/x",
			Body: []parser.Node{{
				Type:           parser.NodeLLM,
				LLMMode:        "agent",
				LLMAgentBudget: 0.1,
				LLMAgentMCP:    []string{"filesystem"},
			}},
		}},
	}
	if d := checkLLMAgentMCPRefs(app); len(d) != 0 {
		t.Errorf("expected no diagnostics for resolved name, got %#v", d)
	}
}

func TestHasLLMAgentBlock(t *testing.T) {
	app := &parser.App{
		Actions: []parser.Page{{
			Body: []parser.Node{{Type: parser.NodeLLM, LLMMode: "agent"}},
		}},
	}
	if !HasLLMAgentBlock(app) {
		t.Error("expected true")
	}
	app2 := &parser.App{
		Actions: []parser.Page{{
			Body: []parser.Node{{Type: parser.NodeLLM, LLMMode: "response"}},
		}},
	}
	if HasLLMAgentBlock(app2) {
		t.Error("expected false")
	}
}
