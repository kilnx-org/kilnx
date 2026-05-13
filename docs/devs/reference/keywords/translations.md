# `translations`

> i18n translation strings keyed by language and key.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
translations
```

## Description

The `translations` block contains nested language sub-blocks; each language block is a list of `key: "value"` entries. Templates reference keys via `{t 'key'}`. Multiple `translations` blocks merge by language. The active language is selected per-request based on the `detect language` strategy in `config`.

## Examples

### English and Portuguese

```kilnx
translations
  en
    welcome: "Welcome back"
    users: "Users"
  pt
    welcome: "Bem-vindo de volta"
    users: "Usuarios"
```

## See also

- [`config`](config.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `87ebbf6` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

