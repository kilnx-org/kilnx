# `send`

> Send an email.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
send email to <target>
```

## Description

Address with `to <expression>` (column, parameter, or string). Body sub-fields: `subject:`, `template:` (named layout), `body:` (inline text), `attach:` (file). Email transport is configured at runtime via env vars.

## Used in

- [`page`](../keywords/page.md)
- [`action`](../keywords/action.md)
- [`fragment`](../keywords/fragment.md)
- [`api`](../keywords/api.md)
- [`schedule`](../keywords/schedule.md)
- [`job`](../keywords/job.md)
- [`webhook`](../keywords/webhook.md)

## Examples

### Welcome email

```kilnx
action /signup method POST
  send email to :email
    subject: "Welcome to our app"
    template: welcome-email
```

## Provenance

| | |
|---|---|
| **Spec last touched** | `56b81a7` (2026-05-13) |
| **Source last touched** | `2a440f8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

