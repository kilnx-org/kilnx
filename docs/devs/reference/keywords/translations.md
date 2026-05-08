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

