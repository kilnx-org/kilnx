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
			"static", "uploads", "upload-extensions",
			"serve-roots",
			"default_language", "detect_language", "cors",
			"workspace-root",
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
		{Name: "uploads", Summary: "Filesystem path and max size (MB) for user uploads. Form bindings serialise as JSON array of `/_uploads/...` paths (use `json_each`).", Syntax: "uploads: <path> max <mb>"},
		{Name: "default_language", Summary: "Fallback language code for translations.", Syntax: "default language: <code>"},
		{Name: "detect_language", Summary: "How to detect the user's language at request time.", Syntax: "detect language: <strategy>"},
		{Name: "cors", Summary: "Comma-separated list of allowed CORS origins.", Syntax: "cors: <origin>, ..."},
	} {
		e.Kind = spec.KindAttribute
		e.ParentScope = []string{"config"}
		e.Since = "0.1.0"
		spec.Register(e)
	}

	spec.Register(spec.Entity{
		Name:        "workspace-root",
		Kind:        spec.KindAttribute,
		Summary:     "Filesystem root for agent cwd resolution and tmp dirs.",
		Description: "Required when any llm agent block declares cwd. All resolved cwd paths must stay inside this prefix after symlink eval.",
		Syntax:      "workspace-root: <path>",
		Args:        []spec.Arg{{Name: "path", Type: "path", Required: true}},
		ParentScope: []string{"config"},
		Since:       "0.1.3",
	})

	spec.Register(spec.Entity{
		Name:    "upload-extensions",
		Kind:    spec.KindAttribute,
		Summary: "Extend or replace the upload extension whitelist.",
		Description: "Comma-separated tokens. Each token is one of:\n\n" +
			"- `+<ext>`: add `<ext>` to the default whitelist.\n" +
			"- `-<ext>`: remove `<ext>` from the default whitelist.\n" +
			"- `*`: allow any extension (analyzer warning). Cannot mix with `+`/`-`.\n\n" +
			"Effective whitelist = `default ∪ Add − Remove`. Bare tokens (e.g. `pdf`) are rejected. `/_uploads/` is served as a static filesystem, so any uploaded file becomes a public URL: enabling `*` disables MIME-based safety.",
		Syntax:      "upload-extensions: +<ext>, -<ext>, *",
		ParentScope: []string{"config"},
		Since:       "0.1.5",
		Examples: []spec.Example{{
			Title: "Add CAD formats, drop legacy doc",
			Code: `config
  uploads: ./uploads max 50
  upload-extensions: +dxf, +dwg, +step, +stp, -doc`,
		}},
	})

	spec.Register(spec.Entity{
		Name:        "serve-roots",
		Kind:        spec.KindAttribute,
		Summary:     "Directories from which `respond file` may serve.",
		Description: "Required when any action uses [`respond file`](../attributes/respond.md). Comma-separated absolute or app-relative paths. The runtime rejects any `respond file` whose resolved path is not inside one of these roots (HTTP 403). The analyzer raises an error if `respond file` appears without `serve-roots` (fail-safe).",
		Syntax:      "serve-roots: <path>, <path>, ...",
		ParentScope: []string{"config"},
		Since:       "0.1.5",
		Examples: []spec.Example{{
			Title: "Allow uploads and generated reports",
			Code: `config
  uploads: ./uploads max 50
  serve-roots: ./uploads, ./reports, /tmp/bundles`,
		}},
	})
}
