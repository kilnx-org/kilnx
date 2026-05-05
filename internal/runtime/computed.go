package runtime

import (
	"strconv"
	"strings"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// findComputedField returns the parser.Field for a model's computed field by name,
// or nil if no such computed field exists.
func findComputedField(models []parser.Model, modelName, fieldName string) *parser.Field {
	if modelName == "" || fieldName == "" {
		return nil
	}
	for i := range models {
		if models[i].Name != modelName {
			continue
		}
		for j := range models[i].Fields {
			f := &models[i].Fields[j]
			if f.Name == fieldName && f.Computed && f.ComputedExpr != "" {
				return f
			}
		}
	}
	return nil
}

// evaluateComputedExpr evaluates a computed field expression against a row.
// Supports: identifiers (resolved against the row), int/float literals,
// + - * / operators, parentheses. Returns the formatted value, or "" if
// any identifier is missing or non-numeric.
func evaluateComputedExpr(expr string, row database.Row) string {
	p := &exprParser{src: expr, row: row}
	p.skipSpaces()
	val, ok := p.parseExpr()
	if !ok {
		return ""
	}
	p.skipSpaces()
	if p.pos != len(p.src) {
		return ""
	}
	return formatComputedNumber(val)
}

type exprParser struct {
	src string
	pos int
	row database.Row
}

func (p *exprParser) skipSpaces() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
		p.pos++
	}
}

// parseExpr handles + and -
func (p *exprParser) parseExpr() (float64, bool) {
	left, ok := p.parseTerm()
	if !ok {
		return 0, false
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.src) {
			return left, true
		}
		op := p.src[p.pos]
		if op != '+' && op != '-' {
			return left, true
		}
		p.pos++
		right, ok := p.parseTerm()
		if !ok {
			return 0, false
		}
		if op == '+' {
			left = left + right
		} else {
			left = left - right
		}
	}
}

// parseTerm handles * and /
func (p *exprParser) parseTerm() (float64, bool) {
	left, ok := p.parseFactor()
	if !ok {
		return 0, false
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.src) {
			return left, true
		}
		op := p.src[p.pos]
		if op != '*' && op != '/' {
			return left, true
		}
		p.pos++
		right, ok := p.parseFactor()
		if !ok {
			return 0, false
		}
		if op == '*' {
			left = left * right
		} else {
			if right == 0 {
				return 0, false
			}
			left = left / right
		}
	}
}

// parseFactor handles unary minus, parens, identifiers, numbers
func (p *exprParser) parseFactor() (float64, bool) {
	p.skipSpaces()
	if p.pos >= len(p.src) {
		return 0, false
	}
	ch := p.src[p.pos]

	// Unary minus
	if ch == '-' {
		p.pos++
		v, ok := p.parseFactor()
		if !ok {
			return 0, false
		}
		return -v, true
	}
	// Unary plus (rare but harmless)
	if ch == '+' {
		p.pos++
		return p.parseFactor()
	}

	// Parens
	if ch == '(' {
		p.pos++
		v, ok := p.parseExpr()
		if !ok {
			return 0, false
		}
		p.skipSpaces()
		if p.pos >= len(p.src) || p.src[p.pos] != ')' {
			return 0, false
		}
		p.pos++
		return v, true
	}

	// Number literal
	if (ch >= '0' && ch <= '9') || ch == '.' {
		return p.parseNumber()
	}

	// Identifier
	if isIdentStart(ch) {
		return p.parseIdent()
	}

	return 0, false
}

func (p *exprParser) parseNumber() (float64, bool) {
	start := p.pos
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if (c >= '0' && c <= '9') || c == '.' {
			p.pos++
			continue
		}
		break
	}
	if start == p.pos {
		return 0, false
	}
	v, err := strconv.ParseFloat(p.src[start:p.pos], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func (p *exprParser) parseIdent() (float64, bool) {
	start := p.pos
	for p.pos < len(p.src) && isIdentPart(p.src[p.pos]) {
		p.pos++
	}
	name := p.src[start:p.pos]
	if name == "" {
		return 0, false
	}
	raw, ok := p.row[name]
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

// formatComputedNumber renders a float as a string, dropping the trailing ".0" for
// integral values so that "2 * 3" renders as "6" rather than "6.000000".
func formatComputedNumber(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	// Trim trailing zeros while keeping precision.
	s := strconv.FormatFloat(v, 'f', -1, 64)
	return s
}

// resolveComputedFromQuery attempts to evaluate a computed field {queryName.field}
// using the first row of the named query and the model that produced it. Returns
// (value, true) on success; otherwise ("", false) so the caller can fall through.
func resolveComputedFromQuery(ctx *renderContext, queryName, field string) (string, bool) {
	if ctx == nil {
		return "", false
	}
	modelName := ctx.querySourceModels[queryName]
	cf := findComputedField(ctx.models, modelName, field)
	if cf == nil {
		return "", false
	}
	rows, ok := ctx.queries[queryName]
	if !ok || len(rows) == 0 {
		return "", false
	}
	return evaluateComputedExpr(cf.ComputedExpr, rows[0]), true
}

// resolveComputedFromRow attempts to evaluate a computed field on a row, using
// the model name carried alongside the row. Returns (value, true) on success.
func resolveComputedFromRow(ctx *renderContext, row database.Row, modelName, field string) (string, bool) {
	if ctx == nil || row == nil {
		return "", false
	}
	cf := findComputedField(ctx.models, modelName, field)
	if cf == nil {
		return "", false
	}
	return evaluateComputedExpr(cf.ComputedExpr, row), true
}
