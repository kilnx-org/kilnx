# `config`

> Application-wide configuration.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |

## Syntax

```
config
```

## Description

The `config` block sets values that the runtime reads at startup: app name, database URL, listen port, secret key, static asset roots, upload limits, default language, and CORS origins. Most values support `env <VAR> default <fallback>` syntax to read from environment variables.

## Children

- [`name`](../attributes/name.md)
- [`database`](../attributes/database.md)
- [`port`](../attributes/port.md)
- [`secret`](../attributes/secret.md)
- [`static`](../attributes/static.md)
- [`uploads`](../attributes/uploads.md)
- [`default_language`](../attributes/default_language.md)
- [`detect_language`](../attributes/detect_language.md)
- [`cors`](../attributes/cors.md)
- [`workspace-root`](../attributes/workspace-root.md)

## Examples

### Production-ready config

```kilnx
config
  name: "My App"
  database: env DATABASE_URL default "sqlite://app.db"
  port: env PORT default 8080
  secret: env SECRET_KEY required
  static: ./public
  uploads: ./uploads max 10
```

## See also

- [`translations`](translations.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `56b81a7` (2026-05-13) |
| **Source last touched** | `2a440f8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

