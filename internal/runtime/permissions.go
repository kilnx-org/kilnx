package runtime

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// PermissionRule represents a single parsed rule from the permissions block.
type PermissionRule struct {
	Action    string // "all", "read", "write"
	Resource  string // lower-cased model name
	Condition string // raw condition after "where", or empty
}

// PermissionMap indexes role -> model -> rules for fast runtime lookup.
// Model names are lower-cased.  The special key "*" holds wildcard rules
// produced by the "all" action.
type PermissionMap map[string]map[string][]PermissionRule

// BuildPermissionMap parses the raw permission strings from the AST into a
// structured map for fast runtime lookups.
func BuildPermissionMap(app *parser.App) PermissionMap {
	pm := make(PermissionMap)
	if app == nil {
		return pm
	}
	for _, perm := range app.Permissions {
		role := perm.Role
		if pm[role] == nil {
			pm[role] = make(map[string][]PermissionRule)
		}
		for _, raw := range perm.Rules {
			rule := parsePermissionRule(raw)
			if rule == nil {
				continue
			}
			if rule.Action == "all" {
				pm[role]["*"] = append(pm[role]["*"], *rule)
				continue
			}
			pm[role][rule.Resource] = append(pm[role][rule.Resource], *rule)
		}
	}
	return pm
}

var permissionRuleRe = regexp.MustCompile(`^(?i)(all|read|write)(?:\s+([a-zA-Z_][a-zA-Z0-9_]*))?(?:\s+where\s+(.+))?$`)

func parsePermissionRule(raw string) *PermissionRule {
	m := permissionRuleRe.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return nil
	}
	return &PermissionRule{
		Action:    strings.ToLower(m[1]),
		Resource:  strings.ToLower(m[2]),
		Condition: strings.TrimSpace(m[3]),
	}
}

// CanAccess reports whether role has any permission (read or write) for
// resource.  It respects the hard-coded role hierarchy as a fallback.
func (pm PermissionMap) CanAccess(role, resource string) bool {
	resource = strings.ToLower(resource)
	if pm.hasRule(role, resource, "") {
		return true
	}
	roleHierarchy := map[string]int{
		"admin":  100,
		"editor": 50,
		"viewer": 10,
	}
	userLevel, userOk := roleHierarchy[role]
	requiredLevel, reqOk := roleHierarchy["viewer"]
	if userOk && reqOk && userLevel >= requiredLevel {
		return true
	}
	return false
}

// CanRead reports whether role may read the resource.
func (pm PermissionMap) CanRead(role, resource string) bool {
	resource = strings.ToLower(resource)
	return pm.hasRule(role, resource, "read")
}

// CanWrite reports whether role may write the resource.
func (pm PermissionMap) CanWrite(role, resource string) bool {
	resource = strings.ToLower(resource)
	return pm.hasRule(role, resource, "write")
}

func (pm PermissionMap) hasRule(role, resource, action string) bool {
	modelRules, ok := pm[role]
	if !ok {
		return false
	}
	for _, r := range modelRules["*"] {
		if action == "" || r.Action == "all" || r.Action == action {
			return true
		}
	}
	for _, r := range modelRules[resource] {
		if action == "" || r.Action == action {
			return true
		}
	}
	return false
}

// ConditionForRead returns the first read-condition for role+resource, or "".
func (pm PermissionMap) ConditionForRead(role, resource string) string {
	resource = strings.ToLower(resource)
	modelRules, ok := pm[role]
	if !ok {
		return ""
	}
	for _, r := range modelRules[resource] {
		if r.Action == "read" && r.Condition != "" {
			return r.Condition
		}
	}
	return ""
}

// ConditionForWrite returns the first write-condition for role+resource, or "".
func (pm PermissionMap) ConditionForWrite(role, resource string) string {
	resource = strings.ToLower(resource)
	modelRules, ok := pm[role]
	if !ok {
		return ""
	}
	for _, r := range modelRules[resource] {
		if r.Action == "write" && r.Condition != "" {
			return r.Condition
		}
	}
	return ""
}

// RewritePermissionSQL rewrites a SELECT query to include the permission
// condition in the WHERE clause.  It follows the same structural conventions
// as RewriteTenantSQL but operates on permission rules rather than tenants.
//
// If the user's role has no conditional restriction for the queried table,
// the SQL is returned unchanged.
func RewritePermissionSQL(sql string, pm PermissionMap, role string, params map[string]string) (string, error) {
	if len(pm) == 0 || role == "" {
		return sql, nil
	}

	if sqlCommentRe.MatchString(sql) {
		return "", fmt.Errorf("permission: SQL comments are not allowed: %s", firstLine(sql))
	}
	scrubbed := stripSQLNoise(sql)
	trimmed := strings.TrimSpace(scrubbed)
	lower := strings.ToLower(trimmed)

	if isMutationStart(lower) {
		return sql, nil
	}
	if !strings.HasPrefix(lower, "select") {
		return sql, nil
	}

	match := tableFromSelectRe.FindStringSubmatchIndex(scrubbed)
	if match == nil {
		return sql, nil
	}
	table := scrubbed[match[2]:match[3]]

	if unsafeShapeRe.MatchString(scrubbed) {
		return sql, nil
	}

	condition := pm.ConditionForRead(role, table)
	if condition == "" {
		return sql, nil
	}

	// Determine qualifier (table name or alias).
	qualifier := table
	endPos := match[3]
	if match[4] >= 0 && match[5] > match[4] {
		alias := sql[match[4]:match[5]]
		if !isSQLKeyword(alias) {
			qualifier = alias
			endPos = match[5]
		}
	}

	filter, err := translatePermissionCondition(condition, qualifier)
	if err != nil {
		return "", err
	}
	filter = resolvePermissionPlaceholders(filter, params)
	if filter == "" {
		return sql, nil
	}

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

// translatePermissionCondition converts a simple permission condition like
// "author = current_user" into a SQL predicate.
// Supported shapes (case-insensitive):
//   field = current_user
//   field = 'literal'
//   field = 123
func translatePermissionCondition(condition, qualifier string) (string, error) {
	condition = strings.TrimSpace(condition)
	parts := strings.SplitN(condition, "=", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("permission: unsupported condition shape: %q", condition)
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])

	col := left
	// If left side does not already reference the qualifier, prefix it.
	if !strings.Contains(col, ".") {
		col = fmt.Sprintf("%s.%s", qualifier, col)
	}

	if strings.EqualFold(right, "current_user") {
		return fmt.Sprintf("%s = :current_user.id", col), nil
	}
	return fmt.Sprintf("%s = %s", col, right), nil
}

// resolvePermissionPlaceholders validates that any :current_user.id reference
// has a bound parameter available.
func resolvePermissionPlaceholders(filter string, params map[string]string) string {
	if strings.Contains(filter, ":current_user.id") {
		if _, ok := params["current_user.id"]; !ok {
			return ""
		}
	}
	return filter
}
