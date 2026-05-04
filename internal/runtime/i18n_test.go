package runtime

import (
	"net/http/httptest"
	"testing"
)

func TestNewI18n_DefaultLangEmpty(t *testing.T) {
	i18n := NewI18n(nil, "", false)
	if i18n.defaultLanguage != "en" {
		t.Errorf("default language = %q, want en", i18n.defaultLanguage)
	}
}

func TestTranslate_FallbackToDefault(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"hello": "Hello"},
		"pt": {"bye":   "Tchau"},
	}, "en", true)

	// pt has "bye" but not "hello" — should fallback to en
	req := httptest.NewRequest("GET", "/?lang=pt", nil)
	if got := i18n.Translate("hello", req); got != "Hello" {
		t.Errorf("translate = %q, want Hello", got)
	}
}

func TestDetectLanguage_NilRequest(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"hello": "Hello"},
	}, "en", true)

	if got := i18n.Translate("hello", nil); got != "Hello" {
		t.Errorf("translate = %q, want Hello", got)
	}
}

func TestDetectLanguage_QueryParamNotFound(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"hello": "Hello"},
	}, "en", true)

	req := httptest.NewRequest("GET", "/?lang=fr", nil)
	if got := i18n.Translate("hello", req); got != "Hello" {
		t.Errorf("translate = %q, want Hello", got)
	}
}

func TestDetectLanguage_EmptyAcceptLanguage(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"hello": "Hello"},
	}, "en", true)

	req := httptest.NewRequest("GET", "/", nil)
	if got := i18n.Translate("hello", req); got != "Hello" {
		t.Errorf("translate = %q, want Hello", got)
	}
}

func TestDetectLanguage_BaseLanguageMatch(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"hello": "Hello"},
		"pt": {"hello": "Olá"},
	}, "en", true)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Language", "pt-BR,en;q=0.8")
	if got := i18n.Translate("hello", req); got != "Olá" {
		t.Errorf("translate = %q, want Olá", got)
	}
}
