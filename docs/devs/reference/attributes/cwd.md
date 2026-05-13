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

| | |
|---|---|
| **Spec last touched** | `7df9033` (2026-05-13) |
| **Source last touched** | `69981b8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

