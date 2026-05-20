# `upload-extensions`

> Extend or replace the upload extension whitelist.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.5` |

## Syntax

```
upload-extensions: +<ext>, -<ext>, *
```

## Description

Comma-separated tokens. Each token is one of:

- `+<ext>`: add `<ext>` to the default whitelist.
- `-<ext>`: remove `<ext>` from the default whitelist.
- `*`: allow any extension (analyzer warning). Cannot mix with `+`/`-`.

Effective whitelist = `default ∪ Add − Remove`. Bare tokens (e.g. `pdf`) are rejected. `/_uploads/` is served as a static filesystem, so any uploaded file becomes a public URL: enabling `*` disables MIME-based safety.

## Used in

- [`config`](../keywords/config.md)

## Examples

### Add CAD formats, drop legacy doc

```kilnx
config
  uploads: ./uploads max 50
  upload-extensions: +dxf, +dwg, +step, +stp, -doc
```

## Provenance

| | |
|---|---|
| **Spec last touched** | `72e9177` (2026-05-13) |

