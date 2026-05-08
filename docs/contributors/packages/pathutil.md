# `internal/pathutil`

> Package pathutil provides helpers for matching declarative route templates (e.g. "/tasks/:id/delete") against concrete request paths (e.g. "/tasks/123/delete").

| | |
|---|---|
| **Import path** | `github.com/kilnx-org/kilnx/internal/pathutil` |
| **Source last touched** | `cadd7a0` (2026-05-05) |


## Files

| File | Summary |
|------|---------|
| [`match.go`](../../../internal/pathutil/match.go) | _no file-level doc_ |

## Functions

### `Match`

```go
func Match(template, path string) bool
```

Match reports whether template (a route pattern with optional :param segments)
matches path (a concrete URL path). Both must have the same number of segments.
Segments starting with ':' in template act as wildcards.


## Notes

<!-- MANUAL-NOTES START -->
# `internal/pathutil`

Tiny helper for matching declarative route templates against concrete request paths. One file, one function.

## Purpose

Kilnx routes are written as templates like `/tasks/:id/delete`. At request time the runtime needs to know whether a concrete path like `/tasks/123/delete` matches a declared template, and (separately) extract the wildcard values. `pathutil.Match` answers the boolean half of that question.

## File map

- [`match.go`](../../../internal/pathutil/match.go): the entire package. Defines [`Match(template, path string) bool`](../../../internal/pathutil/match.go).

## Public surface

```go
func Match(template, path string) bool
```

That is the whole API.

## Semantics

- Both inputs are split on `/`. Empty segments are kept (so leading/trailing slashes count).
- The two splits must produce the **same number of segments**. A template with N parts never matches a path with N+1 parts, even if the extra segment is empty.
- A template segment that begins with `:` is a wildcard. It accepts any value, including empty string.
- Every other template segment must match the corresponding path segment by exact byte equality. No case folding, no normalization, no escape handling.

## Examples

| Template | Path | Result |
|---|---|---|
| `/tasks/:id` | `/tasks/123` | true |
| `/tasks/:id` | `/tasks/123/edit` | false (segment count differs) |
| `/tasks/:id` | `/tasks/` | true (`:id` matches empty) |
| `/tasks/new` | `/tasks/New` | false (case sensitive) |
| `/a/:b/:c` | `/a/x/y` | true |

## Gotchas

- **Equal-segment requirement**: the function never treats `:id` as "match the rest". There is no greedy or catch-all wildcard. If you need optional trailing segments, declare separate routes.
- **Empty segments match wildcards**: `/tasks/:id` matches `/tasks/`. The runtime relies on this for some default-value behaviors. Validate emptiness at the handler if you need a non-empty id.
- **No parameter extraction here**: `Match` only returns a boolean. The runtime extracts the actual `:param` values via `matchPathParams` in [`internal/runtime/server.go`](../../../internal/runtime/server.go), which uses the same splitting rule. If you change `Match`'s splitting strategy, change both.
- **Case sensitivity**: paths are compared byte-for-byte. URL paths arriving from `net/http` are not normalized by Kilnx before this check.

## When to touch this file

Almost never. The function is small, well covered by [`match_test.go`](../../../internal/pathutil/match_test.go), and used from the hot path of every request. Behavior changes here ripple through the entire runtime router.
<!-- MANUAL-NOTES END -->
