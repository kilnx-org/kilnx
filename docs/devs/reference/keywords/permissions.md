# `permissions`

> Role-based access control rules.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
permissions
```

## Description

The `permissions` block declares per-role rules that drive authorization checks. Rules are written in a small DSL: `all`, `read <resource>`, `write <resource>`, optionally with `where <expression>` clauses that reference `current_user` or row attributes. Routes opt in by writing `requires <role>`.

## Examples

### Three-role hierarchy

```kilnx
permissions
  admin: all
  editor: read post, write post where author = current_user
  viewer: read post where status = published
```

## See also

- [`auth`](auth.md)
- [`requires`](../attributes/requires.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `69981b8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

