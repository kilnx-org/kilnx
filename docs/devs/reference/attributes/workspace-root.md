# `workspace-root`

> Filesystem root for agent cwd resolution and tmp dirs.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.3` |

## Syntax

```
workspace-root: <path>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `path` | `path` | yes |

## Description

Required when any llm agent block declares cwd. All resolved cwd paths must stay inside this prefix after symlink eval.

## Used in

- [`config`](../keywords/config.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `56b81a7` (2026-05-13) |

