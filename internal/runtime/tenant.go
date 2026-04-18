package runtime

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// TenantMap indexes model name (lowercased) to the name of its tenant
// reference. e.g. {"quote": "org", "customer": "org"} means rows of
// "quote" and "customer" are scoped by org_id.
type TenantMap map[string]string

// BuildTenantMap returns a lookup of tenant-scoped models from an app.
// Model names are lowercased for case-insensitive matching. The tenant
// reference value is preserved as written so FK column generation stays
// consistent with `fieldToColumnName`.
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

// Error types returned by the rewriter. Callers MUST fail the query when
// they observe any of these: the rewriter refuses to execute a query it
// cannot guarantee is tenant-scoped.
var (
	ErrUnsafeTenantShape  = errors.New("tenant: SQL shape is not supported by the rewriter; add the tenant predicate explicitly or refactor")
	ErrMissingTenantParam = errors.New("tenant: current_user tenant id is not available in this context; use a context with a logged-in user or bind the tenant id manually")
	ErrMutationNotScoped  = errors.New("tenant: mutation on a tenant-scoped table must bind :current_user.<tenant>_id explicitly")
)

// tableFromSelectRe finds the first FROM clause's primary table in a
// SELECT statement. Intentionally strict: only unqualified, unquoted or
// double-quoted bare table names are recognised. Anything else triggers
// ErrUnsafeTenantShape.
var tableFromSelectRe = regexp.MustCompile(`(?is)\bfrom\s+"?([a-zA-Z_][a-zA-Z0-9_]*)"?(?:\s+(?:as\s+)?([a-zA-Z_][a-zA-Z0-9_]*))?`)

// simpleWhereRe locates the WHERE keyword that follows the FROM clause.
// This rewriter does NOT handle subquery WHEREs; any SQL with a subquery
// after FROM is rejected upstream via the unsupported-shape checks.
var simpleWhereRe = regexp.MustCompile(`(?is)\bwhere\b`)

// trailingRe locates the first clause that ends the implicit WHERE
// position (GROUP BY, ORDER BY, HAVING, LIMIT, OFFSET, PAGINATE).
var trailingRe = regexp.MustCompile(`(?is)\b(group\s+by|order\s+by|having|limit|offset|paginate)\b`)

// mutationTableRe extracts the target table of a mutation. Only bare
// unqualified identifiers are matched; schema-qualified names force the
// fail-closed path.
var mutationTableRe = regexp.MustCompile(`(?is)^\s*(?:(insert\s+into)|(update)|(delete\s+from))\s+"?([a-zA-Z_][a-zA-Z0-9_]*)"?`)

// unsafeShapeSignals are substrings / patterns that indicate the SELECT
// is outside the rewriter's safe-to-modify envelope. Presence of any one
// triggers ErrUnsafeTenantShape so the caller fails the query instead of
// executing something potentially unscoped.
var unsafeShapeRe = regexp.MustCompile(`(?is)(--|/\*|\bwith\s+|\bunion\b|\bjoin\b|\bintersect\b|\bexcept\b|;\s*\S|\bfrom\s+\(|\bfrom\s+"?[a-zA-Z_][a-zA-Z0-9_]*"?\s*\.)`)

// RewriteTenantSQL rewrites a SELECT query against a tenant-scoped table
// to include `WHERE <qualifier>.<tenant>_id = :current_user.<tenant>_id`.
// Behaviour summary:
//
//   - Empty tenant map: no-op, returns sql unchanged.
//   - Non-tenant table: returns sql unchanged.
//   - Mutations (INSERT/UPDATE/DELETE) on a tenant table: validated via
//     CheckTenantMutation, never rewritten.
//   - Tenant-scoped SELECT we can safely rewrite: returns rewritten SQL.
//   - Tenant-scoped SELECT we cannot parse safely: returns ErrUnsafeTenantShape.
//   - No `current_user.<tenant>_id` in params: returns ErrMissingTenantParam.
//
// Callers MUST NOT execute the original SQL on error. This is the
// defense-in-depth contract: when in doubt, fail the query.
//
// This is not a substitute for application-level authorization. It closes
// one specific class of bug (forgetting the tenant predicate on a SELECT)
// and refuses unsupported shapes so they are addressed by the developer.
func RewriteTenantSQL(sql string, tenants TenantMap, params map[string]string) (string, error) {
	if len(tenants) == 0 {
		return sql, nil
	}

	trimmed := strings.TrimSpace(sql)
	lower := strings.ToLower(trimmed)

	// Route mutations through the mutation checker.
	if isMutationStart(lower) {
		if err := checkTenantMutation(sql, tenants, params); err != nil {
			return "", err
		}
		return sql, nil
	}

	if !strings.HasPrefix(lower, "select") {
		// Non-SELECT, non-mutation shape (CTE `WITH`, pragma, etc.).
		// Refuse if any tenant-scoped table is referenced.
		if touchesTenantTable(sql, tenants) {
			return "", fmt.Errorf("%w: %s", ErrUnsafeTenantShape, firstLine(sql))
		}
		return sql, nil
	}

	match := tableFromSelectRe.FindStringSubmatchIndex(sql)
	if match == nil {
		// A SELECT without a parseable FROM might still touch a tenant
		// table (e.g. CTE). If any tenant-scoped model name appears in
		// the SQL, fail closed.
		if touchesTenantTable(sql, tenants) {
			return "", fmt.Errorf("%w: %s", ErrUnsafeTenantShape, firstLine(sql))
		}
		return sql, nil
	}
	table := sql[match[2]:match[3]]
	tenant, ok := tenants[strings.ToLower(table)]
	if !ok {
		// Primary table is not tenant-scoped, but a JOIN, subquery or CTE
		// could still pull in tenant-scoped rows. Detect and reject.
		if touchesTenantTable(sql, tenants) {
			return "", fmt.Errorf("%w: %s", ErrUnsafeTenantShape, firstLine(sql))
		}
		return sql, nil
	}

	// Reject shapes we know we can't rewrite safely.
	if unsafeShapeRe.MatchString(sql) {
		return "", fmt.Errorf("%w: %s", ErrUnsafeTenantShape, firstLine(sql))
	}

	// Require the bind parameter to be present.
	paramKey := "current_user." + tenant + "_id"
	if _, present := params[paramKey]; !present {
		return "", fmt.Errorf("%w: missing %s", ErrMissingTenantParam, paramKey)
	}

	qualifier := table
	endPos := match[3]
	if match[4] >= 0 && match[5] > match[4] {
		alias := sql[match[4]:match[5]]
		if !isSQLKeyword(alias) {
			qualifier = alias
			endPos = match[5]
		}
	}

	filter := qualifier + "." + tenant + "_id = :" + paramKey
	afterFrom := sql[endPos:]
	if whereMatch := simpleWhereRe.FindStringIndex(afterFrom); whereMatch != nil {
		wherePos := endPos + whereMatch[1]
		return sql[:wherePos] + " " + filter + " AND" + sql[wherePos:], nil
	}
	if trailing := trailingRe.FindStringIndex(afterFrom); trailing != nil {
		insertIdx := endPos + trailing[0]
		head := strings.TrimRight(sql[:insertIdx], " \t\n")
		return head + " WHERE " + filter + " " + sql[insertIdx:], nil
	}
	return strings.TrimRight(sql, " \t\n") + " WHERE " + filter, nil
}

// checkTenantMutation verifies that an INSERT / UPDATE / DELETE on a
// tenant-scoped table binds the tenant column explicitly. This is not a
// rewrite: intent must remain visible in the .kilnx source. We accept
// the mutation if the SQL text textually references
// `:current_user.<tenant>_id` anywhere.
func checkTenantMutation(sql string, tenants TenantMap, params map[string]string) error {
	m := mutationTableRe.FindStringSubmatch(sql)
	if m == nil {
		// Unparseable mutation on unknown table: fail closed if any
		// tenant-scoped model name appears in the SQL.
		if touchesTenantTable(sql, tenants) {
			return fmt.Errorf("%w: %s", ErrUnsafeTenantShape, firstLine(sql))
		}
		return nil
	}
	table := m[4]
	tenant, ok := tenants[strings.ToLower(table)]
	if !ok {
		return nil
	}
	needle := ":current_user." + tenant + "_id"
	if !strings.Contains(strings.ToLower(sql), strings.ToLower(needle)) {
		return fmt.Errorf("%w: %s", ErrMutationNotScoped, firstLine(sql))
	}
	if _, present := params["current_user."+tenant+"_id"]; !present {
		return fmt.Errorf("%w: missing current_user.%s_id", ErrMissingTenantParam, tenant)
	}
	return nil
}

// touchesTenantTable returns true if the SQL (whitespace-tokenised) mentions
// any tenant-scoped model name as a standalone identifier. Conservative: we
// would rather reject an innocuous query than miss a tenant leak.
func touchesTenantTable(sql string, tenants TenantMap) bool {
	tokens := identRe.FindAllString(sql, -1)
	for _, t := range tokens {
		if _, ok := tenants[strings.ToLower(t)]; ok {
			return true
		}
	}
	return false
}

var identRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

var mutationStartRe = regexp.MustCompile(`^\s*(insert\s+into|update|delete\s+from)\b`)

func isMutationStart(lower string) bool {
	return mutationStartRe.MatchString(lower)
}

func firstLine(sql string) string {
	trimmed := strings.TrimSpace(sql)
	if i := strings.IndexByte(trimmed, '\n'); i >= 0 {
		return trimmed[:i] + "..."
	}
	if len(trimmed) > 120 {
		return trimmed[:120] + "..."
	}
	return trimmed
}

var sqlKeywords = map[string]bool{
	"where": true, "group": true, "order": true, "having": true, "limit": true, "offset": true,
	"paginate": true, "join": true, "inner": true, "left": true, "right": true, "outer": true,
	"cross": true, "on": true, "union": true, "intersect": true, "except": true,
}

func isSQLKeyword(s string) bool {
	return sqlKeywords[strings.ToLower(s)]
}

// sensitiveFieldRe matches column names that must never be exposed as
// bind parameters derived from the current_user row. The list covers
// password hashes, reset/verification tokens, API keys, and secrets.
// Matching is case-insensitive.
var sensitiveFieldRe = regexp.MustCompile(`(?i)(^|_)(password|secret|api_?key|token|salt|hash|reset|verification)($|_)`)

// populateCurrentUserParams copies the logged-in user's row into `params`
// under the `current_user.<column>` prefix, redacting known-sensitive
// columns (password, tokens, secrets) plus the `auth.password` column
// declared in the .kilnx config. Callers SHOULD use this helper instead
// of iterating session.Data directly.
func (s *Server) populateCurrentUserParams(params map[string]string, sess *Session) {
	if sess == nil {
		return
	}
	params["current_user.id"] = sess.UserID
	params["current_user.identity"] = sess.Identity
	params["current_user.role"] = sess.Role
	params["current_user_id"] = sess.UserID
	params["current_user_identity"] = sess.Identity
	params["current_user_role"] = sess.Role

	pwField := ""
	if s.app != nil && s.app.Auth != nil {
		pwField = s.app.Auth.Password
	}
	for k, v := range sess.Data {
		if isSensitiveField(k, pwField) {
			continue
		}
		params["current_user."+k] = v
	}
}

func isSensitiveField(name, passwordField string) bool {
	if passwordField != "" && strings.EqualFold(name, passwordField) {
		return true
	}
	return sensitiveFieldRe.MatchString(name)
}
