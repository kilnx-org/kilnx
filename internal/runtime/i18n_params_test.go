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
		i18n:     i18n,
		request:  &http.Request{},
		queries:  map[string][]database.Row{},
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
