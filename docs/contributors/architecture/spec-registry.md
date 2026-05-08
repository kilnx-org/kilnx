# Spec Registry

`internal/spec` is the canonical schema describing every Kilnx language entity (keyword or attribute). The registry is the single source of truth consumed by [`cmd/kilnx-gendocs`](../../../cmd/kilnx-gendocs) to emit user-facing reference Markdown under [`docs/devs/reference/`](../../devs/reference/).

## Package shape

[`internal/spec/spec.go`](../../../internal/spec/spec.go) defines:

- [`Kind`](../../../internal/spec/spec.go): `KindKeyword` or `KindAttribute`.
- [`Entity`](../../../internal/spec/spec.go): name, kind, summary, description, syntax, args, parent scope, children, repeatable, required, default, since, examples, see-also.
- [`Arg`](../../../internal/spec/spec.go): one positional or named argument (`Name`, `Type`, `Required`, `Variadic`).
- [`Example`](../../../internal/spec/spec.go): titled code snippet.
- A package-level `entities` map and the API: [`Register`](../../../internal/spec/spec.go), [`All`](../../../internal/spec/spec.go), [`Get`](../../../internal/spec/spec.go), [`ByKind`](../../../internal/spec/spec.go), [`ChildrenOf`](../../../internal/spec/spec.go), [`ParentsOf`](../../../internal/spec/spec.go).

The package itself declares no entities. By design.

## Self-registration via `init()`

Each language entity is declared next to the code that implements it. The implementing package adds a `<name>_spec.go` file that calls `spec.Register` from `init()`. Examples:

- [`internal/parser/page_spec.go`](../../../internal/parser/page_spec.go) registers the `page` keyword.
- [`internal/parser/model_spec.go`](../../../internal/parser/model_spec.go) registers `model`.
- [`internal/parser/attrs_spec.go`](../../../internal/parser/attrs_spec.go) and [`internal/parser/field_attrs_spec.go`](../../../internal/parser/field_attrs_spec.go) register attributes (`required`, `unique`, `auto`, `default`, `min`, `max`, `index`, `tenant`, `requires`, `method`, `title`, `layout`, ...).
- [`internal/parser/body_nodes_spec.go`](../../../internal/parser/body_nodes_spec.go) registers body statements (`query`, `redirect`, `validate`, `respond`, `html`, `send`, `enqueue`, `on`, `broadcast`, `fetch`, `llm`, ...).

A typical registration looks like the `model` entry in [`model_spec.go`](../../../internal/parser/model_spec.go):

```go
func init() {
    spec.Register(spec.Entity{
        Name:    "model",
        Kind:    spec.KindKeyword,
        Summary: "Declare a database table with typed fields and constraints.",
        Syntax:  "model <name>",
        Args:    []spec.Arg{{Name: "name", Type: "identifier", Required: true}},
        Children: []string{"required", "unique", "auto", "auto_update", "default",
            "min", "max", "index", "tenant", "custom", "dynamic_fields"},
        Repeatable: true,
        Since:      "0.1.0",
        Examples:   []spec.Example{ /* ... */ },
        SeeAlso:    []string{"query", "auth"},
    })
}
```

## Overloaded attributes

Some attribute names appear under multiple parents (for example `requests` inside both `log` and `limit`). `Register` permits re-registering an attribute with the same `Name` and merges:

- `ParentScope` is unioned.
- New `Description` text is appended under a `When used in <parent>:` header.
- First non-empty `Summary`, `Syntax`, `Default`, `Args` win. Examples are concatenated.

Re-registering a keyword (`KindKeyword`) panics. Mixing kinds for the same name panics. See the merge logic in [`spec.go`](../../../internal/spec/spec.go).

## Lookup and traversal

- `spec.ByKind(spec.KindKeyword)` returns all keywords sorted by name. Used by [`cmd/kilnx-gendocs/main.go`](../../../cmd/kilnx-gendocs/main.go) to drive the index page.
- `spec.ChildrenOf(parent)` returns the union of `parent.Children` and entities whose `ParentScope` includes `parent`, deduped and sorted. The reverse lookup means a child can opt in by setting `ParentScope` without editing the parent's `Children` list.
- `spec.ParentsOf(attr)` returns the parents of an attribute via `ParentScope`.

## Doc generation

[`cmd/kilnx-gendocs`](../../../cmd/kilnx-gendocs/main.go) is a thin renderer:

1. Blank-import packages that register entities. Today this is `internal/parser`. The blank import triggers each `<name>_spec.go` `init()`, which populates `spec.entities`.
2. Pull keywords and attributes via `spec.ByKind`.
3. Augment `SeeAlso` with reverse links.
4. Resolve provenance from `git log`: SHA and date for the spec file, plus the implementation file(s) discovered by grepping the parser for `case "<name>":` and `Value == "<name>"` patterns. Mark entities `Stale` when the implementation was touched after the spec.
5. Render the embedded templates ([`cmd/kilnx-gendocs/templates`](../../../cmd/kilnx-gendocs/templates)) into `docs/devs/reference/keywords/<name>.md` and `docs/devs/reference/attributes/<name>.md`.

Run it locally:

```bash
go run ./cmd/kilnx-gendocs            # writes to docs/devs/reference
go run ./cmd/kilnx-gendocs -o /tmp    # writes to /tmp/devs/reference
go run ./cmd/kilnx-gendocs -check-stale
```

`-check-stale` exits non-zero when any entity's implementation is newer than its spec. Wire it into CI to catch undocumented behaviour drift.

## Adding a new entity

1. Implement parsing and AST in `internal/parser` (see [ast.md](ast.md)).
2. Create `<name>_spec.go` in the same package. Populate `Name`, `Kind`, `Summary`, `Description`, `Syntax`, `Args`, `ParentScope` (for attributes), `Children` (for keywords), `Examples`, `Since`, `SeeAlso`.
3. Run `go run ./cmd/kilnx-gendocs` and commit the generated Markdown.
4. Run `go run ./cmd/kilnx-gendocs -check-stale` to confirm the new entity is not flagged.

## Why init-time registration

Co-locating the spec with the implementation keeps the two from drifting. Reviewers see DSL surface changes in the same diff as the parser change, and the staleness detector catches the rest.
