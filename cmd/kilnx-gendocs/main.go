// Command kilnx-gendocs regenerates documentation from source.
//
//	go run ./cmd/kilnx-gendocs                          # writes all targets
//	go run ./cmd/kilnx-gendocs -only=reference          # docs/devs/reference only
//	go run ./cmd/kilnx-gendocs -only=packages           # docs/contributors/packages only
//	go run ./cmd/kilnx-gendocs -only=agents             # AGENTS.md only (at repo root)
//	go run ./cmd/kilnx-gendocs -o /tmp                  # writes to /tmp (AGENTS.md goes to /tmp/AGENTS.md)
//	go run ./cmd/kilnx-gendocs -check-stale             # exit 1 if any target is stale
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

//go:embed templates/*.md.tmpl
var templatesFS embed.FS

// repoRoot is the absolute path to the repo top-level. Resolved at startup
// so provenance lookups work regardless of `go generate` working dir.
var repoRoot string

func main() {
	out := flag.String("o", "docs", "output root directory (devs/reference and contributors/packages live under here)")
	only := flag.String("only", "all", "what to generate: all|reference|packages")
	checkStale := flag.Bool("check-stale", false, "exit 1 if any target's source was touched after its doc")
	flag.Parse()

	repoRoot = resolveRepoRoot()

	tmpl, err := loadTemplates()
	if err != nil {
		log.Fatalf("load templates: %v", err)
	}

	var stale []string

	switch *only {
	case "all", "reference":
		s, err := generateReference(filepath.Join(*out, "devs", "reference"), tmpl)
		if err != nil {
			log.Fatalf("reference: %v", err)
		}
		stale = append(stale, s...)
		if *only == "reference" {
			break
		}
		fallthrough
	case "packages":
		s, err := generatePackages(filepath.Join(*out, "contributors", "packages"), tmpl)
		if err != nil {
			log.Fatalf("packages: %v", err)
		}
		stale = append(stale, s...)
		if *only == "packages" {
			break
		}
		fallthrough
	case "agents":
		if err := generateAgents(*out, tmpl); err != nil {
			log.Fatalf("agents: %v", err)
		}
	default:
		log.Fatalf("unknown -only value %q (want all|reference|packages|agents)", *only)
	}

	if *checkStale && len(stale) > 0 {
		fmt.Fprintln(os.Stderr, "stale entries (source touched after spec/doc):")
		for _, line := range stale {
			fmt.Fprintln(os.Stderr, "  -", line)
		}
		fmt.Fprintln(os.Stderr, "\nReview and update accordingly.")
		os.Exit(1)
	}
}

// gitLastTouch returns the short SHA and committer date (YYYY-MM-DD) of
// the most recent commit that modified any of the given paths.
func gitLastTouch(paths []string) (sha, date string) {
	if len(paths) == 0 || repoRoot == "" {
		return "", ""
	}
	args := append([]string{"-C", repoRoot, "log", "-1", "--format=%h%x09%cs", "--"}, paths...)
	cmd := exec.Command("git", args...) //nolint:gosec // args are repo-relative paths from spec/AST scan, not user input; gendocs is a build-time tool
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return "", ""
	}
	line := strings.TrimSpace(string(out))
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// resolveRepoRoot finds the repo top-level via `git rev-parse`. Falls
// back to the current working dir if git is unavailable.
func resolveRepoRoot() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	wd, _ := os.Getwd()
	return wd
}

func loadTemplates() (*template.Template, error) {
	t := template.New("").Funcs(template.FuncMap{
		"seeAlsoPath": seeAlsoPath,
		"entityPath":  seeAlsoPath,
		"firstLine":   firstLine,
	})
	entries, err := fs.ReadDir(templatesFS, "templates")
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, ent := range entries {
		b, err := templatesFS.ReadFile("templates/" + ent.Name())
		if err != nil {
			return nil, err
		}
		if _, err := t.New(ent.Name()).Parse(string(b)); err != nil {
			return nil, fmt.Errorf("parse %s: %w", ent.Name(), err)
		}
	}
	return t, nil
}

func writeTemplate(tmpl *template.Template, name, path string, data any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return tmpl.ExecuteTemplate(f, name, data)
}
