package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateMainGo(t *testing.T) {
	src := `model user
  name: text required
`
	out := generateMainGo(src)

	if !strings.Contains(out, "package main") {
		t.Error("generated main.go missing package declaration")
	}
	if !strings.Contains(out, "embeddedSource") {
		t.Error("generated main.go missing embeddedSource constant")
	}
	if !strings.Contains(out, "lexer.StripComments") {
		t.Error("generated main.go missing lexer call")
	}
	if !strings.Contains(out, "parser.Parse") {
		t.Error("generated main.go missing parser call")
	}
	if !strings.Contains(out, "analyzer.Analyze") {
		t.Error("generated main.go missing analyzer call")
	}
	if !strings.Contains(out, "runtime.NewServer") {
		t.Error("generated main.go missing runtime.NewServer call")
	}
	// Verify backtick escaping
	if strings.Contains(src, "`") && !strings.Contains(out, "` + \"`\" + `") {
		t.Error("generated main.go did not escape backticks")
	}
}

func TestFindKilnxRoot(t *testing.T) {
	root := findKilnxRoot()
	if root == "" {
		t.Skip("findKilnxRoot returned empty; may be running outside repo")
	}
	gomod := filepath.Join(root, "go.mod")
	data, err := os.ReadFile(gomod)
	if err != nil {
		t.Fatalf("could not read go.mod at %s: %v", gomod, err)
	}
	if !strings.Contains(string(data), "kilnx-org/kilnx") {
		t.Errorf("go.mod at %s does not contain expected module name", gomod)
	}
}

func TestBuild_FileMissing(t *testing.T) {
	if err := Build("/nonexistent/file.kilnx", ""); err == nil {
		t.Error("expected error for missing input file")
	}
}

func TestBuild_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skip slow build in short mode")
	}
	if findKilnxRoot() == "" {
		t.Skip("kilnx root not found")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "tiny.kilnx")
	body := "page /\n  \"hi\"\n"
	if err := os.WriteFile(src, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "tinybin")
	if err := Build(src, out); err != nil {
		t.Fatalf("Build: %v", err)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("output not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output binary empty")
	}
}

func TestBuild_DefaultOutputPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skip slow build in short mode")
	}
	root := findKilnxRoot()
	if root == "" {
		t.Skip("kilnx root not found")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "auto.kilnx")
	body := "page /\n  \"hi\"\n"
	if err := os.WriteFile(src, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	// Stay inside the kilnx tree so findKilnxRoot still resolves, but use a
	// distinct subdir we own so we can clean up the default-named binary.
	workDir := filepath.Join(root, ".claude", "build-default-test")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)
	os.Chdir(workDir)

	if err := Build(src, ""); err != nil {
		t.Fatalf("Build with empty output: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "auto")); err != nil {
		t.Errorf("default output 'auto' not created: %v", err)
	}
}
