package lsp

import (
	"bytes"
	"strings"
	"testing"
)

func TestIsWordChar(t *testing.T) {
	for _, c := range "abcXYZ_0129" {
		if !isWordChar(byte(c)) {
			t.Errorf("isWordChar(%q) = false, want true", c)
		}
	}
	for _, c := range " \t\n.:-/" {
		if isWordChar(byte(c)) {
			t.Errorf("isWordChar(%q) = true, want false", c)
		}
	}
}

func TestWordAt(t *testing.T) {
	cases := []struct {
		line string
		col  int
		want string
	}{
		{"model post", 0, "model"},
		{"model post", 6, "post"},
		{"  page /", 4, "page"},
		{"foo_bar baz", 2, "foo_bar"},
		{"x", 0, "x"},
		{"   ", 1, ""},
		{"abc", 100, "abc"},
		{"abc", -1, ""},
	}
	for _, tc := range cases {
		got := wordAt(tc.line, tc.col)
		if got != tc.want {
			t.Errorf("wordAt(%q, %d) = %q, want %q", tc.line, tc.col, got, tc.want)
		}
	}
}

func TestRangeFromErrorLine(t *testing.T) {
	cases := []struct {
		msg     string
		want    int // expected start.Line (0-indexed)
	}{
		{"line 5: parse error", 4},
		{"line 1: bad", 0},
		{"unrelated message", 0},
		{"line zzz: bad", 0},
	}
	for _, tc := range cases {
		r := rangeFromErrorLine(tc.msg)
		if r.Start.Line != tc.want {
			t.Errorf("rangeFromErrorLine(%q).Start.Line = %d, want %d", tc.msg, r.Start.Line, tc.want)
		}
	}
}

func TestLineRange(t *testing.T) {
	r := lineRange(7)
	if r.Start.Line != 7 || r.End.Line != 7 || r.End.Character != 999 {
		t.Errorf("lineRange(7) = %+v, unexpected", r)
	}
}

func TestFindDefinitionLine(t *testing.T) {
	lines := strings.Split("model post\n  title: text\nmodel user\n  email: text\npage /home\n", "\n")
	if got := findDefinitionLine(lines, "model", "post"); got != 0 {
		t.Errorf("expected line 0 for model post, got %d", got)
	}
	if got := findDefinitionLine(lines, "model", "user"); got != 2 {
		t.Errorf("expected line 2 for model user, got %d", got)
	}
	if got := findDefinitionLine(lines, "page", "/home"); got != 4 {
		t.Errorf("expected line 4 for page /home, got %d", got)
	}
	if got := findDefinitionLine(lines, "model", "missing"); got != -1 {
		t.Errorf("expected -1 for missing model, got %d", got)
	}
}

func TestFindFieldLine(t *testing.T) {
	lines := strings.Split("model post\n  title: text\n  body: text\nmodel user\n", "\n")
	if got := findFieldLine(lines, "title", 1); got != 1 {
		t.Errorf("expected line 1 for title, got %d", got)
	}
	if got := findFieldLine(lines, "body", 1); got != 2 {
		t.Errorf("expected line 2 for body, got %d", got)
	}
	if got := findFieldLine(lines, "missing", 1); got != -1 {
		t.Errorf("expected -1 for missing field, got %d", got)
	}
}

func TestWriteMessage(t *testing.T) {
	var buf bytes.Buffer
	writeMessage(&buf, map[string]string{"foo": "bar"})
	out := buf.String()
	if !strings.HasPrefix(out, "Content-Length: ") {
		t.Errorf("missing Content-Length header: %s", out)
	}
	if !strings.Contains(out, `"foo":"bar"`) {
		t.Errorf("body missing: %s", out)
	}
}

func TestPublishDiagnostics_BadParse(t *testing.T) {
	var buf bytes.Buffer
	publishDiagnostics(&buf, "file:///x", "model post\n  bogus syntax\\n@@@")
	if buf.Len() == 0 {
		t.Error("expected diagnostics output")
	}
	if !strings.Contains(buf.String(), "publishDiagnostics") {
		t.Errorf("missing notification method: %s", buf.String())
	}
}

func TestPublishDiagnostics_Clean(t *testing.T) {
	var buf bytes.Buffer
	publishDiagnostics(&buf, "file:///x", "page /\n  \"hi\"\n")
	if buf.Len() == 0 {
		t.Error("expected output (notification still sent even when clean)")
	}
}

func TestGetCompletions_TopLevel(t *testing.T) {
	items := getCompletions("", position{Line: 0, Character: 0})
	if len(items) == 0 {
		t.Fatal("expected top-level keyword completions")
	}
	found := false
	for _, it := range items {
		if it.Label == "model" || it.Label == "page" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected model/page in top-level completions, got %v", items)
	}
}

func TestGetCompletions_AfterLayout(t *testing.T) {
	src := "layout admin\n  \"x\"\npage /home\n  layout "
	items := getCompletions(src, position{Line: 3, Character: 9})
	for _, it := range items {
		if it.Label == "admin" {
			return
		}
	}
	t.Error("expected layout name 'admin' in completions")
}

func TestGetCompletions_InSQLContext(t *testing.T) {
	src := "model post\n  title: text\npage /list\n  query posts: SELECT * FROM "
	items := getCompletions(src, position{Line: 3, Character: 60})
	for _, it := range items {
		if it.Label == "post" {
			return
		}
	}
	t.Error("expected model 'post' in SQL completions")
}

func TestGetCompletions_BodyContext(t *testing.T) {
	src := "model post\n  "
	items := getCompletions(src, position{Line: 1, Character: 2})
	if len(items) == 0 {
		t.Error("expected body completions")
	}
}

func TestGetHover_Keyword(t *testing.T) {
	src := "model post\n"
	got := getHover(src, position{Line: 0, Character: 2})
	if got == nil {
		t.Fatal("expected hover for keyword 'model'")
	}
	if !strings.Contains(got.Contents.Value, "model") {
		t.Errorf("expected hover content to mention 'model', got %q", got.Contents.Value)
	}
}

func TestGetHover_ModelName(t *testing.T) {
	src := "model post\n  title: text required\n"
	got := getHover(src, position{Line: 0, Character: 7})
	if got == nil {
		t.Fatal("expected hover for model name")
	}
	if !strings.Contains(got.Contents.Value, "post") || !strings.Contains(got.Contents.Value, "title") {
		t.Errorf("hover should describe model post/title, got: %s", got.Contents.Value)
	}
}

func TestGetHover_LayoutName(t *testing.T) {
	src := "layout admin\n  \"x\"\n"
	got := getHover(src, position{Line: 0, Character: 8})
	if got == nil {
		t.Fatal("expected hover for layout")
	}
	if !strings.Contains(got.Contents.Value, "admin") {
		t.Errorf("expected layout name in hover, got: %s", got.Contents.Value)
	}
}

func TestGetHover_NoWord(t *testing.T) {
	if getHover("page /\n", position{Line: 0, Character: 100}) != nil {
		t.Error("expected nil hover when out of range")
	}
	if getHover("", position{Line: 5, Character: 0}) != nil {
		t.Error("expected nil hover when line out of range")
	}
}

func TestGetDefinition_Model(t *testing.T) {
	src := "model post\n  title: text\nmodel user\n  ref: post\n"
	got := getDefinition(src, position{Line: 3, Character: 8}, "file:///x")
	if got == nil {
		t.Fatal("expected definition for 'post'")
	}
	if got.Range.Start.Line != 0 {
		t.Errorf("expected def line 0, got %d", got.Range.Start.Line)
	}
}

func TestGetDefinition_NoMatch(t *testing.T) {
	src := "page /\n"
	if getDefinition(src, position{Line: 99, Character: 0}, "file:///x") != nil {
		t.Error("expected nil for out-of-range")
	}
}

func TestGetDocumentSymbols(t *testing.T) {
	src := `model post
  title: text
page /home
  "hi"
action /create method POST
  redirect /
fragment /card
  "x"
layout admin
  "x"
api /api/v1/posts
  query items: SELECT id FROM post
`
	syms := getDocumentSymbols(src)
	if len(syms) < 4 {
		t.Fatalf("expected at least 4 symbols, got %d: %+v", len(syms), syms)
	}
	kinds := make(map[int]int)
	for _, s := range syms {
		kinds[s.Kind]++
	}
	if kinds[5] == 0 {
		t.Error("expected at least one model (kind=5)")
	}
	if kinds[12] == 0 {
		t.Error("expected at least one function-kind symbol (page/action/fragment)")
	}
}
