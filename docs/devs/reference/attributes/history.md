# `history`

> SQL query whose rows become the message history.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.3` |

## Syntax

```
history: <SQL>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `sql` | `string` | yes |

## Description

Each row must expose `papel` (`user`, `assistente`, or `sistema`) and `conteudo` columns.

## Used in

- [`response`](../keywords/response.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-21`, but this entity's spec was last edited on `2026-05-20`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `36bcf1b` (2026-05-20) |
| **Source last touched** | `9276716` (2026-05-21) |
| **Source files** | `internal/parser/parser.go` |

