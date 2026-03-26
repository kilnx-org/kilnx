package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/kilnx-org/kilnx/internal/database"
)

// Template directives:
//   {{each queryName}}...{{else}}...{{end}}  - iterate query rows
//   {{if expr}}...{{else}}...{{end}}         - conditional rendering
//   {queryName.field}                        - first row value (escaped)
//   {queryName.count}                        - row count
//   {field}                                  - search all queries (escaped)
//   {field | raw}                            - unescaped output (for richtext)
//   {csrf}                                   - CSRF token (unique per occurrence)
//   {paginate.queryName.field}               - pagination metadata
//   {params.key}                             - URL query parameter

var rawFilterRe = regexp.MustCompile(`\{([a-zA-Z_]\w*(?:\.[a-zA-Z_]\w*)*)\s*\|\s*raw\}`)

// renderHTML is the main template processing function for NodeHTML content.
// It processes all template directives and returns the final HTML string.
func renderHTML(content string, ctx *renderContext) string {
	result := content

	// Step 1: Replace each {csrf} with a unique token (one per occurrence for multi-form pages)
	for strings.Contains(result, "{csrf}") {
		token := generateCSRFToken()
		result = strings.Replace(result, "{csrf}", token, 1)
	}

	// Step 2: Process {field | raw} outside of {{each}} blocks
	// Use a random nonce to prevent placeholder collision with user data
	nonce := generateNonce()
	rawPlaceholders := make(map[string]string)
	rawCounter := 0
	result = rawFilterRe.ReplaceAllStringFunc(result, func(match string) string {
		parts := rawFilterRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		expr := parts[1]
		val := resolveValue(expr, ctx, nil)
		if val == "{"+expr+"}" {
			return match // not resolved, leave unchanged
		}
		placeholder := fmt.Sprintf("\x00KILNX_RAW_%s_%d\x00", nonce, rawCounter)
		rawCounter++
		rawPlaceholders[placeholder] = val
		return placeholder
	})

	// Step 3: Process {{each queryName}}...{{else}}...{{end}} blocks
	result = expandEachBlocks(result, ctx, nonce)

	// Step 4: Process {{if expr}}...{{else}}...{{end}} blocks
	result = expandIfBlocks(result, ctx, nil)

	// Step 5: Run standard interpolation (with escaping)
	result = interpolateEscaped(result, ctx)

	// Step 6: Restore raw placeholders (unescaped values)
	for placeholder, val := range rawPlaceholders {
		result = strings.ReplaceAll(result, placeholder, val)
	}

	return result
}

// generateNonce returns a short random hex string for placeholder uniqueness
func generateNonce() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// expandEachBlocks processes all {{each queryName}}...{{else}}...{{end}} blocks.
// Uses a stack-based parser to handle nested blocks correctly.
func expandEachBlocks(content string, ctx *renderContext, nonce string) string {
	var result strings.Builder
	remaining := content

	for {
		idx := strings.Index(remaining, "{{each ")
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
		tagContent := remaining[idx+7 : tagEnd-2]
		queryName := strings.TrimSpace(tagContent)

		bodyStart := tagEnd
		body, elseBody, endPos := findMatchingEnd(remaining[bodyStart:])
		if endPos < 0 {
			result.WriteString(remaining[idx:])
			break
		}

		rows, ok := ctx.queries[queryName]
		if !ok || len(rows) == 0 {
			if elseBody != "" {
				expanded := expandIfBlocks(elseBody, ctx, nil)
				expanded = interpolateEscaped(expanded, ctx)
				result.WriteString(expanded)
			}
		} else {
			for _, row := range rows {
				// Process raw filters inside each body with current row context
				expanded := processRawInRow(body, row, ctx, nonce)
				// Process nested {{each}} blocks
				expanded = expandEachBlocks(expanded, ctx, nonce)
				// Process {{if}} blocks with current row context
				expanded = expandIfBlocks(expanded, ctx, row)
				// Interpolate row fields
				expanded = interpolateRow(expanded, row, ctx)
				result.WriteString(expanded)
			}
		}

		remaining = remaining[bodyStart+endPos:]
	}

	return result.String()
}

// processRawInRow handles {field | raw} inside {{each}} blocks where field comes from the current row
func processRawInRow(content string, row database.Row, ctx *renderContext, nonce string) string {
	counter := 0
	placeholders := make(map[string]string)

	result := rawFilterRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := rawFilterRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		expr := parts[1]
		val := resolveValue(expr, ctx, row)
		if val == "{"+expr+"}" {
			return match
		}
		placeholder := fmt.Sprintf("\x00KILNX_RROW_%s_%d\x00", nonce, counter)
		counter++
		placeholders[placeholder] = val
		return placeholder
	})

	// After all other processing, restore these placeholders
	// But we need to return them embedded so they survive interpolateRow
	// Store in the result directly since \x00 won't be touched by interpolateRe
	for placeholder, val := range placeholders {
		result = strings.ReplaceAll(result, placeholder, val)
	}

	return result
}

// findMatchingEnd finds the body, else body, and position after {{end}} for a block,
// accounting for nested {{each}}/{{if}} blocks.
func findMatchingEnd(content string) (body, elseBody string, endPos int) {
	depth := 1
	pos := 0
	bodyEnd := -1
	elseStart := -1

	for pos < len(content) {
		nextTag := strings.Index(content[pos:], "{{")
		if nextTag < 0 {
			return "", "", -1
		}
		nextTag += pos

		closeTag := strings.Index(content[nextTag:], "}}")
		if closeTag < 0 {
			return "", "", -1
		}
		closeTag += nextTag + 2

		tagInner := strings.TrimSpace(content[nextTag+2 : closeTag-2])

		if strings.HasPrefix(tagInner, "each ") || strings.HasPrefix(tagInner, "if ") {
			depth++
		} else if tagInner == "end" {
			depth--
			if depth == 0 {
				if elseStart >= 0 {
					body = content[:bodyEnd]
					elseBody = content[elseStart:nextTag]
				} else {
					body = content[:nextTag]
				}
				return body, elseBody, closeTag
			}
		} else if tagInner == "else" && depth == 1 {
			bodyEnd = nextTag
			elseStart = closeTag
		}

		pos = closeTag
	}

	return "", "", -1
}

// expandIfBlocks processes all {{if expr}}...{{else}}...{{end}} blocks.
// currentRow is non-nil when inside an {{each}} block.
func expandIfBlocks(content string, ctx *renderContext, currentRow database.Row) string {
	var result strings.Builder
	remaining := content

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
		body, elseBody, endPos := findMatchingEnd(remaining[bodyStart:])
		if endPos < 0 {
			result.WriteString(remaining[idx:])
			break
		}

		if evaluateCondition(condition, ctx, currentRow) {
			expanded := expandIfBlocks(body, ctx, currentRow)
			result.WriteString(expanded)
		} else {
			expanded := expandIfBlocks(elseBody, ctx, currentRow)
			result.WriteString(expanded)
		}

		remaining = remaining[bodyStart+endPos:]
	}

	return result.String()
}

// evaluateCondition evaluates a condition expression.
// Supported forms:
//
//	field                        -> truthy check (non-empty, non-zero, non-false)
//	field == "value"             -> equality
//	field != "value"             -> inequality
//	queryName.count > 0          -> numeric comparison
//	queryName.count == 0         -> numeric/string equality
func evaluateCondition(condition string, ctx *renderContext, currentRow database.Row) bool {
	var left, right, op string

	// Find operator, but only OUTSIDE of quoted strings
	left, op, right = splitCondition(condition)

	if op == "" {
		// No operator: treat as truthy check
		val := resolveValue(condition, ctx, currentRow)
		if val == "{"+condition+"}" {
			val = ""
		}
		return val != "" && val != "0" && val != "false"
	}

	// Resolve left side
	leftVal := resolveValue(left, ctx, currentRow)
	if leftVal == "{"+left+"}" {
		leftVal = ""
	}

	// Resolve right side: strip quotes if present, otherwise resolve as variable
	rightVal := stripQuotes(right)
	if rightVal == right {
		// No quotes stripped, try to resolve as variable
		resolved := resolveValue(rightVal, ctx, currentRow)
		if resolved != "{"+rightVal+"}" {
			rightVal = resolved
		}
	}

	switch op {
	case "==":
		return leftVal == rightVal
	case "!=":
		return leftVal != rightVal
	case ">":
		return compareNumeric(leftVal, rightVal) > 0
	case "<":
		return compareNumeric(leftVal, rightVal) < 0
	case ">=":
		return compareNumeric(leftVal, rightVal) >= 0
	case "<=":
		return compareNumeric(leftVal, rightVal) <= 0
	}

	return false
}

// splitCondition splits "left op right" while respecting quoted strings.
// Returns left, operator, right. If no operator found, returns condition, "", "".
func splitCondition(condition string) (string, string, string) {
	inQuote := byte(0)
	for i := 0; i < len(condition); i++ {
		c := condition[i]
		if c == '"' || c == '\'' {
			if inQuote == 0 {
				inQuote = c
			} else if inQuote == c {
				inQuote = 0
			}
			continue
		}
		if inQuote != 0 {
			continue
		}
		// Check for two-char operators first
		if i+1 < len(condition) {
			two := condition[i : i+2]
			if two == "!=" || two == "==" || two == ">=" || two == "<=" {
				return strings.TrimSpace(condition[:i]), two, strings.TrimSpace(condition[i+2:])
			}
		}
		// Single-char operators
		if c == '>' || c == '<' {
			return strings.TrimSpace(condition[:i]), string(c), strings.TrimSpace(condition[i+1:])
		}
	}
	return condition, "", ""
}

// stripQuotes removes surrounding double or single quotes from a string.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// compareNumeric compares two string values as numbers. Returns -1, 0, or 1.
func compareNumeric(a, b string) int {
	na, errA := strconv.ParseFloat(a, 64)
	nb, errB := strconv.ParseFloat(b, 64)
	if errA != nil || errB != nil {
		return strings.Compare(a, b)
	}
	if na < nb {
		return -1
	}
	if na > nb {
		return 1
	}
	return 0
}

// resolveValue resolves a template expression to its value.
// Handles: "paginate.query.field", "params.key", "queryName.field", "queryName.count", bare "field"
// Returns the original "{expr}" string if not found.
func resolveValue(expr string, ctx *renderContext, currentRow database.Row) string {
	allParts := strings.Split(expr, ".")

	// paginate.queryName.field (3 parts)
	if len(allParts) == 3 && allParts[0] == "paginate" {
		if info, ok := ctx.paginate[allParts[1]]; ok {
			return getPaginateField(info, allParts[2])
		}
		return "{" + expr + "}"
	}

	parts := strings.SplitN(expr, ".", 2)

	if len(parts) == 2 {
		prefix := parts[0]
		field := parts[1]

		// params.key
		if prefix == "params" {
			if ctx.queryParams != nil {
				if val, ok := ctx.queryParams[field]; ok {
					return val
				}
			}
			return ""
		}

		// Check current row for the field part (inside {{each}})
		// e.g., inside {{each contacts}}, {contact.name} is not valid,
		// but {name} is. Dotted access resolves against queries.

		// queryName.count
		rows, ok := ctx.queries[prefix]
		if !ok {
			return "{" + expr + "}"
		}
		if field == "count" {
			return fmt.Sprintf("%d", len(rows))
		}
		if len(rows) > 0 {
			if val, ok := rows[0][field]; ok {
				return val
			}
		}
		return ""
	}

	// Single name: check current row first, then all queries
	if currentRow != nil {
		if val, ok := currentRow[expr]; ok {
			return val
		}
	}
	for _, rows := range ctx.queries {
		if len(rows) > 0 {
			if val, ok := rows[0][expr]; ok {
				return val
			}
		}
	}

	return "{" + expr + "}"
}

// getPaginateField returns a pagination metadata field value
func getPaginateField(info PaginateInfo, field string) string {
	switch field {
	case "page":
		return fmt.Sprintf("%d", info.Page)
	case "per_page":
		return fmt.Sprintf("%d", info.PerPage)
	case "total":
		return fmt.Sprintf("%d", info.Total)
	case "has_prev":
		if info.HasPrev {
			return "true"
		}
		return "false"
	case "has_next":
		if info.HasNext {
			return "true"
		}
		return "false"
	case "prev":
		if info.HasPrev {
			return fmt.Sprintf("%d", info.Page-1)
		}
		return fmt.Sprintf("%d", info.Page)
	case "next":
		if info.HasNext {
			return fmt.Sprintf("%d", info.Page+1)
		}
		return fmt.Sprintf("%d", info.Page)
	case "total_pages":
		if info.PerPage > 0 {
			return fmt.Sprintf("%d", (info.Total+info.PerPage-1)/info.PerPage)
		}
		return "0"
	}
	return ""
}

// interpolateRow replaces {field} patterns with the current row's values (escaped),
// and {query.field} with cross-query values (also escaped).
func interpolateRow(text string, row database.Row, ctx *renderContext) string {
	return interpolateRe.ReplaceAllStringFunc(text, func(match string) string {
		expr := match[1 : len(match)-1]

		val := resolveValue(expr, ctx, row)
		if val != "{"+expr+"}" {
			return html.EscapeString(val)
		}

		return match
	})
}

// interpolateEscaped replaces {query.field} patterns with escaped values.
// Used for content outside {{each}} blocks.
func interpolateEscaped(text string, ctx *renderContext) string {
	return interpolateRe.ReplaceAllStringFunc(text, func(match string) string {
		expr := match[1 : len(match)-1]

		val := resolveValue(expr, ctx, nil)
		if val != "{"+expr+"}" {
			return html.EscapeString(val)
		}

		return match
	})
}
