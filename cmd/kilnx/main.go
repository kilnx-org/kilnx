package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kilnx-org/kilnx/internal/analyzer"
	"github.com/kilnx-org/kilnx/internal/build"
	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/lexer"
	"github.com/kilnx-org/kilnx/internal/lsp"
	"github.com/kilnx-org/kilnx/internal/optimizer"
	"github.com/kilnx-org/kilnx/internal/parser"
	"github.com/kilnx-org/kilnx/internal/runtime"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		if len(os.Args) < 3 {
			fmt.Println("Usage: kilnx run <file.kilnx>")
			os.Exit(1)
		}
		if err := cmdRun(os.Args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "migrate":
		if len(os.Args) < 3 {
			fmt.Println("Usage: kilnx migrate <file.kilnx>")
			os.Exit(1)
		}
		if err := cmdMigrate(os.Args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "test":
		if len(os.Args) < 3 {
			fmt.Println("Usage: kilnx test <file.kilnx>")
			os.Exit(1)
		}
		if err := cmdTest(os.Args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "build":
		if len(os.Args) < 3 {
			fmt.Println("Usage: kilnx build <file.kilnx> [-o output]")
			os.Exit(1)
		}
		output := ""
		if len(os.Args) > 4 && os.Args[3] == "-o" {
			output = os.Args[4]
		}
		if err := build.Build(os.Args[2], output); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "check":
		if len(os.Args) < 3 {
			fmt.Println("Usage: kilnx check <file.kilnx>")
			os.Exit(1)
		}
		if err := cmdCheck(os.Args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "lsp":
		lsp.Serve()
	case "version":
		fmt.Println("kilnx v0.1.0")
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdCheck(filename string) error {
	app, err := loadApp(filename)
	if err != nil {
		return err
	}

	diags := analyzer.Analyze(app)
	if len(diags) == 0 {
		fmt.Println("No issues found.")
		return nil
	}

	hasErrors := printDiagnostics(diags)
	if hasErrors {
		return fmt.Errorf("static analysis found errors")
	}
	return nil
}

func cmdRun(filename string) error {
	app, err := loadApp(filename)
	if err != nil {
		return err
	}

	if diags := analyzer.Analyze(app); len(diags) > 0 {
		if printDiagnostics(diags) {
			return fmt.Errorf("static analysis found errors, not starting server")
		}
	}

	optimizer.Optimize(app)

	// Resolve config
	port := 8080
	dbPath := dbPathFor(filename)
	if app.Config != nil {
		if app.Config.Port > 0 {
			port = app.Config.Port
		}
		if app.Config.Database != "" {
			dbPath = app.Config.Database
			// Handle sqlite:// prefix
			dbPath = strings.TrimPrefix(dbPath, "sqlite://")
		}
	}

	// PaaS platforms (Railway, Fly.io, Render, Cloud Run) set PORT env var
	if envPort := os.Getenv("PORT"); envPort != "" {
		var p int
		if n, err := fmt.Sscanf(envPort, "%d", &p); n == 1 && err == nil && p > 0 && p < 65536 {
			port = p
		} else {
			fmt.Fprintf(os.Stderr, "kilnx: invalid PORT=%q, using %d\n", envPort, port)
		}
	}

	db, err := database.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Create internal tables (sessions, jobs)
	if err := db.MigrateInternal(); err != nil {
		return err
	}

	// Auto-migrate if models exist
	if len(app.Models) > 0 {
		stmts, err := db.Migrate(app.Models)
		if err != nil {
			return err
		}
		if len(stmts) > 0 {
			fmt.Printf("Auto-migrated %d change(s) to %s\n", len(stmts), dbPath)
		}
	}

	return runtime.WatchAndServe(filename, db, port)
}

func cmdMigrate(filename string) error {
	app, err := loadApp(filename)
	if err != nil {
		return err
	}

	if len(app.Models) == 0 {
		fmt.Println("No models found.")
		return nil
	}

	dbPath := dbPathFor(filename)
	fmt.Printf("Database: %s\n", dbPath)
	fmt.Printf("Models:   %d\n\n", len(app.Models))

	db, err := database.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	stmts, err := db.Migrate(app.Models)
	if err != nil {
		return err
	}

	if len(stmts) == 0 {
		fmt.Println("Nothing to migrate. Database is up to date.")
		return nil
	}

	fmt.Printf("Applied %d migration(s):\n", len(stmts))
	for _, stmt := range stmts {
		// Print a short summary of each statement
		if strings.HasPrefix(stmt, "CREATE TABLE") {
			table := strings.Fields(stmt)[2]
			fmt.Printf("  + Created table '%s'\n", table)
		} else if strings.HasPrefix(stmt, "ALTER TABLE") {
			parts := strings.Fields(stmt)
			if len(parts) > 5 {
				fmt.Printf("  + Added column '%s' to '%s'\n", parts[5], parts[2])
			} else {
				fmt.Printf("  %s\n", stmt)
			}
		} else {
			fmt.Printf("  %s\n", stmt)
		}
	}

	return nil
}

func cmdTest(filename string) error {
	app, err := loadApp(filename)
	if err != nil {
		return err
	}

	if diags := analyzer.Analyze(app); len(diags) > 0 {
		if printDiagnostics(diags) {
			return fmt.Errorf("static analysis found errors")
		}
	}

	optimizer.Optimize(app)

	if len(app.Tests) == 0 {
		fmt.Println("No tests found.")
		return nil
	}

	// Setup: migrate DB, start server in background
	dbPath := dbPathFor(filename) + ".test"
	// Clean test DB
	os.Remove(dbPath)

	db, err := database.Open(dbPath)
	if err != nil {
		return err
	}
	defer func() {
		db.Close()
		os.Remove(dbPath)
	}()

	if len(app.Models) > 0 {
		stmts, err := db.Migrate(app.Models)
		if err != nil {
			return err
		}
		if len(stmts) > 0 {
			fmt.Printf("Test DB: %d table(s) created\n", len(stmts))
		}
	}

	// Start server on a test port
	srv := runtime.NewServer(app, db, 9999)
	go srv.Start()

	// Give server time to start
	fmt.Println()
	fmt.Printf("Running %d test(s):\n", len(app.Tests))

	// Small delay for server startup
	time.Sleep(500 * time.Millisecond)

	passed, failed := runtime.RunTests(app, db, "http://localhost:9999")

	fmt.Println()
	if failed == 0 {
		fmt.Printf("All %d test(s) passed.\n", passed)
	} else {
		fmt.Printf("%d passed, %d failed.\n", passed, failed)
		os.Exit(1)
	}

	return nil
}

func loadApp(filename string) (*parser.App, error) {
	raw, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filename, err)
	}

	source := lexer.StripComments(string(raw))
	tokens := lexer.Tokenize(source)
	return parser.Parse(tokens, source)
}

func dbPathFor(kilnxFile string) string {
	dir := filepath.Dir(kilnxFile)
	base := filepath.Base(kilnxFile)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(dir, name+".db")
}

func printDiagnostics(diags []analyzer.Diagnostic) bool {
	hasErrors := false
	for _, d := range diags {
		prefix := "warning"
		if d.Level == "error" {
			prefix = "error"
			hasErrors = true
		}
		if d.Line > 0 {
			fmt.Fprintf(os.Stderr, "kilnx %s: [%s] %s (line %d)\n", prefix, d.Context, d.Message, d.Line)
		} else {
			fmt.Fprintf(os.Stderr, "kilnx %s: [%s] %s\n", prefix, d.Context, d.Message)
		}
	}
	return hasErrors
}

func printUsage() {
	fmt.Println("Usage: kilnx <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  run <file.kilnx>        Start the server (auto-migrates)")
	fmt.Println("  check <file.kilnx>      Run static analysis")
	fmt.Println("  build <file.kilnx> [-o] Compile to standalone binary")
	fmt.Println("  migrate <file.kilnx>    Apply database migrations")
	fmt.Println("  test <file.kilnx>       Run declarative tests")
	fmt.Println("  lsp                     Start Language Server Protocol server")
	fmt.Println("  version                 Print version")
}
