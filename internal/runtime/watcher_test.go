package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTemp creates a temp file with content and returns its absolute path.
func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", p, err)
	}
	return p
}

func TestResolveImports_Basic(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "models.kilnx", "model user\n  name: text\n")
	entry := writeTemp(t, dir, "app.kilnx", `import "models.kilnx"
page /
  "hi"
`)
	out, err := resolveImports(entry, dir, nil, 0)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !strings.Contains(out, "model user") {
		t.Errorf("expected imported content, got:\n%s", out)
	}
	if strings.Contains(out, `import "models.kilnx"`) {
		t.Errorf("import line should be replaced, got:\n%s", out)
	}
}

// TestResolveImports_IndentedIgnored verifies that 'import ...' is only
// treated as a directive when at column 0. Indented text containing
// 'import ...' must pass through as content (html blocks, i18n strings).
func TestResolveImports_IndentedIgnored(t *testing.T) {
	dir := t.TempDir()
	entry := writeTemp(t, dir, "app.kilnx", `page /
  html
    import "not-an-import.txt"
    <p>How to import files?</p>
`)
	out, err := resolveImports(entry, dir, nil, 0)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !strings.Contains(out, `import "not-an-import.txt"`) {
		t.Errorf("indented import should be preserved, got:\n%s", out)
	}
}

func TestResolveImports_Circular(t *testing.T) {
	dir := t.TempDir()
	a := writeTemp(t, dir, "a.kilnx", `import "b.kilnx"
page /a
  "a"
`)
	writeTemp(t, dir, "b.kilnx", `import "a.kilnx"
page /b
  "b"
`)
	_, err := resolveImports(a, dir, nil, 0)
	if err == nil || !strings.Contains(err.Error(), "circular import") {
		t.Fatalf("expected circular import error, got: %v", err)
	}
}

func TestResolveImports_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	entry := writeTemp(t, dir, "app.kilnx", `import "missing.kilnx"
page /
  "hi"
`)
	_, err := resolveImports(entry, dir, nil, 0)
	if err == nil || !strings.Contains(err.Error(), "reading") {
		t.Fatalf("expected file read error, got: %v", err)
	}
}

func TestResolveImports_InvalidExtension(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "bad.txt", "page /\n  \"hi\"\n")
	entry := writeTemp(t, dir, "app.kilnx", `import "bad.txt"
page /
  "hi"
`)
	_, err := resolveImports(entry, dir, nil, 0)
	if err == nil || !strings.Contains(err.Error(), "must be a .kilnx file") {
		t.Fatalf("expected extension error, got: %v", err)
	}
}

func TestResolveImports_EscapesProject(t *testing.T) {
	dir := t.TempDir()
	// Create a file outside the project root
	outsideDir := t.TempDir()
	writeTemp(t, outsideDir, "evil.kilnx", "page /evil\n  \"bad\"\n")

	entry := writeTemp(t, dir, "app.kilnx", fmt.Sprintf(`import "%s"
page /
  "hi"
`, filepath.Join("..", filepath.Base(outsideDir), "evil.kilnx")))
	_, err := resolveImports(entry, dir, nil, 0)
	if err == nil || !strings.Contains(err.Error(), "escapes project directory") {
		t.Fatalf("expected escape error, got: %v", err)
	}
}

func TestResolveImports_MaxDepth(t *testing.T) {
	dir := t.TempDir()
	// Create a chain of imports deeper than maxImportDepth (64)
	var files []string
	for i := 0; i < 66; i++ {
		name := fmt.Sprintf("level%d.kilnx", i)
		content := fmt.Sprintf(`import "level%d.kilnx"`, i+1)
		if i == 65 {
			content = "page /\n  \"done\"\n"
		}
		writeTemp(t, dir, name, content+"\n")
		files = append(files, filepath.Join(dir, name))
	}
	_, err := resolveImports(files[0], dir, nil, 0)
	if err == nil || !strings.Contains(err.Error(), "depth exceeds") {
		t.Fatalf("expected max depth error, got: %v", err)
	}
}

func TestResolveImports_EmptyImport(t *testing.T) {
	dir := t.TempDir()
	entry := writeTemp(t, dir, "app.kilnx", "import \npage /\n  \"hi\"\n")
	out, err := resolveImports(entry, dir, nil, 0)
	if err != nil {
		t.Fatalf("expected empty import to be skipped, got error: %v", err)
	}
	if !strings.Contains(out, "page /") {
		t.Errorf("expected page to remain, got:\n%s", out)
	}
}

// ---------- loadApp tests ----------

func TestLoadApp_Success(t *testing.T) {
	dir := t.TempDir()
	entry := writeTemp(t, dir, "app.kilnx", `page /
  "hello"
`)
	app, err := loadApp(entry)
	if err != nil {
		t.Fatalf("loadApp: %v", err)
	}
	if len(app.Pages) != 1 || app.Pages[0].Path != "/" {
		t.Errorf("expected one page with path /, got %+v", app.Pages)
	}
}

func TestLoadApp_FileNotFound(t *testing.T) {
	_, err := loadApp("/nonexistent/path/app.kilnx")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadApp_ParseError(t *testing.T) {
	dir := t.TempDir()
	entry := writeTemp(t, dir, "app.kilnx", "model user\n  index(\n")
	_, err := loadApp(entry)
	if err == nil || !strings.Contains(err.Error(), "parsing") {
		t.Fatalf("expected parsing error, got: %v", err)
	}
}

func TestLoadApp_ResolveImportError(t *testing.T) {
	dir := t.TempDir()
	entry := writeTemp(t, dir, "app.kilnx", `import "missing.kilnx"
page /
  "hi"
`)
	_, err := loadApp(entry)
	if err == nil || !strings.Contains(err.Error(), "reading") {
		t.Fatalf("expected import read error, got: %v", err)
	}
}
