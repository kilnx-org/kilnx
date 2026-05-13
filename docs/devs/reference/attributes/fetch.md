# `fetch`

> Make an HTTP request to an external API.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
fetch [<name>:] <METHOD> <URL>
```

## Description

Sub-keywords: `header <name>: <value>`, `body <key>: <value>`. The named result is bound for use by subsequent body nodes.

## Used in

- [`page`](../keywords/page.md)
- [`action`](../keywords/action.md)
- [`fragment`](../keywords/fragment.md)
- [`api`](../keywords/api.md)
- [`schedule`](../keywords/schedule.md)
- [`job`](../keywords/job.md)
- [`webhook`](../keywords/webhook.md)

## Examples

### Call a weather API

```kilnx
page /weather
  fetch weather: GET https://api.weather.com/forecast?city=:city
    header Authorization: env API_KEY
  html
    <p>{weather.summary}</p>
```

## See also

- [`llm`](../keywords/llm.md)
- [`webhook`](../keywords/webhook.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `7df9033` (2026-05-13) |
| **Source last touched** | `69981b8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

