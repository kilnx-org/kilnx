package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/lexer"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func WatchAndServe(filename string, db *database.DB, port int) error {
	app, err := loadApp(filename)
	if err != nil {
		return err
	}

	printRoutes(app)

	srv := NewServer(app, db, port)
	srv.StartScheduler()
	srv.StartJobQueue()

	go watchFile(filename, srv)

	return srv.Start()
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
		return nil, fmt.Errorf("parsing %s: %w", filename, err)
	}

	return app, nil
}

const maxImportDepth = 64

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
			if !strings.HasSuffix(importPath, ".kilnx") {
				return "", fmt.Errorf("import must be a .kilnx file: %s", importPath)
			}
			resolved, err := filepath.Abs(filepath.Join(baseDir, importPath))
			if err != nil {
				return "", fmt.Errorf("resolving import path %s: %w", importPath, err)
			}
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

func watchFile(filename string, srv *Server) {
	var lastMod time.Time

	info, err := os.Stat(filename)
	if err == nil {
		lastMod = info.ModTime()
	}

	for {
		time.Sleep(500 * time.Millisecond)

		info, err := os.Stat(filename)
		if err != nil {
			continue
		}

		if info.ModTime().After(lastMod) {
			lastMod = info.ModTime()

			app, err := loadApp(filename)
			if err != nil {
				fmt.Printf("  reload error: %v\n", err)
				continue
			}

			srv.Reload(app)
			// Restart schedulers and update job queue with new definitions
			srv.StartScheduler()
			srv.RefreshJobQueue()
			fmt.Printf("  reloaded %s (%d pages)\n", filename, len(app.Pages))
			printRoutes(app)
		}
	}
}

func printRoutes(app *parser.App) {
	fmt.Printf("Parsed %d page(s), %d action(s), %d fragment(s), %d api(s)\n",
		len(app.Pages), len(app.Actions), len(app.Fragments), len(app.APIs))
	for _, p := range app.Pages {
		label := p.Path
		if p.Title != "" {
			label += " (" + p.Title + ")"
		}
		fmt.Printf("  %s %s\n", p.Method, label)
	}
	for _, a := range app.Actions {
		fmt.Printf("  %s %s\n", a.Method, a.Path)
	}
	for _, f := range app.Fragments {
		fmt.Printf("  FRAG %s\n", f.Path)
	}
	for _, a := range app.APIs {
		fmt.Printf("  API  %s %s\n", a.Method, a.Path)
	}
	for _, s := range app.Streams {
		fmt.Printf("  SSE  %s (every %ds)\n", s.Path, s.IntervalSecs)
	}
	for _, s := range app.Schedules {
		fmt.Printf("  CRON %s (every %ds)\n", s.Name, s.IntervalSecs)
	}
	for _, j := range app.Jobs {
		fmt.Printf("  JOB  %s\n", j.Name)
	}
	for _, wh := range app.Webhooks {
		fmt.Printf("  HOOK %s\n", wh.Path)
	}
	for _, sock := range app.Sockets {
		fmt.Printf("  WS   %s\n", sock.Path)
	}
	for _, rl := range app.RateLimits {
		fmt.Printf("  LIMIT %s (%d per %s per %s)\n", rl.PathPattern, rl.Requests, rl.Window, rl.Per)
	}
}
