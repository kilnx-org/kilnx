package runtime

import (
	"net/http"
	"strings"
)

// I18n manages translations
type I18n struct {
	translations    map[string]map[string]string // lang -> key -> value
	defaultLanguage string
}

func NewI18n(translations map[string]map[string]string, defaultLang string) *I18n {
	if defaultLang == "" {
		defaultLang = "en"
	}
	if translations == nil {
		translations = make(map[string]map[string]string)
	}
	return &I18n{translations: translations, defaultLanguage: defaultLang}
}

// Translate returns the translation for a key in the detected language
func (i *I18n) Translate(key string, r *http.Request) string {
	lang := i.detectLanguage(r)

	// Try requested language
	if entries, ok := i.translations[lang]; ok {
		if val, ok := entries[key]; ok {
			return val
		}
	}

	// Fall back to default language
	if entries, ok := i.translations[i.defaultLanguage]; ok {
		if val, ok := entries[key]; ok {
			return val
		}
	}

	// Return key itself as fallback
	return key
}

// detectLanguage reads Accept-Language header and returns best match
func (i *I18n) detectLanguage(r *http.Request) string {
	if r == nil {
		return i.defaultLanguage
	}

	// Check query param ?lang=pt
	if lang := r.URL.Query().Get("lang"); lang != "" {
		if _, ok := i.translations[lang]; ok {
			return lang
		}
	}

	// Parse Accept-Language header
	accept := r.Header.Get("Accept-Language")
	if accept == "" {
		return i.defaultLanguage
	}

	// Simple parser: "pt-BR,pt;q=0.9,en;q=0.8" -> try "pt-BR", "pt", "en"
	for _, part := range strings.Split(accept, ",") {
		lang := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])

		// Try exact match
		if _, ok := i.translations[lang]; ok {
			return lang
		}

		// Try base language (pt-BR -> pt)
		if idx := strings.Index(lang, "-"); idx > 0 {
			base := lang[:idx]
			if _, ok := i.translations[base]; ok {
				return base
			}
		}
	}

	return i.defaultLanguage
}

// TranslateAll replaces {t.key} patterns in text with translations
func (i *I18n) TranslateAll(text string, r *http.Request) string {
	result := text
	for _, langEntries := range i.translations {
		for key := range langEntries {
			placeholder := "{t." + key + "}"
			if strings.Contains(result, placeholder) {
				result = strings.ReplaceAll(result, placeholder, i.Translate(key, r))
			}
		}
	}
	return result
}
