# `test`

> End-to-end browser-style test scenario.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
test "<name>"
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `name` | `string` | yes |

## Description

A `test` block declares a sequence of steps that drive the application like a user: visiting URLs, filling form fields, submitting, and asserting the result. Tests run via `kilnx test <file.kilnx>` and fail the run if any `expect` step does not hold.

## Children

- [`visit`](../attributes/visit.md)
- [`fill`](../attributes/fill.md)
- [`submit`](../attributes/submit.md)
- [`expect`](../attributes/expect.md)
- [`as`](../attributes/as.md)

## Examples

### Create a user via the form

```kilnx
test "Create user"
  visit /users/new
  fill name "John"
  submit
  expect page /users contains "John"
```

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `69981b8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

