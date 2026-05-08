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
// Panics on duplicate name to surface registration bugs at startup.
func Register(e Entity) {
	if e.Name == "" {
		panic("spec.Register: empty Name")
	}
	if _, dup := entities[e.Name]; dup {
		panic("spec.Register: duplicate entity " + e.Name)
	}
	entities[e.Name] = e
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

func ChildrenOf(parent string) []Entity {
	p, ok := entities[parent]
	if !ok {
		return nil
	}
	out := make([]Entity, 0, len(p.Children))
	for _, name := range p.Children {
		if c, ok := entities[name]; ok {
			out = append(out, c)
		}
	}
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
