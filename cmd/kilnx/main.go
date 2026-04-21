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
			fmt.Println("Usage: kilnx migrate <file.kilnx> [--dry-run|--status]")
			os.Exit(1)
		}
		if err := cmdMigrate(os.Args[2], os.Args[3:]); err != nil {
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
	dbURL := dbPathFor(filename)
	if app.Config != nil {
		if app.Config.Port > 0 {
			port = app.Config.Port
		}
		if app.Config.Database != "" {
			dbURL = app.Config.Database
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

	db, err := database.Open(dbURL)
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
		stmts, err := db.Migrate(app.Models, app.CustomManifests)
		if err != nil {
			return err
		}
		if len(stmts) > 0 {
			fmt.Printf("Auto-migrated %d change(s) to %s\n", len(stmts), dbURL)
		}
	}

	return runtime.WatchAndServe(filename, db, port)
}

func cmdMigrate(filename string, flags []string) error {
	dryRun := false
	status := false
	for _, f := range flags {
		switch f {
		case "--dry-run":
			dryRun = true
		case "--status":
			status = true
		}
	}

	app, err := loadApp(filename)
	if err != nil {
		return err
	}

	if len(app.Models) == 0 {
		fmt.Println("No models found.")
		return nil
	}

	dbURL := dbPathFor(filename)
	if app.Config != nil && app.Config.Database != "" {
		dbURL = app.Config.Database
	}
	fmt.Printf("Database: %s\n", dbURL)
	fmt.Printf("Models:   %d\n\n", len(app.Models))

	db, err := database.Open(dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.MigrateInternal(); err != nil {
		return err
	}

	if status {
		// Show migration history
		history, err := db.MigrationHistory()
		if err != nil {
			return err
		}
		if len(history) == 0 {
			fmt.Println("No migration history.")
		} else {
			fmt.Printf("Migration history (%d entries):\n", len(history))
			for _, r := range history {
				fmt.Printf("  #%d  %s  hash=%s\n", r.ID, r.AppliedAt, r.SchemaHash)
			}
		}

		// Show pending changes
		pending, err := db.PlanMigration(app.Models, app.CustomManifests)
		if err != nil {
			return err
		}
		fmt.Println()
		if len(pending) == 0 {
			fmt.Println("Database is up to date.")
		} else {
			fmt.Printf("Pending changes (%d):\n", len(pending))
			for _, stmt := range pending {
				printMigrationStmt(stmt)
			}
		}
		return nil
	}

	if dryRun {
		stmts, err := db.PlanMigration(app.Models, app.CustomManifests)
		if err != nil {
			return err
		}
		if len(stmts) == 0 {
			fmt.Println("Nothing to migrate. Database is up to date.")
			return nil
		}
		fmt.Printf("Would apply %d migration(s):\n", len(stmts))
		for _, stmt := range stmts {
			fmt.Printf("\n%s;\n", stmt)
		}
		return nil
	}

	stmts, err := db.Migrate(app.Models, app.CustomManifests)
	if err != nil {
		return err
	}

	if len(stmts) == 0 {
		fmt.Println("Nothing to migrate. Database is up to date.")
		return nil
	}

	fmt.Printf("Applied %d migration(s):\n", len(stmts))
	for _, stmt := range stmts {
		printMigrationStmt(stmt)
	}

	return nil
}

func printMigrationStmt(stmt string) {
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
		stmts, err := db.Migrate(app.Models, app.CustomManifests)
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
	absEntry, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("resolving path %s: %w", filename, err)
	}
	projectRoot := filepath.Dir(absEntry)
	source, err := resolveImports(absEntry, projectRoot, nil, 0)
	if err != nil {
		return nil, err
	}

	source = lexer.StripComments(source)
	tokens := lexer.Tokenize(source)
	app, err := parser.Parse(tokens, source)
	if err != nil {
		return nil, err
	}

	app.CustomManifests = make(map[string]*parser.CustomFieldManifest)
	for _, model := range app.Models {
		if model.CustomFieldsFile == "" {
			continue
		}
		// Dynamic paths (containing {placeholder}) are resolved at request time.
		// Load the fallback manifest at startup if it is a static path.
		if strings.Contains(model.CustomFieldsFile, "{") {
			if model.CustomFieldsFallback != "" && !strings.Contains(model.CustomFieldsFallback, "{") {
				m, err := loadManifest(projectRoot, model.CustomFieldsFallback, model.Name)
				if err != nil {
					return nil, err
				}
				app.CustomManifests[model.Name] = m
			}
			continue
		}
		m, err := loadManifest(projectRoot, model.CustomFieldsFile, model.Name)
		if err != nil {
			return nil, err
		}
		app.CustomManifests[model.Name] = m
	}

	return app, nil
}

func loadManifest(projectRoot, path, modelName string) (*parser.CustomFieldManifest, error) {
	if !strings.HasSuffix(path, ".kilnx") {
		return nil, fmt.Errorf("custom fields manifest must be a .kilnx file: %s", path)
	}
	manifestPath := filepath.Join(projectRoot, path)
	rel, err := filepath.Rel(projectRoot, manifestPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("custom fields manifest escapes project directory: %s", path)
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading manifest %s: %w", path, err)
	}
	m, err := parser.ParseManifest(string(raw), modelName)
	if err != nil {
		return nil, fmt.Errorf("parsing manifest %s: %w", path, err)
	}
	return m, nil
}

const maxImportDepth = 64

// resolveImports reads a .kilnx file and recursively resolves import statements.
// Import syntax: import "path/to/file.kilnx"
// Paths are relative to the importing file's directory.
// Security: imported files must have .kilnx extension and stay within projectRoot.
// Circular imports are detected via the seen map. Depth is limited to 64 levels.
func resolveImports(absPath, projectRoot string, seen map[string]bool, depth int) (string, error) {
	if depth > maxImportDepth {
		return "", fmt.Errorf("import depth exceeds %d levels", maxImportDepth)
	}

	if seen == nil {
		seen = make(map[string]bool)
	}
	if seen[absPath] {
		return "", fmt.Errorf("circular import detected: %s", absPath)
	}
	seen[absPath] = true

	raw, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", absPath, err)
	}

	baseDir := filepath.Dir(absPath)
	var result strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") {
			importPath := strings.TrimPrefix(trimmed, "import ")
			importPath = strings.Trim(importPath, "\"' ")
			if importPath == "" {
				continue
			}
			// Enforce .kilnx extension
			if !strings.HasSuffix(importPath, ".kilnx") {
				return "", fmt.Errorf("import must be a .kilnx file: %s", importPath)
			}
			resolved, err := filepath.Abs(filepath.Join(baseDir, importPath))
			if err != nil {
				return "", fmt.Errorf("resolving import path %s: %w", importPath, err)
			}
			// Prevent path traversal outside project root
			if !strings.HasPrefix(resolved, projectRoot+string(filepath.Separator)) && resolved != projectRoot {
				return "", fmt.Errorf("import escapes project directory: %s", importPath)
			}
			imported, err := resolveImports(resolved, projectRoot, seen, depth+1)
			if err != nil {
				return "", fmt.Errorf("importing %s: %w", importPath, err)
			}
			result.WriteString(imported)
			result.WriteString("\n")
		} else {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	return result.String(), nil
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
	fmt.Println("          --dry-run       Show SQL without applying")
	fmt.Println("          --status        Show migration history and pending changes")
	fmt.Println("  test <file.kilnx>       Run declarative tests")
	fmt.Println("  lsp                     Start Language Server Protocol server")
	fmt.Println("  version                 Print version")
}
