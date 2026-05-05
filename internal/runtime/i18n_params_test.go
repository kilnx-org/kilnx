package runtime

import (
	"net/http"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
)

func TestExpandTranslationsBasic(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"greeting": "Hello {name}"},
	}, "en", false)
	ctx := &renderContext{
		i18n:    i18n,
		request: &http.Request{},
		queries: map[string][]database.Row{},
	}
	content := `{t.greeting name="World"}`
	got := expandTranslations(content, ctx)
	want := "Hello World"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandTranslationsFallback(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"greeting": "Hello {name}"},
	}, "en", false)
	ctx := &renderContext{
		i18n:    i18n,
		request: &http.Request{},
	}
	content := `{t.unknown}`
	got := expandTranslations(content, ctx)
	if got != content {
		t.Errorf("expected unchanged for unknown key, got %q", got)
	}
}

func TestExpandTranslationsPluralSimple(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"items": "{count} {count|plural:'item','items'}"},
	}, "en", false)
	ctx := &renderContext{
		i18n:    i18n,
		request: &http.Request{},
	}
	content := `{t.items count=1}`
	got := expandTranslations(content, ctx)
	want := "1 item"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	content = `{t.items count=5}`
	got = expandTranslations(content, ctx)
	want = "5 items"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandTranslationsPluralExtended(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"pt": {"items": "{count|plural: zero='sem itens', one='1 item', other='{count} itens'}"},
	}, "pt", false)
	ctx := &renderContext{
		i18n:    i18n,
		request: &http.Request{},
	}

	content := `{t.items count=0}`
	got := expandTranslations(content, ctx)
	want := "sem itens"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	content = `{t.items count=1}`
	got = expandTranslations(content, ctx)
	want = "1 item"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	content = `{t.items count=5}`
	got = expandTranslations(content, ctx)
	want = "5 itens"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestExpandTranslationsXSSEscape verifies that attacker-controlled parameter
// values are HTML-escaped before substitution. expandTranslations runs as
// Step 0 of renderHTML, before interpolateEscaped, so the substituted value
// has no surrounding {...} for the later escape pass to catch.
func TestExpandTranslationsXSSEscape(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"greeting": "Hello {name}"},
	}, "en", false)
	ctx := &renderContext{
		i18n:    i18n,
		request: &http.Request{},
	}
	content := `{t.greeting name="<script>alert(1)</script>"}`
	got := expandTranslations(content, ctx)
	want := "Hello &lt;script&gt;alert(1)&lt;/script&gt;"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestExpandTranslationsQuotedValueWithSpaces verifies that quoted argument
// values containing spaces survive tokenization (e.g. name="John Doe").
func TestExpandTranslationsQuotedValueWithSpaces(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"greeting": "Hello {name}"},
	}, "en", false)
	ctx := &renderContext{
		i18n:    i18n,
		request: &http.Request{},
	}
	content := `{t.greeting name="John Doe"}`
	got := expandTranslations(content, ctx)
	want := "Hello John Doe"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestExpandTranslationsPluralOtherDoesNotShortCircuit verifies that an
// "other" branch declared before "one" does not preempt the matching
// specific category for count=1.
func TestExpandTranslationsPluralOtherDoesNotShortCircuit(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"items": "{count|plural: other='X', one='Y'}"},
	}, "en", false)
	ctx := &renderContext{
		i18n:    i18n,
		request: &http.Request{},
	}
	content := `{t.items count=1}`
	got := expandTranslations(content, ctx)
	want := "Y"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	content = `{t.items count=7}`
	got = expandTranslations(content, ctx)
	want = "X"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestExpandTranslationsPluralFew verifies the basic Slavic-family heuristic
// for the CLDR "few" category (n%10 in [3..6] and n%100 not in [13..16]).
func TestExpandTranslationsPluralFew(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"items": "{count|plural: one='1 item', few='{count} items (few)', other='{count} items'}"},
	}, "en", false)
	ctx := &renderContext{
		i18n:    i18n,
		request: &http.Request{},
	}
	content := `{t.items count=3}`
	got := expandTranslations(content, ctx)
	want := "3 items (few)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandTranslationsWithQuery(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"order": "Order {number} totals {total}"},
	}, "en", false)
	ctx := &renderContext{
		i18n:    i18n,
		request: &http.Request{},
		queries: map[string][]database.Row{
			"order": {{"number": "123", "total": "99.50"}},
		},
	}
	content := `{t.order number=order.number total=order.total}`
	got := expandTranslations(content, ctx)
	want := "Order 123 totals 99.50"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandTranslationsMissingParam(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"greeting": "Hello {name} from {place}"},
	}, "en", false)
	ctx := &renderContext{
		i18n:    i18n,
		request: &http.Request{},
	}
	content := `{t.greeting name="World"}`
	got := expandTranslations(content, ctx)
	want := "Hello World from {place}"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandTranslationsNoParams(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"hello": "Hello"},
	}, "en", false)
	ctx := &renderContext{
		i18n:    i18n,
		request: &http.Request{},
	}
	content := `{t.hello}`
	got := expandTranslations(content, ctx)
	want := "Hello"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandPluralizationTwo(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"items": "{count|plural: two='a pair', other='{count} items'}"},
	}, "en", false)
	ctx := &renderContext{
		i18n:    i18n,
		request: &http.Request{},
	}
	content := `{t.items count=2}`
	got := expandTranslations(content, ctx)
	want := "a pair"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandPluralizationNonNumeric(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"items": "{count|plural:'item','items'}"},
	}, "en", false)
	ctx := &renderContext{
		i18n:    i18n,
		request: &http.Request{},
	}
	content := `{t.items count=xyz}`
	got := expandTranslations(content, ctx)
	// Non-numeric count parses as 0, which falls to plural form
	want := "items"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
