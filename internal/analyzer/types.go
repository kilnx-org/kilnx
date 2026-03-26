package analyzer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

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
