// Command kilnx-gendocs regenerates the reference docs from internal/spec.
//
//	go run ./cmd/kilnx-gendocs            # writes to docs/devs/reference
//	go run ./cmd/kilnx-gendocs -o /tmp    # writes to /tmp/devs/reference
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
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/kilnx-org/kilnx/internal/spec"

	// Blank imports to trigger init()-time entity registration. Each
	// package registers the keywords/attributes it implements.
	_ "github.com/kilnx-org/kilnx/internal/parser"
)

//go:embed templates/*.md.tmpl
var templatesFS embed.FS

// repoRoot is the absolute path to the repo top-level. Resolved at startup
// so provenance lookups work regardless of `go generate` working dir.
var repoRoot string

type indexCtx struct {
	Keywords   []spec.Entity
	Attributes []spec.Entity
}

// entityCtx wraps a spec.Entity with provenance data sourced from `git log`.
// SpecSHA/Date track the spec file (where the entity is registered);
// SourceSHA/Date track the implementation file(s) found heuristically by
// grepping for `case "<name>":` and `Value == "<name>"` patterns. Stale
// is true when source was touched after spec — a hint that Description
// may be out of date.
type entityCtx struct {
	spec.Entity
	SpecSHA     string
	SpecDate    string
	SourceSHA   string
	SourceDate  string
	SourceFiles []string
	Stale       bool
}

func main() {
	out := flag.String("o", "docs/devs/reference", "output directory")
	checkStale := flag.Bool("check-stale", false, "exit 1 if any entity's source was touched after its spec")
	flag.Parse()

	repoRoot = resolveRepoRoot()

	tmpl, err := loadTemplates()
	if err != nil {
		log.Fatalf("load templates: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(*out, "keywords"), 0o750); err != nil {
		log.Fatalf("mkdir keywords: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(*out, "attributes"), 0o750); err != nil {
		log.Fatalf("mkdir attributes: %v", err)
	}

	keywords := spec.ByKind(spec.KindKeyword)
	attributes := spec.ByKind(spec.KindAttribute)

	augmentSeeAlso(keywords, attributes)

	var stale []string
	render := func(kind string, e spec.Entity) {
		ctx := buildCtx(e)
		if ctx.Stale {
			stale = append(stale, fmt.Sprintf("%s %q (source %s on %s, spec %s on %s)",
				kind, e.Name, ctx.SourceSHA, ctx.SourceDate, ctx.SpecSHA, ctx.SpecDate))
		}
		path := filepath.Join(*out, kind+"s", e.Name+".md")
		if err := writeTemplate(tmpl, kind+".md.tmpl", path, ctx); err != nil {
			log.Fatalf("render %s: %v", e.Name, err)
		}
		fmt.Println("wrote", path)
	}
	for _, e := range keywords {
		render("keyword", e)
	}
	for _, e := range attributes {
		render("attribute", e)
	}

	indexPath := filepath.Join(*out, "index.md")
	idx := indexCtx{Keywords: keywords, Attributes: attributes}
	if err := writeTemplate(tmpl, "index.md.tmpl", indexPath, idx); err != nil {
		log.Fatalf("render index: %v", err)
	}
	fmt.Println("wrote", indexPath)

	if *checkStale && len(stale) > 0 {
		fmt.Fprintln(os.Stderr, "stale entities (source touched after spec):")
		for _, line := range stale {
			fmt.Fprintln(os.Stderr, "  -", line)
		}
		fmt.Fprintln(os.Stderr, "\nReview the description against current behavior and update the *_spec.go file.")
		os.Exit(1)
	}
}

// augmentSeeAlso makes SeeAlso bidirectional: if A.SeeAlso contains B,
// B's rendered page also lists A. Mutates the entries of the supplied
// slices in place. Self-references and duplicates are filtered.
func augmentSeeAlso(groups ...[]spec.Entity) {
	reverse := map[string]map[string]bool{}
	for _, g := range groups {
		for _, e := range g {
			for _, target := range e.SeeAlso {
				if target == e.Name {
					continue
				}
				if reverse[target] == nil {
					reverse[target] = map[string]bool{}
				}
				reverse[target][e.Name] = true
			}
		}
	}
	for _, g := range groups {
		for i := range g {
			existing := map[string]bool{}
			for _, s := range g[i].SeeAlso {
				existing[s] = true
			}
			added := make([]string, 0, len(reverse[g[i].Name]))
			for ref := range reverse[g[i].Name] {
				if !existing[ref] {
					added = append(added, ref)
				}
			}
			if len(added) == 0 {
				continue
			}
			sort.Strings(added)
			g[i].SeeAlso = append(g[i].SeeAlso, added...)
		}
	}
}

// buildCtx enriches a spec.Entity with provenance: the commit that last
// touched its spec registration and the commit that last touched its
// implementation source. Falls back gracefully if not run inside a git
// checkout (so external builds still produce docs).
func buildCtx(e spec.Entity) entityCtx {
	ctx := entityCtx{Entity: e}

	if specFile := findSpecFile(e.Name); specFile != "" {
		if rel, err := filepath.Rel(repoRoot, specFile); err == nil {
			specFile = rel
		}
		ctx.SpecSHA, ctx.SpecDate = gitLastTouch([]string{specFile})
	}

	ctx.SourceFiles = findSourceFiles(e.Name)
	if len(ctx.SourceFiles) > 0 {
		ctx.SourceSHA, ctx.SourceDate = gitLastTouch(ctx.SourceFiles)
	}

	if ctx.SpecDate != "" && ctx.SourceDate != "" {
		spec, err1 := time.Parse("2006-01-02", ctx.SpecDate)
		src, err2 := time.Parse("2006-01-02", ctx.SourceDate)
		if err1 == nil && err2 == nil && src.After(spec) {
			ctx.Stale = true
		}
	}

	return ctx
}

// findSpecFile locates the *_spec.go file that registers an entity by
// grepping for its Name field. Returns "" if not found.
func findSpecFile(name string) string {
	pattern := fmt.Sprintf(`Name:\s*"%s"`, regexp.QuoteMeta(name))
	matches := grepFiles(pattern, []string{filepath.Join(repoRoot, "internal", "parser")}, "_spec.go")
	if len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[0]
}

// findSourceFiles locates implementation files for an entity by grepping
// for the dispatch patterns Kilnx uses to recognize keywords:
//   - `case "<name>":` in switch statements
//   - `Value == "<name>"` in token comparisons
//   - `Value: "<name>"` in struct literals
//
// Excludes *_test.go and *_spec.go (those describe, not implement).
func findSourceFiles(name string) []string {
	q := regexp.QuoteMeta(name)
	pattern := fmt.Sprintf(`case "%s":|Value == "%s"|Value: "%s"`, q, q, q)
	matches := grepFiles(pattern, []string{filepath.Join(repoRoot, "internal")}, "")
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if strings.HasSuffix(m, "_test.go") || strings.HasSuffix(m, "_spec.go") {
			continue
		}
		rel, err := filepath.Rel(repoRoot, m)
		if err == nil {
			m = rel
		}
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// grepFiles returns absolute repo paths of .go files in roots whose
// content matches the regex. Pure-Go scan keeps the gendocs binary
// independent of grep flavor.
func grepFiles(pattern string, roots []string, suffixFilter string) []string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	var out []string
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			if suffixFilter != "" && !strings.HasSuffix(path, suffixFilter) {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			if re.Match(b) {
				out = append(out, path)
			}
			return nil
		})
	}
	return out
}

// gitLastTouch returns the short SHA and committer date (YYYY-MM-DD) of
// the most recent commit that modified any of the given paths. Paths
// may be absolute or repo-relative. Returns empty strings if git fails
// (e.g. running outside a checkout).
func gitLastTouch(paths []string) (sha, date string) {
	if len(paths) == 0 || repoRoot == "" {
		return "", ""
	}
	args := append([]string{"-C", repoRoot, "log", "-1", "--format=%h%x09%cs", "--"}, paths...)
	cmd := exec.Command("git", args...)
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
		"seeAlsoPath": seeAlsoPath, // alias for back-compat in templates
		"entityPath":  seeAlsoPath,
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

// seeAlsoPath returns a relative markdown link target for a SeeAlso entry,
// resolved against the directory of the document doing the linking.
func seeAlsoPath(target, fromDir string) string {
	e, ok := spec.Get(target)
	if !ok {
		return target + ".md"
	}
	var targetDir string
	switch e.Kind {
	case spec.KindKeyword:
		targetDir = "keywords"
	case spec.KindAttribute:
		targetDir = "attributes"
	default:
		return target + ".md"
	}
	if targetDir == fromDir {
		return target + ".md"
	}
	return "../" + targetDir + "/" + target + ".md"
}

func writeTemplate(tmpl *template.Template, name, path string, data any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return tmpl.ExecuteTemplate(f, name, data)
}
