package optimizer

import (
	"regexp"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// interpolateRe matches {queryName.field} patterns used in text/html interpolation.
var interpolateRe = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\.([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// selectStarRe matches SELECT * or SELECT DISTINCT * at the start of a SQL query (case-insensitive).
var selectStarRe = regexp.MustCompile(`(?i)^(SELECT\s+(?:DISTINCT\s+)?)\*(\s+FROM\s+)`)

// joinRe matches JOIN clauses with optional type prefix and optional alias.
// Groups: 1=join type (LEFT/RIGHT/etc, optional), 2=table name, 3=alias (optional)
var joinRe = regexp.MustCompile(`(?i)\s+((?:LEFT|RIGHT|INNER|OUTER|CROSS|FULL)\s+)*JOIN\s+(\w+)(?:\s+(\w+))?\s+ON\s+`)

// aggregateRe matches aggregate function calls in SQL.
var aggregateRe = regexp.MustCompile(`(?i)\b(COUNT|SUM|AVG|MIN|MAX)\s*\(`)

// Optimize performs domain-aware query optimization on a parsed Kilnx app.
func Optimize(app *parser.App) {
	for i := range app.Pages {
		optimizePage(&app.Pages[i])
		deduplicateQueries(&app.Pages[i])
	}
	for i := range app.Fragments {
		optimizePage(&app.Fragments[i])
		deduplicateQueries(&app.Fragments[i])
	}
	for i := range app.APIs {
		optimizePage(&app.APIs[i])
		deduplicateQueries(&app.APIs[i])
	}
	markStreamCandidates(app)
}

func optimizePage(page *parser.Page) {
	// Build a map of query name -> set of fields used by consumers
	usedFields := collectUsedFields(page.Body)

	// Count unnamed queries to avoid unsafe _last optimization
	unnamedCount := countUnnamedQueries(page.Body)

	// Rewrite SELECT * queries where we know all consumed fields
	for i, node := range page.Body {
		if node.Type != parser.NodeQuery {
			continue
		}
		queryName := node.Name
		if queryName == "" {
			if unnamedCount > 1 {
				continue // multiple unnamed queries share _last, skip to avoid wrong columns
			}
			queryName = "_last"
		}

		fields, known := usedFields[queryName]
		if !known || fields == nil || len(fields.fields) == 0 {
			continue
		}

		rewritten := rewriteSelectStar(node.SQL, fields)
		if rewritten != node.SQL {
			page.Body[i].SQL = rewritten
		}

		// Prune JOINs where no columns from the joined table are used
		pruned := pruneUnusedJoins(page.Body[i].SQL, fields)
		if pruned != page.Body[i].SQL {
			page.Body[i].SQL = pruned
		}
	}
}

// collectUsedFields walks all nodes in a page body and collects which fields
// each query name needs. Returns nil for a query if field usage can't be
// fully determined (e.g. table with no explicit columns).
func collectUsedFields(nodes []parser.Node) map[string]*fieldSet {
	result := make(map[string]*fieldSet)

	for _, node := range nodes {
		switch node.Type {
		case parser.NodeTable:
			queryName := node.Name
			if queryName == "" {
				queryName = "_last"
			}
			if len(node.Columns) == 0 {
				// Auto-detect mode: we can't know which columns are needed
				// Mark as unknowable so we skip optimization
				result[queryName] = nil
				continue
			}
			if result[queryName] == nil && !hasKey(result, queryName) {
				result[queryName] = newFieldSet()
			}
			fs := result[queryName]
			if fs == nil {
				continue // already marked unknowable
			}
			for _, col := range node.Columns {
				fs.add(col.Field)
			}
			// Row actions may interpolate :field from query rows
			for _, action := range node.RowActions {
				for _, f := range extractPathParams(action.Path) {
					fs.add(f)
				}
			}

		case parser.NodeList:
			queryName := node.Name
			if queryName == "" {
				queryName = "_last"
			}
			if result[queryName] == nil && !hasKey(result, queryName) {
				result[queryName] = newFieldSet()
			}
			fs := result[queryName]
			if fs == nil {
				continue
			}
			for _, prop := range []string{"title", "subtitle", "image"} {
				if v, ok := node.Props[prop]; ok && v != "" {
					fs.add(v)
				}
			}
			if v, ok := node.Props["action_path"]; ok && v != "" {
				for _, f := range extractPathParams(v) {
					fs.add(f)
				}
			}

		case parser.NodeSearch:
			queryName := node.Name
			if queryName == "" {
				queryName = "_last"
			}
			if result[queryName] == nil && !hasKey(result, queryName) {
				result[queryName] = newFieldSet()
			}
			fs := result[queryName]
			if fs == nil {
				continue
			}
			for _, f := range node.SearchFields {
				fs.add(f)
			}

		case parser.NodeText:
			addInterpolatedFields(result, node.Value)

		case parser.NodeHTML:
			addInterpolatedFields(result, node.HTMLContent)

		case parser.NodeOn:
			childFields := collectUsedFields(node.Children)
			for k, v := range childFields {
				if v == nil {
					result[k] = nil
				} else if !hasKey(result, k) {
					result[k] = v
				} else if existing := result[k]; existing != nil {
					for _, f := range v.fields {
						existing.add(f)
					}
				}
				// else: result[k] is nil (unknowable), keep it nil
			}
		}
	}

	return result
}

// addInterpolatedFields extracts {queryName.field} references from text
// and adds the fields to the appropriate query's field set.
func addInterpolatedFields(result map[string]*fieldSet, text string) {
	matches := interpolateRe.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		queryName := m[1]
		field := m[2]
		if field == "count" {
			continue // built-in, not a real column
		}
		if result[queryName] == nil && !hasKey(result, queryName) {
			result[queryName] = newFieldSet()
		}
		fs := result[queryName]
		if fs == nil {
			continue
		}
		fs.add(field)
	}
}

// extractPathParams pulls :param names from a URL path like /users/:id/edit.
func extractPathParams(path string) []string {
	var params []string
	for _, segment := range strings.Split(path, "/") {
		if strings.HasPrefix(segment, ":") {
			params = append(params, segment[1:])
		}
	}
	return params
}

// rewriteSelectStar replaces SELECT * FROM with SELECT col1, col2, ... FROM
// only if the SQL starts with SELECT [DISTINCT] * FROM.
func rewriteSelectStar(sql string, fields *fieldSet) string {
	trimmed := strings.TrimSpace(sql)
	loc := selectStarRe.FindStringSubmatchIndex(trimmed)
	if loc == nil {
		return sql
	}

	// Preserve leading whitespace from original
	leadingWS := sql[:len(sql)-len(strings.TrimLeft(sql, " \t\n\r"))]

	prefix := trimmed[loc[2]:loc[3]]   // "SELECT " or "SELECT DISTINCT "
	fromPart := trimmed[loc[4]:loc[5]] // " FROM "
	rest := trimmed[loc[1]:]           // everything after "* FROM "

	cols := fields.sorted()
	return leadingWS + prefix + strings.Join(cols, ", ") + fromPart + rest
}

// fieldSet is an ordered set of field names.
type fieldSet struct {
	fields []string
	seen   map[string]bool
}

func newFieldSet() *fieldSet {
	return &fieldSet{seen: make(map[string]bool)}
}

func (fs *fieldSet) add(field string) {
	// Handle dot notation: author.name -> just use the full field as-is
	if fs.seen[field] {
		return
	}
	fs.seen[field] = true
	fs.fields = append(fs.fields, field)
}

func (fs *fieldSet) sorted() []string {
	// Return in insertion order for deterministic output
	return fs.fields
}

func hasKey(m map[string]*fieldSet, key string) bool {
	_, ok := m[key]
	return ok
}

func countUnnamedQueries(nodes []parser.Node) int {
	count := 0
	for _, node := range nodes {
		if node.Type == parser.NodeQuery && node.Name == "" {
			count++
		}
		if node.Type == parser.NodeOn {
			count += countUnnamedQueries(node.Children)
		}
	}
	return count
}

// --- Query deduplication ---

type queryEntry struct {
	name  string
	sql   string
	index int // index in page.Body (-1 if nested)
}

// deduplicateQueries finds named queries with identical SQL on the same page
// and removes duplicates, rewriting consumer references to point to the original.
func deduplicateQueries(page *parser.Page) {
	seen := make(map[string]string) // sql -> first query name

	var entries []queryEntry
	collectNamedQueries(page.Body, &entries, 0)

	for _, e := range entries {
		if e.sql == "" || e.name == "" {
			continue
		}
		if original, exists := seen[e.sql]; exists {
			// Duplicate found: rename consumers and clear the duplicate
			renameConsumerRefs(page.Body, e.name, original)
			if e.index >= 0 && e.index < len(page.Body) {
				page.Body[e.index].SQL = ""
			}
		} else {
			seen[e.sql] = e.name
		}
	}
}

func collectNamedQueries(nodes []parser.Node, entries *[]queryEntry, baseIndex int) {
	for i, node := range nodes {
		if node.Type == parser.NodeQuery && node.Name != "" && node.SQL != "" {
			*entries = append(*entries, queryEntry{name: node.Name, sql: node.SQL, index: baseIndex + i})
		}
		if node.Type == parser.NodeOn {
			collectNamedQueries(node.Children, entries, -1)
		}
	}
}

func renameConsumerRefs(nodes []parser.Node, oldName, newName string) {
	for i := range nodes {
		node := &nodes[i]
		if node.Name == oldName {
			switch node.Type {
			case parser.NodeTable, parser.NodeList, parser.NodeSearch:
				node.Name = newName
			}
		}
		if node.Type == parser.NodeOn {
			renameConsumerRefs(node.Children, oldName, newName)
		}
	}
}

// --- JOIN pruning ---

// pruneUnusedJoins removes JOIN clauses when no columns from the joined table
// are consumed by any component. Skips if any consumed field uses plain names
// (no dot notation) since we can't determine which table owns the column.
func pruneUnusedJoins(sql string, fields *fieldSet) string {
	if fields == nil || len(fields.fields) == 0 {
		return sql
	}

	// Check if all consumed fields use qualified (alias.column) notation
	// If any field is unqualified, we can't safely prune
	qualifiedPrefixes := make(map[string]bool)
	for _, f := range fields.fields {
		parts := strings.SplitN(f, ".", 2)
		if len(parts) != 2 {
			return sql // unqualified field, skip pruning
		}
		qualifiedPrefixes[strings.ToLower(parts[0])] = true
	}

	matches := joinRe.FindAllStringSubmatchIndex(sql, -1)
	if len(matches) == 0 {
		return sql
	}

	// Process matches in reverse order to preserve indices
	result := sql
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		// Group 2: table name
		tableName := strings.ToLower(sql[m[4]:m[5]])
		// Group 3: alias (may be empty)
		alias := tableName
		if m[6] >= 0 && m[7] >= 0 {
			candidate := strings.ToLower(sql[m[6]:m[7]])
			// Make sure the alias is not a SQL keyword
			if candidate != "on" {
				alias = candidate
			}
		}

		// Check if the alias is used by any consumed field
		if qualifiedPrefixes[alias] {
			continue // columns from this JOIN are used, keep it
		}

		// Find the ON clause extent (match everything up to next JOIN or WHERE/ORDER/GROUP/LIMIT)
		onEnd := findJoinClauseEnd(result, m[1]-3) // -3 to back up before "ON "
		if onEnd > m[0] {
			result = result[:m[0]] + result[onEnd:]
		}
	}

	return result
}

func findJoinClauseEnd(sql string, start int) int {
	upper := strings.ToUpper(sql)
	// Find the extent of the ON condition
	depth := 0
	i := start
	for i < len(sql) {
		if sql[i] == '(' {
			depth++
		} else if sql[i] == ')' {
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 {
			// Check for clause boundaries
			remaining := upper[i:]
			for _, kw := range []string{" JOIN ", " LEFT ", " RIGHT ", " INNER ", " OUTER ", " CROSS ", " FULL ", " WHERE ", " ORDER ", " GROUP ", " HAVING ", " LIMIT ", " UNION "} {
				if strings.HasPrefix(remaining, kw) {
					return i
				}
			}
		}
		i++
	}
	return len(sql)
}

// --- Stream materialization hints ---

// markStreamCandidates marks stream queries with aggregate functions as
// materialization candidates by prepending a hint comment.
func markStreamCandidates(app *parser.App) {
	for i := range app.Streams {
		s := &app.Streams[i]
		if s.SQL == "" || s.IntervalSecs <= 0 {
			continue
		}
		if strings.HasPrefix(s.SQL, "/* kilnx:materialize-candidate */") {
			continue // already marked
		}
		if aggregateRe.MatchString(s.SQL) {
			s.SQL = "/* kilnx:materialize-candidate */ " + s.SQL
		}
	}
}
