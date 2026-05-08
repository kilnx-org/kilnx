package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "config",
		Kind:    spec.KindKeyword,
		Summary: "Application-wide configuration.",
		Description: "The `config` block sets values that the runtime reads at startup: " +
			"app name, database URL, listen port, secret key, static asset roots, upload " +
			"limits, default language, and CORS origins. Most values support `env <VAR> " +
			"default <fallback>` syntax to read from environment variables.",
		Syntax: "config",
		Children: []string{
			"name", "database", "port", "secret",
			"static", "uploads",
			"default_language", "detect_language", "cors",
		},
		Since: "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Production-ready config",
				Code: `config
  name: "My App"
  database: env DATABASE_URL default "sqlite://app.db"
  port: env PORT default 8080
  secret: env SECRET_KEY required
  static: ./public
  uploads: ./uploads max 10`,
			},
		},
	})

	for _, e := range []spec.Entity{
		{Name: "name", Summary: "Human-readable application name.", Syntax: `name: "<text>"`},
		{Name: "database", Summary: "Database connection URL.", Syntax: "database: env <VAR> default <url>"},
		{Name: "port", Summary: "TCP port to listen on.", Syntax: "port: env <VAR> default <n>", Default: "8080"},
		{Name: "secret", Summary: "Secret key for session cookies and CSRF tokens.", Syntax: "secret: env <VAR> required"},
		{Name: "static", Summary: "Filesystem path served as static assets.", Syntax: "static: <path>"},
		{Name: "uploads", Summary: "Filesystem path and max size (MB) for user uploads.", Syntax: "uploads: <path> max <mb>"},
		{Name: "default_language", Summary: "Fallback language code for translations.", Syntax: "default language: <code>"},
		{Name: "detect_language", Summary: "How to detect the user's language at request time.", Syntax: "detect language: <strategy>"},
		{Name: "cors", Summary: "Comma-separated list of allowed CORS origins.", Syntax: "cors: <origin>, ..."},
	} {
		e.Kind = spec.KindAttribute
		e.ParentScope = []string{"config"}
		e.Since = "0.1.0"
		spec.Register(e)
	}
}
