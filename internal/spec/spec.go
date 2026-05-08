// Package spec is the canonical schema for Kilnx language entities
// (keywords and attributes). Entities are not declared here: they live
// alongside their implementation (e.g. internal/parser/page_spec.go) and
// register themselves via Register() at init() time. cmd/kilnx-gendocs
// imports those packages and reads the populated registry.
package spec

import "sort"

type Kind int

const (
	KindKeyword Kind = iota
	KindAttribute
)

func (k Kind) String() string {
	switch k {
	case KindKeyword:
		return "Keyword"
	case KindAttribute:
		return "Attribute"
	}
	return "Unknown"
}

type Entity struct {
	Name        string
	Kind        Kind
	Summary     string
	Description string
	Syntax      string
	Args        []Arg
	ParentScope []string
	Children    []string
	Repeatable  bool
	Required    bool
	Default     string
	Since       string
	Examples    []Example
	SeeAlso     []string
}

type Arg struct {
	Name     string
	Type     string
	Required bool
	Variadic bool
}

type Example struct {
	Title string
	Code  string
}

var entities = map[string]Entity{}

// Register adds an entity to the registry. Intended to be called from
// init() in the package that implements the entity (parser, lexer, etc).
//
// Re-registering an attribute with the same Name is allowed for overloaded
// DSL keywords that appear in multiple parent contexts (e.g. `requests`
// inside `log` and `limit`). The ParentScope of the existing entity is
// extended with the new entry's parents, and a context-specific note is
// appended to the Description. For keywords, duplicate registration panics.
func Register(e Entity) {
	if e.Name == "" {
		panic("spec.Register: empty Name")
	}
	if existing, dup := entities[e.Name]; dup {
		if existing.Kind != KindAttribute || e.Kind != KindAttribute {
			panic("spec.Register: duplicate keyword " + e.Name)
		}
		if existing.Kind != e.Kind {
			panic("spec.Register: kind mismatch for " + e.Name)
		}
		for _, p := range e.ParentScope {
			if !contains(existing.ParentScope, p) {
				existing.ParentScope = append(existing.ParentScope, p)
			}
		}
		// Compose context-specific description for overloaded attributes.
		// Each registration may carry only Summary (terse) or full
		// Description; surface both under a "When used in <parent>:" header.
		newText := e.Description
		if newText == "" {
			newText = e.Summary
		}
		if newText != "" && newText != existing.Description && newText != existing.Summary {
			scope := "additional context"
			if len(e.ParentScope) > 0 {
				scope = "`" + e.ParentScope[0] + "`"
			}
			if existing.Description == "" {
				existing.Description = newText
			} else {
				existing.Description += "\n\nWhen used in " + scope + ": " + newText
			}
		}
		if existing.Summary == "" {
			existing.Summary = e.Summary
		}
		if existing.Syntax == "" {
			existing.Syntax = e.Syntax
		}
		if existing.Default == "" {
			existing.Default = e.Default
		}
		if len(existing.Args) == 0 {
			existing.Args = e.Args
		}
		existing.Examples = append(existing.Examples, e.Examples...)
		entities[e.Name] = existing
		return
	}
	entities[e.Name] = e
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// All returns a copy of the registry. Order is unspecified.
func All() map[string]Entity {
	out := make(map[string]Entity, len(entities))
	for k, v := range entities {
		out[k] = v
	}
	return out
}

func Get(name string) (Entity, bool) {
	e, ok := entities[name]
	return e, ok
}

func ByKind(k Kind) []Entity {
	out := make([]Entity, 0)
	for _, e := range entities {
		if e.Kind == k {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ChildrenOf returns the union of:
//   - entities listed explicitly in parent.Children
//   - entities whose ParentScope contains parent (reverse lookup)
//
// Result is deduped by Name and sorted alphabetically.
func ChildrenOf(parent string) []Entity {
	p, ok := entities[parent]
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	out := make([]Entity, 0, len(p.Children))
	for _, name := range p.Children {
		if c, ok := entities[name]; ok && !seen[name] {
			out = append(out, c)
			seen[name] = true
		}
	}
	for _, c := range entities {
		if seen[c.Name] {
			continue
		}
		for _, ps := range c.ParentScope {
			if ps == parent {
				out = append(out, c)
				seen[c.Name] = true
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func ParentsOf(attr string) []Entity {
	a, ok := entities[attr]
	if !ok || a.Kind != KindAttribute {
		return nil
	}
	out := make([]Entity, 0, len(a.ParentScope))
	for _, name := range a.ParentScope {
		if p, ok := entities[name]; ok {
			out = append(out, p)
		}
	}
	return out
}
