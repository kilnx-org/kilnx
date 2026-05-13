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

| | |
|---|---|
| **Spec last touched** | `7df9033` (2026-05-13) |
| **Source last touched** | `69981b8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

