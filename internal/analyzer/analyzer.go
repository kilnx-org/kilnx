package analyzer

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// Diagnostic represents a compile-time finding from static analysis.
type Diagnostic struct {
	Level   string // "error" or "warning"
	Message string
	Line    int    // source line (0 if unknown)
	Context string // location, e.g. "page /users"
}

// TableInfo holds the known columns for a model-derived table.
type TableInfo struct {
	Name    string
	Columns map[string]bool
}

// Schema is the compile-time view of the database derived from model declarations.
type Schema struct {
	Tables map[string]*TableInfo
}

func BuildSchema(models []parser.Model) *Schema {
	s := &Schema{Tables: make(map[string]*TableInfo)}
	for _, m := range models {
		info := &TableInfo{Name: m.Name, Columns: make(map[string]bool)}
		info.Columns["id"] = true
		for _, f := range m.Fields {
			if f.Type == parser.FieldReference {
				info.Columns[f.Name+"_id"] = true
			} else {
				info.Columns[f.Name] = true
			}
		}
		s.Tables[m.Name] = info
	}
	return s
}

// Analyze performs static analysis on a parsed Kilnx app, checking SQL
// references against declared models.
func Analyze(app *parser.App) []Diagnostic {
	var diags []Diagnostic
	schema := BuildSchema(app.Models)

	diags = append(diags, checkAuthRef(app, schema)...)
	diags = append(diags, checkModelRefs(app, schema)...)
	diags = append(diags, checkAllSQL(app, schema)...)

	return diags
}

func checkAuthRef(app *parser.App, schema *Schema) []Diagnostic {
	if app.Auth == nil {
		return nil
	}
	if _, ok := schema.Tables[app.Auth.Table]; !ok {
		return []Diagnostic{{
			Level:   "error",
			Message: fmt.Sprintf("auth references table '%s' which is not defined as a model", app.Auth.Table),
			Context: "auth",
		}}
	}
	return nil
}

func checkModelRefs(app *parser.App, schema *Schema) []Diagnostic {
	var diags []Diagnostic
	for _, m := range app.Models {
		for _, f := range m.Fields {
			if f.Type == parser.FieldReference {
				if _, ok := schema.Tables[f.Reference]; !ok {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("field '%s' references model '%s' which is not defined", f.Name, f.Reference),
						Context: fmt.Sprintf("model %s", m.Name),
					})
				}
			}
		}
	}
	return diags
}

func checkAllSQL(app *parser.App, schema *Schema) []Diagnostic {
	var diags []Diagnostic

	for _, p := range app.Pages {
		ctx := fmt.Sprintf("page %s", p.Path)
		diags = append(diags, analyzeNodes(p.Body, schema, ctx)...)
	}
	for _, a := range app.Actions {
		ctx := fmt.Sprintf("action %s", a.Path)
		diags = append(diags, analyzeNodes(a.Body, schema, ctx)...)
	}
	for _, f := range app.Fragments {
		ctx := fmt.Sprintf("fragment %s", f.Path)
		diags = append(diags, analyzeNodes(f.Body, schema, ctx)...)
	}
	for _, a := range app.APIs {
		ctx := fmt.Sprintf("api %s", a.Path)
		diags = append(diags, analyzeNodes(a.Body, schema, ctx)...)
	}
	for _, s := range app.Streams {
		if s.SQL != "" {
			ctx := fmt.Sprintf("stream %s", s.Path)
			diags = append(diags, analyzeSQL(s.SQL, schema, ctx)...)
		}
	}
	for _, s := range app.Schedules {
		ctx := fmt.Sprintf("schedule %s", s.Name)
		diags = append(diags, analyzeNodes(s.Body, schema, ctx)...)
	}
	for _, j := range app.Jobs {
		ctx := fmt.Sprintf("job %s", j.Name)
		diags = append(diags, analyzeNodes(j.Body, schema, ctx)...)
	}
	for _, wh := range app.Webhooks {
		for _, ev := range wh.Events {
			ctx := fmt.Sprintf("webhook %s on %s", wh.Path, ev.Name)
			diags = append(diags, analyzeNodes(ev.Body, schema, ctx)...)
		}
	}
	for _, sock := range app.Sockets {
		base := fmt.Sprintf("socket %s", sock.Path)
		diags = append(diags, analyzeNodes(sock.OnConnect, schema, base+" on connect")...)
		diags = append(diags, analyzeNodes(sock.OnMessage, schema, base+" on message")...)
		diags = append(diags, analyzeNodes(sock.OnDisconnect, schema, base+" on disconnect")...)
	}

	return diags
}

func analyzeNodes(nodes []parser.Node, schema *Schema, context string) []Diagnostic {
	var diags []Diagnostic
	for _, node := range nodes {
		switch node.Type {
		case parser.NodeQuery:
			if node.SQL != "" {
				diags = append(diags, analyzeSQL(node.SQL, schema, context)...)
			}
		case parser.NodeForm:
			if node.ModelName != "" {
				if _, ok := schema.Tables[node.ModelName]; !ok {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("form references model '%s' which is not defined", node.ModelName),
						Context: context,
					})
				}
			}
			if node.QuerySQL != "" {
				diags = append(diags, analyzeSQL(node.QuerySQL, schema, context)...)
			}
		case parser.NodeRespond:
			if node.QuerySQL != "" {
				diags = append(diags, analyzeSQL(node.QuerySQL, schema, context)...)
			}
		case parser.NodeValidate:
			if node.ModelName != "" {
				if _, ok := schema.Tables[node.ModelName]; !ok {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("validate references model '%s' which is not defined", node.ModelName),
						Context: context,
					})
				}
			}
		case parser.NodeSearch:
			diags = append(diags, checkSearchFields(node, schema, context)...)
		case parser.NodeOn:
			diags = append(diags, analyzeNodes(node.Children, schema, context)...)
		}
	}
	return diags
}

func checkSearchFields(node parser.Node, schema *Schema, context string) []Diagnostic {
	var diags []Diagnostic
	for _, field := range node.SearchFields {
		found := false
		for _, table := range schema.Tables {
			if table.Columns[field] {
				found = true
				break
			}
		}
		if !found {
			diags = append(diags, Diagnostic{
				Level:   "warning",
				Message: fmt.Sprintf("search field '%s' not found in any model", field),
				Context: context,
			})
		}
	}
	return diags
}

// --- SQL analysis ---

func analyzeSQL(sql string, schema *Schema, context string) []Diagnostic {
	var diags []Diagnostic
	tokens := tokenizeSQL(sql)
	if len(tokens) == 0 {
		return nil
	}

	tableRefs := extractTableRefs(tokens)
	aliasToTable := make(map[string]string)
	for _, ref := range tableRefs {
		if _, ok := schema.Tables[ref.name]; !ok {
			diags = append(diags, Diagnostic{
				Level:   "error",
				Message: fmt.Sprintf("table '%s' is not defined as a model", ref.name),
				Context: context,
			})
		}
		aliasToTable[ref.name] = ref.name
		if ref.alias != "" {
			aliasToTable[ref.alias] = ref.name
		}
	}

	stmtType := ""
	if len(tokens) > 0 && tokens[0].typ == stKeyword {
		stmtType = tokens[0].lower
	}

	switch stmtType {
	case "insert":
		table, cols := extractInsertColumns(tokens)
		if info, ok := schema.Tables[table]; ok {
			for _, col := range cols {
				if !info.Columns[col] {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("column '%s' does not exist in model '%s'", col, table),
						Context: context,
					})
				}
			}
		}
	case "update":
		table, cols := extractUpdateColumns(tokens)
		if info, ok := schema.Tables[table]; ok {
			for _, col := range cols {
				if !info.Columns[col] {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("column '%s' does not exist in model '%s'", col, table),
						Context: context,
					})
				}
			}
		}
	case "select":
		diags = append(diags, checkSelectColumns(tokens, tableRefs, aliasToTable, schema, context)...)
	}

	return diags
}

func checkSelectColumns(tokens []sqlToken, tableRefs []tableRef, aliasToTable map[string]string, schema *Schema, context string) []Diagnostic {
	var diags []Diagnostic
	cols := extractSelectColumns(tokens)
	if cols == nil {
		return nil
	}

	for _, col := range cols {
		if col.table != "" {
			realTable, ok := aliasToTable[col.table]
			if !ok {
				continue
			}
			if info, ok := schema.Tables[realTable]; ok {
				if !info.Columns[col.column] {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("column '%s' does not exist in model '%s'", col.column, realTable),
						Context: context,
					})
				}
			}
		} else if len(tableRefs) == 1 {
			table := tableRefs[0].name
			if info, ok := schema.Tables[table]; ok {
				if !info.Columns[col.column] {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("column '%s' does not exist in model '%s'", col.column, table),
						Context: context,
					})
				}
			}
		}
	}
	return diags
}

// --- SQL tokenizer ---

const (
	stKeyword = iota
	stIdent
	stString
	stNumber
	stParam
	stPunct
	stStar
	stOperator
)

type sqlToken struct {
	typ   int
	value string
	lower string
}

var sqlKeywords = map[string]bool{
	"select": true, "from": true, "where": true, "and": true, "or": true,
	"insert": true, "into": true, "values": true, "update": true, "set": true,
	"delete": true, "join": true, "left": true, "right": true, "inner": true,
	"outer": true, "cross": true, "on": true, "as": true, "order": true,
	"by": true, "group": true, "having": true, "limit": true, "offset": true,
	"desc": true, "asc": true, "not": true, "null": true, "is": true,
	"in": true, "like": true, "between": true, "exists": true, "union": true,
	"distinct": true, "all": true, "case": true, "when": true, "then": true,
	"else": true, "end": true, "true": true, "false": true,
	"count": true, "sum": true, "avg": true, "min": true, "max": true,
	"now": true, "datetime": true, "date": true, "time": true,
	"cast": true, "coalesce": true, "ifnull": true, "typeof": true,
	"interval": true, "replace": true, "substr": true, "length": true,
	"lower": true, "upper": true, "trim": true, "abs": true, "round": true,
}

func tokenizeSQL(sql string) []sqlToken {
	var tokens []sqlToken
	runes := []rune(sql)
	i := 0

	for i < len(runes) {
		ch := runes[i]

		if unicode.IsSpace(ch) {
			i++
			continue
		}

		if ch == '\'' {
			j := i + 1
			for j < len(runes) {
				if runes[j] == '\'' {
					if j+1 < len(runes) && runes[j+1] == '\'' {
						j += 2
						continue
					}
					break
				}
				j++
			}
			if j < len(runes) {
				j++
			}
			tokens = append(tokens, sqlToken{typ: stString, value: string(runes[i:j])})
			i = j
			continue
		}

		if ch == ':' {
			j := i + 1
			for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_' || runes[j] == '.') {
				j++
			}
			if j > i+1 {
				tokens = append(tokens, sqlToken{typ: stParam, value: string(runes[i:j])})
				i = j
				continue
			}
			tokens = append(tokens, sqlToken{typ: stPunct, value: ":"})
			i++
			continue
		}

		if unicode.IsDigit(ch) {
			j := i
			for j < len(runes) && (unicode.IsDigit(runes[j]) || runes[j] == '.') {
				j++
			}
			tokens = append(tokens, sqlToken{typ: stNumber, value: string(runes[i:j])})
			i = j
			continue
		}

		if ch == '*' {
			tokens = append(tokens, sqlToken{typ: stStar, value: "*"})
			i++
			continue
		}

		if ch == '|' && i+1 < len(runes) && runes[i+1] == '|' {
			tokens = append(tokens, sqlToken{typ: stOperator, value: "||"})
			i += 2
			continue
		}
		if ch == '<' || ch == '>' || ch == '=' || ch == '!' {
			j := i + 1
			if j < len(runes) && runes[j] == '=' {
				j++
			}
			tokens = append(tokens, sqlToken{typ: stOperator, value: string(runes[i:j])})
			i = j
			continue
		}

		if ch == '(' || ch == ')' || ch == ',' || ch == '.' || ch == ';' || ch == '+' || ch == '-' || ch == '%' {
			tokens = append(tokens, sqlToken{typ: stPunct, value: string(ch)})
			i++
			continue
		}

		if ch == '"' {
			j := i + 1
			for j < len(runes) && runes[j] != '"' {
				j++
			}
			if j < len(runes) {
				j++
			}
			word := string(runes[i+1 : j-1])
			tokens = append(tokens, sqlToken{typ: stIdent, value: word, lower: strings.ToLower(word)})
			i = j
			continue
		}

		if unicode.IsLetter(ch) || ch == '_' {
			j := i
			for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_') {
				j++
			}
			word := string(runes[i:j])
			lower := strings.ToLower(word)
			if sqlKeywords[lower] {
				tokens = append(tokens, sqlToken{typ: stKeyword, value: word, lower: lower})
			} else {
				tokens = append(tokens, sqlToken{typ: stIdent, value: word, lower: lower})
			}
			i = j
			continue
		}

		i++
	}

	return tokens
}

// --- SQL reference extraction ---

type tableRef struct {
	name  string
	alias string
}

type columnRef struct {
	table  string
	column string
}

func extractTableRefs(tokens []sqlToken) []tableRef {
	var refs []tableRef

	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.typ != stKeyword {
			continue
		}
		if t.lower != "from" && t.lower != "join" && t.lower != "into" && t.lower != "update" {
			continue
		}
		if i+1 >= len(tokens) || tokens[i+1].typ != stIdent {
			continue
		}

		ref := tableRef{name: tokens[i+1].lower}

		if i+2 < len(tokens) {
			next := tokens[i+2]
			if next.typ == stKeyword && next.lower == "as" && i+3 < len(tokens) && tokens[i+3].typ == stIdent {
				ref.alias = tokens[i+3].lower
			} else if next.typ == stIdent {
				nl := next.lower
				if !isClauseKeyword(nl) {
					ref.alias = nl
				}
			}
		}

		refs = append(refs, ref)
	}

	return refs
}

func isClauseKeyword(s string) bool {
	switch s {
	case "where", "on", "set", "order", "group", "having", "limit",
		"values", "inner", "left", "right", "outer", "cross", "join",
		"and", "or", "not", "union":
		return true
	}
	return false
}

func extractInsertColumns(tokens []sqlToken) (string, []string) {
	var table string
	var cols []string

	for i := 0; i < len(tokens)-1; i++ {
		if tokens[i].typ == stKeyword && tokens[i].lower == "into" && tokens[i+1].typ == stIdent {
			table = tokens[i+1].lower
			for j := i + 2; j < len(tokens); j++ {
				if tokens[j].typ == stPunct && tokens[j].value == "(" {
					for k := j + 1; k < len(tokens); k++ {
						if tokens[k].typ == stPunct && tokens[k].value == ")" {
							break
						}
						if tokens[k].typ == stIdent {
							cols = append(cols, tokens[k].lower)
						}
					}
					break
				}
				if tokens[j].typ == stKeyword && tokens[j].lower == "values" {
					break
				}
			}
			break
		}
	}

	return table, cols
}

func extractUpdateColumns(tokens []sqlToken) (string, []string) {
	var table string
	var cols []string

	for i := 0; i < len(tokens)-1; i++ {
		if tokens[i].typ == stKeyword && tokens[i].lower == "update" && tokens[i+1].typ == stIdent {
			table = tokens[i+1].lower
		}
		if tokens[i].typ == stKeyword && tokens[i].lower == "set" {
			for j := i + 1; j < len(tokens); j++ {
				if tokens[j].typ == stKeyword && (tokens[j].lower == "where" || tokens[j].lower == "order" || tokens[j].lower == "limit") {
					break
				}
				if tokens[j].typ == stIdent && j+1 < len(tokens) && tokens[j+1].typ == stOperator && tokens[j+1].value == "=" {
					cols = append(cols, tokens[j].lower)
				}
			}
			break
		}
	}

	return table, cols
}

// extractSelectColumns returns column references from a SELECT clause.
// Returns nil if SELECT * is used (meaning "all columns, no individual check").
func extractSelectColumns(tokens []sqlToken) []columnRef {
	var cols []columnRef
	inSelect := false
	parenDepth := 0

	for i := 0; i < len(tokens); i++ {
		if tokens[i].typ == stKeyword && tokens[i].lower == "select" {
			inSelect = true
			if i+1 < len(tokens) && tokens[i+1].typ == stKeyword && tokens[i+1].lower == "distinct" {
				i++
			}
			continue
		}
		if !inSelect {
			continue
		}

		if tokens[i].typ == stKeyword && tokens[i].lower == "from" && parenDepth == 0 {
			break
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

		if tokens[i].typ == stStar {
			return nil
		}

		// Skip aggregate/scalar function calls: func(...)
		if tokens[i].typ == stKeyword && i+1 < len(tokens) && tokens[i+1].typ == stPunct && tokens[i+1].value == "(" {
			depth := 1
			i += 2
			for i < len(tokens) && depth > 0 {
				if tokens[i].typ == stPunct && tokens[i].value == "(" {
					depth++
				}
				if tokens[i].typ == stPunct && tokens[i].value == ")" {
					depth--
				}
				i++
			}
			if i < len(tokens) && tokens[i].typ == stKeyword && tokens[i].lower == "as" {
				i++
				if i < len(tokens) {
					i++
				}
			}
			i--
			continue
		}

		if tokens[i].typ == stKeyword && tokens[i].lower == "case" {
			depth := 1
			i++
			for i < len(tokens) && depth > 0 {
				if tokens[i].typ == stKeyword && tokens[i].lower == "case" {
					depth++
				}
				if tokens[i].typ == stKeyword && tokens[i].lower == "end" {
					depth--
				}
				i++
			}
			if i < len(tokens) && tokens[i].typ == stKeyword && tokens[i].lower == "as" {
				i++
				if i < len(tokens) {
					i++
				}
			}
			i--
			continue
		}

		if tokens[i].typ == stIdent {
			if i+2 < len(tokens) && tokens[i+1].typ == stPunct && tokens[i+1].value == "." && tokens[i+2].typ == stIdent {
				cols = append(cols, columnRef{table: tokens[i].lower, column: tokens[i+2].lower})
				i += 2
			} else {
				cols = append(cols, columnRef{column: tokens[i].lower})
			}

			if i+1 < len(tokens) && tokens[i+1].typ == stKeyword && tokens[i+1].lower == "as" {
				i += 2
			}
			continue
		}

		if tokens[i].typ == stPunct && tokens[i].value == "," {
			continue
		}

		// String literal: skip, including any "as alias"
		if tokens[i].typ == stString {
			if i+1 < len(tokens) && tokens[i+1].typ == stKeyword && tokens[i+1].lower == "as" {
				i += 2
			}
			continue
		}

		// Number literal: skip, including any "as alias"
		if tokens[i].typ == stNumber {
			if i+1 < len(tokens) && tokens[i+1].typ == stKeyword && tokens[i+1].lower == "as" {
				i += 2
			}
			continue
		}
	}

	return cols
}
