package runtime

import (
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
		t.Fatalf("expected indented 'import' to be content, got error: %v", err)
	}
	if !strings.Contains(out, "not-an-import.txt") {
		t.Errorf("indented import line should be preserved as content")
	}
	if !strings.Contains(out, "How to import files?") {
		t.Errorf("html content lost: %s", out)
	}
}

func TestResolveImports_Circular(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "a.kilnx", `import "b.kilnx"`+"\n")
	writeTemp(t, dir, "b.kilnx", `import "a.kilnx"`+"\n")
	entry := filepath.Join(dir, "a.kilnx")
	if _, err := resolveImports(entry, dir, nil, 0); err == nil {
		t.Fatal("expected circular import error")
	}
}

func TestResolveImports_RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Dir(root)
	writeTemp(t, parent, "outside.kilnx", "model leak\n")
	entry := writeTemp(t, root, "app.kilnx", `import "../outside.kilnx"`+"\n")
	_, err := resolveImports(entry, root, nil, 0)
	if err == nil || !strings.Contains(err.Error(), "escapes project") {
		t.Fatalf("expected path-traversal rejection, got: %v", err)
	}
}

func TestResolveImports_RequiresKilnxExtension(t *testing.T) {
	dir := t.TempDir()
	entry := writeTemp(t, dir, "app.kilnx", `import "data.txt"`+"\n")
	_, err := resolveImports(entry, dir, nil, 0)
	if err == nil || !strings.Contains(err.Error(), ".kilnx file") {
		t.Fatalf("expected extension rejection, got: %v", err)
	}
}
