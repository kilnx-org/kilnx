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

func TestCLI_Init_Blog(t *testing.T) {
	bin := buildKilnx(t)
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "myblog")

	cmd := exec.Command(bin, "init", "blog", projDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kilnx init failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Created blog template") {
		t.Errorf("expected success message, got: %s", out)
	}

	appFile := filepath.Join(projDir, "app.kilnx")
	if _, err := os.Stat(appFile); err != nil {
		t.Fatalf("app.kilnx not created: %v", err)
	}

	content, err := os.ReadFile(appFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "myblog") {
		t.Errorf("expected project name substitution, got: %s", content)
	}
}

func TestCLI_Init_Api(t *testing.T) {
	bin := buildKilnx(t)
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "myapi")

	cmd := exec.Command(bin, "init", "api", projDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kilnx init failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Created api template") {
		t.Errorf("expected success message, got: %s", out)
	}

	appFile := filepath.Join(projDir, "app.kilnx")
	if _, err := os.Stat(appFile); err != nil {
		t.Fatalf("app.kilnx not created: %v", err)
	}
}

func TestCLI_Init_UnknownTemplate(t *testing.T) {
	bin := buildKilnx(t)
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "x")

	cmd := exec.Command(bin, "init", "unknown", projDir)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for unknown template")
	}
	if !strings.Contains(string(out), "unknown template") {
		t.Errorf("expected unknown template error, got: %s", out)
	}
}

func TestCLI_Init_Duplicate(t *testing.T) {
	bin := buildKilnx(t)
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "dup")

	// First init succeeds
	cmd1 := exec.Command(bin, "init", "blog", projDir)
	if out, err := cmd1.CombinedOutput(); err != nil {
		t.Fatalf("first init failed: %v\n%s", err, out)
	}

	// Second init fails because app.kilnx exists
	cmd2 := exec.Command(bin, "init", "api", projDir)
	out, err := cmd2.CombinedOutput()
	if err == nil {
		t.Fatal("expected error when target exists")
	}
	if !strings.Contains(string(out), "already exists") {
		t.Errorf("expected already exists error, got: %s", out)
	}
}
