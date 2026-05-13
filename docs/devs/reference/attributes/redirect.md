# `redirect`

> Redirect to another URL.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
redirect <path>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `path` | `path` | yes |

## Description

Issues an HTTP redirect from inside a page or action body. Used in flows that change state and then send the user elsewhere.

## Used in

- [`page`](../keywords/page.md)
- [`action`](../keywords/action.md)

## Examples

### Redirect after successful action

```kilnx
action /logout method POST
  redirect /entrar
```

## See also

- [`action`](../keywords/action.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `87ebbf6` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

