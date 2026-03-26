package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
)

// Version is set at build time via ldflags, falls back to debug.ReadBuildInfo.
var Version = ""

func kilnxModuleVersion() string {
	if Version != "" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return "latest"
}

// Build compiles a .kilnx file into a standalone binary.
// It creates a temporary Go module that imports kilnx packages,
// so it works without the kilnx source tree being present.
func Build(kilnxFile, outputPath string) error {
	source, err := os.ReadFile(kilnxFile)
	if err != nil {
		return fmt.Errorf("reading %s: %w", kilnxFile, err)
	}

	if outputPath == "" {
		base := filepath.Base(kilnxFile)
		outputPath = strings.TrimSuffix(base, filepath.Ext(base))
	}

	absOutput, _ := filepath.Abs(outputPath)

	// Create temporary build directory
	tmpDir, err := os.MkdirTemp("", "kilnx-build-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write main.go
	mainGo := generateMainGo(string(source))
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		return fmt.Errorf("writing main.go: %w", err)
	}

	// Write go.mod
	modVersion := kilnxModuleVersion()
	goMod := fmt.Sprintf(`module kilnx-app

go 1.25.0

require github.com/kilnx-org/kilnx %s
`, modVersion)
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return fmt.Errorf("writing go.mod: %w", err)
	}

	// If running from within the kilnx source tree, use replace directive
	// so local changes are picked up and no network fetch is needed
	if kilnxRoot := findKilnxRoot(); kilnxRoot != "" {
		replaceDirective := fmt.Sprintf("\nreplace github.com/kilnx-org/kilnx => %s\n", kilnxRoot)
		f, err := os.OpenFile(filepath.Join(tmpDir, "go.mod"), os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("appending to go.mod: %w", err)
		}
		f.WriteString(replaceDirective)
		f.Close()
	}

	// Run go mod tidy to resolve dependencies
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = tmpDir
	tidy.Stderr = os.Stderr
	if err := tidy.Run(); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}

	fmt.Printf("Building %s...\n", filepath.Base(absOutput))

	cmd := exec.Command("go", "build", "-o", absOutput, ".")
	cmd.Dir = tmpDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}

	info, err := os.Stat(absOutput)
	if err != nil {
		return err
	}

	fmt.Printf("Built: %s (%.1f MB)\n", absOutput, float64(info.Size())/1024/1024)
	return nil
}

func generateMainGo(source string) string {
	// Escape backticks
	escaped := strings.ReplaceAll(source, "`", "` + \"`\" + `")

	return `package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/kilnx-org/kilnx/internal/analyzer"
	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/lexer"
	"github.com/kilnx-org/kilnx/internal/optimizer"
	"github.com/kilnx-org/kilnx/internal/parser"
	"github.com/kilnx-org/kilnx/internal/runtime"
)

const embeddedSource = ` + "`" + escaped + "`" + `

func main() {
	tokens := lexer.Tokenize(embeddedSource)
	app, err := parser.Parse(tokens, embeddedSource)
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
		os.Exit(1)
	}

	if diags := analyzer.Analyze(app); len(diags) > 0 {
		hasErrors := false
		for _, d := range diags {
			prefix := "warning"
			if d.Level == "error" {
				prefix = "error"
				hasErrors = true
			}
			fmt.Fprintf(os.Stderr, "kilnx %s: [%s] %s\n", prefix, d.Context, d.Message)
		}
		if hasErrors {
			fmt.Fprintln(os.Stderr, "Static analysis found errors, aborting.")
			os.Exit(1)
		}
	}

	optimizer.Optimize(app)

	port := 8080
	dbPath := "app.db"
	if app.Config != nil {
		if app.Config.Port > 0 {
			port = app.Config.Port
		}
		if app.Config.Database != "" {
			dbPath = strings.TrimPrefix(app.Config.Database, "sqlite://")
		}
	}

	// PaaS platforms (Railway, Fly.io, Render, Cloud Run) set PORT env var
	if envPort := os.Getenv("PORT"); envPort != "" {
		fmt.Sscanf(envPort, "%d", &port)
	}

	for i, arg := range os.Args {
		if arg == "--port" && i+1 < len(os.Args) {
			fmt.Sscanf(os.Args[i+1], "%d", &port)
		}
		if arg == "--db" && i+1 < len(os.Args) {
			dbPath = os.Args[i+1]
		}
	}

	db, err := database.Open(dbPath)
	if err != nil {
		fmt.Printf("Database error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if len(app.Models) > 0 {
		stmts, _ := db.Migrate(app.Models)
		if len(stmts) > 0 {
			fmt.Printf("Migrated %d change(s)\n", len(stmts))
		}
	}

	srv := runtime.NewServer(app, db, port)
	srv.StartScheduler()
	srv.StartJobQueue()

	fmt.Printf("Serving on http://localhost:%d\n", port)
	if err := srv.Start(); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
`
}

func findKilnxRoot() string {
	// Start from executable location
	ex, _ := os.Executable()
	dir := filepath.Dir(ex)

	// Also try CWD
	cwd, _ := os.Getwd()

	for _, startDir := range []string{dir, cwd} {
		d := startDir
		for {
			gomod := filepath.Join(d, "go.mod")
			if data, err := os.ReadFile(gomod); err == nil {
				if strings.Contains(string(data), "kilnx-org/kilnx") {
					return d
				}
			}
			parent := filepath.Dir(d)
			if parent == d {
				break
			}
			d = parent
		}
	}

	return ""
}
