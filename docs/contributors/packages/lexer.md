# `internal/lexer`

> Package lexer transforms .kilnx source text into a token stream consumed by internal/parser.

| | |
|---|---|
| **Import path** | `github.com/kilnx-org/kilnx/internal/lexer` |
| **Source last touched** | `72e9177` (2026-05-13) |
| **Doc last touched** | `5da8498` (2026-05-08) |


> **Implementation touched after doc.go.** Source changed on `2026-05-13`, but `doc.go` was last edited on `2026-05-08`. The summary above may be out of date.

## Overview

The lexer is indent-significant: leading whitespace at the start of a
logical line determines block structure, and INDENT/DEDENT tokens are
emitted when the indentation level changes (modeled after Python's
tokenize module). Inline comments starting with '#' are stripped
before tokenization.

Top-level entry point is [Tokenize]. Helpers IsFieldType and
IsFieldConstraint expose the lexer's keyword tables to the parser
and analyzer.

## Files

| File | Summary |
|------|---------|
| [`lexer.go`](../../../internal/lexer/lexer.go) | _no file-level doc_ |

## Types

### `Token`

```go
type Token struct {
	Type	TokenType
	Value	string
	Line	int
	Column	int
}
```

Token is a single lexical unit emitted by [Tokenize]. Line and Column
are 1-based positions in the original source.

### `TokenType`

```go
type TokenType int
```

TokenType identifies the lexical category of a Token (keyword, string,
indent, etc.). Values are defined as the Token* constants below.

## Functions

### `FieldConstraints`

```go
func FieldConstraints() []string
```

FieldConstraints returns the sorted list of recognized field constraint keywords.

### `FieldTypes`

```go
func FieldTypes() []string
```

FieldTypes returns the sorted list of recognized field type keywords.

### `IsFieldConstraint`

```go
func IsFieldConstraint(s string) bool
```

IsFieldConstraint reports whether s is a recognized field constraint
keyword (e.g. required, unique, default, min, max).

### `IsFieldType`

```go
func IsFieldType(s string) bool
```

IsFieldType reports whether s is a recognized model field type keyword
(e.g. text, int, timestamp, reference).

### `StripComments`

```go
func StripComments(source string) string
```

StripComments removes # comments from source code.
Full-line comments (lines starting with #) are replaced with blank lines to preserve line numbers.
Inline comments (# after code) are stripped, respecting quoted strings.
Lines inside html blocks are never stripped (they may contain CSS hex colors like #fff).

### `Tokenize`

```go
func Tokenize(source string) []Token
```

Tokenize transforms .kilnx source text into a token stream. Leading
whitespace drives INDENT/DEDENT tokens, and a trailing TokenEOF is
always appended. Run [StripComments] on the source first to remove
'#' comments.

### `countIndent`

```go
func countIndent(line string) int
```
### `tokenizeLine`

```go
func tokenizeLine(line string, lineNum int) []Token
```

## Notes

<!-- MANUAL-NOTES START -->
# Package `internal/lexer`

Source: [lexer.go](../../../internal/lexer/lexer.go), [doc.go](../../../internal/lexer/doc.go).

## Purpose

Transform `.kilnx` source text into a flat token stream consumed by [`internal/parser`](../../../internal/parser). The lexer is indent significant: leading whitespace at the start of each logical line drives `INDENT` and `DEDENT` tokens, modeled after Python's `tokenize` module. Inline `#` comments are stripped before tokenization.

## Pipeline position

```
source bytes -> StripComments -> Tokenize -> []Token -> parser.Parse
```

1. CLI reads `app.kilnx` from disk.
2. [`StripComments`](../../../internal/lexer/lexer.go) erases `#` comments while keeping line numbers intact.
3. [`Tokenize`](../../../internal/lexer/lexer.go) emits the token stream.
4. The parser consumes tokens; it never sees raw indentation or comments.

## Public API

```go
type TokenType int

const (
    TokenKeyword TokenType = iota
    TokenPath
    TokenString
    TokenIdentifier
    TokenNumber
    TokenColon
    TokenComma
    TokenBracketOpen
    TokenBracketClose
    TokenParenOpen
    TokenParenClose
    TokenAssign
    TokenNewline
    TokenIndent
    TokenDedent
    TokenRawLine
    TokenEOF
)

type Token struct {
    Type   TokenType
    Value  string
    Line   int // 1-based
    Column int // 1-based
}

func StripComments(source string) string
func Tokenize(source string) []Token
func IsFieldType(s string) bool
func IsFieldConstraint(s string) bool
```

`Tokenize` always appends a trailing `TokenEOF`. There is no error return: malformed input degrades to whatever tokens the scanner can salvage. Run `StripComments` first.

`IsFieldType` exposes the model field type table: `text`, `email`, `bool`, `timestamp`, `richtext`, `option`, `int`, `float`, `password`, `image`, `phone`, `reference`, `date`, `url`, `decimal`, `file`, `tags`, `json`, `uuid`, `bigint`.

`IsFieldConstraint` exposes the field constraint table: `required`, `unique`, `default`, `auto`, `min`, `max`, `auto_update`.

The keyword set recognised at the top of a line is `page`, `action`, `fragment`, `stream`, `socket`, `api`, `webhook`, `schedule`, `job`, `model`, `config`, `auth`, `permissions`, `layout`, `query`, `validate`, `redirect`, `on`, `limit`, `log`, `test`, `translations`, `enqueue`, `broadcast`, `send`, `requires`, `method`, `fetch`, `llm`, `title`. Anything else that starts with a letter or underscore becomes `TokenIdentifier`.

## File map

- [`lexer.go`](../../../internal/lexer/lexer.go): the entire scanner, comment stripper, keyword and constraint tables, helpers.
- [`doc.go`](../../../internal/lexer/doc.go): package-level Go doc.
- [`lexer_test.go`](../../../internal/lexer/lexer_test.go): unit tests.
- [`bench_test.go`](../../../internal/lexer/bench_test.go): benchmarks.
- [`fuzz_test.go`](../../../internal/lexer/fuzz_test.go): fuzz harness.

## Key behaviors and gotchas

**INDENT/DEDENT emission.** `Tokenize` keeps an `indentStack` initialised to `{0}`. When a line's indent exceeds the top of the stack it pushes the new column and emits a single `TokenIndent`. When the indent drops, it pops one column at a time, emitting a `TokenDedent` per pop, until the stack head matches. At EOF the lexer flushes any remaining open levels with synthetic `TokenDedent` tokens so the parser sees a balanced stream.

**Tab handling.** `countIndent` treats `\t` as 2 columns. A file that mixes tabs and spaces will be measured against this fixed conversion.

**Comment stripping respects HTML blocks.** Full line `#` comments collapse to empty lines (preserving line numbers for diagnostics). Inline `#` is removed but quoted strings short circuit the scan (`#` inside `"..."` is kept). Inside an `html` block, comment stripping is suspended until indentation drops back to the `html` keyword's column. This keeps CSS hex colours like `#fff` safe.

**Unclosed string tolerance.** `tokenizeLine` walks until the next `"`; if the closing quote is missing it consumes to end of line and produces a `TokenString` whose value is everything after the opening quote.

**Path tokens.** A line position starting with `/` produces a single `TokenPath` running until the next whitespace, including segments such as `/users/:id/edit`.

**Number scanning.** Digits and a single `.` form a `TokenNumber`. The lexer does not validate numeric syntax; the parser converts via `strconv` and surfaces errors there.

**Empty lines are invisible.** Lines whose trim is empty contribute no tokens at all: no newline, no indent change. Indentation is only sampled on non blank lines.

**Identifiers may contain hyphens.** The word scanner accepts `[A-Za-z_][A-Za-z0-9_-]*`. Hyphenated identifiers (`generate-report`, `forgot-password`) flow through as `TokenIdentifier` unless they appear in the keyword table.

**No semantic validation.** The lexer never rejects nonsense like a stray `]`. Such characters are silently skipped by the fall through `i++` at the end of `tokenizeLine`. Higher layers detect structural problems.

**Column numbers are byte offsets.** The `Column` field is set from the byte index inside the trimmed line, not from the original column in the file. Diagnostics that need precise file columns must reconstruct from `Line` plus the original source.

**Indent stack invariants.** The stack starts at `{0}` and never has its base popped. Every push is matched by zero or more pops as the file walks back out. Mixed indentation that increases then decreases to a level not present on the stack does not error in the lexer; the parser detects the resulting structure mismatch.

**`TokenRawLine` is reserved.** The constant exists for callers that want to capture an entire raw line (originally intended for inline SQL). The current scanner does not emit it directly; SQL extraction happens in the parser by indexing back into the original source text via `parserState.lines`.

**No keyword promotion mid line.** A keyword token is emitted only when the matched word equals an entry in the keyword table. Words like `email` or `text` are never keywords at the lexical level: they become `TokenIdentifier` and are interpreted as field types only by the model parser, which calls `IsFieldType`.

**Stable across re-runs.** `Tokenize` is deterministic and stateless. Two calls with the same input produce byte identical token slices; this is exercised by `bench_test.go` and `fuzz_test.go`.

## Testing entry points

- Round trip and shape tests: [`lexer_test.go`](../../../internal/lexer/lexer_test.go).
- Hot path benchmarks: [`bench_test.go`](../../../internal/lexer/bench_test.go).
- Fuzz harness exercising arbitrary byte input: [`fuzz_test.go`](../../../internal/lexer/fuzz_test.go).
<!-- MANUAL-NOTES END -->
