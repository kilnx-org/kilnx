package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/kilnx-org/kilnx/internal/spec"

	// Blank import to trigger init()-time entity registration.
	_ "github.com/kilnx-org/kilnx/internal/parser"
)

type indexCtx struct {
	Keywords   []spec.Entity
	Attributes []spec.Entity
}

// entityCtx wraps a spec.Entity with provenance data sourced from `git log`.
// SpecSHA/Date track the spec file (where the entity is registered);
// SourceSHA/Date track the implementation file(s) found heuristically by
// grepping for `case "<name>":` and `Value == "<name>"` patterns. Stale
// is true when source was touched after spec, hinting Description may be
// out of date.
type entityCtx struct {
	spec.Entity
	SpecSHA     string
	SpecDate    string
	SourceSHA   string
	SourceDate  string
	SourceFiles []string
	Stale       bool
}

// generateReference writes the language reference (keywords + attributes
// + index) to outDir. Returns a list of stale entity descriptions.
func generateReference(outDir string, tmpl *template.Template) ([]string, error) {
	if err := os.MkdirAll(filepath.Join(outDir, "keywords"), 0o750); err != nil {
		return nil, fmt.Errorf("mkdir keywords: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(outDir, "attributes"), 0o750); err != nil {
		return nil, fmt.Errorf("mkdir attributes: %w", err)
	}

	keywords := spec.ByKind(spec.KindKeyword)
	attributes := spec.ByKind(spec.KindAttribute)

	augmentSeeAlso(keywords, attributes)

	var stale []string
	render := func(kind string, e spec.Entity) error {
		ctx := buildEntityCtx(e)
		if ctx.Stale {
			stale = append(stale, fmt.Sprintf("%s %q (source %s on %s, spec %s on %s)",
				kind, e.Name, ctx.SourceSHA, ctx.SourceDate, ctx.SpecSHA, ctx.SpecDate))
		}
		path := filepath.Join(outDir, kind+"s", e.Name+".md")
		if err := writeTemplate(tmpl, kind+".md.tmpl", path, ctx); err != nil {
			return fmt.Errorf("render %s: %w", e.Name, err)
		}
		fmt.Println("wrote", path)
		return nil
	}
	for _, e := range keywords {
		if err := render("keyword", e); err != nil {
			return nil, err
		}
	}
	for _, e := range attributes {
		if err := render("attribute", e); err != nil {
			return nil, err
		}
	}

	indexPath := filepath.Join(outDir, "index.md")
	idx := indexCtx{Keywords: keywords, Attributes: attributes}
	if err := writeTemplate(tmpl, "index.md.tmpl", indexPath, idx); err != nil {
		return nil, fmt.Errorf("render index: %w", err)
	}
	fmt.Println("wrote", indexPath)

	return stale, nil
}

// augmentSeeAlso makes SeeAlso bidirectional: if A.SeeAlso contains B,
// B's rendered page also lists A. Mutates in place. Self-references and
// duplicates filtered.
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

// buildEntityCtx enriches a spec.Entity with provenance.
func buildEntityCtx(e spec.Entity) entityCtx {
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

func findSpecFile(name string) string {
	pattern := fmt.Sprintf(`Name:\s*"%s"`, regexp.QuoteMeta(name))
	matches := grepFiles(pattern, []string{filepath.Join(repoRoot, "internal", "parser")}, "_spec.go")
	if len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[0]
}

// findSourceFiles locates implementation files for an entity.
// Excludes *_test.go and *_spec.go.
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
// content matches the regex.
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
			b, err := os.ReadFile(path) //nolint:gosec // path comes from WalkDir over the repo root; gendocs is a build-time tool
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
