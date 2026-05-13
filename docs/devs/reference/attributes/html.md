# `html`

> Render an inline HTML template block.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
html
```

## Description

The `html` body emits the indented template literal. Bindings use `{name}` or `{model.field}` syntax; control flow uses `{#if ...}`/`{/if}` and `{#each ...}`/`{/each}`. Inside a `layout`, `html` defines the wrapper template.

## Used in

- [`page`](../keywords/page.md)
- [`action`](../keywords/action.md)
- [`fragment`](../keywords/fragment.md)
- [`api`](../keywords/api.md)
- [`schedule`](../keywords/schedule.md)
- [`job`](../keywords/job.md)
- [`webhook`](../keywords/webhook.md)
- [`layout`](../keywords/layout.md)

## Examples

### Inline template

```kilnx
page /
  html
    <h1>Welcome, {current_user.name}!</h1>
```

## See also

- [`fragment`](../keywords/fragment.md)
- [`layout`](../keywords/layout.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `7df9033` (2026-05-13) |
| **Source last touched** | `69981b8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

