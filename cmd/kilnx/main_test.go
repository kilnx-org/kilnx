package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildKilnx(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "kilnx")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "../../cmd/kilnx"
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build kilnx: %v\n%s", err, out)
	}
	return bin
}

func TestCLI_Version(t *testing.T) {
	bin := buildKilnx(t)
	cmd := exec.Command(bin, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kilnx version failed: %v\n%s", err, out)
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		t.Error("version output is empty")
	}
}

func TestCLI_NoArgs(t *testing.T) {
	bin := buildKilnx(t)
	cmd := exec.Command(bin)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit code when called without args")
	}
	if !strings.Contains(string(out), "Usage:") {
		t.Errorf("expected usage message, got: %s", out)
	}
}

func TestCLI_Check(t *testing.T) {
	bin := buildKilnx(t)

	// Create a minimal valid .kilnx file
	tmpDir := t.TempDir()
	kilnxFile := filepath.Join(tmpDir, "app.kilnx")
	content := `page /
  html
    <h1>Hello</h1>
`
	if err := os.WriteFile(kilnxFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "check", kilnxFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kilnx check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "OK") && !strings.Contains(string(out), "warning") {
		t.Logf("check output: %s", out)
	}
}

// agentKilnxFile writes a minimal kilnx app that declares one `llm agent`
// action so HasLLMAgentBlock returns true. Used by the claude-CLI startup
// check tests below.
func agentKilnxFile(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	wsRoot := filepath.Join(tmpDir, "ws")
	if err := os.MkdirAll(wsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	kilnxFile := filepath.Join(tmpDir, "app.kilnx")
	content := `config
  workspace-root: ` + wsRoot + `

action /run method POST
  llm task
    agent
      max-budget-usd: 0.10
  redirect: /
`
	if err := os.WriteFile(kilnxFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return kilnxFile
}

// TestCLI_Check_AgentMissingClaudeWarns: with an agent block present and
// `claude` absent from PATH, `kilnx check` must emit a warning but exit 0.
func TestCLI_Check_AgentMissingClaudeWarns(t *testing.T) {
	bin := buildKilnx(t)
	kilnxFile := agentKilnxFile(t)

	cmd := exec.Command(bin, "check", kilnxFile)
	cmd.Env = append(os.Environ(), "PATH=/nonexistent-path-for-test")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kilnx check should not error on warning, got %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "claude") || !strings.Contains(strings.ToLower(string(out)), "warning") {
		t.Errorf("expected claude-cli warning, got: %s", out)
	}
}

// TestCLI_Run_AgentMissingClaudeFails: with an agent block present and
// `claude` absent from PATH, `kilnx run` must refuse to start.
func TestCLI_Run_AgentMissingClaudeFails(t *testing.T) {
	bin := buildKilnx(t)
	kilnxFile := agentKilnxFile(t)

	cmd := exec.Command(bin, "run", kilnxFile)
	cmd.Env = append(os.Environ(), "PATH=/nonexistent-path-for-test")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit when claude is missing, got out=%s", out)
	}
	if !strings.Contains(string(out), "claude") {
		t.Errorf("expected claude-missing error, got: %s", out)
	}
}
