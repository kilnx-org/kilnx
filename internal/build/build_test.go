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
