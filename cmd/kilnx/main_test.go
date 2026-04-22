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
	if err := os.WriteFile(kilnxFile, []byte(content), 0644); err != nil {
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
