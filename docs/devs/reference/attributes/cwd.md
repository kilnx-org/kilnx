# `cwd`

> Working directory for the agent subprocess.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.3` |

## Syntax

```
cwd: <path>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `path` | `path` | yes |

## Description

Resolved relative to `config workspace-root`. Must stay within `workspace-root` after symlink resolution. `:param` substitution is supported (e.g. `/workspaces/:tenant_id/:user_id`). If omitted, a temporary directory is created per request and removed on exit.

## Used in

- [`agent`](../keywords/agent.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-21`, but this entity's spec was last edited on `2026-05-20`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `36bcf1b` (2026-05-20) |
| **Source last touched** | `9276716` (2026-05-21) |
| **Source files** | `internal/parser/parser.go` |

