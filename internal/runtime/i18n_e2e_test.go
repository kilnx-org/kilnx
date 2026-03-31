package runtime

import (
	"net/http"
	"strings"
	"testing"
)

func TestE2E_I18n_DefaultLanguage(t *testing.T) {
	src := "translations\n  en\n    greeting: \"Hello\"\n    farewell: \"Goodbye\"\n  pt\n    greeting: \"Ola\"\n    farewell: \"Tchau\"\n\npage /\n  html\n    <h1>{t.greeting}</h1>\n    <p>{t.farewell}</p>\n"

	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	status, body := httpGet(t, baseURL+"/")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if !strings.Contains(body, "Hello") {
		t.Errorf("expected English 'Hello' by default, got %s", body)
	}
	if !strings.Contains(body, "Goodbye") {
		t.Errorf("expected English 'Goodbye' by default, got %s", body)
	}
}

func TestE2E_I18n_AcceptLanguageHeader(t *testing.T) {
	src := "translations\n  en\n    greeting: \"Hello\"\n  pt\n    greeting: \"Ola\"\n\npage /\n  html\n    <h1>{t.greeting}</h1>\n"

	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	status, body := httpGetWithHeader(t, baseURL+"/", "Accept-Language", "pt")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if !strings.Contains(body, "Ola") {
		t.Errorf("expected Portuguese 'Ola' with Accept-Language: pt, got %s", body)
	}
}

func TestE2E_I18n_QueryParamOverride(t *testing.T) {
	src := "translations\n  en\n    greeting: \"Hello\"\n  es\n    greeting: \"Hola\"\n\npage /\n  html\n    <h1>{t.greeting}</h1>\n"

	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	status, body := httpGet(t, baseURL+"/?lang=es")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if !strings.Contains(body, "Hola") {
		t.Errorf("expected Spanish 'Hola' with ?lang=es, got %s", body)
	}
}

func TestE2E_I18n_FallbackToDefault(t *testing.T) {
	src := "translations\n  en\n    greeting: \"Hello\"\n  pt\n    greeting: \"Ola\"\n\npage /\n  html\n    <h1>{t.greeting}</h1>\n"

	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	status, body := httpGetWithHeader(t, baseURL+"/", "Accept-Language", "ja")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if !strings.Contains(body, "Hello") {
		t.Errorf("expected English fallback 'Hello' for unsupported language, got %s", body)
	}
}

func TestE2E_I18n_RegionalFallback(t *testing.T) {
	src := "translations\n  en\n    greeting: \"Hello\"\n  pt\n    greeting: \"Ola\"\n\npage /\n  html\n    <h1>{t.greeting}</h1>\n"

	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	status, body := httpGetWithHeader(t, baseURL+"/", "Accept-Language", "pt-BR,pt;q=0.9,en;q=0.8")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if !strings.Contains(body, "Ola") {
		t.Errorf("expected Portuguese 'Ola' for pt-BR fallback, got %s", body)
	}
}
