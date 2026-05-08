# `identity`

> Field used as the unique login identifier.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |
| **Required** | yes |

## Syntax

```
identity: <field>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `field` | `identifier` | yes |

## Description

Usually `email` or `username`. Must exist on the auth table and be unique.

## Used in

- [`auth`](../keywords/auth.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

