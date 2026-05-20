# `serve-roots`

> Directories from which `respond file` may serve.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.5` |

## Syntax

```
serve-roots: <path>, <path>, ...
```

## Description

Required when any action uses [`respond file`](../attributes/respond.md). Comma-separated absolute or app-relative paths. The runtime rejects any `respond file` whose resolved path is not inside one of these roots (HTTP 403). The analyzer raises an error if `respond file` appears without `serve-roots` (fail-safe).

## Used in

- [`config`](../keywords/config.md)

## Examples

### Allow uploads and generated reports

```kilnx
config
  uploads: ./uploads max 50
  serve-roots: ./uploads, ./reports, /tmp/bundles
```

## Provenance

| | |
|---|---|
| **Spec last touched** | `72e9177` (2026-05-13) |

