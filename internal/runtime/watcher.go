package runtime

import (
	"fmt"
	"os"
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

	go watchFile(filename, srv)

	return srv.Start()
}

func loadApp(filename string) (*parser.App, error) {
	source, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filename, err)
	}

	tokens := lexer.Tokenize(string(source))
	app, err := parser.Parse(tokens, string(source))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filename, err)
	}

	return app, nil
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
			fmt.Printf("  reloaded %s (%d pages)\n", filename, len(app.Pages))
			printRoutes(app)
		}
	}
}

func printRoutes(app *parser.App) {
	fmt.Printf("Parsed %d page(s), %d action(s), %d fragment(s)\n",
		len(app.Pages), len(app.Actions), len(app.Fragments))
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
}
