package analyzer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// templateInterpolationRe matches {queryName.field} patterns in HTML content.
var templateInterpolationRe = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\.([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// ColumnInfo holds the inferred type for a single database column.
type ColumnInfo struct {
	FieldType parser.FieldType
}

// TypeCategory groups field types for compatibility checking.
type TypeCategory int

const (
	CategoryText    TypeCategory = iota // text, email, richtext, password, image, phone, option
	CategoryNumeric                     // int, float
	CategoryBool                        // bool
	CategoryTime                        // timestamp
)

func categoryOf(ft parser.FieldType) TypeCategory {
	switch ft {
	case parser.FieldInt, parser.FieldFloat:
		return CategoryNumeric
	case parser.FieldBool:
		return CategoryBool
	case parser.FieldTimestamp:
		return CategoryTime
	default:
		return CategoryText
	}
}

func categoryName(c TypeCategory) string {
	switch c {
	case CategoryNumeric:
		return "numeric"
	case CategoryBool:
		return "bool"
	case CategoryTime:
		return "timestamp"
	default:
		return "text"
	}
}

// typesCompatible checks if two type categories can be compared.
// Bool is compatible with numeric because SQLite stores bools as INTEGER.
func typesCompatible(a, b TypeCategory) bool {
	if a == b {
		return true
	}
	if (a == CategoryBool && b == CategoryNumeric) || (a == CategoryNumeric && b == CategoryBool) {
		return true
	}
	return false
}

// --- Model-level type checks ---

func checkModelDefaults(models []parser.Model) []Diagnostic {
	var diags []Diagnostic
	for _, m := range models {
		ctx := fmt.Sprintf("model %s", m.Name)
		for _, f := range m.Fields {
			if f.Default == "" {
				continue
			}
			switch f.Type {
			case parser.FieldInt:
				if _, err := strconv.Atoi(f.Default); err != nil {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("field '%s' is type int but has default value '%s' which is not a valid integer", f.Name, f.Default),
						Context: ctx,
					})
				}
			case parser.FieldFloat:
				if _, err := strconv.ParseFloat(f.Default, 64); err != nil {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("field '%s' is type float but has default value '%s' which is not a valid number", f.Name, f.Default),
						Context: ctx,
					})
				}
			case parser.FieldBool:
				if f.Default != "true" && f.Default != "false" {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("field '%s' is type bool but has default value '%s' - use 'true' or 'false'", f.Name, f.Default),
						Context: ctx,
					})
				}
			case parser.FieldOption:
				if len(f.Options) > 0 {
					found := false
					for _, opt := range f.Options {
						if opt == f.Default {
							found = true
							break
						}
					}
					if !found {
						diags = append(diags, Diagnostic{
							Level:   "error",
							Message: fmt.Sprintf("field '%s' has default '%s' but valid options are: %s", f.Name, f.Default, strings.Join(f.Options, ", ")),
							Context: ctx,
						})
					}
				}
			}
		}
	}
	return diags
}

func checkModelMinMax(models []parser.Model) []Diagnostic {
	var diags []Diagnostic
	for _, m := range models {
		ctx := fmt.Sprintf("model %s", m.Name)
		for _, f := range m.Fields {
			if f.Min != "" {
				if err := validateMinMax(f.Min, f.Type); err != nil {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("field '%s' has invalid min value '%s': %s", f.Name, f.Min, err),
						Context: ctx,
					})
				}
			}
			if f.Max != "" {
				if err := validateMinMax(f.Max, f.Type); err != nil {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("field '%s' has invalid max value '%s': %s", f.Name, f.Max, err),
						Context: ctx,
					})
				}
			}
		}
	}
	return diags
}

func validateMinMax(val string, ft parser.FieldType) error {
	switch ft {
	case parser.FieldInt:
		if _, err := strconv.Atoi(val); err != nil {
			return fmt.Errorf("expected integer for int field")
		}
	case parser.FieldFloat:
		if _, err := strconv.ParseFloat(val, 64); err != nil {
			return fmt.Errorf("expected number for float field")
		}
	case parser.FieldText, parser.FieldEmail, parser.FieldPassword, parser.FieldPhone:
		if n, err := strconv.Atoi(val); err != nil || n < 0 {
			return fmt.Errorf("expected non-negative integer (string length)")
		}
	}
	return nil
}

// --- SQL type checking ---

// ExprType represents the inferred type of a SQL expression.
type ExprType struct {
	Category TypeCategory
	Source   string // description for error messages
}

// sqlComparison represents a binary comparison in a WHERE clause.
type sqlComparison struct {
	left       sqlToken
	right      sqlToken
	leftTable  string
	rightTable string
	op         string
}

// extractWhereComparisons extracts simple col op value comparisons from WHERE clauses.
func extractWhereComparisons(tokens []sqlToken) []sqlComparison {
	var comps []sqlComparison
	inWhere := false
	parenDepth := 0

	for i := 0; i < len(tokens); i++ {
		if tokens[i].typ == stKeyword && tokens[i].lower == "where" {
			inWhere = true
			continue
		}
		if !inWhere {
			continue
		}
		if tokens[i].typ == stKeyword && parenDepth == 0 {
			switch tokens[i].lower {
			case "order", "group", "limit", "having", "union":
				return comps
			}
		}
		if tokens[i].typ == stPunct && tokens[i].value == "(" {
			parenDepth++
			continue
		}
		if tokens[i].typ == stPunct && tokens[i].value == ")" {
			if parenDepth > 0 {
				parenDepth--
			}
			continue
		}
		if parenDepth > 0 {
			continue
		}

		if i+2 < len(tokens) && tokens[i+1].typ == stOperator {
			op := tokens[i+1].value
			if op == "=" || op == "!=" || op == "<" || op == ">" || op == "<=" || op == ">=" {
				left := tokens[i]
				right := tokens[i+2]
				c := sqlComparison{left: left, right: right, op: op}

				if i >= 2 && tokens[i-1].typ == stPunct && tokens[i-1].value == "." && tokens[i-2].typ == stIdent {
					c.leftTable = tokens[i-2].lower
				}
				if i+4 < len(tokens) && tokens[i+3].typ == stPunct && tokens[i+3].value == "." && tokens[i+4].typ == stIdent {
					c.rightTable = tokens[i+2].lower
					c.right = tokens[i+4]
				}

				comps = append(comps, c)
			}
		}
	}
	return comps
}

// inferTokenType determines the type category of a SQL token based on schema.
func inferTokenType(tok sqlToken, tableName string, aliasToTable map[string]string, schema *Schema) *ExprType {
	switch tok.typ {
	case stString:
		return &ExprType{Category: CategoryText, Source: fmt.Sprintf("string literal %s", tok.value)}
	case stNumber:
		return &ExprType{Category: CategoryNumeric, Source: fmt.Sprintf("number %s", tok.value)}
	case stIdent:
		realTable := tableName
		if aliasToTable != nil {
			if mapped, ok := aliasToTable[tableName]; ok {
				realTable = mapped
			}
		}
		if realTable == "" {
			for _, info := range schema.Tables {
				if col, ok := info.Columns[tok.lower]; ok {
					return &ExprType{
						Category: categoryOf(col.FieldType),
						Source:   fmt.Sprintf("column '%s' (%s)", tok.lower, string(col.FieldType)),
					}
				}
			}
			return nil
		}
		if info, ok := schema.Tables[realTable]; ok {
			if col, ok := info.Columns[tok.lower]; ok {
				return &ExprType{
					Category: categoryOf(col.FieldType),
					Source:   fmt.Sprintf("column '%s' (%s)", tok.lower, string(col.FieldType)),
				}
			}
		}
		return nil
	case stKeyword:
		if tok.lower == "true" || tok.lower == "false" {
			return &ExprType{Category: CategoryBool, Source: fmt.Sprintf("boolean %s", tok.lower)}
		}
		if tok.lower == "null" {
			return nil
		}
		return nil
	case stParam:
		return nil
	default:
		return nil
	}
}

// checkWhereTypes validates type compatibility in WHERE clause comparisons.
func checkWhereTypes(tokens []sqlToken, tableRefs []tableRef, aliasToTable map[string]string, schema *Schema, context string) []Diagnostic {
	var diags []Diagnostic
	comps := extractWhereComparisons(tokens)

	defaultTable := ""
	if len(tableRefs) == 1 {
		defaultTable = tableRefs[0].name
	}

	for _, c := range comps {
		leftTable := c.leftTable
		if leftTable == "" {
			leftTable = defaultTable
		}
		rightTable := c.rightTable
		if rightTable == "" {
			rightTable = defaultTable
		}

		leftType := inferTokenType(c.left, leftTable, aliasToTable, schema)
		rightType := inferTokenType(c.right, rightTable, aliasToTable, schema)

		if leftType == nil || rightType == nil {
			continue
		}

		if !typesCompatible(leftType.Category, rightType.Category) {
			if leftType.Category == CategoryBool && rightType.Category == CategoryText {
				val := strings.Trim(c.right.value, "'")
				if val == "true" || val == "false" {
					diags = append(diags, Diagnostic{
						Level:   "warning",
						Message: fmt.Sprintf("comparing %s with %s - use 1 (true) or 0 (false) instead of string", leftType.Source, rightType.Source),
						Context: context,
					})
					continue
				}
			}
			diags = append(diags, Diagnostic{
				Level:   "error",
				Message: fmt.Sprintf("comparing %s with %s - these types are not compatible", leftType.Source, rightType.Source),
				Context: context,
			})
		}
	}
	return diags
}

// checkInsertValueTypes validates that INSERT literal values match column types.
func checkInsertValueTypes(tokens []sqlToken, table string, schema *Schema, context string) []Diagnostic {
	info, ok := schema.Tables[table]
	if !ok {
		return nil
	}

	var cols []string
	var vals []sqlToken
	inCols := false
	inVals := false
	parenDepth := 0

	for i := 0; i < len(tokens); i++ {
		if tokens[i].typ == stKeyword && tokens[i].lower == "values" {
			inCols = false
			continue
		}
		if tokens[i].typ == stPunct && tokens[i].value == "(" {
			parenDepth++
			if parenDepth == 1 {
				if !inVals && len(cols) == 0 {
					inCols = true
				} else {
					inVals = true
				}
			}
			continue
		}
		if tokens[i].typ == stPunct && tokens[i].value == ")" {
			parenDepth--
			if parenDepth == 0 {
				inCols = false
				inVals = false
			}
			continue
		}
		if tokens[i].typ == stPunct && tokens[i].value == "," {
			continue
		}
		if inCols && tokens[i].typ == stIdent {
			cols = append(cols, tokens[i].lower)
		}
		if inVals && parenDepth == 1 {
			vals = append(vals, tokens[i])
		}
	}

	var diags []Diagnostic
	for i := 0; i < len(cols) && i < len(vals); i++ {
		colInfo, ok := info.Columns[cols[i]]
		if !ok {
			continue
		}
		valType := inferTokenType(vals[i], "", nil, schema)
		if valType == nil {
			continue
		}
		colCat := categoryOf(colInfo.FieldType)
		if !typesCompatible(colCat, valType.Category) {
			diags = append(diags, Diagnostic{
				Level:   "error",
				Message: fmt.Sprintf("inserting %s into column '%s' which is type %s", valType.Source, cols[i], string(colInfo.FieldType)),
				Context: context,
			})
		}
	}
	return diags
}

// checkUpdateSetTypes validates that UPDATE SET literal values match column types.
func checkUpdateSetTypes(tokens []sqlToken, table string, schema *Schema, context string) []Diagnostic {
	info, ok := schema.Tables[table]
	if !ok {
		return nil
	}

	var diags []Diagnostic
	inSet := false
	for i := 0; i < len(tokens); i++ {
		if tokens[i].typ == stKeyword && tokens[i].lower == "set" {
			inSet = true
			continue
		}
		if !inSet {
			continue
		}
		if tokens[i].typ == stKeyword && (tokens[i].lower == "where" || tokens[i].lower == "order" || tokens[i].lower == "limit") {
			break
		}
		if tokens[i].typ == stIdent && i+2 < len(tokens) && tokens[i+1].typ == stOperator && tokens[i+1].value == "=" {
			col := tokens[i].lower
			val := tokens[i+2]

			colInfo, ok := info.Columns[col]
			if !ok {
				continue
			}
			valType := inferTokenType(val, "", nil, schema)
			if valType == nil {
				continue
			}
			colCat := categoryOf(colInfo.FieldType)
			if !typesCompatible(colCat, valType.Category) {
				diags = append(diags, Diagnostic{
					Level:   "error",
					Message: fmt.Sprintf("setting column '%s' (%s) to %s - these types are not compatible", col, string(colInfo.FieldType), valType.Source),
					Context: context,
				})
			}
		}
	}
	return diags
}

// --- Template interpolation and component column checks ---

// queryModelMap builds a mapping from query name to the primary table (model)
// referenced in that query's SQL, by scanning all nodes in the given slices.
func queryModelMap(pages []parser.Page, fragments []parser.Page, apis []parser.Page) map[string]string {
	m := make(map[string]string)
	collect := func(nodes []parser.Node) {
		for _, n := range nodes {
			if n.Type == parser.NodeQuery && n.Name != "" && n.SQL != "" {
				tokens := tokenizeSQL(n.SQL)
				refs := extractTableRefs(tokens)
				if len(refs) > 0 {
					m[n.Name] = refs[0].name
				}
			}
		}
	}
	for _, p := range pages {
		collect(p.Body)
	}
	for _, f := range fragments {
		collect(f.Body)
	}
	for _, a := range apis {
		collect(a.Body)
	}
	return m
}

// builtinFields are virtual fields available on any query result that should
// not trigger a validation error (e.g. {users.count}).
var builtinFields = map[string]bool{
	"count": true,
}

// reservedInterpolations are top-level names that the runtime resolves
// without requiring a matching query (e.g. {page.title}, {page.content}).
var reservedInterpolations = map[string]bool{
	"page":  true,
	"kilnx": true,
	"t":     true,
}

// checkTemplateInterpolations validates {queryName.field} references inside
// HTML content of pages, fragments, and layouts against the schema.
func checkTemplateInterpolations(app *parser.App, schema *Schema) []Diagnostic {
	qMap := queryModelMap(app.Pages, app.Fragments, app.APIs)

	var diags []Diagnostic

	scanNodes := func(nodes []parser.Node, context string) {
		// First pass: collect query names defined in this scope.
		localQueries := make(map[string]string)
		for _, n := range nodes {
			if n.Type == parser.NodeQuery && n.Name != "" && n.SQL != "" {
				tokens := tokenizeSQL(n.SQL)
				refs := extractTableRefs(tokens)
				if len(refs) > 0 {
					localQueries[n.Name] = refs[0].name
				}
			}
		}

		for _, n := range nodes {
			html := ""
			switch n.Type {
			case parser.NodeHTML:
				html = n.HTMLContent
			}
			if html == "" {
				continue
			}
			matches := templateInterpolationRe.FindAllStringSubmatch(html, -1)
			for _, m := range matches {
				queryName := m[1]
				fieldName := m[2]

				if reservedInterpolations[queryName] {
					continue
				}
				if builtinFields[fieldName] {
					continue
				}

				// Resolve the model for this query name.
				modelName := ""
				if mn, ok := localQueries[queryName]; ok {
					modelName = mn
				} else if mn, ok := qMap[queryName]; ok {
					modelName = mn
				}

				if modelName == "" {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("template reference '{%s.%s}' uses unknown query '%s'", queryName, fieldName, queryName),
						Context: context,
					})
					continue
				}

				info, ok := schema.Tables[modelName]
				if !ok {
					continue // model itself not found; already reported elsewhere
				}
				if _, colExists := info.Columns[fieldName]; !colExists {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("template reference '{%s.%s}': field '%s' does not exist in model '%s'", queryName, fieldName, fieldName, modelName),
						Context: context,
					})
				}
			}
		}
	}

	for _, p := range app.Pages {
		scanNodes(p.Body, fmt.Sprintf("page %s", p.Path))
	}
	for _, f := range app.Fragments {
		scanNodes(f.Body, fmt.Sprintf("fragment %s", f.Path))
	}

	// Layouts use {page.title}, {page.content}, {nav}, {kilnx.css}, {kilnx.js}
	// which are all reserved. We still scan for user query refs if any.
	for _, l := range app.Layouts {
		if l.HTMLContent == "" {
			continue
		}
		matches := templateInterpolationRe.FindAllStringSubmatch(l.HTMLContent, -1)
		for _, m := range matches {
			queryName := m[1]
			fieldName := m[2]

			if reservedInterpolations[queryName] {
				continue
			}
			if builtinFields[fieldName] {
				continue
			}

			if _, ok := qMap[queryName]; !ok {
				diags = append(diags, Diagnostic{
					Level:   "error",
					Message: fmt.Sprintf("template reference '{%s.%s}' uses unknown query '%s'", queryName, fieldName, queryName),
					Context: fmt.Sprintf("layout %s", l.Name),
				})
			}
		}
	}

	return diags
}

// checkTableColumnRefs validates that columns declared in table components
// exist in the model referenced by the table's query.
// extractSelectAliases returns the set of AS aliases defined in a SELECT query.
// For example, "SELECT count(*) as total, c.name as contact FROM ..." returns {"total", "contact"}.
func extractSelectAliases(tokens []sqlToken) map[string]bool {
	aliases := make(map[string]bool)
	for i := 0; i < len(tokens); i++ {
		if tokens[i].typ == stKeyword && tokens[i].lower == "as" && i+1 < len(tokens) {
			next := tokens[i+1]
			// Accept both identifiers and keywords as alias names (e.g., "count", "date", "type")
			if next.typ == stIdent || next.typ == stKeyword {
				aliases[next.lower] = true
			}
		}
	}
	return aliases
}

func checkTableColumnRefs(app *parser.App, schema *Schema) []Diagnostic {
	qMap := queryModelMap(app.Pages, app.Fragments, app.APIs)

	var diags []Diagnostic

	// Store query SQL for alias extraction
	type queryInfo struct {
		modelName string
		sql       string
	}

	scanNodes := func(nodes []parser.Node, context string) {
		// Collect local query-to-model mappings and SQL.
		localQueries := make(map[string]queryInfo)
		for _, n := range nodes {
			if n.Type == parser.NodeQuery && n.Name != "" && n.SQL != "" {
				tokens := tokenizeSQL(n.SQL)
				refs := extractTableRefs(tokens)
				if len(refs) > 0 {
					localQueries[n.Name] = queryInfo{modelName: refs[0].name, sql: n.SQL}
				}
			}
		}

		for _, n := range nodes {
			if n.Type != parser.NodeTable || n.Name == "" || len(n.Columns) == 0 {
				continue
			}

			qi, hasLocal := localQueries[n.Name]
			modelName := ""
			querySql := ""
			if hasLocal {
				modelName = qi.modelName
				querySql = qi.sql
			} else if mn, ok := qMap[n.Name]; ok {
				modelName = mn
			}
			if modelName == "" {
				continue // query not found; not our concern here
			}

			info, ok := schema.Tables[modelName]
			if !ok {
				continue
			}

			// Build a set of known columns: model columns + SQL aliases + columns from all joined tables
			knownCols := make(map[string]bool)
			for col := range info.Columns {
				knownCols[col] = true
			}

			if querySql != "" {
				tokens := tokenizeSQL(querySql)
				// Add SQL aliases (e.g., count(*) as total, c.name as contact)
				for alias := range extractSelectAliases(tokens) {
					knownCols[alias] = true
				}
				// Add columns from all joined tables
				refs := extractTableRefs(tokens)
				for _, ref := range refs {
					if joinInfo, ok := schema.Tables[ref.name]; ok {
						for col := range joinInfo.Columns {
							knownCols[col] = true
						}
					}
				}
			}

			for _, col := range n.Columns {
				if !knownCols[col.Field] {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("table column '%s' does not exist in model '%s' (query '%s')", col.Field, modelName, n.Name),
						Context: context,
					})
				}
			}
		}
	}

	for _, p := range app.Pages {
		scanNodes(p.Body, fmt.Sprintf("page %s", p.Path))
	}
	for _, f := range app.Fragments {
		scanNodes(f.Body, fmt.Sprintf("fragment %s", f.Path))
	}

	return diags
}
