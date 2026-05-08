# Adding an Attribute

Worked example: how the `requires` attribute is wired across the parser, spec registry, analyzer, and runtime. Use this as a template when adding a new attribute (something that decorates a parent keyword, like `method`, `requires`, `title`, `redirect`, `default`, `unique`, `min`, `max`, `index`, `tenant`).

The short checklist lives in [CONTRIBUTING.md](../../CONTRIBUTING.md). Here we walk every layer with concrete file pointers.

## Where attributes live

The Kilnx grammar has two flavors of attributes:

1. **Inline attributes on a parent keyword line**, e.g. `page /admin requires admin method POST`. Spec and parser sit alongside the parent keywords.
2. **Field-level constraints inside a `model` block**, e.g. `email: email required unique`. Spec lives in [`field_attrs_spec.go`](../../internal/parser/field_attrs_spec.go); parsing lives inside the model field loop in [`parser.go`](../../internal/parser/parser.go).

The general-purpose registration file is [`internal/parser/attrs_spec.go`](../../internal/parser/attrs_spec.go) for attributes shared across multiple keywords. Attributes scoped to a single keyword belong in that keyword's `_spec.go`.

## 1. Decide the parent scope

An attribute lists every keyword it can appear under in `ParentScope`. This drives doc generation (`spec.ChildrenOf`, `spec.ParentsOf`) and powers the reverse lookup so the parent keyword does not need to enumerate the attribute in `Children`.

`requires` is multi-parent, applying to `page`, `action`, `api`, and (since later versions) `socket` and `stream`. Its registration in [`attrs_spec.go`](../../internal/parser/attrs_spec.go):

```go
spec.Register(spec.Entity{
    Name:        "requires",
    Kind:        spec.KindAttribute,
    Summary:     "Require authentication or a specific role/permission.",
    Description: "Gates the parent endpoint behind authentication. " +
        "Accepts `auth`, a role name, or a permission expression.",
    Syntax:      "requires <clause>",
    Args: []spec.Arg{
        {Name: "clause", Type: "identifier", Required: true},
    },
    ParentScope: []string{"page", "action", "api"},
    Repeatable:  true,
    Since:       "0.1.0",
    Examples:    []spec.Example{ /* ... */ },
    SeeAlso:     []string{"method", "permissions"},
})
```

For overloaded attributes (the same name under multiple parents with parent-specific semantics, like `requests` inside both `log` and `limit`), call `spec.Register` once per parent. The registry merges them: see the merge logic in [`spec.go`](../../internal/spec/spec.go) lines 70 to 115. New `Description` text is appended under a `When used in <parent>:` header, `ParentScope` is unioned, and examples are concatenated.

## 2. Lex

Most attributes do not need lexer changes. The lexer treats unknown words as `TokenIdentifier`, and the parser checks the value when scanning attribute lists. An attribute name only needs to be added to `keywords` in [`lexer.go`](../../internal/lexer/lexer.go) if it must be tokenized as `TokenKeyword` so the parser dispatch can rely on `tok.Type == lexer.TokenKeyword` checks. Look at the existing entries (`requires`, `method`, `title` are all in the `keywords` map) and follow the same convention if your attribute participates in the same parse loops.

For field-level constraints (model body), add to `fieldConstraints` and rely on [`IsFieldConstraint`](../../internal/lexer/lexer.go).

## 3. Wire the attribute parser

Inline attributes are consumed inside the parent keyword's parse loop. `requires` is wired in three different parse loops because it appears under three keywords. The cleanest reference is the page parser, around line 1122 of [`parser.go`](../../internal/parser/parser.go):

```go
for !p.isEOF() && p.current().Type != lexer.TokenNewline {
    tok := p.current()
    if tok.Type == lexer.TokenKeyword {
        switch tok.Value {
        case "layout":
            // ...
        case "requires":
            requiresLine := tok.Line
            p.advance()
            page.Auth = true
            page.RequiresClauses = p.parseRequiresClauses(requiresLine)
            page.RequiresRole = firstRoleValue(page.RequiresClauses)
            p.skipRequiresClauses()
        case "method":
            p.advance()
            if p.current().Type == lexer.TokenIdentifier {
                page.Method = p.advance().Value
            }
        }
    }
}
```

The same `case "requires":` and `case "method":` blocks reappear inside `parseAction` (around line 1479) and `parseAPI` (around line 2051). Keep the three branches in sync, or extract a shared helper if your attribute starts to spread across parents.

For field-level constraints, the dispatch is in the model-field loop (around line 1054):

```go
case "required":
    field.Required = true
case "unique":
    field.Unique = true
case "default":
    p.advance()
    field.Default = parseDefaultValue(p)
case "min":
    // ...
case "max":
    // ...
```

## 4. Attach to the AST

Add the field on the parent's AST struct. For `requires`, the parent stores both the legacy single role and the full clause list:

```go
type Page struct {
    Path            string
    Auth            bool
    RequiresRole    string
    RequiresClauses []RequiresClause
    // ...
}
```

For richer attributes, model the parsed shape with a dedicated type the way [`RequiresClause`](../../internal/parser/parser.go) does. Simple boolean flags (`required`, `unique`) can sit directly on the field/parent.

## 5. Add analyzer validation

If an attribute makes some configurations illegal (or unsafe), add a check in [`internal/analyzer/`](../../internal/analyzer/). For `requires`, the analyzer cross-checks role names against declared `permissions` blocks and rejects unknown roles. Auth-related security checks live in [`security.go`](../../internal/analyzer/security.go); type-level checks live in [`analyzer.go`](../../internal/analyzer/analyzer.go).

Each check emits [`Diagnostic`](../../internal/analyzer/types.go) values with severity, message, and context. Wire your check into [`Analyze`](../../internal/analyzer/analyzer.go) by appending to the diagnostics slice.

## 6. Add runtime semantics

Attribute semantics fire inside the request handler. For `requires`, the runtime is in [`internal/runtime/permissions.go`](../../internal/runtime/permissions.go) and the gating logic in [`requires_clauses_test.go`](../../internal/runtime/requires_clauses_test.go) covers role checks, auth checks, expression clauses, and superuser bypass. The handler in [`server.go`](../../internal/runtime/server.go) calls into the requires evaluator before invoking the page or action body.

For trivial attributes, the runtime change can be one line: read the field off the AST node and branch on it.

## 7. Regenerate the reference docs

```bash
go generate ./...
```

This regenerates `docs/devs/reference/attributes/<name>.md` via [`cmd/kilnx-gendocs`](../../cmd/kilnx-gendocs/main.go). Stale detection runs in CI; run it locally before committing:

```bash
go run ./cmd/kilnx-gendocs -check-stale
```

## 8. Tests

Add coverage at the layers you touched:

- Spec invariants: [`internal/parser/spec_test.go`](../../internal/parser/spec_test.go) already enforces `Summary`, `Syntax`, `Since`, parent registration, and children registration. New attributes inherit those checks for free.
- Parser: an attribute test in [`parser_test.go`](../../internal/parser/parser_test.go) using the `parse(t, src)` helper, asserting the parsed AST shape.
- Analyzer: a test in the matching `_test.go` next to the check.
- Runtime: an integration test using `httptest`, e.g. [`requires_clauses_e2e_test.go`](../../internal/runtime/requires_clauses_e2e_test.go).

See [testing.md](testing.md) for the conventions used across these files.

## 9. Final checks

```bash
go vet ./...
gofmt -l .
go test -race ./...
go run ./cmd/kilnx-gendocs -check-stale
```

Commit spec, parser, analyzer, runtime, regenerated docs, and tests in one PR.
