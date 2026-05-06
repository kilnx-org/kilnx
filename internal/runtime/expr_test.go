package runtime

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// resolveKxValue must leave plain templates alone so existing apps keep working.
func TestResolveKxValue_LegacyTemplate(t *testing.T) {
	got := resolveKxValue("Bearer :token", map[string]string{"token": "abc"})
	if got != "Bearer abc" {
		t.Errorf("got %q, want Bearer abc", got)
	}
}

// resolveKxValue activates the evaluator only when the value starts with
// `ident(`. Bare `:foo` and arithmetic-looking text stay literal.
func TestResolveKxValue_OptInShape(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"hello", "hello"},
		{":a", "X"},
		{":a + :b", "X + Y"},
		{"upper(:a)", "X"},
		{"upper('hello')", "HELLO"},
	}
	params := map[string]string{"a": "X", "b": "Y"}
	for _, tc := range cases {
		if got := resolveKxValue(tc.in, params); got != tc.want {
			t.Errorf("resolveKxValue(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEvalExpression_Arithmetic(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"round(1.5 + 2.7)", "4"},
		{"round(1.234, 2)", "1.23"},
		{"floor(2.9)", "2"},
		{"ceil(2.1)", "3"},
		{"abs(-3)", "3"},
		{"min(5, 2, 8)", "2"},
		{"max(5, 2, 8)", "8"},
		{"clamp(15, 0, 10)", "10"},
		{"int(3.9)", "3"},
	}
	for _, tc := range cases {
		got, err := evalExpression(tc.in, nil)
		if err != nil {
			t.Errorf("%s: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEvalExpression_StringFns(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"slugify('Hello World!')", "hello-world"},
		{"upper('abc')", "ABC"},
		{"lower('ABC')", "abc"},
		{"trim('  x  ')", "x"},
		{"len('hello')", "5"},
		{"replace('foo bar', 'foo', 'baz')", "baz bar"},
		{"contains('hello', 'ell')", "true"},
		{"starts('hello', 'he')", "true"},
		{"ends('hello', 'lo')", "true"},
		{"format('Hi {0}, you are {1}', 'Ada', '42')", "Hi Ada, you are 42"},
		{"coalesce('', '', 'fallback')", "fallback"},
		{"sha256('abc')", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{"base64('hi')", "aGk="},
		{"unbase64('aGk=')", "hi"},
	}
	for _, tc := range cases {
		got, err := evalExpression(tc.in, nil)
		if err != nil {
			t.Errorf("%s: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEvalExpression_WithParams(t *testing.T) {
	params := map[string]string{
		"total": "1000",
		"name":  "Hello World",
		"rate":  "0.5",
	}
	cases := []struct {
		in, want string
	}{
		{"round(:total * 1.5)", "1500"},
		{"slugify(:name)", "hello-world"},
		{"int(:total * :rate)", "500"},
		{"upper(:name)", "HELLO WORLD"},
	}
	for _, tc := range cases {
		got, err := evalExpression(tc.in, params)
		if err != nil {
			t.Errorf("%s: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEvalExpression_BcryptProducesValidHash(t *testing.T) {
	got, err := evalExpression("bcrypt('secret')", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got), []byte("secret")); err != nil {
		t.Errorf("bcrypt output does not verify: %v", err)
	}
}

func TestEvalExpression_UUIDShape(t *testing.T) {
	got, err := evalExpression("uuid()", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 36 || strings.Count(got, "-") != 4 {
		t.Errorf("uuid = %q, want 36-char dashed", got)
	}
}

func TestEvalExpression_JSONGet(t *testing.T) {
	got, err := evalExpression(`json_get('{"user":{"name":"Ada","ids":[7,8]}}', 'user.name')`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Ada" {
		t.Errorf("json_get = %q, want Ada", got)
	}
	got, err = evalExpression(`json_get('{"user":{"name":"Ada","ids":[7,8]}}', 'user.ids.1')`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "8" {
		t.Errorf("json_get index = %q, want 8", got)
	}
}

func TestEvalExpression_RegexMatches(t *testing.T) {
	got, err := evalExpression(`matches('foo@bar.com', '^[^@]+@[^@]+$')`, nil)
	if err != nil || got != "true" {
		t.Errorf("matches: got %q err %v", got, err)
	}
	got, err = evalExpression(`regex('order_42_done', '\\d+')`, nil)
	if err != nil || got != "42" {
		t.Errorf("regex: got %q err %v", got, err)
	}
}

func TestEvalExpression_DivisionByZero(t *testing.T) {
	if _, err := evalExpression("1 / 0", nil); err == nil {
		t.Error("expected division by zero error")
	}
}

func TestEvalExpression_UnknownFunction(t *testing.T) {
	if _, err := evalExpression("nope(1)", nil); err == nil {
		t.Error("expected unknown function error")
	}
}

// Expression failure must fall back to legacy template substitution so a bad
// expression never breaks existing apps that happen to use parens in literal
// text. The :param refs still get substituted as plain text.
func TestResolveKxValue_FallbackOnError(t *testing.T) {
	got := resolveKxValue("nope(:x)", map[string]string{"x": "y"})
	if got != "nope(y)" {
		t.Errorf("got %q, want nope(y) (legacy substitution after eval failure)", got)
	}
}
