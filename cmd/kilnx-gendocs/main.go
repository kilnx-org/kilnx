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
	"path/filepath"
	"sort"
	"text/template"

	"github.com/kilnx-org/kilnx/internal/spec"

	// Blank imports to trigger init()-time entity registration. Each
	// package registers the keywords/attributes it implements.
	_ "github.com/kilnx-org/kilnx/internal/parser"
)

//go:embed templates/*.md.tmpl
var templatesFS embed.FS

type indexCtx struct {
	Keywords   []spec.Entity
	Attributes []spec.Entity
}

func main() {
	out := flag.String("o", "docs/devs/reference", "output directory")
	flag.Parse()

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

	for _, e := range keywords {
		path := filepath.Join(*out, "keywords", e.Name+".md")
		if err := render(tmpl, "keyword.md.tmpl", path, e); err != nil {
			log.Fatalf("render %s: %v", e.Name, err)
		}
		fmt.Println("wrote", path)
	}
	for _, e := range attributes {
		path := filepath.Join(*out, "attributes", e.Name+".md")
		if err := render(tmpl, "attribute.md.tmpl", path, e); err != nil {
			log.Fatalf("render %s: %v", e.Name, err)
		}
		fmt.Println("wrote", path)
	}

	indexPath := filepath.Join(*out, "index.md")
	idx := indexCtx{Keywords: keywords, Attributes: attributes}
	if err := render(tmpl, "index.md.tmpl", indexPath, idx); err != nil {
		log.Fatalf("render index: %v", err)
	}
	fmt.Println("wrote", indexPath)
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

func render(tmpl *template.Template, name, path string, data any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return tmpl.ExecuteTemplate(f, name, data)
}
