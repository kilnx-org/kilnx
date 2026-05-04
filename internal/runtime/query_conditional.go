package runtime

import "strings"

// expandQueryConditionals processes {{if expr}}...{{end}} blocks inside a SQL
// string, using the provided parameters for condition evaluation. It returns the
// SQL with conditional fragments included or removed based on the evaluation.
func expandQueryConditionals(sql string, params map[string]string) string {
	var result strings.Builder
	remaining := sql

	for {
		idx := strings.Index(remaining, "{{if ")
		if idx < 0 {
			result.WriteString(remaining)
			break
		}

		result.WriteString(remaining[:idx])

		tagEnd := strings.Index(remaining[idx:], "}}")
		if tagEnd < 0 {
			result.WriteString(remaining[idx:])
			break
		}
		tagEnd += idx + 2
		condition := strings.TrimSpace(remaining[idx+5 : tagEnd-2])

		bodyStart := tagEnd
		body, _, endPos := findMatchingEnd(remaining[bodyStart:])
		if endPos < 0 {
			result.WriteString(remaining[idx:])
			break
		}

		if evaluateQueryCondition(condition, params) {
			expanded := expandQueryConditionals(body, params)
			result.WriteString(expanded)
		}

		remaining = remaining[bodyStart+endPos:]
	}

	return result.String()
}

// evaluateQueryCondition evaluates a simple condition against query parameters.
// Supported forms:
//
//	params.key                -> truthy check (non-empty)
//	params.key == "value"     -> equality
//	params.key != "value"     -> inequality
func evaluateQueryCondition(condition string, params map[string]string) bool {
	condition = strings.TrimSpace(condition)

	// Split on "==" or "!="
	if eqIdx := strings.Index(condition, "=="); eqIdx >= 0 {
		left := strings.TrimSpace(condition[:eqIdx])
		right := strings.TrimSpace(condition[eqIdx+2:])
		return resolveQueryParam(left, params) == stripQuotes(right)
	}
	if neIdx := strings.Index(condition, "!="); neIdx >= 0 {
		left := strings.TrimSpace(condition[:neIdx])
		right := strings.TrimSpace(condition[neIdx+2:])
		return resolveQueryParam(left, params) != stripQuotes(right)
	}

	// Truthy check
	return resolveQueryParam(condition, params) != ""
}

// resolveQueryParam resolves a parameter reference like "params.status" or "status"
// from the provided params map.
func resolveQueryParam(expr string, params map[string]string) string {
	expr = strings.TrimSpace(expr)
	// Support both "params.key" and just "key"
	if strings.HasPrefix(expr, "params.") {
		expr = strings.TrimPrefix(expr, "params.")
	}
	return params[expr]
}
