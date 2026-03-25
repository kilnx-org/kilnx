package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/lexer"
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
	case "version":
		fmt.Println("kilnx v0.2.0")
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdRun(filename string) error {
	app, err := loadApp(filename)
	if err != nil {
		return err
	}

	// Auto-migrate if models exist
	if len(app.Models) > 0 {
		dbPath := dbPathFor(filename)
		db, err := database.Open(dbPath)
		if err != nil {
			return err
		}
		stmts, err := db.Migrate(app.Models)
		if err != nil {
			db.Close()
			return err
		}
		if len(stmts) > 0 {
			fmt.Printf("Auto-migrated %d change(s) to %s\n", len(stmts), dbPath)
		}
		db.Close()
	}

	return runtime.WatchAndServe(filename, 8080)
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
			fmt.Printf("  + Added column '%s' to '%s'\n", parts[5], parts[2])
		} else {
			fmt.Printf("  %s\n", stmt)
		}
	}

	return nil
}

func loadApp(filename string) (*parser.App, error) {
	source, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filename, err)
	}

	tokens := lexer.Tokenize(string(source))
	return parser.Parse(tokens)
}

func dbPathFor(kilnxFile string) string {
	dir := filepath.Dir(kilnxFile)
	base := filepath.Base(kilnxFile)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(dir, name+".db")
}

func printUsage() {
	fmt.Println("Usage: kilnx <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  run <file.kilnx>      Start the server (auto-migrates)")
	fmt.Println("  migrate <file.kilnx>  Apply database migrations")
	fmt.Println("  version               Print version")
}
