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

func TestSpec_ChildrenAreRegistered(t *testing.T) {
	all := spec.All()
	for name, e := range all {
		if e.Kind != spec.KindKeyword {
			continue
		}
		for _, child := range e.Children {
			if _, ok := all[child]; !ok {
				t.Errorf("%s: child %q is not registered", name, child)
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

// Page must surface its direct sub-fields as children. Body nodes (html,
// query, etc.) reach page via ParentScope and are unioned in by ChildrenOf,
// so we only assert the direct attrs here.
func TestSpec_PageHasDirectChildren(t *testing.T) {
	children := spec.ChildrenOf("page")
	got := map[string]bool{}
	for _, c := range children {
		got[c.Name] = true
	}
	for _, want := range []string{"method", "requires", "title", "redirect"} {
		if !got[want] {
			t.Errorf("page is missing expected child %q", want)
		}
	}
}
