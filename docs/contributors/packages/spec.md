# `internal/spec`

> Package spec is the canonical schema for Kilnx language entities (keywords and attributes).

| | |
|---|---|
| **Import path** | `github.com/kilnx-org/kilnx/internal/spec` |
| **Source last touched** | `5da8498` (2026-05-08) |


## Files

| File | Summary |
|------|---------|
| [`spec.go`](../../../internal/spec/spec.go) | _no file-level doc_ |

## Types

### `Arg`

```go
type Arg struct {
	Name		string
	Type		string
	Required	bool
	Variadic	bool
}
```
### `Entity`

```go
type Entity struct {
	Name		string
	Kind		Kind
	Summary		string
	Description	string
	Syntax		string
	Args		[]Arg
	ParentScope	[]string
	Children	[]string
	Repeatable	bool
	Required	bool
	Default		string
	Since		string
	Examples	[]Example
	SeeAlso		[]string
}
```
### `Example`

```go
type Example struct {
	Title	string
	Code	string
}
```
### `Kind`

```go
type Kind int
```
## Functions

### `All`

```go
func All() map[string]Entity
```

All returns a copy of the registry. Order is unspecified.

### `ByKind`

```go
func ByKind(k Kind) []Entity
```
### `ChildrenOf`

```go
func ChildrenOf(parent string) []Entity
```

ChildrenOf returns the union of:
  - entities listed explicitly in parent.Children
  - entities whose ParentScope contains parent (reverse lookup)

Result is deduped by Name and sorted alphabetically.

### `Get`

```go
func Get(name string) (Entity, bool)
```
### `ParentsOf`

```go
func ParentsOf(attr string) []Entity
```
### `Register`

```go
func Register(e Entity)
```

Register adds an entity to the registry. Intended to be called from
init() in the package that implements the entity (parser, lexer, etc).

Re-registering an attribute with the same Name is allowed for overloaded
DSL keywords that appear in multiple parent contexts (e.g. `requests`
inside `log` and `limit`). The ParentScope of the existing entity is
extended with the new entry's parents, and a context-specific note is
appended to the Description. For keywords, duplicate registration panics.

### `contains`

```go
func contains(s []string, v string) bool
```
### `(Kind) String`

```go
func (k Kind) String() string
```

## Notes

<!-- MANUAL-NOTES START -->
# Package `internal/spec`

Source: [spec.go](../../../internal/spec/spec.go).

## Purpose

Define the canonical schema for Kilnx language entities (keywords and attributes) and host the registry that documentation tooling reads. Entities are not declared in this package: they live next to their implementation (for example [`internal/parser/page_spec.go`](../../../internal/parser/page_spec.go)) and register themselves via `Register` at `init()` time. [`cmd/kilnx-gendocs`](../../../cmd/kilnx-gendocs/main.go) then imports those packages and walks the populated registry.

## Pipeline position

```
parser/*_spec.go init()  -> spec.Register -> in-memory registry
                                              |
cmd/kilnx-gendocs main()  ----- spec.All / spec.ByKind / spec.ChildrenOf -----> docs/devs/reference/*.md
```

The package has no runtime role beyond `init()` time registration. Production binaries built without the gendocs tool still import the registry transitively because each `*_spec.go` lives in `internal/parser`. This is harmless: the registry is a tiny in process map.

## Public API

```go
type Kind int

const (
    KindKeyword Kind = iota
    KindAttribute
)

func (k Kind) String() string

type Entity struct {
    Name        string
    Kind        Kind
    Summary     string
    Description string
    Syntax      string
    Args        []Arg
    ParentScope []string
    Children    []string
    Repeatable  bool
    Required    bool
    Default     string
    Since       string
    Examples    []Example
    SeeAlso     []string
}

type Arg struct {
    Name     string
    Type     string
    Required bool
    Variadic bool
}

type Example struct {
    Title string
    Code  string
}

func Register(e Entity)
func All() map[string]Entity
func Get(name string) (Entity, bool)
func ByKind(k Kind) []Entity
func ChildrenOf(parent string) []Entity
func ParentsOf(attr string) []Entity
```

## File map

- [`spec.go`](../../../internal/spec/spec.go): `Kind`, `Entity`, `Arg`, `Example`, registry storage, `Register`, lookup helpers, sort behaviour.
- [`spec_test.go`](../../../internal/spec/spec_test.go): tests for re-registration semantics and lookup helpers.

## Entity fields

- `Name`: identifier as written in source (`page`, `model`, `requires`, ...). Case sensitive. Must be non empty or `Register` panics.
- `Kind`: `KindKeyword` for top level constructs, `KindAttribute` for child attributes that appear inside a parent block.
- `Summary`: one line description used as a section header in generated docs.
- `Description`: long form prose; markdown formatting (`backticks`, paragraph breaks) is preserved verbatim.
- `Syntax`: a single line showing the canonical written form, e.g. `action <path> [method <verb>] [requires <clause>]`.
- `Args`: positional arguments. Each `Arg` carries `Name`, `Type`, `Required`, and `Variadic`.
- `ParentScope`: for attributes, the list of keyword names that may host this attribute. Empty for keywords. Extended automatically when an attribute is registered more than once.
- `Children`: explicit list of child entity names. Used together with the `ParentScope` reverse lookup by `ChildrenOf`.
- `Repeatable`: whether the entity may appear more than once in the same scope.
- `Required`: whether the entity is mandatory in its scope.
- `Default`: string form of the default value (used by the `method` attribute, for example).
- `Since`: semver of the release that introduced the entity.
- `Examples`: list of `(Title, Code)` snippets rendered in the docs.
- `SeeAlso`: cross reference list of related entity names.

## Registry storage and lookup

The registry is a package level `var entities = map[string]Entity{}`. There is no mutex: registration happens at `init()` time, which Go serialises per package, so no concurrent writes occur. After `init()` the map is treated as read only.

`All` returns a defensive copy; mutations on the result do not affect the registry. Iteration order is unspecified.

`Get` is a plain map lookup with `ok` semantics.

`ByKind` filters the registry and **sorts alphabetically by `Name`** before returning, so doc output is stable across runs.

`ChildrenOf(parent)` returns the union of:

- entities listed in `parent.Children`;
- entities whose `ParentScope` slice contains `parent` (reverse lookup).

The result is deduplicated by name and sorted alphabetically. This dual lookup means the parser can express the same parent/child relationship from either side.

`ParentsOf(attr)` returns the entities named in the attribute's `ParentScope`, in registration order. It returns nil if the entity is missing or is a keyword.

## `Register` semantics

```go
func Register(e Entity)
```

Calling `Register` with an empty `Name` panics. The behaviour for duplicate `Name` depends on `Kind`:

**Duplicate keyword: panic.** Two `KindKeyword` registrations under the same name fail with `panic("spec.Register: duplicate keyword <name>")`. Keywords are unique identifiers in the language, so a duplicate is always a programming error.

**Duplicate attribute: merge.** Attributes may be registered multiple times to express that the same attribute appears under different parents (for example `requests` inside both `log` and `limit`). The existing entry is updated:

- new entries from `e.ParentScope` are appended (deduped via `contains`);
- a context-specific note `When used in <parent>: <text>` is appended to `Description`, where `<text>` is the new `Description` (or `Summary` if `Description` is empty);
- empty fields on the existing entry (`Summary`, `Syntax`, `Default`, `Args`) are filled from the new entry;
- `Examples` are concatenated.

Mismatched kinds (keyword versus attribute under the same name) panic with `kind mismatch`.

## Consumed by `cmd/kilnx-gendocs`

[`cmd/kilnx-gendocs`](../../../cmd/kilnx-gendocs/main.go) imports `internal/parser` (which transitively imports every `*_spec.go`), then calls `spec.ByKind(KindKeyword)` and `spec.ByKind(KindAttribute)` to drive its templates under `cmd/kilnx-gendocs/templates`. Output goes to `docs/devs/reference/<name>.md`. Run via the `go:generate` directive in [`internal/parser/doc.go`](../../../internal/parser/doc.go):

```
go generate ./...
```

## Key behaviors and gotchas

**Registration order is import order.** Two attributes registered under the same name pick up fields from the first registration that supplies them. If two registrations both set `Summary`, the first wins. Authors should treat the first registration of an attribute as the canonical definition and let later registrations only extend `ParentScope` and `Examples`.

**No removal API.** The registry is append only. Tests that need a clean registry must run in their own process or accept the cumulative state.

**Sort stability.** Alphabetic sort in `ByKind` and `ChildrenOf` is the only guaranteed ordering. Do not rely on registration order for output.

**Empty `ParentScope` plus `KindAttribute` is legal but useless.** Such an attribute will never be returned by `ChildrenOf` and will be filtered out of any parent based docs section. Authors should always set `ParentScope` for attributes.
<!-- MANUAL-NOTES END -->
