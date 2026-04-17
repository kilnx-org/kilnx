package runtime

import (
	"regexp"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// TenantMap indexes model name to the name of its tenant reference.
// e.g. {"quote": "org", "customer": "org"} means rows of "quote" and
// "customer" are scoped by org_id.
type TenantMap map[string]string

// BuildTenantMap returns a lookup of tenant-scoped models from an app.
// The key is the model/table name. The value is the referenced tenant model
// (which drives the FK column, e.g. tenant "org" -> column "org_id").
func BuildTenantMap(app *parser.App) TenantMap {
	m := make(TenantMap)
	if app == nil {
		return m
	}
	for _, model := range app.Models {
		if model.Tenant != "" {
			m[strings.ToLower(model.Name)] = model.Tenant
		}
	}
	return m
}

// tableFromSelectRe finds the first FROM clause's primary table in a SELECT
// statement. It is deliberately conservative: it matches a bare table name
// (optionally double-quoted) and stops at any JOIN/subquery/alias. Queries
// more complex than `SELECT ... FROM <table> [alias] [WHERE ...]` are left
// alone by the rewriter and the developer must add the tenant filter by hand.
var tableFromSelectRe = regexp.MustCompile(`(?is)\bfrom\s+"?([a-zA-Z_][a-zA-Z0-9_]*)"?(?:\s+(?:as\s+)?([a-zA-Z_][a-zA-Z0-9_]*))?`)

// simpleWhereRe finds a top-level WHERE clause location. It does not look
// inside subqueries; we use it only to decide whether to append `AND ...`
// or append a new `WHERE ...`.
var simpleWhereRe = regexp.MustCompile(`(?is)\bwhere\b`)

// RewriteTenantSQL rewrites a query to enforce tenant scoping when the
// primary table of a SELECT is a tenant-scoped model. If the statement is
// not a SELECT, or if the rewriter cannot safely parse the shape, the SQL
// is returned unchanged and the caller/developer is responsible for
// including the tenant filter explicitly.
//
// Returned SQL references :current_user.<tenant>_id. The caller must ensure
// that bind parameter is populated (renderContext already exposes every
// user row column as current_user.<col>).
func RewriteTenantSQL(sql string, tenants TenantMap) string {
	if len(tenants) == 0 {
		return sql
	}

	trimmed := strings.TrimSpace(sql)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "select") {
		return sql
	}

	match := tableFromSelectRe.FindStringSubmatchIndex(sql)
	if match == nil {
		return sql
	}
	table := sql[match[2]:match[3]]
	tenant, ok := tenants[strings.ToLower(table)]
	if !ok {
		return sql
	}

	// The regex may greedily swallow the next word as an "alias". Decide
	// whether it is really an alias (non-keyword identifier) or a SQL
	// keyword (WHERE, ORDER, GROUP, LIMIT, ...) that must be pushed back.
	qualifier := table
	endPos := match[3] // end of table name (our baseline insertion boundary)
	if match[4] >= 0 && match[5] > match[4] {
		alias := sql[match[4]:match[5]]
		if !isSQLKeyword(alias) {
			qualifier = alias
			endPos = match[5]
		}
	}

	filter := qualifier + "." + tenant + "_id = :current_user." + tenant + "_id"

	afterFrom := sql[endPos:]
	if whereMatch := simpleWhereRe.FindStringIndex(afterFrom); whereMatch != nil {
		// Existing WHERE: inject `<filter> AND ` right after the WHERE keyword.
		wherePos := endPos + whereMatch[1]
		return sql[:wherePos] + " " + filter + " AND" + sql[wherePos:]
	}

	trailingRe := regexp.MustCompile(`(?is)\b(group\s+by|order\s+by|having|limit|offset|paginate)\b`)
	if trailing := trailingRe.FindStringIndex(afterFrom); trailing != nil {
		insertIdx := endPos + trailing[0]
		// Absorb the whitespace immediately before the trailing clause so we
		// do not produce double spaces in the rewritten SQL.
		head := strings.TrimRight(sql[:insertIdx], " \t\n")
		return head + " WHERE " + filter + " " + sql[insertIdx:]
	}
	return strings.TrimRight(sql, " \t\n") + " WHERE " + filter
}

var sqlKeywords = map[string]bool{
	"where": true, "group": true, "order": true, "having": true, "limit": true, "offset": true,
	"paginate": true, "join": true, "inner": true, "left": true, "right": true, "outer": true,
	"cross": true, "on": true, "union": true, "intersect": true, "except": true,
}

func isSQLKeyword(s string) bool {
	return sqlKeywords[strings.ToLower(s)]
}
