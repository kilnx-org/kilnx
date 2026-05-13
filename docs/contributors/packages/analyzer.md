# `internal/analyzer`

> Package analyzer performs static analysis on the parser AST: type checking of field constraints, validation of route templates, permission/role checks, dead-link detection, and security audits.

| | |
|---|---|
| **Import path** | `github.com/kilnx-org/kilnx/internal/analyzer` |
| **Source last touched** | `aef0ef5` (2026-05-13) |
| **Doc last touched** | `5da8498` (2026-05-08) |


> **Implementation touched after doc.go.** Source changed on `2026-05-13`, but `doc.go` was last edited on `2026-05-08`. The summary above may be out of date.

## Overview

The analyzer runs after parsing and before optimization or runtime
execution. It is also the implementation behind `kilnx check`. Errors
and warnings are returned as structured diagnostics so callers (CLI,
CI) can format them.

Public surface centers on Analyze, which takes a parser.App and
returns a slice of diagnostics. Subordinate checks live in
db_check.go (database/SQL coherence) and security.go (auth/CSRF).

## Files

| File | Summary |
|------|---------|
| [`analyzer.go`](../../../internal/analyzer/analyzer.go) | _no file-level doc_ |
| [`analyzer_llm_agent.go`](../../../internal/analyzer/analyzer_llm_agent.go) | _no file-level doc_ |
| [`db_check.go`](../../../internal/analyzer/db_check.go) | _no file-level doc_ |
| [`security.go`](../../../internal/analyzer/security.go) | _no file-level doc_ |
| [`types.go`](../../../internal/analyzer/types.go) | _no file-level doc_ |

## Types

### `ColumnInfo`

```go
type ColumnInfo struct {
	FieldType parser.FieldType
}
```

ColumnInfo holds the inferred type for a single database column.

### `Diagnostic`

```go
type Diagnostic struct {
	Level	string	// "error" or "warning"
	Message	string
	Line	int	// source line (0 if unknown)
	Context	string	// location, e.g. "page /users"
}
```

Diagnostic represents a compile-time finding from static analysis.

### `ExprType`

```go
type ExprType struct {
	Category	TypeCategory
	Source		string	// description for error messages
}
```

ExprType represents the inferred type of a SQL expression.

### `ModelFieldInfo`

```go
type ModelFieldInfo struct {
	FormFields	map[string]bool		// field names as sent by HTML forms
	FieldToColumn	map[string]string	// form field name -> DB column name
	ColumnToField	map[string]string	// DB column name -> form field name
}
```

ModelFieldInfo holds the form-level field names for a model.
This maps model field names (what HTML forms send) to DB column names.

### `Schema`

```go
type Schema struct {
	Tables		map[string]*TableInfo
	ModelFields	map[string]*ModelFieldInfo
}
```

Schema is the compile-time view of the database derived from model declarations.

### `TableInfo`

```go
type TableInfo struct {
	Name	string
	Columns	map[string]*ColumnInfo
}
```

TableInfo holds the known columns and their types for a model-derived table.

### `TypeCategory`

```go
type TypeCategory int
```

TypeCategory groups field types for compatibility checking.

### `columnRef`

```go
type columnRef struct {
	table	string
	column	string
}
```
### `eachBlock`

```go
type eachBlock struct {
	start		int
	end		int
	queryName	string
}
```

eachBlock represents a single {{each queryName}}...{{end}} span in HTML.

### `sqlComparison`

```go
type sqlComparison struct {
	left		sqlToken
	right		sqlToken
	leftTable	string
	rightTable	string
	op		string
}
```

sqlComparison represents a binary comparison in a WHERE clause.

### `sqlToken`

```go
type sqlToken struct {
	typ	int
	value	string
	lower	string
}
```
### `tableRef`

```go
type tableRef struct {
	name	string
	alias	string
}
```
## Functions

### `Analyze`

```go
func Analyze(app *parser.App) []Diagnostic
```

Analyze performs static analysis on a parsed Kilnx app, checking SQL
references against declared models and type compatibility.

### `AnalyzeWithDB`

```go
func AnalyzeWithDB(app *parser.App, db *database.DB) []Diagnostic
```

AnalyzeWithDB runs the standard Analyze pass plus a DB-connected pass that
validates {q.custom.fieldName} references against the actual _field_defs tables.
db may be nil, in which case this is equivalent to Analyze.

### `BuildSchema`

```go
func BuildSchema(models []parser.Model, manifests ...map[string]*parser.CustomFieldManifest) *Schema
```

BuildSchema creates a compile-time view of the database from model declarations.
Pass app.CustomManifests as the optional second argument to include column-mode
custom fields as real schema columns.

### `HasLLMAgentBlock`

```go
func HasLLMAgentBlock(app *parser.App) bool
```

HasLLMAgentBlock reports whether the app declares any `llm ... agent`
block. The runtime startup uses this to decide whether to require the
`claude` CLI on PATH.

### `analyzeNodes`

```go
func analyzeNodes(nodes []parser.Node, schema *Schema, context string) []Diagnostic
```
### `analyzeSQL`

```go
func analyzeSQL(sql string, schema *Schema, context string) []Diagnostic
```
### `categoryName`

```go
func categoryName(c TypeCategory) string
```
### `categoryOf`

```go
func categoryOf(ft parser.FieldType) TypeCategory
```
### `checkActionAttributes`

```go
func checkActionAttributes(app *parser.App) []Diagnostic
```

checkActionAttributes validates that action="/path" attributes on
<button>, <a> and <input> elements reference an existing action block
(or page route, for forms posting to a page). The regex is anchored to
these tag names so native <form action="..."> never trips this check.

### `checkActionParams`

```go
func checkActionParams(action parser.Page, modelName string, schema *Schema, context string) []Diagnostic
```
### `checkActionParamsNodes`

```go
func checkActionParamsNodes(nodes []parser.Node, path string, modelName string, schema *Schema, context string) []Diagnostic
```
### `checkAllSQL`

```go
func checkAllSQL(app *parser.App, schema *Schema) []Diagnostic
```
### `checkAuthPages`

```go
func checkAuthPages(app *parser.App) []Diagnostic
```

checkAuthPages ensures that an app declaring an `auth` block also
declares the four auth entry pages. The runtime only owns the POST
side of those routes; the GET side (form, error display, token
validation UI) must live in user-declared pages so the app controls
branding and i18n. Catching this at compile time prevents a runtime
404 on an otherwise working-looking app.

### `checkAuthRef`

```go
func checkAuthRef(app *parser.App, schema *Schema) []Diagnostic
```
### `checkAuthWithoutPermissions`

```go
func checkAuthWithoutPermissions(app *parser.App) []Diagnostic
```

checkAuthWithoutPermissions warns when auth is configured but no permissions are defined.

### `checkCSRFProtection`

```go
func checkCSRFProtection(app *parser.App) []Diagnostic
```

checkCSRFProtection warns when a page has a raw HTML form targeting a
mutating action instead of using the Kilnx form keyword (which auto-adds
a CSRF token). Only pages whose path matches an action's path are checked.

### `checkCustomFieldRefs`

```go
func checkCustomFieldRefs(app *parser.App, schema *Schema) []Diagnostic
```

checkCustomFieldRefs validates {q.custom.fieldName} template references
against the corresponding model's custom field manifest.

### `checkCustomManifestRefs`

```go
func checkCustomManifestRefs(app *parser.App) []Diagnostic
```

checkCustomManifestRefs validates that kind: reference targets in manifests refer to known models.

### `checkDynamicFieldRefsWithDB`

```go
func checkDynamicFieldRefsWithDB(app *parser.App, db *database.DB) []Diagnostic
```

checkDynamicFieldRefsWithDB queries each _<model>_field_defs table and
validates hardcoded {q.custom.X} references against the actual field names.

### `checkFragmentComponents`

```go
func checkFragmentComponents(app *parser.App) []Diagnostic
```

checkFragmentComponents validates component fragment call sites in NodeHTML bodies.
It checks that every {{name arg=expr}} references a known component, provides all
required arguments, and does not pass unknown arguments.

### `checkIndexes`

```go
func checkIndexes(app *parser.App) []Diagnostic
```

checkIndexes validates each `index (a, b, ...)` directive the same way
as checkUniqueConstraints: declared field names exist on the model, no
repeats within a group, no duplicated groups.

### `checkInsertValueTypes`

```go
func checkInsertValueTypes(tokens []sqlToken, table string, schema *Schema, context string) []Diagnostic
```

checkInsertValueTypes validates that INSERT literal values match column types.

### `checkLLMAgentMCPRefs`

```go
func checkLLMAgentMCPRefs(app *parser.App) []Diagnostic
```

checkLLMAgentMCPRefs validates that every MCP name referenced by an
agent block resolves to a top-level `mcp <name>` declaration.

### `checkLLMAgentRequired`

```go
func checkLLMAgentRequired(app *parser.App) []Diagnostic
```

checkLLMAgentRequired walks every LLM node in agent mode and flags
blocks that miss `max-budget-usd`. The spec registry marks this attr
`Required: true` but the registry does not enforce it; analysis does.

### `checkLLMAgentReservedAttrs`

```go
func checkLLMAgentReservedAttrs(app *parser.App) []Diagnostic
```

checkLLMAgentReservedAttrs warns about reserved attributes that the
runtime currently ignores, and about the dangerous bypassPermissions
mode.

### `checkLLMAgentWorkspaceRoot`

```go
func checkLLMAgentWorkspaceRoot(app *parser.App) []Diagnostic
```

checkLLMAgentWorkspaceRoot ensures `config workspace-root` is set when
any agent block declares an explicit cwd, OR when any agent block is
declared at all (tmp dirs need a root to live under).

### `checkModelDefaults`

```go
func checkModelDefaults(models []parser.Model) []Diagnostic
```
### `checkModelMinMax`

```go
func checkModelMinMax(models []parser.Model) []Diagnostic
```
### `checkModelRefs`

```go
func checkModelRefs(app *parser.App, schema *Schema) []Diagnostic
```
### `checkNamedParams`

```go
func checkNamedParams(sql string, tokens []sqlToken, path string, modelName string, schema *Schema, context string) []Diagnostic
```
### `checkNamedParamsExtra`

```go
func checkNamedParamsExtra(sql string, tokens []sqlToken, path string, modelName string, schema *Schema, context string, extra map[string]bool) []Diagnostic
```
### `checkNodesForPasswordExposure`

```go
func checkNodesForPasswordExposure(nodes []parser.Node, passwordTables map[string][]string, schema *Schema, context string) []Diagnostic
```

checkNodesForPasswordExposure checks if query nodes expose password columns.

### `checkPasswordExposure`

```go
func checkPasswordExposure(app *parser.App, schema *Schema) []Diagnostic
```

checkPasswordExposure warns when queries in public-facing endpoints
select password fields from the auth table.

### `checkSQLCustomFieldRefs`

```go
func checkSQLCustomFieldRefs(app *parser.App, schema *Schema) []Diagnostic
```

checkSQLCustomFieldRefs validates json_extract(custom, '$.field') and
custom->>'field' patterns in SQL query nodes against the custom field manifest.

### `checkSecurity`

```go
func checkSecurity(app *parser.App, schema *Schema) []Diagnostic
```

checkSecurity performs security gap analysis on the app.
It detects missing auth, exposed sensitive fields, unprotected webhooks,
and other common security misconfigurations.

### `checkSelectColumns`

```go
func checkSelectColumns(tokens []sqlToken, tableRefs []tableRef, aliasToTable map[string]string, schema *Schema, context string) []Diagnostic
```
### `checkTemplateInterpolations`

```go
func checkTemplateInterpolations(app *parser.App, schema *Schema) []Diagnostic
```

checkTemplateInterpolations validates {queryName.field}, {^field}, and bare {field}
references inside HTML content of pages, fragments, and layouts against the schema.

### `checkTenantRefs`

```go
func checkTenantRefs(app *parser.App, schema *Schema) []Diagnostic
```

checkTenantRefs makes sure every `tenant: <model>` directive points
at an actual model in the app, and that a model does not set itself
as its own tenant.

### `checkTranslationParams`

```go
func checkTranslationParams(app *parser.App) []Diagnostic
```

checkTranslationParams validates that:
1. Every {name} placeholder in a translation value is supplied at call sites
2. Parameter placeholders are consistent across locales

### `checkUnauthAPIs`

```go
func checkUnauthAPIs(app *parser.App) []Diagnostic
```

checkUnauthAPIs warns when API endpoints that write data don't require auth.

### `checkUnauthActions`

```go
func checkUnauthActions(app *parser.App) []Diagnostic
```

checkUnauthActions warns when actions (POST/PUT/DELETE) don't require auth.

### `checkUnauthSockets`

```go
func checkUnauthSockets(app *parser.App) []Diagnostic
```

checkUnauthSockets warns when sockets don't require auth.

### `checkUnauthStreams`

```go
func checkUnauthStreams(app *parser.App) []Diagnostic
```

checkUnauthStreams warns when streams don't require auth.

### `checkUniqueConstraints`

```go
func checkUniqueConstraints(app *parser.App) []Diagnostic
```

checkUniqueConstraints validates that every field named inside a
`unique (a, b, ...)` directive exists on its model, that no field is
repeated within a single group, and that no two groups are identical.

### `checkUpdateSetTypes`

```go
func checkUpdateSetTypes(tokens []sqlToken, table string, schema *Schema, context string) []Diagnostic
```

checkUpdateSetTypes validates that UPDATE SET literal values match column types.

### `checkWebhookSecrets`

```go
func checkWebhookSecrets(app *parser.App) []Diagnostic
```

checkWebhookSecrets warns when webhooks don't have a secret configured.

### `checkWhereTypes`

```go
func checkWhereTypes(tokens []sqlToken, tableRefs []tableRef, aliasToTable map[string]string, schema *Schema, context string) []Diagnostic
```

checkWhereTypes validates type compatibility in WHERE clause comparisons.

### `countEachEnclosing`

```go
func countEachEnclosing(pos int, blocks []eachBlock) []string
```

countEachEnclosing returns the names of {{each}} blocks that enclose the given position.

### `customKindToFieldType`

```go
func customKindToFieldType(kind parser.CustomFieldKind) parser.FieldType
```

customKindToFieldType maps a manifest field kind to a parser FieldType for schema purposes.

### `editDistance`

```go
func editDistance(a, b string) int
```
### `extractInsertColumns`

```go
func extractInsertColumns(tokens []sqlToken) (string, []string)
```
### `extractNamedParams`

```go
func extractNamedParams(tokens []sqlToken) []string
```
### `extractSelectAliases`

```go
func extractSelectAliases(tokens []sqlToken) map[string]bool
```

checkTableColumnRefs validates that columns declared in table components
exist in the model referenced by the table's query.
extractSelectAliases returns the set of AS aliases defined in a SELECT query.
For example, "SELECT count(*) as total, c.name as contact FROM ..." returns {"total", "contact"}.

### `extractSelectColumns`

```go
func extractSelectColumns(tokens []sqlToken) []columnRef
```
### `extractSubqueries`

```go
func extractSubqueries(tokens []sqlToken) []string
```

extractSubqueries finds inner SELECT statements inside WHERE ... IN (SELECT ...) clauses
and returns them as raw SQL strings for independent validation.

### `extractTableRefs`

```go
func extractTableRefs(tokens []sqlToken) []tableRef
```
### `extractURLParams`

```go
func extractURLParams(path string) map[string]bool
```
### `extractUpdateColumns`

```go
func extractUpdateColumns(tokens []sqlToken) (string, []string)
```
### `extractWhereComparisons`

```go
func extractWhereComparisons(tokens []sqlToken) []sqlComparison
```

extractWhereComparisons extracts simple col op value comparisons from WHERE clauses.

### `findActionModel`

```go
func findActionModel(action parser.Page, app *parser.App) string
```
### `findClosestMatch`

```go
func findClosestMatch(target string, candidates map[string]bool) string
```
### `findEachBlocks`

```go
func findEachBlocks(text string) []eachBlock
```

findEachBlocks returns all {{each}} blocks in the given text, including nested ones.

### `findMatchingEnd`

```go
func findMatchingEnd(content string) (body, elseBody string, endPos int)
```

checkTemplateInterpolations validates {queryName.field} references inside
HTML content of pages, fragments, and layouts against the schema.
findMatchingEnd finds the body, else body, and position after {{end}} for a block,
accounting for nested {{each}}/{{if}} blocks.

### `hasMutatingSQL`

```go
func hasMutatingSQL(nodes []parser.Node) bool
```

hasMutatingSQL checks if any nodes contain INSERT, UPDATE, or DELETE queries.

### `hasRawHTMLForm`

```go
func hasRawHTMLForm(nodes []parser.Node) bool
```

hasRawHTMLForm checks if any NodeHTML in the tree contains a <form tag.

### `inferTokenType`

```go
func inferTokenType(tok sqlToken, tableName string, aliasToTable map[string]string, schema *Schema) *ExprType
```

inferTokenType determines the type category of a SQL token based on schema.

### `isClauseKeyword`

```go
func isClauseKeyword(s string) bool
```
### `isDBDynamic`

```go
func isDBDynamic(app *parser.App, modelName string) bool
```

isDBDynamic reports whether a model opts into DB-backed runtime field definitions.

### `isDynamicManifest`

```go
func isDynamicManifest(app *parser.App, modelName string) bool
```

isDynamicManifest reports whether a model uses a dynamic manifest path or dynamic DB fields.

### `populateSourceModels`

```go
func populateSourceModels(app *parser.App)
```

populateSourceModels sets Node.SourceModel on all query nodes by extracting
the primary table name from each query's SQL FROM clause.

### `queryModelMap`

```go
func queryModelMap(pages []parser.Page, fragments []parser.Page, apis []parser.Page) map[string]string
```

queryModelMap builds a mapping from query name to the primary table (model)
referenced in that query's SQL, by scanning all nodes in the given slices.

### `splitArgStr`

```go
func splitArgStr(s string) []string
```

splitArgStr tokenizes a fragment call argument string into space-separated
tokens, respecting single and double quoted substrings. Whitespace inside
quotes is preserved within the token (e.g. title="Hello World").

Duplicated in internal/runtime/render.go to keep the analyzer free of
runtime imports; they share the same semantics.

### `tokenizeSQL`

```go
func tokenizeSQL(sql string) []sqlToken
```
### `typesCompatible`

```go
func typesCompatible(a, b TypeCategory) bool
```

typesCompatible checks if two type categories can be compared.
Bool is compatible with numeric because SQLite stores bools as INTEGER.

### `validateMinMax`

```go
func validateMinMax(val string, ft parser.FieldType) error
```
### `walkLLMNodes`

```go
func walkLLMNodes(app *parser.App, fn func(node parser.Node, where string))
```

walkLLMNodes invokes fn for every NodeLLM in pages, actions, fragments,
APIs, jobs, schedules, and webhooks.


## Notes

<!-- MANUAL-NOTES START -->
# Package `internal/analyzer`

Source: [analyzer.go](../../../internal/analyzer/analyzer.go), [db_check.go](../../../internal/analyzer/db_check.go), [security.go](../../../internal/analyzer/security.go), [types.go](../../../internal/analyzer/types.go), [doc.go](../../../internal/analyzer/doc.go).

## Purpose

Run static analysis on the parser AST: type checking of field constraints, validation of route templates, permission and role checks, dead link detection, SQL coherence against declared models, and a security audit (CSRF, password exposure, missing auth on mutating endpoints). The analyzer powers `kilnx check` and is also called as part of `kilnx run` and `kilnx build`.

## Pipeline position

```
parser.App -> analyzer.Analyze -> []Diagnostic -> CLI/CI
              optionally analyzer.AnalyzeWithDB(app, db) for live DB checks
```

The analyzer runs after parsing and before the optimizer or runtime. It does not mutate the AST except to populate `Node.SourceModel` via `populateSourceModels`, which the optimizer and runtime consult.

## Public API

```go
type Diagnostic struct {
    Level   string // "error" or "warning"
    Message string
    Line    int    // source line, 0 if unknown
    Context string // e.g. "page /users"
}

type TableInfo struct {
    Name    string
    Columns map[string]*ColumnInfo
}

type ColumnInfo struct {
    FieldType parser.FieldType
}

type ModelFieldInfo struct {
    FormFields    map[string]bool
    FieldToColumn map[string]string
    ColumnToField map[string]string
}

type Schema struct {
    Tables      map[string]*TableInfo
    ModelFields map[string]*ModelFieldInfo
}

func Analyze(app *parser.App) []Diagnostic
func AnalyzeWithDB(app *parser.App, db *database.DB) []Diagnostic
func BuildSchema(models []parser.Model, manifests ...map[string]*parser.CustomFieldManifest) *Schema
```

`Diagnostic.Level` is a string, not an enum: it is either `"error"` or `"warning"`. Callers filter on the literal.

`Analyze` runs every check unconditionally and returns all findings concatenated. `AnalyzeWithDB` runs the same checks then, if `db` is not nil, adds `checkDynamicFieldRefsWithDB`, which queries each `_<model>_field_defs` table to validate hardcoded `{q.custom.X}` template references against the live runtime field definitions.

`BuildSchema` constructs the compile time view of the database from declared models. It synthesises an `id` column, rewrites `reference` fields to their `<name>_id` columns, and (when manifests are passed) materialises column mode custom fields as real columns.

## Sub checks

Dispatched in order from `Analyze`:

- **Model defaults and ranges**: `checkModelDefaults`, `checkModelMinMax` validate `default`, `min`, `max` literals against the declared field type.
- **Auth references**: `checkAuthRef`, `checkAuthPages` verify the `auth` block points at a real model and that `login`, `register`, and similar paths are reachable.
- **Model and tenant references**: `checkModelRefs`, `checkTenantRefs` ensure `reference` fields and `tenant: model` directives resolve.
- **Constraint coherence**: `checkUniqueConstraints`, `checkIndexes` validate column groups against model fields.
- **SQL analysis**: `checkAllSQL` walks every `NodeQuery` body, tokenises the SQL via `tokenizeSQL`, extracts table refs, named params, INSERT and UPDATE column lists, and flags unknown tables, unknown columns, missing parameters, and incompatible WHERE comparisons (`checkWhereTypes`, `checkInsertValueTypes`, `checkUpdateSetTypes`).
- **Template interpolation**: `checkTemplateInterpolations` resolves `{q.field}` and `{^field}` references inside HTML and text bodies against the SELECT alias set, including `{{each}}` enclosure depth tracking.
- **Custom fields**: `checkCustomFieldRefs`, `checkSQLCustomFieldRefs`, `checkCustomManifestRefs` validate manifest backed and dynamic field references.
- **Fragment components**: `checkFragmentComponents` validates component invocations and required argument coverage.
- **Translations**: `checkTranslationParams` cross checks `{tr key}` calls.
- **Action attributes**: `checkActionAttributes` flags unsupported attribute combinations.
- **Security** (in [security.go](../../../internal/analyzer/security.go)): `checkUnauthActions`, `checkUnauthAPIs`, `checkUnauthStreams`, `checkUnauthSockets`, `checkWebhookSecrets`, `checkPasswordExposure`, `checkAuthWithoutPermissions`, `checkCSRFProtection`. Mutating endpoints without `requires` raise warnings; password columns leaking into SELECT lists raise errors; raw HTML forms posting to actions without CSRF tokens are flagged.
- **DB coherence** (in [db_check.go](../../../internal/analyzer/db_check.go)): `AnalyzeWithDB` adds `checkDynamicFieldRefsWithDB`, which compares hardcoded `{q.custom.X}` to the actual `_<model>_field_defs` rows.

## File map

- [`analyzer.go`](../../../internal/analyzer/analyzer.go): `Analyze`, schema construction, model and SQL checks, fragment and translation checks, SQL tokeniser, named param extraction.
- [`types.go`](../../../internal/analyzer/types.go): `ColumnInfo`, `TypeCategory`, default/min/max validators, WHERE/INSERT/UPDATE type checks, `{{each}}` aware template interpolation checker, custom field reference checks.
- [`security.go`](../../../internal/analyzer/security.go): all `checkUnauth*`, `checkWebhookSecrets`, `checkPasswordExposure`, `checkAuthWithoutPermissions`, `checkCSRFProtection`, mutating SQL detection.
- [`db_check.go`](../../../internal/analyzer/db_check.go): `AnalyzeWithDB` plus DB backed `_field_defs` validation.
- [`doc.go`](../../../internal/analyzer/doc.go): package doc.
- Test files (`analyzer_test.go`, `security_test.go`, `action_attribute_test.go`, `fragment_component_test.go`, `translation_params_test.go`, `bench_test.go`) cover the corresponding checks.

## Key behaviors and gotchas

**Severity is by string literal.** `"error"` blocks `kilnx run` and `kilnx build`; `"warning"` is informational only. There is no separate `Info` level.

**Schema is a compile time projection, not the live DB.** `BuildSchema` does not connect to the database. It is built solely from `parser.Model`. This is why dynamic field validation against `_field_defs` rows requires `AnalyzeWithDB` plus a `*database.DB` handle.

**Reference fields surface twice.** A `reference` field named `author` produces both an `author` form field (mapped via `ModelFieldInfo`) and an `author_id` column (in `TableInfo`). SQL checks see the `_id` column; form and template checks see the bare name. Mismatches between the two are a frequent source of analyzer findings.

**Tenant columns are synthesised.** A model with `tenant: org` is not a parse error if it omits an `org_id` field: `BuildSchema` and the runtime add it. The analyzer accepts `WHERE org_id = :org_id` style queries against such tables.

**SQL tokeniser is hand rolled.** `tokenizeSQL` in [analyzer.go](../../../internal/analyzer/analyzer.go) understands strings, identifiers, numbers, named params (`:name`), and SQL operators. It is approximate: extremely exotic SQL may parse incorrectly. When that happens the relevant check usually returns no diagnostic rather than a false positive.

**Interpolation analysis is `{{each}}` aware.** `findEachBlocks` and `countEachEnclosing` (in [types.go](../../../internal/analyzer/types.go)) build the same enclosure model the optimizer uses for `{^field}` resolution. Modifying one side requires keeping the other in step.

**`populateSourceModels` mutates the AST.** It sets `Node.SourceModel` on every `NodeQuery` so the analyzer (and downstream optimizer and runtime) can answer "which model owns this query" without re-parsing the SQL. This is the only side effect `Analyze` produces on the AST.

**Custom manifest checks tolerate absence.** `checkCustomManifestRefs` short circuits when `app.CustomManifests` is empty, so models without `*_fields.kilnx` files do not generate spurious findings.
<!-- MANUAL-NOTES END -->
