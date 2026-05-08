# `auto`

> Auto-populate the field on insert.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
<field>: <type> auto
```

## Description

Behavior depends on field type: `uuid` generates a v4 UUID, `timestamp` sets the current time, integer ID fields auto-increment.

## Used in

- [`model`](../keywords/model.md)

