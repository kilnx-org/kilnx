# `internal/optimizer`

> Package optimizer rewrites the parser AST so the runtime can execute it efficiently.

| | |
|---|---|
| **Import path** | `github.com/kilnx-org/kilnx/internal/optimizer` |
| **Source last touched** | `5da8498` (2026-05-08) |
| **Doc last touched** | `d2d0978` (2026-05-08) |


## Overview

Named-query reference resolution ({queryName.field}, {^field},
{^^field}, bare {field}) is performed earlier, by parser.Parse, before
the optimizer runs. This package consumes those interpolations to
drive its own rewrites.

Current responsibilities:

  - SELECT * rewriting: when consumer interpolations reveal exactly
    which columns a page/fragment/api uses, rewrite "SELECT *" to an
    explicit projection (optimizePage in optimizer.go).
  - JOIN pruning: drop joins whose tables are not referenced by any
    interpolation or by remaining SQL.
  - Query deduplication: merge identical queries within a single
    page/fragment/api body (deduplicateQueries).
  - Stream candidacy: tag scheduled tasks that can be served as SSE
    streams without extra work (markStreamCandidates).

The optimizer is purely AST-to-AST and never returns errors: callers
invoke Optimize(app) after parsing/analysis and use the same *App.

## Files

| File | Summary |
|------|---------|
| [`optimizer.go`](../../../internal/optimizer/optimizer.go) | _no file-level doc_ |

## Types

### `eachBlock`

```go
type eachBlock struct {
	start		int
	end		int
	queryName	string
}
```

eachBlock represents a single {{each queryName}}...{{end}} span.

### `fieldSet`

```go
type fieldSet struct {
	fields	[]string
	seen	map[string]bool
}
```

fieldSet is an ordered set of field names.

### `queryEntry`

```go
type queryEntry struct {
	name	string
	sql	string
	index	int	// index in page.Body (-1 if nested)
}
```
## Functions

### `Optimize`

```go
func Optimize(app *parser.App)
```

Optimize performs domain-aware query optimization on a parsed Kilnx app.

### `addInterpolatedFields`

```go
func addInterpolatedFields(result map[string]*fieldSet, text string)
```

addInterpolatedFields extracts {queryName.field} and {^field} references from text
and adds the fields to the appropriate query's field set.

### `collectNamedQueries`

```go
func collectNamedQueries(nodes []parser.Node, entries *[]queryEntry, baseIndex int)
```
### `collectUsedFields`

```go
func collectUsedFields(nodes []parser.Node) map[string]*fieldSet
```

collectUsedFields walks all nodes in a page body and collects which fields
each query name needs. Returns nil for a query if field usage can't be
fully determined (e.g. table with no explicit columns).

### `countEachEnclosing`

```go
func countEachEnclosing(pos int, blocks []eachBlock) []string
```

countEachEnclosing returns the names of {{each}} blocks that enclose the given position.

### `countUnnamedQueries`

```go
func countUnnamedQueries(nodes []parser.Node) int
```
### `deduplicateQueries`

```go
func deduplicateQueries(page *parser.Page)
```
### `extractPathParams`

```go
func extractPathParams(path string) []string
```

extractPathParams pulls :param names from a URL path like /users/:id/edit.

### `findEachBlocks`

```go
func findEachBlocks(text string) []eachBlock
```

findEachBlocks returns all {{each}} blocks in the given text, including nested ones.

### `findJoinClauseEnd`

```go
func findJoinClauseEnd(sql string, start int) int
```
### `findMatchingEnd`

```go
func findMatchingEnd(content string) (body, elseBody string, endPos int)
```

findMatchingEnd finds the body, else body, and position after {{end}} for a block,
accounting for nested {{each}}/{{if}} blocks.

### `hasKey`

```go
func hasKey(m map[string]*fieldSet, key string) bool
```
### `markStreamCandidates`

```go
func markStreamCandidates(app *parser.App)
```

markStreamCandidates marks stream queries with aggregate functions as
materialization candidates by prepending a hint comment.

### `newFieldSet`

```go
func newFieldSet() *fieldSet
```
### `normalizeSQL`

```go
func normalizeSQL(sql string) string
```

deduplicateQueries finds named queries with identical SQL on the same page
and removes duplicates, rewriting consumer references to point to the original.
normalizeSQL returns a canonical form of a SQL string for deduplication.
It trims whitespace, collapses multiple spaces, and uppercases keywords.

### `optimizePage`

```go
func optimizePage(page *parser.Page)
```
### `pruneUnusedJoins`

```go
func pruneUnusedJoins(sql string, fields *fieldSet) string
```

pruneUnusedJoins removes JOIN clauses when no columns from the joined table
are consumed by any component. Skips if any consumed field uses plain names
(no dot notation) since we can't determine which table owns the column.

### `renameConsumerRefs`

```go
func renameConsumerRefs(nodes []parser.Node, oldName, newName string)
```
### `rewriteSelectStar`

```go
func rewriteSelectStar(sql string, fields *fieldSet) string
```

rewriteSelectStar replaces SELECT * FROM with SELECT col1, col2, ... FROM
only if the SQL starts with SELECT [DISTINCT] * FROM.

### `(fieldSet) add`

```go
func (fs *fieldSet) add(field string)
```
### `(fieldSet) sorted`

```go
func (fs *fieldSet) sorted() []string
```

## Notes

<!-- MANUAL-NOTES START -->
# Package `internal/optimizer`

Source: [optimizer.go](../../../internal/optimizer/optimizer.go), [doc.go](../../../internal/optimizer/doc.go).

## Purpose

Rewrite the parser AST so the runtime can execute it efficiently. Current passes:

- Replace `SELECT *` with the explicit column set actually consumed by the page or fragment, derived from `{queryName.field}` and `{^field}` interpolations in HTML and text bodies.
- Prune `JOIN` clauses whose joined alias is never referenced by any consumer.
- Deduplicate named queries on the same page that share identical SQL, rewriting consumers to point at the surviving query.
- Mark `stream` queries that contain aggregate functions with a `kilnx:materialize-candidate` hint comment so the runtime can opt them into materialised storage.

## Pipeline position

```
parser.App (named queries already resolved)
  -> analyzer.Analyze (errors gate this stage)
  -> optimizer.Optimize  (mutates AST in place)
  -> runtime
```

The optimizer is purely AST to AST. It assumes the parser has already substituted bare named query references and the analyzer has populated `Node.SourceModel`. It does not connect to the database.

## Public API

```go
func Optimize(app *parser.App)
```

`Optimize` mutates `app` in place. There is no return value: pages, fragments, APIs, and streams are rewritten directly. Calling it twice is safe; the stream materialise hint check skips queries already prefixed with the marker comment.

For each page, fragment, and API the optimizer:

1. Walks the body to build a `map[queryName]*fieldSet` of fields each query is observed to need.
2. Counts unnamed queries to avoid the unsafe `_last` rewrite when more than one is present.
3. Rewrites `SELECT * FROM ...` and `SELECT DISTINCT * FROM ...` for each named query whose consumed field set is fully known.
4. Prunes JOINs whose alias is not referenced by any qualified field.
5. Deduplicates identical SQL across named queries on the same page.

After the page level passes, `markStreamCandidates` walks `app.Streams` and prepends `/* kilnx:materialize-candidate */` to any SQL that matches `(?i)\b(COUNT|SUM|AVG|MIN|MAX)\s*\(` and has a positive `IntervalSecs`.

## File map

- [`optimizer.go`](../../../internal/optimizer/optimizer.go): `Optimize`, field collection, `SELECT *` rewriting, JOIN pruning, named query deduplication, stream materialisation hints, all helpers (`fieldSet`, `findEachBlocks`, `countEachEnclosing`, `normalizeSQL`, `renameConsumerRefs`, `findJoinClauseEnd`).
- [`doc.go`](../../../internal/optimizer/doc.go): package level documentation.
- [`optimizer_test.go`](../../../internal/optimizer/optimizer_test.go): unit tests for each pass.

## Named query reference syntax

The interpolation regex is `\{(\^*[a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)*)\}`. The optimizer recognises three patterns inside `NodeText` and `NodeHTML` content:

**`{queryName.field}`**: a qualified reference. Records that `field` is consumed from the result set named `queryName`. The collector accepts arbitrary identifiers on the left, including dot chains; only the first segment is treated as the query name. The literal `.count` is ignored because it is a built in aggregate, not a real column.

**`{^field}`**: a parent scope reference, valid only inside an enclosing `{{each queryName}}...{{end}}` block. The optimizer counts the leading `^` characters: depth `1` (`{^field}`) refers to the immediately enclosing `each`, depth `2` (`{^^field}`) refers to its parent, and so on. `findEachBlocks` builds the spanning list of `each` enclosures for the current position; the resolved query name is `eachNames[len(eachNames)-depth-1]`. If `depth` exceeds the available enclosing scopes the reference is ignored.

**`{field}`**: a bare reference with no dot, used inside a single `{{each}}` to access the current row's column directly. The optimizer cannot attribute it to a specific query, so it does not contribute to field collection. The runtime resolves it from the loop's row.

This is consumed only for consumer to query field tracking. The literal `SELECT *` text in source SQL is what gets rewritten; the interpolation patterns themselves are left intact in the template, where the runtime resolves them at render time.

## Key behaviors and gotchas

**Named query *substitution* is not here.** The parser is responsible for rewriting `query: someName` body nodes to the SQL stored in `app.NamedQueries`. The optimizer assumes this has already run. See [parser.md](./parser.md).

**`SELECT *` rewriting is conservative.** It runs only when the consumed field set is fully known. If any consumer uses a bare `{field}` reference (no dot, no `^`), the field set is left nil for that query and the rewrite is skipped. The same applies when more than one unnamed query appears on the same page: they all share the implicit `_last` slot, so the optimizer cannot tell whose columns are being referenced.

**JOIN pruning is conservative too.** `pruneUnusedJoins` aborts as soon as it finds a single unqualified field in the consumed set, because it cannot decide which joined table owns the column. When every consumed field uses an `alias.column` form, JOINs whose alias is unused are removed by walking `joinRe` matches in reverse and snipping out the `JOIN ... ON ...` span up to the next clause boundary.

**Deduplication compares normalised SQL.** `normalizeSQL` trims and collapses whitespace; it does not touch case or comments. Two queries with identical text but different leading or trailing whitespace are treated as duplicates. The duplicate's `SQL` field is cleared (`""`) and `renameConsumerRefs` rewrites `{old.` and `{{each old}}` references in sibling text and HTML nodes to point at the surviving name.

**Stream materialisation is a hint only.** Prepending the comment does not rewrite the query; it tags it for the runtime to consider materialising. Already tagged queries are skipped on re-runs of `Optimize`.

**`NodeOn` recursion.** Field collection descends into `NodeOn.Children` so branching bodies contribute to the field set. The deduplication pass also walks `NodeOn` children but flags nested entries with `index = -1` so they are not eligible for SQL clearing (only the top level node owns its slot in `page.Body`).

**No errors from this pass.** All branches degrade to "no rewrite" rather than failing. Any source level problems should be caught earlier by the analyzer.

**Field set ordering is insertion order.** `fieldSet.sorted()` returns fields in the order they were first observed, not alphabetically. Output is deterministic for a given input but may surprise authors expecting sorted column lists.

**`{{each}}` block discovery is recursive.** `findEachBlocks` walks nested `{{each ...}}{{end}}` and `{{if ...}}{{end}}` pairs by maintaining a `depth` counter inside `findMatchingEnd`. Misnested blocks (missing `{{end}}`, mismatched depth) cause the discovery to bail out, after which any `{^field}` references inside the broken span fall through without contributing to the field set.

**The optimizer never queries the database.** Materialisation candidate marking is purely textual; the runtime decides whether the hint is worth honouring based on its own caching policy.

**Page deduplication is intra page only.** Two pages with identical SQL each retain their own copy. Cross page deduplication would require a runtime cache rather than an AST pass.

**`SELECT DISTINCT *` is also rewritten.** `selectStarRe` accepts `SELECT *` and `SELECT DISTINCT *`. Other forms (`SELECT t.*`, `SELECT a, *`) are left untouched.

**Leading whitespace is preserved.** `rewriteSelectStar` captures the leading whitespace of the original SQL string and prefixes it back onto the rewritten query so error messages and logs that index into the SQL string keep the same column positions.

**`renameConsumerRefs` is text level.** It does string replace on `{old.` and `{{each old}}` patterns inside `NodeText` and `NodeHTML`. Field expressions buried inside attribute values, JSON literals, or multiline strings are still rewritten because the substitution is unconditional. This is intentional: every interpolation site that the parser captured into one of these node types should be redirected.

**JOIN clause boundaries are keyword based.** `findJoinClauseEnd` ends a JOIN span at the next of `JOIN`, `LEFT`, `RIGHT`, `INNER`, `OUTER`, `CROSS`, `FULL`, `WHERE`, `ORDER`, `GROUP`, `HAVING`, `LIMIT`, `UNION`. Subqueries inside `ON` clauses are tolerated via depth counting on parentheses.

**Aggregate detection is keyword based.** `aggregateRe` matches `COUNT(`, `SUM(`, `AVG(`, `MIN(`, `MAX(` case insensitively. A custom aggregate or an aliased aggregate like `cnt(...)` will not be marked. This is conservative: false negatives are preferable to applying materialisation to queries the runtime cannot safely cache.

**The `_last` slot is fragile by design.** When a body has exactly one unnamed `query`, the optimizer treats it as having the implicit name `_last`. When two or more unnamed queries coexist, the optimizer aborts the rewrite for them: it cannot prove which query each interpolation refers to. Authors who want `SELECT *` rewriting should give their queries names.

**Idempotence.** Running `Optimize` twice on the same `App` is safe. Already rewritten `SELECT` clauses no longer match `selectStarRe`; deduplicated queries already have empty SQL and are skipped; the materialisation comment short circuits its own re-application.

## Testing entry points

- Per pass tests covering `SELECT *` rewriting, JOIN pruning, deduplication, and stream marking: [`optimizer_test.go`](../../../internal/optimizer/optimizer_test.go).
<!-- MANUAL-NOTES END -->
