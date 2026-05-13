# `max-budget-usd`

> Hard cost cap in USD per agent invocation.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.3` |
| **Required** | yes |

## Syntax

```
max-budget-usd: <float>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `value` | `float` | yes |

## Description

Required by the analyzer. The subprocess is killed once total token cost crosses this threshold.

## Used in

- [`agent`](../keywords/agent.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `7df9033` (2026-05-13) |

