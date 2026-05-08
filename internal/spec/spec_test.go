package spec

import "testing"

// These tests cover the registry primitives only. Data invariants over
// the actual registered entities live in internal/parser/spec_test.go,
// because the parser package is where entities are registered (avoiding
// import cycles with spec itself).

func TestKind_String(t *testing.T) {
	if KindKeyword.String() != "Keyword" {
		t.Errorf("KindKeyword.String() = %q, want %q", KindKeyword.String(), "Keyword")
	}
	if KindAttribute.String() != "Attribute" {
		t.Errorf("KindAttribute.String() = %q, want %q", KindAttribute.String(), "Attribute")
	}
}

func TestRegister_PanicsOnEmptyName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on empty Name")
		}
	}()
	Register(Entity{Name: ""})
}

func TestRegister_PanicsOnDuplicate(t *testing.T) {
	const name = "_test_dup_entity"
	Register(Entity{Name: name, Kind: KindKeyword, Summary: "x", Syntax: "x", Since: "0.0.0"})
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate Name")
		}
		// Cannot unregister; subsequent runs of this test in the same process
		// would re-panic. Tests run in fresh processes per package, so OK.
	}()
	Register(Entity{Name: name, Kind: KindKeyword, Summary: "x", Syntax: "x", Since: "0.0.0"})
}
