# `cwd`

> Working directory for the agent subprocess.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.2.0` |

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
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

