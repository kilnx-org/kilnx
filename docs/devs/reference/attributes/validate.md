# `validate`

> Validate input against a model or per-field rules.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
validate [<model>]
```

## Description

Bare form `validate <model>` runs all model-level validators. Block form lists per-field rules: `<field>: <rule>, <rule>`. Validation errors populate template-accessible `errors` and short-circuit subsequent body nodes.

## Used in

- [`page`](../keywords/page.md)
- [`action`](../keywords/action.md)
- [`fragment`](../keywords/fragment.md)
- [`api`](../keywords/api.md)
- [`schedule`](../keywords/schedule.md)
- [`job`](../keywords/job.md)
- [`webhook`](../keywords/webhook.md)

## Examples

### Validate against a model

```kilnx
action /users/create method POST
  validate user
```

### Custom rules block

```kilnx
action /signup method POST
  validate
    name: required
    email: required, is email
    age: min 18, max 120
```

## Provenance

| | |
|---|---|
| **Spec last touched** | `66f909b` (2026-05-13) |
| **Source last touched** | `87ebbf6` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

