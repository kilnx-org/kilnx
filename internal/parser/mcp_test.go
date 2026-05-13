package parser

import "testing"

func TestParseMCPServer_Stdio(t *testing.T) {
	src := `mcp filesystem
  command: npx
  args: -y, @modelcontextprotocol/server-filesystem, /tmp/agents
  env: NODE_ENV=production, FOO=bar
`
	app := parse(t, src)
	if len(app.MCPServers) != 1 {
		t.Fatalf("expected 1 mcp server, got %d", len(app.MCPServers))
	}
	s := app.MCPServers[0]
	if s.Name != "filesystem" {
		t.Errorf("name = %q", s.Name)
	}
	if s.Command != "npx" {
		t.Errorf("command = %q", s.Command)
	}
	if len(s.Args) != 3 || s.Args[0] != "-y" || s.Args[2] != "/tmp/agents" {
		t.Errorf("args = %v", s.Args)
	}
	if s.Env["NODE_ENV"] != "production" || s.Env["FOO"] != "bar" {
		t.Errorf("env = %v", s.Env)
	}
	if s.Transport != "stdio" {
		t.Errorf("transport default should be stdio, got %q", s.Transport)
	}
}

func TestParseMCPServer_HTTP(t *testing.T) {
	src := `mcp remote
  url: https://mcp.example.com/sse
  transport: sse
`
	app := parse(t, src)
	if len(app.MCPServers) != 1 {
		t.Fatalf("expected 1 mcp server, got %d", len(app.MCPServers))
	}
	s := app.MCPServers[0]
	if s.URL != "https://mcp.example.com/sse" {
		t.Errorf("url = %q", s.URL)
	}
	if s.Transport != "sse" {
		t.Errorf("transport = %q", s.Transport)
	}
}

func TestParseConfig_WorkspaceRoot(t *testing.T) {
	src := `config
  workspace-root: /tmp/kilnx-agents
`
	app := parse(t, src)
	if app.Config == nil {
		t.Fatal("config nil")
	}
	if app.Config.WorkspaceRoot != "/tmp/kilnx-agents" {
		t.Errorf("workspace-root = %q", app.Config.WorkspaceRoot)
	}
}
