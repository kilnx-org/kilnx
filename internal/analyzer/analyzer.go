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

// TableInfo holds the known columns and their types for a model-derived table.
type TableInfo struct {
	Name    string
	Columns map[string]*ColumnInfo
}

// ModelFieldInfo holds the form-level field names for a model.
// This maps model field names (what HTML forms send) to DB column names.
type ModelFieldInfo struct {
	FormFields    map[string]bool   // field names as sent by HTML forms
	FieldToColumn map[string]string // form field name -> DB column name
	ColumnToField map[string]string // DB column name -> form field name
}

// Schema is the compile-time view of the database derived from model declarations.
type Schema struct {
	Tables      map[string]*TableInfo
	ModelFields map[string]*ModelFieldInfo
}

// BuildSchema creates a compile-time view of the database from model declarations.
func BuildSchema(models []parser.Model) *Schema {
	s := &Schema{
		Tables:      make(map[string]*TableInfo),
		ModelFields: make(map[string]*ModelFieldInfo),
	}
	for _, m := range models {
		info := &TableInfo{Name: m.Name, Columns: make(map[string]*ColumnInfo)}
		mf := &ModelFieldInfo{
			FormFields:    make(map[string]bool),
			FieldToColumn: make(map[string]string),
			ColumnToField: make(map[string]string),
		}
		info.Columns["id"] = &ColumnInfo{FieldType: parser.FieldInt}
		mf.FormFields["id"] = true
		mf.FieldToColumn["id"] = "id"
		mf.ColumnToField["id"] = "id"
		for _, f := range m.Fields {
			if f.Type == parser.FieldReference {
				colName := f.Name + "_id"
				info.Columns[colName] = &ColumnInfo{FieldType: parser.FieldInt}
				mf.FormFields[f.Name] = true
				mf.FieldToColumn[f.Name] = colName
				mf.ColumnToField[colName] = f.Name
			} else {
				info.Columns[f.Name] = &ColumnInfo{FieldType: f.Type}
				mf.FormFields[f.Name] = true
				mf.FieldToColumn[f.Name] = f.Name
				mf.ColumnToField[f.Name] = f.Name
			}
		}
		s.Tables[m.Name] = info
		s.ModelFields[m.Name] = mf
	}
	return s
}

// Analyze performs static analysis on a parsed Kilnx app, checking SQL
// references against declared models and type compatibility.
func Analyze(app *parser.App) []Diagnostic {
	var diags []Diagnostic
	schema := BuildSchema(app.Models)

	diags = append(diags, checkModelDefaults(app.Models)...)
	diags = append(diags, checkModelMinMax(app.Models)...)
	diags = append(diags, checkAuthRef(app, schema)...)
	diags = append(diags, checkModelRefs(app, schema)...)
	diags = append(diags, checkAllSQL(app, schema)...)
	diags = append(diags, checkSecurity(app, schema)...)
	diags = append(diags, checkTemplateInterpolations(app, schema)...)
	diags = append(diags, checkTableColumnRefs(app, schema)...)

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
		modelName := findActionModel(a, app)
		diags = append(diags, checkActionParams(a, modelName, schema, ctx)...)
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

func checkActionParams(action parser.Page, modelName string, schema *Schema, context string) []Diagnostic {
	return checkActionParamsNodes(action.Body, action.Path, modelName, schema, context)
}

func checkActionParamsNodes(nodes []parser.Node, path string, modelName string, schema *Schema, context string) []Diagnostic {
	var diags []Diagnostic
	for _, node := range nodes {
		sql := ""
		switch {
		case node.Type == parser.NodeQuery && node.SQL != "":
			sql = node.SQL
		case (node.Type == parser.NodeForm || node.Type == parser.NodeRespond) && node.QuerySQL != "":
			sql = node.QuerySQL
		case node.Type == parser.NodeOn:
			diags = append(diags, checkActionParamsNodes(node.Children, path, modelName, schema, context)...)
		}
		if sql != "" {
			tokens := tokenizeSQL(sql)
			diags = append(diags, checkNamedParams(sql, tokens, path, modelName, schema, context)...)
		}
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
			if _, colExists := table.Columns[field]; colExists {
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
				if _, colExists := info.Columns[col]; !colExists {
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
				if _, colExists := info.Columns[col]; !colExists {
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

	// Type-level checks
	diags = append(diags, checkWhereTypes(tokens, tableRefs, aliasToTable, schema, context)...)
	switch stmtType {
	case "insert":
		tbl, _ := extractInsertColumns(tokens)
		diags = append(diags, checkInsertValueTypes(tokens, tbl, schema, context)...)
	case "update":
		tbl, _ := extractUpdateColumns(tokens)
		diags = append(diags, checkUpdateSetTypes(tokens, tbl, schema, context)...)
	}

	// Subquery validation
	for _, sub := range extractSubqueries(tokens) {
		diags = append(diags, analyzeSQL(sub, schema, context)...)
	}

	return diags
}

// extractSubqueries finds inner SELECT statements inside WHERE ... IN (SELECT ...) clauses
// and returns them as raw SQL strings for independent validation.
func extractSubqueries(tokens []sqlToken) []string {
	var subs []string
	for i := 0; i < len(tokens); i++ {
		// Look for IN ( SELECT pattern
		if tokens[i].typ != stKeyword || tokens[i].lower != "in" {
			continue
		}
		if i+2 >= len(tokens) {
			continue
		}
		if tokens[i+1].typ != stPunct || tokens[i+1].value != "(" {
			continue
		}
		if tokens[i+2].typ != stKeyword || tokens[i+2].lower != "select" {
			continue
		}
		// Found a subquery. Extract tokens from SELECT until matching closing paren.
		depth := 1
		start := i + 2
		j := start
		for j < len(tokens) && depth > 0 {
			j++
			if j >= len(tokens) {
				break
			}
			if tokens[j].typ == stPunct && tokens[j].value == "(" {
				depth++
			}
			if tokens[j].typ == stPunct && tokens[j].value == ")" {
				depth--
			}
		}
		// Rebuild the subquery SQL from tokens
		var parts []string
		for k := start; k < j; k++ {
			parts = append(parts, tokens[k].value)
		}
		if len(parts) > 0 {
			subs = append(subs, strings.Join(parts, " "))
		}
		i = j
	}
	return subs
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
				if _, colExists := info.Columns[col.column]; !colExists {
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
				if _, colExists := info.Columns[col.column]; !colExists {
					diags = append(diags, Diagnostic{
						Level:   "error",
						Message: fmt.Sprintf("column '%s' does not exist in model '%s'", col.column, table),
						Context: context,
					})
				}
			}
		} else if len(tableRefs) > 1 {
			found := false
			for _, ref := range tableRefs {
				if info, ok := schema.Tables[ref.name]; ok {
					if _, colExists := info.Columns[col.column]; colExists {
						found = true
						break
					}
				}
			}
			if !found {
				var tableNames []string
				for _, ref := range tableRefs {
					tableNames = append(tableNames, ref.name)
				}
				diags = append(diags, Diagnostic{
					Level:   "error",
					Message: fmt.Sprintf("column '%s' does not exist in any of the referenced models (%s)", col.column, strings.Join(tableNames, ", ")),
					Context: context,
				})
			}
		}
	}
	return diags
}

// --- Named param validation ---

func extractNamedParams(tokens []sqlToken) []string {
	var params []string
	seen := make(map[string]bool)
	for _, t := range tokens {
		if t.typ == stParam {
			name := strings.TrimPrefix(t.value, ":")
			if !seen[name] {
				params = append(params, name)
				seen[name] = true
			}
		}
	}
	return params
}

func extractURLParams(path string) map[string]bool {
	params := make(map[string]bool)
	for _, seg := range strings.Split(path, "/") {
		if strings.HasPrefix(seg, ":") {
			params[seg[1:]] = true
		}
	}
	return params
}

func findActionModel(action parser.Page, app *parser.App) string {
	for _, n := range action.Body {
		if n.Type == parser.NodeValidate && n.ModelName != "" {
			return n.ModelName
		}
		if n.Type == parser.NodeForm && n.ModelName != "" {
			return n.ModelName
		}
	}
	for _, p := range app.Pages {
		if p.Path == action.Path {
			for _, n := range p.Body {
				if n.Type == parser.NodeForm && n.ModelName != "" {
					return n.ModelName
				}
			}
		}
	}
	for _, n := range action.Body {
		if n.Type == parser.NodeQuery && n.SQL != "" {
			tokens := tokenizeSQL(n.SQL)
			if len(tokens) > 0 && tokens[0].typ == stKeyword {
				switch tokens[0].lower {
				case "insert":
					table, _ := extractInsertColumns(tokens)
					if table != "" {
						return table
					}
				case "update":
					table, _ := extractUpdateColumns(tokens)
					if table != "" {
						return table
					}
				}
			}
		}
	}
	return ""
}

func checkNamedParams(sql string, tokens []sqlToken, path string, modelName string, schema *Schema, context string) []Diagnostic {
	var diags []Diagnostic
	params := extractNamedParams(tokens)
	if len(params) == 0 {
		return nil
	}

	available := make(map[string]bool)
	for p := range extractURLParams(path) {
		available[p] = true
	}

	var mf *ModelFieldInfo
	if modelName != "" {
		if m, ok := schema.ModelFields[modelName]; ok {
			mf = m
			for field := range m.FormFields {
				available[field] = true
			}
		}
	}

	for _, param := range params {
		if available[param] {
			continue
		}
		if mf != nil {
			if fieldName, ok := mf.ColumnToField[param]; ok {
				diags = append(diags, Diagnostic{
					Level: "error",
					Message: fmt.Sprintf(
						"named parameter ':%s' will not be provided by the form. "+
							"The model field is '%s' (form sends ':%s', database column is '%s'). "+
							"Use ':%s' instead",
						param, fieldName, fieldName, param, fieldName),
					Context: context,
				})
				continue
			}
		}
		suggestion := findClosestMatch(param, available)
		if suggestion != "" {
			diags = append(diags, Diagnostic{
				Level: "error",
				Message: fmt.Sprintf(
					"named parameter ':%s' does not match any form field or URL parameter. "+
						"Did you mean ':%s'?",
					param, suggestion),
				Context: context,
			})
		} else {
			var avail []string
			for a := range available {
				avail = append(avail, ":"+a)
			}
			diags = append(diags, Diagnostic{
				Level: "error",
				Message: fmt.Sprintf(
					"named parameter ':%s' does not match any form field or URL parameter. "+
						"Available: %s",
					param, strings.Join(avail, ", ")),
				Context: context,
			})
		}
	}

	return diags
}

func findClosestMatch(target string, candidates map[string]bool) string {
	best := ""
	bestDist := (len(target) + 2) / 3
	if bestDist < 1 {
		bestDist = 1
	}
	for c := range candidates {
		d := editDistance(target, c)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	return best
}

func editDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			curr[j] = ins
			if del < curr[j] {
				curr[j] = del
			}
			if sub < curr[j] {
				curr[j] = sub
			}
		}
		prev, curr = curr, prev
	}
	return prev[lb]
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
			if i < len(tokens) && tokens[i].typ == stKeyword && tokens[i].lower == "as" && i+1 < len(tokens) {
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
			if i < len(tokens) && tokens[i].typ == stKeyword && tokens[i].lower == "as" && i+1 < len(tokens) {
				i++
				if i < len(tokens) {
					i++
				}
			}
			i--
			continue
		}

		if tokens[i].typ == stIdent {
			if i+2 < len(tokens) && tokens[i+1].typ == stPunct && tokens[i+1].value == "." && (tokens[i+2].typ == stIdent || tokens[i+2].typ == stKeyword) {
				// Handle table.column where column might be a SQL keyword (e.g., a.date, a.type, a.count)
				cols = append(cols, columnRef{table: tokens[i].lower, column: tokens[i+2].lower})
				i += 2
			} else {
				cols = append(cols, columnRef{column: tokens[i].lower})
			}

			if i+1 < len(tokens) && tokens[i+1].typ == stKeyword && tokens[i+1].lower == "as" && i+2 < len(tokens) {
				i += 2
			}
			continue
		}

		if tokens[i].typ == stPunct && tokens[i].value == "," {
			continue
		}

		if tokens[i].typ == stString {
			if i+1 < len(tokens) && tokens[i+1].typ == stKeyword && tokens[i+1].lower == "as" && i+2 < len(tokens) {
				i += 2
			}
			continue
		}

		if tokens[i].typ == stNumber {
			if i+1 < len(tokens) && tokens[i+1].typ == stKeyword && tokens[i+1].lower == "as" && i+2 < len(tokens) {
				i += 2
			}
			continue
		}
	}

	return cols
}
