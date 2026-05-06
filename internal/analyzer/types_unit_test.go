package analyzer

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestCategoryName(t *testing.T) {
	cases := []struct {
		c    TypeCategory
		want string
	}{
		{CategoryNumeric, "numeric"},
		{CategoryBool, "bool"},
		{CategoryTime, "timestamp"},
		{CategoryText, "text"},
	}
	for _, tc := range cases {
		if got := categoryName(tc.c); got != tc.want {
			t.Errorf("categoryName(%d) = %q, want %q", tc.c, got, tc.want)
		}
	}
}

func TestValidateMinMax(t *testing.T) {
	cases := []struct {
		val     string
		ft      parser.FieldType
		wantErr bool
	}{
		{"42", parser.FieldInt, false},
		{"abc", parser.FieldInt, true},
		{"3.14", parser.FieldFloat, false},
		{"abc", parser.FieldFloat, true},
		{"10", parser.FieldText, false},
		{"-1", parser.FieldText, true},
		{"x", parser.FieldText, true},
		{"5", parser.FieldEmail, false},
		{"9999999999", parser.FieldBigInt, false},
		{"abc", parser.FieldBigInt, true},
		{"anything", parser.FieldBool, false}, // bool doesn't validate min/max
	}
	for _, tc := range cases {
		err := validateMinMax(tc.val, tc.ft)
		if tc.wantErr && err == nil {
			t.Errorf("validateMinMax(%q,%v) expected error, got nil", tc.val, tc.ft)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("validateMinMax(%q,%v) unexpected error: %v", tc.val, tc.ft, err)
		}
	}
}

func TestIsDBDynamic(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "users", DynamicFields: false},
			{Name: "posts", DynamicFields: true},
		},
	}
	if isDBDynamic(app, "users") {
		t.Error("users should not be DB dynamic")
	}
	if !isDBDynamic(app, "posts") {
		t.Error("posts should be DB dynamic")
	}
	if isDBDynamic(app, "missing") {
		t.Error("missing model should return false")
	}
}
