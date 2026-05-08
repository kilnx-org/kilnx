# Adding a Keyword

Worked example: how the `webhook` keyword was added end to end. Use this as a template when introducing a new top-level keyword (something that appears at indent 0 in a `.kilnx` file, like `page`, `action`, `model`, `webhook`, `socket`, `schedule`, `job`, `api`, `stream`, `auth`, `permissions`, `layout`, `config`, `log`, `translations`, `test`).

This is the deep tutorial. The short checklist lives in [CONTRIBUTING.md](../../CONTRIBUTING.md) and the registry-side picture lives in [architecture/spec-registry.md](architecture/spec-registry.md).

## 1. Register the keyword in the lexer

Top-level keywords must be recognized by the tokenizer, otherwise they emit `TokenIdentifier` and the parser dispatch never fires.

Edit [`internal/lexer/lexer.go`](../../internal/lexer/lexer.go) and add an entry to the `keywords` map:

```go
var keywords = map[string]bool{
    "page": true, "action": true, "fragment": true,
    // ...
    "webhook": true,
    // ...
}
```

If the new keyword introduces a model field type, add it to `fieldTypes` instead and use [`IsFieldType`](../../internal/lexer/lexer.go) from the parser. If it introduces a model field constraint, add it to `fieldConstraints` and use [`IsFieldConstraint`](../../internal/lexer/lexer.go). Field-level keywords do not need to be in the top-level `keywords` map.

## 2. Add the AST node

All AST types live in [`internal/parser/parser.go`](../../internal/parser/parser.go) next to the parser. For `webhook`:

```go
// Webhook is a `webhook` declaration: an HTTP endpoint that receives
// signed events from an external service and dispatches them by name.
type Webhook struct {
    Path      string
    SecretEnv string
    Events    []WebhookEvent
}

type WebhookEvent struct {
    Name string
    Body []Node
}
```

Then add a slice on the root [`App`](../../internal/parser/parser.go) struct so the runtime can iterate it:

```go
type App struct {
    // ...
    Webhooks []Webhook
    // ...
}
```

## 3. Write the parse function

Despite the file name suggested by some patterns elsewhere, all parser logic for top-level entities lives inside [`parser.go`](../../internal/parser/parser.go) as a method on `parserState`. Look at [`parseWebhook`](../../internal/parser/parser.go) at line 2505 for the canonical shape:

```go
func (p *parserState) parseWebhook() Webhook {
    wh := Webhook{}
    p.advance() // consume "webhook"

    if p.current().Type == lexer.TokenPath {
        wh.Path = p.advance().Value
    }
    if wh.Path == "" {
        p.addError(fmt.Errorf("webhook path is required"))
    }

    // parse same-line modifiers (secret env VAR_NAME, ...)
    // ...

    // descend into the indented body
    p.skipNewlines()
    if p.current().Type == lexer.TokenIndent {
        p.advance()
        // parse children, leveraging p.parseBody() when the body is
        // a generic node list, or a custom loop for keyword-specific
        // sub-statements like `on event ...`.
    }
    return wh
}
```

Reuse helpers already defined on `parserState`: [`p.advance`, `p.current`, `p.skipNewlines`, `p.skipToEndOfLine`, `p.parseBody`, `p.parseRequiresClauses`, `p.addError`, `p.synchronize`](../../internal/parser/parser.go). Custom logic should be the smallest delta on top of those.

## 4. Wire the dispatch table

[`Parse`](../../internal/parser/parser.go) walks the token stream and dispatches on `tok.Value` for each keyword. Add a `case` in the top-level switch (around line 413):

```go
switch tok.Value {
case "model":
    // ...
case "webhook":
    wh := p.parseWebhook()
    app.Webhooks = append(app.Webhooks, wh)
case "socket":
    // ...
}
```

If the parse function can fail, follow the error pattern used by `parseModel`, `parsePage`, and friends: return `(T, error)`, append to `p.addError`, then call `p.synchronize()` to skip to the next top-level entity.

## 5. Write the spec registration

Create [`internal/parser/<keyword>_spec.go`](../../internal/parser/) with an `init()` that calls `spec.Register`. For `webhook` see [`webhook_spec.go`](../../internal/parser/webhook_spec.go):

```go
package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
    spec.Register(spec.Entity{
        Name:    "webhook",
        Kind:    spec.KindKeyword,
        Summary: "Receive external events at a path.",
        Description: "A `webhook` is an HTTP POST endpoint that " +
            "authenticates requests via a shared secret and dispatches " +
            "to an `on event <name>` handler.",
        Syntax: "webhook <path> [secret env <VAR>]",
        Args: []spec.Arg{
            {Name: "path", Type: "path", Required: true},
        },
        Children:   []string{"redirect"},
        Repeatable: true,
        Since:      "0.1.0",
        Examples:   []spec.Example{ /* ... */ },
        SeeAlso:    []string{"on", "fetch"},
    })
}
```

Required fields enforced by [`internal/parser/spec_test.go`](../../internal/parser/spec_test.go): `Summary`, `Syntax`, `Since`. Children and attributes wired here also have to be registered somewhere, see [`TestSpec_ChildrenAreRegistered`](../../internal/parser/spec_test.go).

## 6. Add analyzer checks (optional but expected)

If the keyword has security implications or invariants, add a check in [`internal/analyzer/`](../../internal/analyzer/). For `webhook` the analyzer warns when no signing secret is configured. See [`checkWebhookSecrets`](../../internal/analyzer/security.go) and [`Analyze`](../../internal/analyzer/security.go) which composes the checks:

```go
func checkWebhookSecrets(app *parser.App) []Diagnostic {
    var diags []Diagnostic
    for _, wh := range app.Webhooks {
        if wh.SecretEnv == "" {
            diags = append(diags, Diagnostic{
                Severity: SeverityWarning,
                Message:  "webhook has no secret configured",
                Context:  fmt.Sprintf("webhook %s", wh.Path),
            })
        }
    }
    return diags
}
```

Register the check by appending its result to `diags` inside [`Analyze`](../../internal/analyzer/security.go).

## 7. Add runtime handling

The runtime is an AST interpreter rooted in [`internal/runtime/server.go`](../../internal/runtime/server.go). For most new keywords you will:

1. Add a slice scan inside the main HTTP handler. Webhooks live at lines 217 to 223:

   ```go
   for _, wh := range app.Webhooks {
       if r.URL.Path == wh.Path {
           s.handleWebhook(w, r, wh)
           return
       }
   }
   ```

2. Add a per-keyword file with the handler. See [`internal/runtime/webhook.go`](../../internal/runtime/webhook.go) for `handleWebhook`. Background-only keywords (`schedule`, `job`) wire into [`scheduler.go`](../../internal/runtime/scheduler.go) instead of the HTTP handler.

3. Reuse existing helpers when executing body nodes: [`s.executeBody`](../../internal/runtime/server.go), [`s.runQuery`](../../internal/runtime/server.go), and the param resolution machinery in [`expr.go`](../../internal/runtime/expr.go).

## 8. Regenerate the reference docs

```bash
go generate ./...
```

This invokes the `go:generate` directive in [`internal/parser/doc.go`](../../internal/parser/doc.go) which runs [`cmd/kilnx-gendocs`](../../cmd/kilnx-gendocs/main.go) and writes `docs/devs/reference/keywords/<name>.md`. The pre-commit hook in `.githooks/pre-commit` runs the same command, and CI fails if the regenerated output differs from what is committed.

Verify staleness is clean:

```bash
go run ./cmd/kilnx-gendocs -check-stale
```

## 9. Add tests at every layer

Lexer recognition is exercised through parser tests, so usually you do not need a dedicated lexer test. Add:

- A parser test in [`internal/parser/parser_test.go`](../../internal/parser/parser_test.go) using the `parse(t, src)` helper.
- An analyzer test next to the check, e.g. [`security_test.go`](../../internal/analyzer/security_test.go).
- A runtime test using `httptest`, e.g. [`webhook_test.go`](../../internal/runtime/webhook_test.go).
- Optionally a smoke `.kilnx` file under [`smoke/`](../../smoke/) if the feature is small enough to live in a self-contained example.

See [testing.md](testing.md) for conventions and patterns.

## 10. Final checks

```bash
go vet ./...
gofmt -l .
go test -race ./...
go run ./cmd/kilnx-gendocs -check-stale
```

Commit the spec change, the regenerated `docs/devs/reference/`, and the parser, analyzer, runtime, and test changes together.
