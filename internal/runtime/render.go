package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
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

var rawFilterRe = regexp.MustCompile(`\{(\^*[a-zA-Z_]\w*(?:\.[a-zA-Z_]\w*)*)\s*\|\s*raw\}`)

// filterRe matches {expr | filter} or {expr | filter1 | filter2: arg}
var filterRe = regexp.MustCompile(`\{(\^*[a-zA-Z_]\w*(?:\.[a-zA-Z_]\w*)*)\s*\|([^}]+)\}`)

// parsedFilter represents a single filter in a chain
type parsedFilter struct {
	Name string
	Args []string
}

// parseFilterChain splits "upcase | truncate: 30 | default: N/A" into filters
func parseFilterChain(chain string) []parsedFilter {
	parts := strings.Split(chain, "|")
	var filters []parsedFilter
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		colonIdx := strings.Index(part, ":")
		if colonIdx < 0 {
			filters = append(filters, parsedFilter{Name: part})
		} else {
			name := strings.TrimSpace(part[:colonIdx])
			argStr := strings.TrimSpace(part[colonIdx+1:])
			var args []string
			for _, a := range strings.Split(argStr, ",") {
				a = strings.TrimSpace(a)
				a = strings.Trim(a, "\"'")
				args = append(args, a)
			}
			filters = append(filters, parsedFilter{Name: name, Args: args})
		}
	}
	return filters
}

// builtinFilters maps filter names to functions
var builtinFilters = map[string]func(string, []string) string{
	"raw": func(v string, _ []string) string { return v },
	"upcase": func(v string, _ []string) string {
		return strings.ToUpper(v)
	},
	"downcase": func(v string, _ []string) string {
		return strings.ToLower(v)
	},
	"capitalize": func(v string, _ []string) string {
		if len(v) == 0 {
			return v
		}
		return strings.ToUpper(v[:1]) + v[1:]
	},
	"truncate": func(v string, args []string) string {
		n := 50
		if len(args) > 0 {
			if parsed, err := strconv.Atoi(args[0]); err == nil {
				n = parsed
			}
		}
		runes := []rune(v)
		if len(runes) <= n {
			return v
		}
		return string(runes[:n]) + "..."
	},
	"default": func(v string, args []string) string {
		if v == "" || v == "<nil>" {
			if len(args) > 0 {
				return args[0]
			}
			return ""
		}
		return v
	},
	"date": func(v string, args []string) string {
		format := "Jan 02, 2006"
		if len(args) > 0 {
			format = goDateFormat(args[0])
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05 -0700 MST", "2006-01-02 15:04:05 +0000 UTC", "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, v); err == nil {
				return t.Format(format)
			}
		}
		return v
	},
	"timeago": func(v string, _ []string) string {
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05 -0700 MST", "2006-01-02 15:04:05 +0000 UTC", "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, v); err == nil {
				return timeAgo(t)
			}
		}
		return v
	},
	"currency": func(v string, args []string) string {
		symbol := "$"
		if len(args) > 0 {
			symbol = args[0]
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return v
		}
		return symbol + formatNumber(f, 2)
	},
	"number": func(v string, args []string) string {
		decimals := 0
		if len(args) > 0 {
			if d, err := strconv.Atoi(args[0]); err == nil {
				decimals = d
			}
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return v
		}
		return formatNumber(f, decimals)
	},
	"pluralize": func(v string, args []string) string {
		singular := "item"
		plural := "items"
		if len(args) >= 2 {
			singular = args[0]
			plural = args[1]
		} else if len(args) == 1 {
			singular = args[0]
			plural = args[0] + "s"
		}
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || n != 1 {
			return plural
		}
		return singular
	},
	"replace": func(v string, args []string) string {
		if len(args) >= 2 {
			return strings.ReplaceAll(v, args[0], args[1])
		}
		return v
	},
	"strip": func(v string, _ []string) string {
		return strings.TrimSpace(v)
	},
	"markdown": func(v string, _ []string) string {
		return renderMarkdown(v)
	},
	"links": func(v string, _ []string) string {
		return linkify(v)
	},
	"unfurl": func(v string, _ []string) string {
		return v + unfurlURLs(v)
	},
}

// applyFilters runs a filter chain on a value, returns (result, isRaw)
func applyFilters(value string, chain string) (string, bool) {
	filters := parseFilterChain(chain)
	isRaw := false
	for _, f := range filters {
		if f.Name == "raw" {
			isRaw = true
			continue
		}
		if fn, ok := builtinFilters[f.Name]; ok {
			value = fn(value, f.Args)
		}
	}
	return value, isRaw
}

// goDateFormat converts strftime-like format to Go layout
func goDateFormat(format string) string {
	r := strings.NewReplacer(
		"%Y", "2006", "%m", "01", "%d", "02",
		"%H", "15", "%M", "04", "%S", "05",
		"%b", "Jan", "%B", "January",
		"%a", "Mon", "%A", "Monday",
		"%p", "PM", "%I", "03",
	)
	return r.Replace(format)
}

// timeAgo returns a human-readable relative time string
func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		months := int(d.Hours() / 24 / 30)
		if months <= 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
}

// formatNumber formats a float with thousand separators and given decimal places
func formatNumber(f float64, decimals int) string {
	neg := ""
	if f < 0 {
		neg = "-"
		f = math.Abs(f)
	}
	intPart := int64(f)
	fracPart := f - float64(intPart)

	// Format integer part with commas
	s := fmt.Sprintf("%d", intPart)
	if len(s) > 3 {
		var parts []string
		for len(s) > 3 {
			parts = append([]string{s[len(s)-3:]}, parts...)
			s = s[:len(s)-3]
		}
		parts = append([]string{s}, parts...)
		s = strings.Join(parts, ",")
	}

	if decimals > 0 {
		frac := fmt.Sprintf("%.*f", decimals, fracPart)
		s += frac[1:] // skip the leading "0"
	}

	return neg + s
}

// renderHTML is the main template processing function for NodeHTML content.
// It processes all template directives and returns the final HTML string.
func renderHTML(content string, ctx *renderContext) string {
	result := content

	// Step 0: Expand translation placeholders {t.key} and {t.key param=expr}
	if ctx.i18n != nil {
		result = expandTranslations(result, ctx)
	}

	// Step 1: Replace each {csrf} with a unique token (one per occurrence for multi-form pages)
	for strings.Contains(result, "{csrf}") {
		token := generateCSRFToken()
		result = strings.Replace(result, "{csrf}", token, 1)
	}

	// Step 2: Process {field | filters} outside of {{each}} blocks
	// Use a random nonce to prevent placeholder collision with user data
	nonce := generateNonce()
	rawPlaceholders := make(map[string]string)
	rawCounter := 0

	// Process all pipe expressions (including raw and other filters)
	// Skip expressions inside {{each}}...{{end}} blocks (they're handled in Step 3)
	insideEach := isInsideEachBlock(result)
	result = filterRe.ReplaceAllStringFunc(result, func(match string) string {
		parts := filterRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		expr := parts[1]
		chain := strings.TrimSpace(parts[2])
		// Skip if this expression is inside an {{each}} block - defer to processRawInRow
		matchIdx := strings.Index(result, match)
		if matchIdx >= 0 && insideEach(matchIdx) {
			return match
		}
		val := resolveValue(expr, ctx, nil)
		if val == "{"+expr+"}" {
			return match // not resolved, leave unchanged
		}
		filtered, isRaw := applyFilters(val, chain)
		if isRaw {
			placeholder := fmt.Sprintf("\x00KILNX_RAW_%s_%d\x00", nonce, rawCounter)
			rawCounter++
			rawPlaceholders[placeholder] = filtered
			return placeholder
		}
		// Non-raw filters: escape and return directly
		placeholder := fmt.Sprintf("\x00KILNX_FILT_%s_%d\x00", nonce, rawCounter)
		rawCounter++
		rawPlaceholders[placeholder] = html.EscapeString(filtered)
		return placeholder
	})

	// Step 2.5: Expand component fragment calls that live OUTSIDE {{each}}
	// blocks. Calls inside {{each}} are expanded per-row in expandEachBlocks
	// so they have access to row context. We deliberately do NOT do another
	// pass after each/interpolate: by then row data has been written into
	// the output and re-scanning would let user content like {{name}} be
	// interpreted as a component call (template injection).
	result = expandFragmentCallsOutsideEach(result, ctx)

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

// isInsideEachBlock returns a closure that checks if a position in the text
// falls inside a {{each}}...{{end}} block. Used to defer filter processing
// to the per-row iteration in expandEachBlocks.
func isInsideEachBlock(text string) func(int) bool {
	type span struct{ start, end int }
	var spans []span
	remaining := text
	offset := 0
	for {
		idx := strings.Index(remaining, "{{each ")
		if idx < 0 {
			break
		}
		tagEnd := strings.Index(remaining[idx:], "}}")
		if tagEnd < 0 {
			break
		}
		bodyStart := idx + tagEnd + 2
		_, _, endPos := findMatchingEnd(remaining[bodyStart:])
		if endPos < 0 {
			break
		}
		endAbs := bodyStart + endPos
		spans = append(spans, span{offset + idx, offset + endAbs})
		remaining = remaining[endAbs:]
		offset += endAbs
	}
	return func(pos int) bool {
		for _, s := range spans {
			if pos >= s.start && pos < s.end {
				return true
			}
		}
		return false
	}
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
				// Push current row onto eachStack for parent-scope resolution
				ctx.eachStack = append(ctx.eachStack, row)
				// Track source model on a parallel stack for computed-field resolution
				ctx.eachModels = append(ctx.eachModels, ctx.querySourceModels[queryName])
				ctx.currentRow = row
				// Rebind q.custom per row so {{each q.custom}} works on list pages (#66)
				if sourceModel, ok := ctx.querySourceModels[queryName]; ok {
					if manifest, ok := ctx.customManifests[sourceModel]; ok {
						ctx.queries[queryName+".custom"] = buildCustomIterRows(row, manifest)
					}
				}
				// Process raw filters inside each body with current row context
				expanded := processRawInRow(body, row, ctx, nonce)
				// Process nested {{each}} blocks
				expanded = expandEachBlocks(expanded, ctx, nonce)
				// Process {{if}} blocks with current row context
				expanded = expandIfBlocks(expanded, ctx, row)
				// Process fragment component calls with current row context
				expanded = expandFragmentCalls(expanded, ctx)
				// Interpolate row fields
				expanded = interpolateRow(expanded, row, ctx)
				result.WriteString(expanded)
				// Pop row from eachStack
				ctx.eachStack = ctx.eachStack[:len(ctx.eachStack)-1]
				ctx.eachModels = ctx.eachModels[:len(ctx.eachModels)-1]
				ctx.currentRow = nil
			}
		}

		remaining = remaining[bodyStart+endPos:]
	}

	return result.String()
}

// processRawInRow handles {field | filters} inside {{each}} blocks where field comes from the current row.
// Skips expressions that are inside nested {{each}} blocks so the inner loop processes them.
func processRawInRow(content string, row database.Row, ctx *renderContext, nonce string) string {
	counter := 0
	placeholders := make(map[string]string)
	insideEach := isInsideEachBlock(content)

	result := filterRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := filterRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		expr := parts[1]
		chain := strings.TrimSpace(parts[2])
		// Skip if this expression is inside a nested {{each}} block
		matchIdx := strings.Index(content, match)
		if matchIdx >= 0 && insideEach(matchIdx) {
			return match
		}
		val := resolveValue(expr, ctx, row)
		if val == "{"+expr+"}" {
			return match
		}
		filtered, isRaw := applyFilters(val, chain)
		placeholder := fmt.Sprintf("\x00KILNX_RROW_%s_%d\x00", nonce, counter)
		counter++
		if isRaw {
			placeholders[placeholder] = filtered
		} else {
			placeholders[placeholder] = html.EscapeString(filtered)
		}
		return placeholder
	})

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
//	field                                -> truthy check (non-empty, non-zero, non-false)
//	field == "value"                     -> equality
//	field != "value"                     -> inequality
//	queryName.count > 0                  -> numeric comparison
//	queryName.count == 0                 -> numeric/string equality
//	expr1 and expr2                      -> logical AND
//	expr1 or expr2                       -> logical OR
func evaluateCondition(condition string, ctx *renderContext, currentRow database.Row) bool {
	// Split on " or " first (lower precedence), respecting quoted strings
	if parts := splitLogical(condition, " or "); len(parts) > 1 {
		for _, part := range parts {
			if evaluateCondition(strings.TrimSpace(part), ctx, currentRow) {
				return true
			}
		}
		return false
	}

	// Split on " and " (higher precedence)
	if parts := splitLogical(condition, " and "); len(parts) > 1 {
		for _, part := range parts {
			if !evaluateCondition(strings.TrimSpace(part), ctx, currentRow) {
				return false
			}
		}
		return true
	}

	return evaluateSingleCondition(condition, ctx, currentRow)
}

// splitLogical splits a condition string on a logical keyword (" and " or " or "),
// respecting quoted strings. Returns the original string in a single-element slice
// if the keyword is not found outside quotes.
func splitLogical(condition, keyword string) []string {
	var parts []string
	inQuote := byte(0)
	last := 0
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
		if i+len(keyword) <= len(condition) && condition[i:i+len(keyword)] == keyword {
			parts = append(parts, condition[last:i])
			last = i + len(keyword)
			i += len(keyword) - 1
		}
	}
	if len(parts) > 0 {
		parts = append(parts, condition[last:])
	}
	return parts
}

// evaluateSingleCondition evaluates a single comparison or truthy check.
func evaluateSingleCondition(condition string, ctx *renderContext, currentRow database.Row) bool {
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
// Handles: "paginate.query.field", "params.key", "queryName.field", "queryName.count",
// bare "field", and parent-scope "^field", "^^field", etc.
// Returns the original "{expr}" string if not found.
func resolveValue(expr string, ctx *renderContext, currentRow database.Row) string {
	// Parent-scope reference: count leading ^ to walk up the eachStack
	if strings.HasPrefix(expr, "^") {
		depth := 0
		for depth < len(expr) && expr[depth] == '^' {
			depth++
		}
		field := expr[depth:]
		if len(ctx.eachStack) > depth {
			idx := len(ctx.eachStack) - depth - 1
			if val, ok := ctx.eachStack[idx][field]; ok {
				return val
			}
		}
		return "{" + expr + "}"
	}

	// Check fragment component arguments first
	if ctx.fragmentArgs != nil {
		if val, ok := ctx.fragmentArgs[expr]; ok {
			return val
		}
	}

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
		// Fall back to computed-field evaluation: {orders.total} where total
		// is declared as `total: computed quantity * unit_price` on the
		// model behind the orders query.
		if val, ok := resolveComputedFromQuery(ctx, prefix, field); ok {
			return val
		}
		return ""
	}

	// Single name: check current row first, then all queries
	if currentRow != nil {
		if val, ok := currentRow[expr]; ok {
			return val
		}
		// Inside an {{each}}, resolve computed fields against the current row
		// using the source model recorded on eachModels.
		if len(ctx.eachModels) > 0 {
			modelName := ctx.eachModels[len(ctx.eachModels)-1]
			if val, ok := resolveComputedFromRow(ctx, currentRow, modelName, expr); ok {
				return val
			}
		}
	}
	for queryName, rows := range ctx.queries {
		if len(rows) > 0 {
			if val, ok := rows[0][expr]; ok {
				return val
			}
			if val, ok := resolveComputedFromQuery(ctx, queryName, expr); ok {
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

// renderMarkdown converts a subset of markdown to HTML.
// Supports: **bold**, *italic*, `inline code`, ```code blocks```,
// [links](url), ~strikethrough~, and newlines to <br>.
func renderMarkdown(text string) string {
	// Escape HTML first (markdown output is raw)
	text = html.EscapeString(text)

	// Code blocks: ```...```
	codeBlockRe := regexp.MustCompile("(?s)```(\\w*)\\n?(.*?)```")
	text = codeBlockRe.ReplaceAllStringFunc(text, func(m string) string {
		parts := codeBlockRe.FindStringSubmatch(m)
		code := parts[2]
		return "<pre><code>" + code + "</code></pre>"
	})

	// Inline code: `...`
	inlineCodeRe := regexp.MustCompile("`([^`]+)`")
	text = inlineCodeRe.ReplaceAllString(text, "<code>$1</code>")

	// Bold: **text**
	boldRe := regexp.MustCompile(`\*\*([^*]+)\*\*`)
	text = boldRe.ReplaceAllString(text, "<strong>$1</strong>")

	// Italic: *text*
	italicRe := regexp.MustCompile(`\*([^*]+)\*`)
	text = italicRe.ReplaceAllString(text, "<em>$1</em>")

	// Strikethrough: ~text~
	strikeRe := regexp.MustCompile(`~([^~]+)~`)
	text = strikeRe.ReplaceAllString(text, "<del>$1</del>")

	// Links: [text](url) — sanitize scheme to prevent javascript: XSS
	linkRe := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	text = linkRe.ReplaceAllStringFunc(text, func(m string) string {
		parts := linkRe.FindStringSubmatch(m)
		if len(parts) < 3 {
			return m
		}
		label, rawURL := parts[1], parts[2]
		if !isAllowedURLScheme(rawURL) {
			return label
		}
		return fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener">%s</a>`, rawURL, label)
	})

	// @mentions
	mentionRe := regexp.MustCompile(`@([a-zA-Z0-9_]+)`)
	text = mentionRe.ReplaceAllString(text, `<span style="color:#1d9bd1;background:rgba(29,155,209,0.1);padding:1px 4px;border-radius:3px;font-weight:600">@$1</span>`)

	// Auto-link bare URLs
	text = linkify(text)

	// Newlines to <br>
	text = strings.ReplaceAll(text, "\n", "<br>")

	return text
}

// allowedURLSchemes is the whitelist of safe URL schemes for links.
var allowedURLSchemes = map[string]bool{
	"http": true, "https": true, "mailto": true, "tel": true,
}

// isAllowedURLScheme returns true if the URL uses an allowed scheme.
func isAllowedURLScheme(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme == "" {
		// Allow schemeless relative URLs (e.g. /path or #anchor)
		return true
	}
	return allowedURLSchemes[strings.ToLower(u.Scheme)]
}

// linkify converts bare URLs in text to clickable <a> tags.
var urlRe = regexp.MustCompile(`(^|[\s(])(https?://[^\s<>")\]]+)`)

func linkify(text string) string {
	return urlRe.ReplaceAllString(text, `$1<a href="$2" target="_blank" rel="noopener">$2</a>`)
}

// fragmentCallRe matches {{name key=expr key2=expr}} (args optional)
var fragmentCallRe = regexp.MustCompile(`\{\{(\w+)(?:\s+([^}]+))?\}\}`)

// splitArgStr tokenizes a fragment call argument string into space-separated
// tokens, respecting single and double quoted substrings. Whitespace inside
// quotes is preserved within the token (e.g. title="Hello World").
func splitArgStr(s string) []string {
	var tokens []string
	var cur strings.Builder
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote != 0 {
			cur.WriteByte(c)
			if c == inQuote {
				inQuote = 0
			}
		} else if c == '"' || c == '\'' {
			inQuote = c
			cur.WriteByte(c)
		} else if c == ' ' || c == '\t' {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		} else {
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// expandFragmentCallsOutsideEach expands fragment component calls that are
// NOT inside a {{each}}...{{end}} block. Calls inside each-blocks are handled
// per-row inside expandEachBlocks (so they can reference the current row).
func expandFragmentCallsOutsideEach(content string, ctx *renderContext) string {
	if ctx.fragmentComponents == nil || len(ctx.fragmentComponents) == 0 {
		return content
	}
	insideEach := isInsideEachBlock(content)
	matches := fragmentCallRe.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return content
	}
	var b strings.Builder
	b.Grow(len(content))
	prev := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		b.WriteString(content[prev:start])
		match := content[start:end]
		if insideEach(start) {
			b.WriteString(match)
		} else {
			b.WriteString(expandSingleFragmentCall(match, ctx))
		}
		prev = end
	}
	b.WriteString(content[prev:])
	return b.String()
}

// expandSingleFragmentCall expands one {{name args}} occurrence using ctx.
// Returns the original match if the name is not a known component or if the
// recursion guard has fired.
func expandSingleFragmentCall(match string, ctx *renderContext) string {
	if ctx.fragmentComponents == nil || len(ctx.fragmentComponents) == 0 {
		return match
	}
	if ctx.fragmentDepth >= 2 {
		return match
	}
	parts := fragmentCallRe.FindStringSubmatch(match)
	if len(parts) < 2 {
		return match
	}
	name := parts[1]
	argStr := ""
	if len(parts) > 2 {
		argStr = parts[2]
	}
	frag, ok := ctx.fragmentComponents[name]
	if !ok {
		return match
	}
	argMap := buildFragmentArgs(argStr, frag, ctx)
	var fragHTML string
	for _, node := range frag.Body {
		if node.Type == parser.NodeHTML {
			fragHTML = node.HTMLContent
			break
		}
	}
	if fragHTML == "" {
		return ""
	}
	childCtx := &renderContext{
		queries:            ctx.queries,
		paginate:           ctx.paginate,
		currentUser:        ctx.currentUser,
		queryParams:        ctx.queryParams,
		querySourceModels:  ctx.querySourceModels,
		customManifests:    ctx.customManifests,
		eachStack:          ctx.eachStack,
		fragmentArgs:       argMap,
		fragmentDepth:      ctx.fragmentDepth + 1,
		fragmentComponents: ctx.fragmentComponents,
	}
	return renderHTML(fragHTML, childCtx)
}

// buildFragmentArgs parses an argument string and applies defaults for a fragment.
func buildFragmentArgs(argStr string, frag *parser.Page, ctx *renderContext) map[string]string {
	argMap := make(map[string]string)
	for _, pair := range splitArgStr(argStr) {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		eqIdx := strings.Index(pair, "=")
		if eqIdx < 0 {
			continue
		}
		key := strings.TrimSpace(pair[:eqIdx])
		expr := strings.TrimSpace(pair[eqIdx+1:])
		var val string
		if strings.HasPrefix(expr, `"`) && strings.HasSuffix(expr, `"`) && len(expr) >= 2 {
			val = expr[1 : len(expr)-1]
		} else if strings.HasPrefix(expr, `'`) && strings.HasSuffix(expr, `'`) && len(expr) >= 2 {
			val = expr[1 : len(expr)-1]
		} else {
			resolved := resolveValue(expr, ctx, ctx.currentRow)
			if resolved == "{"+expr+"}" {
				val = expr
			} else {
				val = resolved
			}
		}
		argMap[key] = val
	}
	for _, arg := range frag.FragmentArgs {
		if _, ok := argMap[arg.Name]; !ok && arg.HasDefault {
			argMap[arg.Name] = arg.DefaultValue
		}
	}
	return argMap
}

// expandFragmentCalls processes {{componentName arg=expr}} blocks inside HTML content.
// It looks up the component fragment, binds arguments, and renders the body inline.
func expandFragmentCalls(content string, ctx *renderContext) string {
	if ctx.fragmentComponents == nil || len(ctx.fragmentComponents) == 0 {
		return content
	}
	if ctx.fragmentDepth >= 2 {
		// Recursion guard: don't expand beyond depth 2
		return content
	}

	return fragmentCallRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := fragmentCallRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		name := parts[1]
		argStr := ""
		if len(parts) > 2 {
			argStr = parts[2]
		}

		frag, ok := ctx.fragmentComponents[name]
		if !ok {
			return match // unknown component, leave for analyzer to catch
		}

		// Parse arguments: key=expr key2=expr (quoted values may contain spaces)
		argMap := make(map[string]string)
		for _, pair := range splitArgStr(argStr) {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			eqIdx := strings.Index(pair, "=")
			if eqIdx < 0 {
				continue // malformed, skip
			}
			key := strings.TrimSpace(pair[:eqIdx])
			expr := strings.TrimSpace(pair[eqIdx+1:])
			var val string
			if strings.HasPrefix(expr, `"`) && strings.HasSuffix(expr, `"`) {
				// String literal: "value"
				val = expr[1 : len(expr)-1]
			} else if strings.HasPrefix(expr, `'`) && strings.HasSuffix(expr, `'`) {
				// String literal: 'value'
				val = expr[1 : len(expr)-1]
			} else {
				// Try to resolve as variable/query/field reference
				resolved := resolveValue(expr, ctx, ctx.currentRow)
				if resolved == "{"+expr+"}" {
					// Not resolved — treat as literal string
					val = expr
				} else {
					val = resolved
				}
			}
			argMap[key] = val
		}

		// Apply defaults for missing args
		for _, arg := range frag.FragmentArgs {
			if _, ok := argMap[arg.Name]; !ok && arg.HasDefault {
				argMap[arg.Name] = arg.DefaultValue
			}
		}

		// Find the HTML body of the component
		var fragHTML string
		for _, node := range frag.Body {
			if node.Type == parser.NodeHTML {
				fragHTML = node.HTMLContent
				break
			}
		}
		if fragHTML == "" {
			return "" // no HTML body
		}

		// Create child context with bound args and incremented depth
		childCtx := &renderContext{
			queries:            ctx.queries,
			paginate:           ctx.paginate,
			currentUser:        ctx.currentUser,
			queryParams:        ctx.queryParams,
			querySourceModels:  ctx.querySourceModels,
			customManifests:    ctx.customManifests,
			eachStack:          ctx.eachStack,
			fragmentArgs:       argMap,
			fragmentDepth:      ctx.fragmentDepth + 1,
			fragmentComponents: ctx.fragmentComponents,
		}

		rendered := renderHTML(fragHTML, childCtx)
		return rendered
	})
}

// translationRe matches {t.key} and {t.key param=expr}
var translationRe = regexp.MustCompile(`\{t\.(\w+)(?:\s+([^}]+))?\}`)

// expandTranslations processes {t.key} and {t.key param=expr} placeholders.
// It looks up the translation in the detected language, resolves any parameters,
// and applies pluralization filters within the translated value.
func expandTranslations(content string, ctx *renderContext) string {
	return translationRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := translationRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		key := parts[1]
		argStr := ""
		if len(parts) > 2 {
			argStr = parts[2]
		}

		// Get translated value
		val := ctx.i18n.Translate(key, ctx.request)
		if val == key {
			return match // translation not found, leave placeholder
		}

		// Parse and resolve parameters
		params := make(map[string]string)
		for _, pair := range strings.Split(argStr, " ") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			eqIdx := strings.Index(pair, "=")
			if eqIdx < 0 {
				continue
			}
			pname := strings.TrimSpace(pair[:eqIdx])
			expr := strings.TrimSpace(pair[eqIdx+1:])
			// Strip leading : from parameter references
			expr = strings.TrimPrefix(expr, ":")
			var pval string
			if strings.HasPrefix(expr, `"`) && strings.HasSuffix(expr, `"`) {
				pval = expr[1 : len(expr)-1]
			} else if strings.HasPrefix(expr, `'`) && strings.HasSuffix(expr, `'`) {
				pval = expr[1 : len(expr)-1]
			} else {
				resolved := resolveValue(expr, ctx, ctx.currentRow)
				if resolved == "{"+expr+"}" {
					pval = expr // fallback to literal
				} else {
					pval = resolved
				}
			}
			params[pname] = pval
		}

		// Process pluralization filter first (before parameter substitution)
		result := expandPluralization(val, ctx, params)

		// Substitute parameters into translation value
		paramRe := regexp.MustCompile(`\{(\w+)\}`)
		result = paramRe.ReplaceAllStringFunc(result, func(m string) string {
			pparts := paramRe.FindStringSubmatch(m)
			if len(pparts) < 2 {
				return m
			}
			pname := pparts[1]
			if pval, ok := params[pname]; ok {
				return pval
			}
			return m // missing param, leave placeholder
		})

		return result
	})
}

// expandPluralization processes {expr|plural:'singular','plural'} within a string.
func expandPluralization(content string, ctx *renderContext, params map[string]string) string {
	result := content
	for {
		idx := strings.Index(result, "|plural")
		if idx < 0 {
			break
		}
		// Find the opening '{' of this expression
		start := strings.LastIndex(result[:idx], "{")
		if start < 0 {
			break
		}
		// Find the matching '}' (accounting for nested braces)
		depth := 1
		end := -1
		for i := start + 1; i < len(result); i++ {
			if result[i] == '{' {
				depth++
			} else if result[i] == '}' {
				depth--
				if depth == 0 {
					end = i
					break
				}
			}
		}
		if end < 0 {
			break
		}

		match := result[start : end+1]
		inner := result[start+1 : end]
		// inner format: expr | plural : spec
		colonIdx := strings.Index(inner, ":")
		if colonIdx < 0 {
			break
		}
		pipeIdx := strings.Index(inner, "|")
		if pipeIdx < 0 {
			break
		}
		countExpr := strings.TrimSpace(inner[:pipeIdx])
		pluralSpec := strings.TrimSpace(inner[colonIdx+1:])

		var countStr string
		if pval, ok := params[countExpr]; ok {
			countStr = pval
		} else {
			countStr = resolveValue(countExpr, ctx, ctx.currentRow)
		}
		count, _ := strconv.Atoi(countStr)

		replacement := match
		if strings.Contains(pluralSpec, "=") {
			// Extended form: one='1 item', other='{count} items'
			for _, part := range splitPluralSpec(pluralSpec) {
				part = strings.TrimSpace(part)
				eqIdx := strings.Index(part, "=")
				if eqIdx < 0 {
					continue
				}
				category := strings.TrimSpace(part[:eqIdx])
				form := strings.TrimSpace(part[eqIdx+1:])
				form = strings.Trim(form, `"'`)
				if category == "zero" && count == 0 {
					replacement = strings.ReplaceAll(form, "{"+countExpr+"}", countStr)
					break
				}
				if category == "one" && count == 1 {
					replacement = strings.ReplaceAll(form, "{"+countExpr+"}", countStr)
					break
				}
				if category == "two" && count == 2 {
					replacement = strings.ReplaceAll(form, "{"+countExpr+"}", countStr)
					break
				}
				if category == "other" {
					replacement = strings.ReplaceAll(form, "{"+countExpr+"}", countStr)
					break
				}
			}
		} else {
			// Simple form: 'one','other' or "one","other"
			forms := strings.Split(pluralSpec, ",")
			if len(forms) >= 2 {
				singular := strings.TrimSpace(forms[0])
				plural := strings.TrimSpace(forms[1])
				singular = strings.Trim(singular, `"'`)
				plural = strings.Trim(plural, `"'`)
				if count == 1 {
					replacement = singular
				} else {
					replacement = plural
				}
			}
		}

		result = result[:start] + replacement + result[end+1:]
	}
	return result
}

// splitPluralSpec splits a plural spec by comma, but not inside quotes.
func splitPluralSpec(spec string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)
	for i := 0; i < len(spec); i++ {
		ch := spec[i]
		if !inQuote && (ch == '"' || ch == '\'') {
			inQuote = true
			quoteChar = ch
			current.WriteByte(ch)
		} else if inQuote && ch == quoteChar {
			inQuote = false
			current.WriteByte(ch)
		} else if ch == ',' && !inQuote {
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}
