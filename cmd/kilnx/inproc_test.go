package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/analyzer"
)

// captureStdout swaps os.Stdout/os.Stderr for a pipe, runs fn, returns combined output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()
	fn()
	w.Close()
	os.Stdout = origOut
	os.Stderr = origErr
	<-done
	return buf.String()
}

func TestDbPathFor(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"app.kilnx", "app.db"},
		{"./app.kilnx", "app.db"},
		{"/tmp/foo/bar.kilnx", "/tmp/foo/bar.db"},
		{"name", "name.db"},
	}
	for _, tc := range cases {
		got := dbPathFor(tc.in)
		if got != tc.want {
			t.Errorf("dbPathFor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPrintUsage(t *testing.T) {
	out := captureStdout(t, func() {
		printUsage()
	})
	for _, want := range []string{"Usage: kilnx", "init", "run", "check", "build", "migrate", "test", "lsp", "mcp", "version"} {
		if !strings.Contains(out, want) {
			t.Errorf("printUsage missing %q. Got: %s", want, out)
		}
	}
}

func TestPrintDiagnostics(t *testing.T) {
	diags := []analyzer.Diagnostic{
		{Level: "warning", Context: "ctx1", Message: "soft hint", Line: 5},
		{Level: "error", Context: "ctx2", Message: "hard fail", Line: 0},
	}
	out := captureStdout(t, func() {
		hasErr := printDiagnostics(diags)
		if !hasErr {
			t.Error("expected hasErrors true")
		}
	})
	if !strings.Contains(out, "warning") || !strings.Contains(out, "error") {
		t.Errorf("expected both warning and error tags, got: %s", out)
	}
	if !strings.Contains(out, "soft hint") || !strings.Contains(out, "hard fail") {
		t.Errorf("expected messages, got: %s", out)
	}
	if !strings.Contains(out, "line 5") {
		t.Errorf("expected line 5, got: %s", out)
	}
}

func TestPrintDiagnostics_NoErrors(t *testing.T) {
	diags := []analyzer.Diagnostic{
		{Level: "warning", Context: "ctx", Message: "hint"},
	}
	out := captureStdout(t, func() {
		if printDiagnostics(diags) {
			t.Error("expected hasErrors false for warnings only")
		}
	})
	if !strings.Contains(out, "warning") {
		t.Errorf("expected warning, got: %s", out)
	}
}

func TestPrintMigrationStmt(t *testing.T) {
	cases := []struct {
		stmt string
		want string
	}{
		{`CREATE TABLE "users" (...)`, "Created table"},
		{`ALTER TABLE "users" ADD COLUMN "email" TEXT`, "Added column"},
		{`CREATE INDEX foo ON bar(x)`, "CREATE INDEX"},
	}
	for _, tc := range cases {
		out := captureStdout(t, func() {
			printMigrationStmt(tc.stmt)
		})
		if !strings.Contains(out, tc.want) {
			t.Errorf("printMigrationStmt(%q) missing %q, got %q", tc.stmt, tc.want, out)
		}
	}
}

func TestCmdInit_Blog_InProcess(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "myapp")
	if err := cmdInit("blog", dir); err != nil {
		t.Fatalf("cmdInit blog: %v", err)
	}
	app := filepath.Join(dir, "app.kilnx")
	data, err := os.ReadFile(app)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "myapp") {
		t.Errorf("project name not substituted: %s", data)
	}
}

func TestCmdInit_Api_InProcess(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "myapi")
	if err := cmdInit("api", dir); err != nil {
		t.Fatalf("cmdInit api: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "app.kilnx")); err != nil {
		t.Fatalf("app.kilnx missing: %v", err)
	}
}

func TestCmdInit_Unknown(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "x")
	err := cmdInit("nope", dir)
	if err == nil || !strings.Contains(err.Error(), "unknown template") {
		t.Errorf("expected unknown template error, got: %v", err)
	}
}

func TestCmdInit_Duplicate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dup")
	if err := cmdInit("blog", dir); err != nil {
		t.Fatalf("first init: %v", err)
	}
	err := cmdInit("api", dir)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected already exists, got: %v", err)
	}
}

func TestCmdCheck_OK(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "app.kilnx")
	if err := os.WriteFile(app, []byte("page /\n  \"Hello\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if err := cmdCheck(app, ""); err != nil {
			t.Errorf("cmdCheck: %v", err)
		}
	})
	if !strings.Contains(out, "No issues found") {
		t.Errorf("expected clean output, got: %s", out)
	}
}

func TestCmdCheck_FileMissing(t *testing.T) {
	if err := cmdCheck("/nonexistent/x.kilnx", ""); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadApp_Imports(t *testing.T) {
	dir := t.TempDir()
	frag := filepath.Join(dir, "models.kilnx")
	if err := os.WriteFile(frag, []byte("model post\n  title: text\n"), 0644); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(dir, "app.kilnx")
	body := `import "models.kilnx"
page /
  "ok"
`
	if err := os.WriteFile(main, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	app, err := loadApp(main)
	if err != nil {
		t.Fatalf("loadApp: %v", err)
	}
	if len(app.Models) != 1 || app.Models[0].Name != "post" {
		t.Errorf("expected post model imported, got: %+v", app.Models)
	}
}

func TestLoadApp_BadImportExtension(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "app.kilnx")
	body := `import "evil.txt"
page /
`
	os.WriteFile(main, []byte(body), 0644)
	if _, err := loadApp(main); err == nil {
		t.Error("expected error on non-.kilnx import")
	}
}

func TestLoadApp_ImportEscapesProjectRoot(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "app.kilnx")
	body := `import "../escape.kilnx"
page /
`
	os.WriteFile(main, []byte(body), 0644)
	if _, err := loadApp(main); err == nil {
		t.Error("expected error on path traversal")
	}
}

func TestResolveImports_Circular(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.kilnx")
	b := filepath.Join(dir, "b.kilnx")
	os.WriteFile(a, []byte(`import "b.kilnx"`), 0644)
	os.WriteFile(b, []byte(`import "a.kilnx"`), 0644)
	if _, err := resolveImports(a, dir, nil, 0); err == nil {
		t.Error("expected circular import error")
	}
}

func TestResolveImports_DepthLimit(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "app.kilnx")
	os.WriteFile(main, []byte(""), 0644)
	if _, err := resolveImports(main, dir, nil, maxImportDepth+1); err == nil {
		t.Error("expected depth limit error")
	}
}

func TestLoadManifest_BadExtension(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadManifest(dir, "evil.txt", "post"); err == nil {
		t.Error("expected extension error")
	}
}

func TestLoadManifest_Escape(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadManifest(dir, "../escape.kilnx", "post"); err == nil {
		t.Error("expected escape error")
	}
}

func TestLoadManifest_Missing(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadManifest(dir, "missing.kilnx", "post"); err == nil {
		t.Error("expected missing manifest error")
	}
}

func writeKilnxApp(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	app := filepath.Join(dir, "app.kilnx")
	if err := os.WriteFile(app, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestCmdMigrate_NoModels(t *testing.T) {
	app := writeKilnxApp(t, "page /\n  \"hi\"\n")
	out := captureStdout(t, func() {
		if err := cmdMigrate(app, nil); err != nil {
			t.Errorf("cmdMigrate: %v", err)
		}
	})
	if !strings.Contains(out, "No models") {
		t.Errorf("expected 'No models', got: %s", out)
	}
}

func TestCmdMigrate_DryRun(t *testing.T) {
	app := writeKilnxApp(t, "model post\n  title: text\n")
	out := captureStdout(t, func() {
		if err := cmdMigrate(app, []string{"--dry-run"}); err != nil {
			t.Errorf("cmdMigrate dry-run: %v", err)
		}
	})
	if !strings.Contains(out, "Would apply") && !strings.Contains(out, "up to date") {
		t.Errorf("expected dry-run output, got: %s", out)
	}
}

func TestCmdMigrate_Apply(t *testing.T) {
	app := writeKilnxApp(t, "model post\n  title: text\n")
	out := captureStdout(t, func() {
		if err := cmdMigrate(app, nil); err != nil {
			t.Errorf("cmdMigrate apply: %v", err)
		}
	})
	if !strings.Contains(out, "Applied") {
		t.Errorf("expected Applied output, got: %s", out)
	}
	// Run again, should be up to date
	out2 := captureStdout(t, func() {
		if err := cmdMigrate(app, nil); err != nil {
			t.Errorf("cmdMigrate second: %v", err)
		}
	})
	if !strings.Contains(out2, "up to date") {
		t.Errorf("expected up to date, got: %s", out2)
	}
}

func TestCmdMigrate_Status(t *testing.T) {
	app := writeKilnxApp(t, "model post\n  title: text\n")
	if err := cmdMigrate(app, nil); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if err := cmdMigrate(app, []string{"--status"}); err != nil {
			t.Errorf("cmdMigrate status: %v", err)
		}
	})
	if !strings.Contains(out, "Migration history") && !strings.Contains(out, "up to date") {
		t.Errorf("expected status output, got: %s", out)
	}
}

func TestCmdMigrate_FileMissing(t *testing.T) {
	if err := cmdMigrate("/nonexistent/x.kilnx", nil); err == nil {
		t.Error("expected error for missing file")
	}
}
