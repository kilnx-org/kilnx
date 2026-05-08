package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
)

// pkgList enumerates which internal packages get autogen'd. Order
// reflects the conceptual pipeline (lexer first, runtime/build last).
var pkgList = []string{
	"lexer", "parser", "analyzer", "optimizer", "spec",
	"database", "pathutil", "pdf", "runtime", "build",
}

type packageCtx struct {
	Name        string
	ImportPath  string
	Summary     string
	Description string
	Files       []fileSummary
	Types       []typeSummary
	Functions   []funcSummary
	ManualNotes string

	SourceSHA  string
	SourceDate string
	DocSHA     string
	DocDate    string
	Stale      bool
}

type fileSummary struct {
	Name    string
	Path    string
	Summary string
}

type typeSummary struct {
	Name string
	Decl string
	Doc  string
}

type funcSummary struct {
	Name      string
	Recv      string
	Signature string
	Doc       string
}

// generatePackages walks pkgList and writes one md file per package
// to outDir, plus an index. Returns stale entries.
func generatePackages(outDir string, tmpl *template.Template) ([]string, error) {
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return nil, fmt.Errorf("mkdir packages: %w", err)
	}

	var stale []string
	var summaries []packageCtx

	for _, name := range pkgList {
		ctx, err := buildPackageCtx(name, outDir)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		path := filepath.Join(outDir, name+".md")
		if err := writeTemplate(tmpl, "package.md.tmpl", path, ctx); err != nil {
			return nil, fmt.Errorf("render %s: %w", name, err)
		}
		fmt.Println("wrote", path)
		if ctx.Stale {
			stale = append(stale, fmt.Sprintf("package %q (source %s on %s, doc %s on %s)",
				name, ctx.SourceSHA, ctx.SourceDate, ctx.DocSHA, ctx.DocDate))
		}
		summaries = append(summaries, ctx)
	}

	indexPath := filepath.Join(outDir, "index.md")
	if err := writeTemplate(tmpl, "package_index.md.tmpl", indexPath, summaries); err != nil {
		return nil, fmt.Errorf("render package index: %w", err)
	}
	fmt.Println("wrote", indexPath)

	return stale, nil
}

func buildPackageCtx(name, outDir string) (packageCtx, error) {
	pkgDir := filepath.Join(repoRoot, "internal", name)
	importPath := "github.com/kilnx-org/kilnx/internal/" + name

	fset := token.NewFileSet()
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return packageCtx{}, fmt.Errorf("read dir: %w", err)
	}
	var files []*ast.File
	filePaths := map[*ast.File]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		abs := filepath.Join(pkgDir, n)
		f, err := parser.ParseFile(fset, abs, nil, parser.ParseComments)
		if err != nil {
			return packageCtx{}, fmt.Errorf("parse %s: %w", n, err)
		}
		files = append(files, f)
		filePaths[f] = abs
	}
	if len(files) == 0 {
		return packageCtx{}, fmt.Errorf("no go files in %s", pkgDir)
	}

	docPkg, err := doc.NewFromFiles(fset, files, importPath, doc.AllDecls)
	if err != nil {
		return packageCtx{}, fmt.Errorf("doc: %w", err)
	}

	ctx := packageCtx{
		Name:       name,
		ImportPath: importPath,
		Summary:    firstSentence(docPkg.Doc),
	}
	ctx.Description = paragraphsAfterFirst(docPkg.Doc)

	ctx.Files = collectFiles(files, filePaths)
	ctx.Types = collectTypes(docPkg, fset)
	ctx.Functions = collectFuncs(docPkg, fset)

	var sourcePaths []string
	for _, f := range ctx.Files {
		sourcePaths = append(sourcePaths, f.Path)
	}
	if len(sourcePaths) > 0 {
		ctx.SourceSHA, ctx.SourceDate = gitLastTouch(sourcePaths)
	}
	if docFile := filepath.Join("internal", name, "doc.go"); fileExists(filepath.Join(repoRoot, docFile)) {
		ctx.DocSHA, ctx.DocDate = gitLastTouch([]string{docFile})
	}
	if ctx.DocDate != "" && ctx.SourceDate != "" {
		d, e1 := time.Parse("2006-01-02", ctx.DocDate)
		s, e2 := time.Parse("2006-01-02", ctx.SourceDate)
		if e1 == nil && e2 == nil && s.After(d) {
			ctx.Stale = true
		}
	}

	// Read manual notes from the canonical committed location, not outDir.
	// CI regenerates to /tmp and diffs; if we read from outDir, notes would
	// be lost on a fresh tmp run and the diff would always fail.
	canonical := filepath.Join(repoRoot, "docs", "contributors", "packages", name+".md")
	ctx.ManualNotes = extractManualNotes(canonical)

	return ctx, nil
}

func collectFiles(files []*ast.File, paths map[*ast.File]string) []fileSummary {
	var out []fileSummary
	for _, file := range files {
		absPath := paths[file]
		base := filepath.Base(absPath)
		if base == "doc.go" {
			continue
		}
		rel, err := filepath.Rel(repoRoot, absPath)
		if err != nil {
			rel = absPath
		}
		summary := ""
		if file.Doc != nil {
			summary = firstSentence(file.Doc.Text())
		}
		out = append(out, fileSummary{Name: base, Path: rel, Summary: summary})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func collectTypes(p *doc.Package, fset *token.FileSet) []typeSummary {
	var out []typeSummary
	for _, t := range p.Types {
		decl := nodeToString(fset, t.Decl)
		out = append(out, typeSummary{
			Name: t.Name,
			Decl: collapseDecl(decl),
			Doc:  strings.TrimSpace(t.Doc),
		})
	}
	return out
}

func collectFuncs(p *doc.Package, fset *token.FileSet) []funcSummary {
	var out []funcSummary
	for _, f := range p.Funcs {
		out = append(out, funcSummary{
			Name:      f.Name,
			Signature: signatureOf(fset, f.Decl),
			Doc:       strings.TrimSpace(f.Doc),
		})
	}
	for _, t := range p.Types {
		for _, f := range t.Funcs {
			out = append(out, funcSummary{
				Name:      f.Name,
				Signature: signatureOf(fset, f.Decl),
				Doc:       strings.TrimSpace(f.Doc),
			})
		}
		for _, m := range t.Methods {
			out = append(out, funcSummary{
				Name:      m.Name,
				Recv:      t.Name,
				Signature: signatureOf(fset, m.Decl),
				Doc:       strings.TrimSpace(m.Doc),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Recv != out[j].Recv {
			return out[i].Recv < out[j].Recv
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func signatureOf(fset *token.FileSet, fn *ast.FuncDecl) string {
	if fn == nil {
		return ""
	}
	stripped := *fn
	stripped.Body = nil
	stripped.Doc = nil
	return collapseDecl(nodeToString(fset, &stripped))
}

func nodeToString(fset *token.FileSet, n any) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, n); err != nil {
		return ""
	}
	return buf.String()
}

func collapseDecl(s string) string {
	s = strings.TrimSpace(s)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Join(lines, "\n")
}

const (
	manualStart = "<!-- MANUAL-NOTES START -->"
	manualEnd   = "<!-- MANUAL-NOTES END -->"
)

// extractManualNotes reads an existing generated md file and returns the
// content between manualStart and manualEnd markers. If the file does not
// exist or the markers are absent, returns an empty string.
func extractManualNotes(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	s := string(b)
	i := strings.Index(s, manualStart)
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], manualEnd)
	if j < 0 {
		return ""
	}
	body := s[i+len(manualStart) : i+j]
	return strings.TrimSpace(body)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// firstSentence returns the first sentence of doc.go-style text. Skips
// false matches caused by abbreviations like "e.g." and "i.e." where the
// period is preceded by a single lower-case letter following another
// period or word boundary.
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Collapse first paragraph to single line.
	if i := strings.Index(s, "\n\n"); i >= 0 {
		s = s[:i]
	}
	s = strings.Join(strings.Fields(s), " ")

	// Walk and find a period that ends a sentence (followed by space + uppercase).
	for i := 0; i < len(s); i++ {
		if s[i] != '.' {
			continue
		}
		// Skip abbreviations: ".g.", ".e.", ".i." and similar.
		// Heuristic: if preceded by a letter and another period within 3 chars, abbrev.
		if i >= 2 && s[i-2] == '.' {
			continue
		}
		if i+2 >= len(s) {
			return s
		}
		if s[i+1] == ' ' && s[i+2] >= 'A' && s[i+2] <= 'Z' {
			return s[:i+1]
		}
	}
	return s
}

// paragraphsAfterFirst returns everything after the first paragraph of
// doc.go-style text. Used for the long-form Description section.
func paragraphsAfterFirst(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "\n\n"); i >= 0 {
		return strings.TrimSpace(s[i+2:])
	}
	return ""
}
