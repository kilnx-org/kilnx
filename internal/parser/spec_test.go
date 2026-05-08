package parser

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/spec"
)

// Invariant tests over the populated spec registry. Lives here (not in
// internal/spec) because parser is the package that registers entities;
// a test in internal/spec would not see them without importing parser
// and creating an import cycle.

func TestSpec_RequiredFields(t *testing.T) {
	for name, e := range spec.All() {
		if e.Summary == "" {
			t.Errorf("%s: Summary is empty", name)
		}
		if e.Syntax == "" {
			t.Errorf("%s: Syntax is empty", name)
		}
		if e.Since == "" {
			t.Errorf("%s: Since is empty", name)
		}
	}
}

func TestSpec_ChildrenExistAsAttributes(t *testing.T) {
	all := spec.All()
	for name, e := range all {
		if e.Kind != spec.KindKeyword {
			continue
		}
		for _, child := range e.Children {
			c, ok := all[child]
			if !ok {
				t.Errorf("%s: child %q is not registered", name, child)
				continue
			}
			if c.Kind != spec.KindAttribute {
				t.Errorf("%s: child %q must be an attribute, got %s", name, child, c.Kind)
			}
		}
	}
}

func TestSpec_ParentScopeExistsAsKeyword(t *testing.T) {
	all := spec.All()
	for name, e := range all {
		if e.Kind != spec.KindAttribute {
			continue
		}
		for _, parent := range e.ParentScope {
			p, ok := all[parent]
			if !ok {
				// Parent may not be registered yet during incremental rollout.
				continue
			}
			if p.Kind != spec.KindKeyword {
				t.Errorf("%s: parent %q must be a keyword, got %s", name, parent, p.Kind)
			}
		}
	}
}

func TestSpec_AttributeReverseConsistency(t *testing.T) {
	all := spec.All()
	for name, attr := range all {
		if attr.Kind != spec.KindAttribute {
			continue
		}
		for _, parentName := range attr.ParentScope {
			parent, ok := all[parentName]
			if !ok {
				continue
			}
			found := false
			for _, child := range parent.Children {
				if child == name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("attribute %q lists %q in ParentScope, but %q.Children does not include %q",
					name, parentName, parentName, name)
			}
		}
	}
}

func TestSpec_PageHasExpectedChildren(t *testing.T) {
	children := spec.ChildrenOf("page")
	want := map[string]bool{"method": true, "requires": true, "layout": true, "title": true, "redirect": true}
	for _, c := range children {
		if !want[c.Name] {
			t.Errorf("unexpected child of page: %q", c.Name)
		}
		delete(want, c.Name)
	}
	for missing := range want {
		t.Errorf("missing expected child of page: %q", missing)
	}
}
