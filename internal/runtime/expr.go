package runtime

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"crypto/rand"

	"golang.org/x/crypto/bcrypt"
)

// stdlibFn implements a Kilnx stdlib function. All values are stringly typed
// (Kilnx is string-first end-to-end); functions parse / format on demand.
type stdlibFn func(args []string) (string, error)

// stdlib is the registry of pure functions callable from Kilnx expressions
// (e.g. `body amount: round(:total * 1.5)`, `body slug: slugify(:title)`).
//
// Functions must be deterministic relative to their arguments OR explicit
// producers (uuid/now). They have no I/O, no DB, no network. WASM-style
// extensibility is reserved for the `plugin` keyword.
var stdlib = map[string]stdlibFn{
	"lower":    fnUnary(strings.ToLower),
	"upper":    fnUnary(strings.ToUpper),
	"trim":     fnUnary(strings.TrimSpace),
	"len":      func(a []string) (string, error) { return strconv.Itoa(stringLen(arg(a, 0))), nil },
	"slugify":  fnUnary(slugify),
	"bcrypt":   fnBcrypt,
	"sha256":   fnUnary(sha256Hex),
	"base64":   fnUnary(func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }),
	"unbase64": fnUnbase64,
	"uuid":     fnUUID,
	"now":      fnNow,
	"coalesce": fnCoalesce,
	"regex":    fnRegexExtract,
	"matches":  fnMatches,
	"json_get": fnJSONGet,
	"round":    fnRound,
	"floor":    fnFloor,
	"ceil":     fnCeil,
	"abs":      fnAbs,
	"min":      fnMin,
	"max":      fnMax,
	"replace":  fnReplace,
	"contains": fnContains,
	"starts":   fnStarts,
	"ends":     fnEnds,
	"int":      fnInt,
	"format":   fnFormat,
	"clamp":    fnClamp,
}

// resolveKxValue is the single entry point used by fetch / future-callers to
// turn a user-supplied string value into its final form. It tries the
// expression evaluator first; on failure it falls back to legacy `:param`
// substitution so existing apps keep working.
func resolveKxValue(value string, params map[string]string) string {
	trimmed := strings.TrimSpace(value)
	if looksLikeExpression(trimmed) {
		if got, err := evalExpression(trimmed, params); err == nil {
			return got
		}
	}
	return substituteParams(value, params)
}

// looksLikeExpression returns true when the value starts with an identifier
// followed by `(` — the only opt-in shape that activates the evaluator.
// Plain `:foo`, "Bearer :token", and arithmetic-looking text without a
// leading function call are intentionally treated as templates so existing
// usage is preserved exactly.
var exprStartRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*\s*\(`)

func looksLikeExpression(s string) bool {
	return exprStartRe.MatchString(s)
}

// substituteParams replaces every `:key` reference in value with its mapped
// param. Keys are tried longest-first so `:user.id` takes precedence over
// `:user`.
func substituteParams(value string, params map[string]string) string {
	if !strings.Contains(value, ":") || len(params) == 0 {
		return value
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	// longest first
	sortStringsByLenDesc(keys)
	for _, k := range keys {
		value = strings.ReplaceAll(value, ":"+k, params[k])
	}
	return value
}

func sortStringsByLenDesc(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && len(s[j]) > len(s[j-1]); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// ---------- Expression evaluator ----------

type kxExprParser struct {
	src    string
	pos    int
	params map[string]string
}

func evalExpression(src string, params map[string]string) (string, error) {
	p := &kxExprParser{src: src, params: params}
	v, err := p.parseAdditive()
	if err != nil {
		return "", err
	}
	p.skipSpace()
	if p.pos != len(p.src) {
		return "", fmt.Errorf("unexpected trailing input %q", p.src[p.pos:])
	}
	return v, nil
}

func (p *kxExprParser) parseAdditive() (string, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return "", err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.src) {
			return left, nil
		}
		op := p.src[p.pos]
		if op != '+' && op != '-' {
			return left, nil
		}
		p.pos++
		right, err := p.parseMultiplicative()
		if err != nil {
			return "", err
		}
		left, err = applyOp(left, op, right)
		if err != nil {
			return "", err
		}
	}
}

func (p *kxExprParser) parseMultiplicative() (string, error) {
	left, err := p.parseUnary()
	if err != nil {
		return "", err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.src) {
			return left, nil
		}
		op := p.src[p.pos]
		if op != '*' && op != '/' {
			return left, nil
		}
		p.pos++
		right, err := p.parseUnary()
		if err != nil {
			return "", err
		}
		left, err = applyOp(left, op, right)
		if err != nil {
			return "", err
		}
	}
}

func (p *kxExprParser) parseUnary() (string, error) {
	p.skipSpace()
	if p.pos < len(p.src) && p.src[p.pos] == '-' {
		p.pos++
		v, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		return applyOp("0", '-', v)
	}
	return p.parsePrimary()
}

func (p *kxExprParser) parsePrimary() (string, error) {
	p.skipSpace()
	if p.pos >= len(p.src) {
		return "", fmt.Errorf("unexpected end of expression")
	}
	ch := p.src[p.pos]

	// Parenthesized
	if ch == '(' {
		p.pos++
		v, err := p.parseAdditive()
		if err != nil {
			return "", err
		}
		p.skipSpace()
		if p.pos >= len(p.src) || p.src[p.pos] != ')' {
			return "", fmt.Errorf("expected `)`")
		}
		p.pos++
		return v, nil
	}

	// String literal
	if ch == '"' || ch == '\'' {
		return p.parseStringLiteral(ch)
	}

	// :param
	if ch == ':' {
		p.pos++
		name := p.readIdentifierWithDots()
		if name == "" {
			return "", fmt.Errorf("expected param name after `:`")
		}
		return p.params[name], nil
	}

	// Numeric literal
	if ch >= '0' && ch <= '9' {
		return p.parseNumber(), nil
	}

	// Identifier => function call OR bare token
	if isIdentStart(ch) {
		name := p.readIdentifier()
		p.skipSpace()
		if p.pos < len(p.src) && p.src[p.pos] == '(' {
			args, err := p.parseCallArgs()
			if err != nil {
				return "", err
			}
			fn, ok := stdlib[name]
			if !ok {
				return "", fmt.Errorf("unknown function %q", name)
			}
			return fn(args)
		}
		// Bare identifier: treat as literal symbol — useful for `currency: usd`
		return name, nil
	}

	return "", fmt.Errorf("unexpected character %q at %d", string(ch), p.pos)
}

func (p *kxExprParser) parseCallArgs() ([]string, error) {
	if p.src[p.pos] != '(' {
		return nil, fmt.Errorf("expected `(`")
	}
	p.pos++
	var args []string
	p.skipSpace()
	if p.pos < len(p.src) && p.src[p.pos] == ')' {
		p.pos++
		return args, nil
	}
	for {
		v, err := p.parseAdditive()
		if err != nil {
			return nil, err
		}
		args = append(args, v)
		p.skipSpace()
		if p.pos >= len(p.src) {
			return nil, fmt.Errorf("unterminated argument list")
		}
		if p.src[p.pos] == ',' {
			p.pos++
			continue
		}
		if p.src[p.pos] == ')' {
			p.pos++
			return args, nil
		}
		return nil, fmt.Errorf("unexpected %q in argument list", string(p.src[p.pos]))
	}
}

func (p *kxExprParser) parseStringLiteral(quote byte) (string, error) {
	p.pos++ // consume opening quote
	var b strings.Builder
	for p.pos < len(p.src) {
		ch := p.src[p.pos]
		if ch == '\\' && p.pos+1 < len(p.src) {
			next := p.src[p.pos+1]
			switch next {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			case quote:
				b.WriteByte(quote)
			default:
				b.WriteByte(next)
			}
			p.pos += 2
			continue
		}
		if ch == quote {
			p.pos++
			return b.String(), nil
		}
		b.WriteByte(ch)
		p.pos++
	}
	return "", fmt.Errorf("unterminated string literal")
}

func (p *kxExprParser) parseNumber() string {
	start := p.pos
	for p.pos < len(p.src) && ((p.src[p.pos] >= '0' && p.src[p.pos] <= '9') || p.src[p.pos] == '.') {
		p.pos++
	}
	return p.src[start:p.pos]
}

func (p *kxExprParser) readIdentifier() string {
	start := p.pos
	for p.pos < len(p.src) && isIdentPart(p.src[p.pos]) {
		p.pos++
	}
	return p.src[start:p.pos]
}

func (p *kxExprParser) readIdentifierWithDots() string {
	start := p.pos
	for p.pos < len(p.src) && (isIdentPart(p.src[p.pos]) || p.src[p.pos] == '.') {
		p.pos++
	}
	return p.src[start:p.pos]
}

func (p *kxExprParser) skipSpace() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
		p.pos++
	}
}

// applyOp performs string-aware arithmetic. If both sides parse as numbers
// the result is numeric (int when result is whole, else float). Otherwise
// `+` falls back to string concatenation (handy for building keys/ids);
// other ops require numbers.
func applyOp(left string, op byte, right string) (string, error) {
	la, aok := strconv.ParseFloat(left, 64)
	lb, bok := strconv.ParseFloat(right, 64)
	if aok == nil && bok == nil {
		var r float64
		switch op {
		case '+':
			r = la + lb
		case '-':
			r = la - lb
		case '*':
			r = la * lb
		case '/':
			if lb == 0 {
				return "", fmt.Errorf("division by zero")
			}
			r = la / lb
		}
		return formatFloat(r), nil
	}
	if op == '+' {
		return left + right, nil
	}
	return "", fmt.Errorf("operator %q requires numbers (got %q, %q)", string(op), left, right)
}

func formatFloat(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// ---------- Stdlib helpers ----------

func arg(a []string, i int) string {
	if i < len(a) {
		return a[i]
	}
	return ""
}

func fnUnary(f func(string) string) stdlibFn {
	return func(a []string) (string, error) {
		if len(a) != 1 {
			return "", fmt.Errorf("expected 1 argument, got %d", len(a))
		}
		return f(a[0]), nil
	}
}

func stringLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func fnBcrypt(a []string) (string, error) {
	if len(a) != 1 {
		return "", fmt.Errorf("bcrypt: expected 1 argument, got %d", len(a))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(a[0]), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func fnUnbase64(a []string) (string, error) {
	if len(a) != 1 {
		return "", fmt.Errorf("unbase64: expected 1 argument, got %d", len(a))
	}
	b, err := base64.StdEncoding.DecodeString(a[0])
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func fnUUID(a []string) (string, error) {
	if len(a) != 0 {
		return "", fmt.Errorf("uuid: expected 0 arguments, got %d", len(a))
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func fnNow(a []string) (string, error) {
	switch len(a) {
	case 0:
		return time.Now().UTC().Format(time.RFC3339), nil
	case 1:
		return time.Now().UTC().Format(goDateFormat(a[0])), nil
	default:
		return "", fmt.Errorf("now: expected 0 or 1 arguments")
	}
}

func fnCoalesce(a []string) (string, error) {
	for _, v := range a {
		if v != "" {
			return v, nil
		}
	}
	return "", nil
}

func fnRegexExtract(a []string) (string, error) {
	if len(a) != 2 {
		return "", fmt.Errorf("regex: expected (input, pattern)")
	}
	re, err := regexp.Compile(a[1])
	if err != nil {
		return "", err
	}
	m := re.FindStringSubmatch(a[0])
	if len(m) == 0 {
		return "", nil
	}
	if len(m) > 1 {
		return m[1], nil
	}
	return m[0], nil
}

func fnMatches(a []string) (string, error) {
	if len(a) != 2 {
		return "", fmt.Errorf("matches: expected (input, pattern)")
	}
	re, err := regexp.Compile(a[1])
	if err != nil {
		return "", err
	}
	if re.MatchString(a[0]) {
		return "true", nil
	}
	return "false", nil
}

func fnJSONGet(a []string) (string, error) {
	if len(a) != 2 {
		return "", fmt.Errorf("json_get: expected (json, path)")
	}
	var v any
	if err := json.Unmarshal([]byte(a[0]), &v); err != nil {
		return "", err
	}
	for _, seg := range strings.Split(a[1], ".") {
		switch m := v.(type) {
		case map[string]any:
			v = m[seg]
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(m) {
				return "", nil
			}
			v = m[idx]
		default:
			return "", nil
		}
	}
	if v == nil {
		return "", nil
	}
	switch x := v.(type) {
	case string:
		return x, nil
	case float64:
		return formatFloat(x), nil
	case bool:
		return strconv.FormatBool(x), nil
	default:
		b, _ := json.Marshal(x)
		return string(b), nil
	}
}

func fnRound(a []string) (string, error) {
	if len(a) < 1 || len(a) > 2 {
		return "", fmt.Errorf("round: expected (n) or (n, digits)")
	}
	f, err := strconv.ParseFloat(a[0], 64)
	if err != nil {
		return "", err
	}
	digits := 0
	if len(a) == 2 {
		d, err := strconv.Atoi(a[1])
		if err != nil {
			return "", err
		}
		digits = d
	}
	mul := math.Pow(10, float64(digits))
	rounded := math.Round(f*mul) / mul
	if digits <= 0 {
		return strconv.FormatInt(int64(rounded), 10), nil
	}
	return strconv.FormatFloat(rounded, 'f', digits, 64), nil
}

func fnFloor(a []string) (string, error) {
	f, err := strconv.ParseFloat(arg(a, 0), 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(int64(math.Floor(f)), 10), nil
}

func fnCeil(a []string) (string, error) {
	f, err := strconv.ParseFloat(arg(a, 0), 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(int64(math.Ceil(f)), 10), nil
}

func fnAbs(a []string) (string, error) {
	f, err := strconv.ParseFloat(arg(a, 0), 64)
	if err != nil {
		return "", err
	}
	return formatFloat(math.Abs(f)), nil
}

func fnMin(a []string) (string, error) {
	if len(a) == 0 {
		return "", fmt.Errorf("min: at least 1 argument")
	}
	min, err := strconv.ParseFloat(a[0], 64)
	if err != nil {
		return "", err
	}
	for _, v := range a[1:] {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return "", err
		}
		if f < min {
			min = f
		}
	}
	return formatFloat(min), nil
}

func fnMax(a []string) (string, error) {
	if len(a) == 0 {
		return "", fmt.Errorf("max: at least 1 argument")
	}
	max, err := strconv.ParseFloat(a[0], 64)
	if err != nil {
		return "", err
	}
	for _, v := range a[1:] {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return "", err
		}
		if f > max {
			max = f
		}
	}
	return formatFloat(max), nil
}

func fnReplace(a []string) (string, error) {
	if len(a) != 3 {
		return "", fmt.Errorf("replace: expected (input, old, new)")
	}
	return strings.ReplaceAll(a[0], a[1], a[2]), nil
}

func fnContains(a []string) (string, error) {
	if len(a) != 2 {
		return "", fmt.Errorf("contains: expected (input, substr)")
	}
	if strings.Contains(a[0], a[1]) {
		return "true", nil
	}
	return "false", nil
}

func fnStarts(a []string) (string, error) {
	if len(a) != 2 {
		return "", fmt.Errorf("starts: expected (input, prefix)")
	}
	if strings.HasPrefix(a[0], a[1]) {
		return "true", nil
	}
	return "false", nil
}

func fnEnds(a []string) (string, error) {
	if len(a) != 2 {
		return "", fmt.Errorf("ends: expected (input, suffix)")
	}
	if strings.HasSuffix(a[0], a[1]) {
		return "true", nil
	}
	return "false", nil
}

func fnInt(a []string) (string, error) {
	f, err := strconv.ParseFloat(arg(a, 0), 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(int64(f), 10), nil
}

func fnFormat(a []string) (string, error) {
	if len(a) < 1 {
		return "", fmt.Errorf("format: expected (template, ...values)")
	}
	tmpl := a[0]
	for i, v := range a[1:] {
		tmpl = strings.ReplaceAll(tmpl, fmt.Sprintf("{%d}", i), v)
	}
	return tmpl, nil
}

func fnClamp(a []string) (string, error) {
	if len(a) != 3 {
		return "", fmt.Errorf("clamp: expected (n, min, max)")
	}
	n, err := strconv.ParseFloat(a[0], 64)
	if err != nil {
		return "", err
	}
	lo, err := strconv.ParseFloat(a[1], 64)
	if err != nil {
		return "", err
	}
	hi, err := strconv.ParseFloat(a[2], 64)
	if err != nil {
		return "", err
	}
	if n < lo {
		n = lo
	}
	if n > hi {
		n = hi
	}
	return formatFloat(n), nil
}
