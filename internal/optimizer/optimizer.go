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

// Optimize performs domain-aware query optimization on a parsed Kilnx app.
// It rewrites SELECT * queries to only select columns that are actually
// consumed by components (table, list, card) and interpolations.
func Optimize(app *parser.App) {
	for i := range app.Pages {
		optimizePage(&app.Pages[i])
	}
	for i := range app.Fragments {
		optimizePage(&app.Fragments[i])
	}
	for i := range app.APIs {
		optimizePage(&app.APIs[i])
	}
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
