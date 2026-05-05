package runtime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// ---------- email.go ----------

func TestResolveEmailRecipient(t *testing.T) {
	tests := []struct {
		to     string
		params map[string]string
		want   string
	}{
		{"admin@example.com", nil, "admin@example.com"},
		{":email", map[string]string{"email": "user@test.com"}, "user@test.com"},
		{":missing", map[string]string{}, ":missing"},
	}
	for _, tc := range tests {
		got := resolveEmailRecipient(tc.to, tc.params)
		if got != tc.want {
			t.Errorf("resolveEmailRecipient(%q, %v) = %q, want %q", tc.to, tc.params, got, tc.want)
		}
	}
}

// ---------- logger.go ----------

func TestTruncateSQL(t *testing.T) {
	short := "SELECT id FROM users"
	if got := truncateSQL(short); got != short {
		t.Errorf("truncateSQL(short) = %q, want %q", got, short)
	}

	long := strings.Repeat("A", 250)
	got := truncateSQL(long)
	if len(got) != 203 { // 200 + "..."
		t.Errorf("truncateSQL(long) length = %d, want 203", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncateSQL(long) should end with ...")
	}
}

// ---------- fetch.go ----------

func TestParseJSONResponse_Array(t *testing.T) {
	body := []byte(`[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]`)
	rows, err := parseJSONResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["id"] != "1" || rows[0]["name"] != "Alice" {
		t.Errorf("unexpected first row: %v", rows[0])
	}
}

func TestParseJSONResponse_Object(t *testing.T) {
	body := []byte(`{"id":1,"name":"Alice"}`)
	rows, err := parseJSONResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["id"] != "1" || rows[0]["name"] != "Alice" {
		t.Errorf("unexpected row: %v", rows[0])
	}
}

func TestParseJSONResponse_Empty(t *testing.T) {
	rows, err := parseJSONResponse([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows != nil {
		t.Errorf("expected nil, got %v", rows)
	}
}

func TestParseJSONResponse_RawFallback(t *testing.T) {
	body := []byte("not json")
	rows, err := parseJSONResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 || rows[0]["_body"] != "not json" {
		t.Errorf("expected raw fallback, got %v", rows)
	}
}

func TestFlattenJSON_Nested(t *testing.T) {
	obj := map[string]interface{}{
		"user": map[string]interface{}{
			"name": "Alice",
			"age":  30,
		},
	}
	row := flattenJSON(obj, "")
	if row["user.name"] != "Alice" {
		t.Errorf("expected user.name = Alice, got %v", row)
	}
}

func TestFlattenJSON_Array(t *testing.T) {
	obj := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"name": "a"},
			map[string]interface{}{"name": "b"},
		},
	}
	row := flattenJSON(obj, "")
	if row["items._count"] != "2" {
		t.Errorf("expected items._count = 2, got %v", row)
	}
	if row["items.0.name"] != "a" {
		t.Errorf("expected items.0.name = a, got %v", row)
	}
}

func TestFlattenJSON_NilBoolDefault(t *testing.T) {
	obj := map[string]interface{}{
		"missing": nil,
		"active":  true,
		"name":    "Alice",
	}
	row := flattenJSON(obj, "")
	if row["missing"] != "" {
		t.Errorf("expected empty string for nil, got %q", row["missing"])
	}
	if row["active"] != "true" {
		t.Errorf("expected true for bool, got %q", row["active"])
	}
	if row["name"] != "Alice" {
		t.Errorf("expected Alice for string, got %q", row["name"])
	}
}

func TestFlattenJSON_ArrayLimit(t *testing.T) {
	items := make([]interface{}, 15)
	for i := 0; i < 15; i++ {
		items[i] = fmt.Sprintf("item-%d", i)
	}
	obj := map[string]interface{}{"items": items}
	row := flattenJSON(obj, "")
	if row["items._count"] != "15" {
		t.Errorf("expected items._count = 15, got %q", row["items._count"])
	}
	if _, ok := row["items.9"]; !ok {
		t.Errorf("expected items.9 to exist")
	}
	if _, ok := row["items.10"]; ok {
		t.Errorf("expected items.10 to be omitted")
	}
}

// ---------- ratelimit.go ----------

func TestWindowDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"second", time.Second},
		{"minute", time.Minute},
		{"hour", time.Hour},
		{"day", time.Minute},
		{"unknown", time.Minute},
	}
	for _, tc := range tests {
		got := windowDuration(tc.input)
		if got != tc.want {
			t.Errorf("windowDuration(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestRateLimiter_Check_AllowsFirstRequest(t *testing.T) {
	rl := NewRateLimiter([]parser.RateLimit{
		{PathPattern: "/api", Requests: 5, Window: "minute", Per: "ip"},
	})
	req := httptest.NewRequest("GET", "/api", nil)
	if !rl.Check(req, nil) {
		t.Error("first request should be allowed")
	}
}

func TestRateLimiter_Check_BlocksAfterLimit(t *testing.T) {
	rl := NewRateLimiter([]parser.RateLimit{
		{PathPattern: "/api", Requests: 2, Window: "minute", Per: "ip"},
	})
	req := httptest.NewRequest("GET", "/api", nil)
	for i := 0; i < 2; i++ {
		if !rl.Check(req, nil) {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if rl.Check(req, nil) {
		t.Error("third request should be blocked")
	}
}

func TestRateLimiter_Check_NoRules(t *testing.T) {
	rl := NewRateLimiter(nil)
	req := httptest.NewRequest("GET", "/api", nil)
	if !rl.Check(req, nil) {
		t.Error("should allow when no rules")
	}
}

func TestRateLimiter_Check_PerUser(t *testing.T) {
	rl := NewRateLimiter([]parser.RateLimit{
		{PathPattern: "/api", Requests: 1, Window: "minute", Per: "user"},
	})
	req := httptest.NewRequest("GET", "/api", nil)
	sess := &Session{UserID: "42"}
	if !rl.Check(req, sess) {
		t.Error("first request for user should be allowed")
	}
	if rl.Check(req, sess) {
		t.Error("second request for same user should be blocked")
	}
}

// ---------- render.go ----------

func TestRenderMarkdown(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"**bold**", "<strong>bold</strong>"},
		{"*italic*", "<em>italic</em>"},
		{"~strike~", "<del>strike</del>"},
		{"`code`", "<code>code</code>"},
		{"```go\nfmt.Println()\n```", "<pre><code>fmt.Println()<br></code></pre>"},
		{"[link](https://example.com)", `<a href="https://example.com" target="_blank" rel="noopener">link</a>`},
		{"[bad](javascript:alert(1))", "bad"},
		{"@alice", `<span style="color:#1d9bd1;background:rgba(29,155,209,0.1);padding:1px 4px;border-radius:3px;font-weight:600">@alice</span>`},
		{"line1\nline2", "line1<br>line2"},
	}
	for _, tc := range tests {
		got := renderMarkdown(tc.input)
		if !strings.Contains(got, tc.want) {
			t.Errorf("renderMarkdown(%q) = %q, want to contain %q", tc.input, got, tc.want)
		}
	}
}

func TestLinkify(t *testing.T) {
	got := linkify("visit https://example.com here")
	want := `<a href="https://example.com" target="_blank" rel="noopener">https://example.com</a>`
	if !strings.Contains(got, want) {
		t.Errorf("linkify = %q, want to contain %q", got, want)
	}
}

func TestParseFilterChain(t *testing.T) {
	tests := []struct {
		input string
		want  []parsedFilter
	}{
		{"upcase", []parsedFilter{{Name: "upcase"}}},
		{"upcase | downcase", []parsedFilter{{Name: "upcase"}, {Name: "downcase"}}},
		{"truncate: 10", []parsedFilter{{Name: "truncate", Args: []string{"10"}}}},
		{"default: \"fallback\", 20", []parsedFilter{{Name: "default", Args: []string{"fallback", "20"}}}},
		{"", nil},
		{"  ", nil},
	}
	for _, tc := range tests {
		got := parseFilterChain(tc.input)
		if len(got) != len(tc.want) {
			t.Fatalf("parseFilterChain(%q) len = %d, want %d", tc.input, len(got), len(tc.want))
		}
		for i := range got {
			if got[i].Name != tc.want[i].Name {
				t.Errorf("filter[%d].Name = %q, want %q", i, got[i].Name, tc.want[i].Name)
			}
			if len(got[i].Args) != len(tc.want[i].Args) {
				t.Errorf("filter[%d].Args len = %d, want %d", i, len(got[i].Args), len(tc.want[i].Args))
				continue
			}
			for j := range got[i].Args {
				if got[i].Args[j] != tc.want[i].Args[j] {
					t.Errorf("filter[%d].Args[%d] = %q, want %q", i, j, got[i].Args[j], tc.want[i].Args[j])
				}
			}
		}
	}
}

func TestGetPaginateField(t *testing.T) {
	info := PaginateInfo{Page: 2, PerPage: 10, Total: 25, HasPrev: true, HasNext: true}
	tests := []struct {
		field string
		want  string
	}{
		{"page", "2"},
		{"per_page", "10"},
		{"total", "25"},
		{"has_prev", "true"},
		{"has_next", "true"},
		{"prev", "1"},
		{"next", "3"},
		{"total_pages", "3"},
		{"unknown", ""},
	}
	for _, tc := range tests {
		got := getPaginateField(info, tc.field)
		if got != tc.want {
			t.Errorf("getPaginateField(%q) = %q, want %q", tc.field, got, tc.want)
		}
	}
}

func TestGetPaginateField_NoPrevNext(t *testing.T) {
	info := PaginateInfo{Page: 1, PerPage: 10, Total: 5, HasPrev: false, HasNext: false}
	if got := getPaginateField(info, "prev"); got != "1" {
		t.Errorf("prev = %q, want 1", got)
	}
	if got := getPaginateField(info, "next"); got != "1" {
		t.Errorf("next = %q, want 1", got)
	}
	if got := getPaginateField(info, "has_prev"); got != "false" {
		t.Errorf("has_prev = %q, want false", got)
	}
	if got := getPaginateField(info, "has_next"); got != "false" {
		t.Errorf("has_next = %q, want false", got)
	}
}

func TestGetPaginateField_ZeroPerPage(t *testing.T) {
	info := PaginateInfo{Page: 1, PerPage: 0, Total: 5}
	if got := getPaginateField(info, "total_pages"); got != "0" {
		t.Errorf("total_pages = %q, want 0", got)
	}
}

// ---------- server.go ----------

func TestHasUserPage(t *testing.T) {
	app := &parser.App{
		Pages: []parser.Page{
			{Path: "/"},
			{Path: "/about"},
		},
	}
	if !hasUserPage(app, "/") {
		t.Error("expected true for /")
	}
	if !hasUserPage(app, "/about") {
		t.Error("expected true for /about")
	}
	if hasUserPage(app, "/missing") {
		t.Error("expected false for /missing")
	}
	if hasUserPage(nil, "/") {
		t.Error("expected false for nil app")
	}
}

func TestFindModel(t *testing.T) {
	models := []parser.Model{
		{Name: "user"},
		{Name: "post"},
	}
	if got := findModel(models, "post"); got == nil || got.Name != "post" {
		t.Error("expected to find post model")
	}
	if got := findModel(models, "comment"); got != nil {
		t.Error("expected nil for missing model")
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"/users/:id", "/users/5", true},
		{"/users/:id", "/users/5/edit", false},
		{"/users", "/users", true},
		{"/users", "/posts", false},
		{"/", "/", true},
	}
	for _, tc := range tests {
		got := matchPath(tc.pattern, tc.path)
		if got != tc.want {
			t.Errorf("matchPath(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestMatchPathParams(t *testing.T) {
	got := matchPathParams("/users/:id", "/users/42")
	if got["id"] != "42" {
		t.Errorf("id = %q, want 42", got["id"])
	}
}

func TestExpandCustomFields(t *testing.T) {
	rows := []database.Row{
		{"name": "Deal A", "custom": `{"revenue":"100"}`},
		{"name": "Deal B", "custom": ``},
		{"name": "Deal C", "custom": `not json`},
	}
	got := expandCustomFields(rows)
	if got[0]["custom.revenue"] != "100" {
		t.Errorf("custom.revenue = %q, want 100", got[0]["custom.revenue"])
	}
	if _, ok := got[1]["custom.revenue"]; ok {
		t.Error("empty custom should not produce fields")
	}
	if _, ok := got[2]["custom.revenue"]; ok {
		t.Error("invalid json should not produce fields")
	}
}

func TestResolveTenantPlaceholder(t *testing.T) {
	sess := &Session{Data: map[string]string{"org_id": "99"}}
	params := map[string]string{"id": "42"}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Tenant", "acme")

	tests := []struct {
		tmpl string
		want string
	}{
		{"{user.org_id}", "99"},
		{"{:id}", "42"},
		{"{header.X-Tenant}", "acme"},
		{"{unknown}", ""},
		{"static", "static"},
	}
	for _, tc := range tests {
		got := resolveTenantPlaceholder(tc.tmpl, sess, params, req)
		if got != tc.want {
			t.Errorf("resolveTenantPlaceholder(%q) = %q, want %q", tc.tmpl, got, tc.want)
		}
	}
}

func TestResolveTenantPlaceholder_NilInputs(t *testing.T) {
	got := resolveTenantPlaceholder("{user.x}", nil, nil, nil)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
	got = resolveTenantPlaceholder("{:x}", nil, nil, nil)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// ---------- llm.go ----------

func TestMergeConsecutiveRoles(t *testing.T) {
	msgs := []anthropicMessage{
		{Role: "user", Content: "hello"},
		{Role: "user", Content: "world"},
		{Role: "assistant", Content: "hi"},
		{Role: "assistant", Content: "there"},
	}
	got := mergeConsecutiveRoles(msgs)
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	if got[0].Content != "hello\nworld" {
		t.Errorf("merged user content = %q, want %q", got[0].Content, "hello\nworld")
	}
	if got[1].Content != "hi\nthere" {
		t.Errorf("merged assistant content = %q, want %q", got[1].Content, "hi\nthere")
	}
}

func TestMergeConsecutiveRoles_Empty(t *testing.T) {
	got := mergeConsecutiveRoles(nil)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// ---------- webhook.go ----------

func TestFlattenPayload(t *testing.T) {
	payload := map[string]interface{}{
		"id":     "evt_123",
		"amount": 99.99,
		"nested": map[string]interface{}{
			"foo": "bar",
		},
		"flag":   true,
		"count":  5,
		"active": false,
	}
	got := flattenPayload(payload, "stripe")
	if got["stripe_id"] != "evt_123" {
		t.Errorf("stripe_id = %q, want evt_123", got["stripe_id"])
	}
	if got["stripe_amount"] != "99.99" {
		t.Errorf("stripe_amount = %q, want 99.99", got["stripe_amount"])
	}
	if got["stripe_nested_foo"] != "bar" {
		t.Errorf("stripe_nested_foo = %q, want bar", got["stripe_nested_foo"])
	}
	if got["stripe_flag"] != "true" {
		t.Errorf("stripe_flag = %q, want true", got["stripe_flag"])
	}
	if got["stripe_count"] != "5" {
		t.Errorf("stripe_count = %q, want 5", got["stripe_count"])
	}
	if got["stripe_active"] != "false" {
		t.Errorf("stripe_active = %q, want false", got["stripe_active"])
	}
}

// ---------- i18n.go ----------

func TestTranslate(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"hello": "Hello"},
		"pt": {"hello": "Olá"},
	}, "en", true)

	// Default language
	req := httptest.NewRequest("GET", "/", nil)
	if got := i18n.Translate("hello", req); got != "Hello" {
		t.Errorf("translate(en) = %q, want Hello", got)
	}

	// Query param language
	reqPT := httptest.NewRequest("GET", "/?lang=pt", nil)
	if got := i18n.Translate("hello", reqPT); got != "Olá" {
		t.Errorf("translate(pt) = %q, want Olá", got)
	}

	// Missing key returns key
	if got := i18n.Translate("missing", req); got != "missing" {
		t.Errorf("translate(missing) = %q, want missing", got)
	}
}

func TestTranslate_NoDetect(t *testing.T) {
	i18n := NewI18n(map[string]map[string]string{
		"en": {"hello": "Hello"},
	}, "en", false)

	req := httptest.NewRequest("GET", "/?lang=pt", nil)
	if got := i18n.Translate("hello", req); got != "Hello" {
		t.Errorf("translate = %q, want Hello (detect disabled)", got)
	}
}

// ---------- auth.go ----------

func TestResolveSessionField(t *testing.T) {
	sess := &Session{
		UserID:   "42",
		Identity: "a@b.com",
		Role:     "admin",
		Data:     map[string]string{"plan": "pro"},
	}
	tests := []struct {
		field string
		want  string
	}{
		{"current_user.id", "42"},
		{"current_user.identity", "a@b.com"},
		{"current_user.email", "a@b.com"},
		{"current_user.role", "admin"},
		{"current_user.plan", "pro"},
		{"id", "42"},
		{"unknown", ""},
	}
	for _, tc := range tests {
		got := resolveSessionField(tc.field, sess)
		if got != tc.want {
			t.Errorf("resolveSessionField(%q) = %q, want %q", tc.field, got, tc.want)
		}
	}
	if got := resolveSessionField("id", nil); got != "" {
		t.Errorf("resolveSessionField with nil session = %q, want empty", got)
	}
}

func TestParseInList_Auth(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a, b, c", []string{"a", "b", "c"}},
		{"'a', 'b'", []string{"a", "b"}},
		{"[a, b], c", []string{"[a, b]", "c"}},
		{"", nil},
		{"a", []string{"a"}},
	}
	for _, tc := range tests {
		got := parseInList(tc.input)
		if len(got) != len(tc.want) {
			t.Fatalf("parseInList(%q) len = %d, want %d", tc.input, len(got), len(tc.want))
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parseInList(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

// ---------- tenant.go ----------

func TestStripSQLNoise(t *testing.T) {
	sql := "SELECT id FROM users WHERE name = 'Alice' /* comment */ -- line"
	got := stripSQLNoise(sql)
	if strings.Contains(got, "comment") {
		t.Error("stripSQLNoise should remove block comments")
	}
	if strings.Contains(got, "line") {
		t.Error("stripSQLNoise should remove line comments")
	}
	if !strings.Contains(got, "SELECT") {
		t.Error("stripSQLNoise should preserve SQL keywords")
	}
	if strings.Contains(got, "Alice") {
		t.Error("stripSQLNoise should remove string literals")
	}
}

// ---------- email.go ----------

func TestLoadEmailConfig(t *testing.T) {
	os.Setenv("KILNX_SMTP_HOST", "smtp.example.com")
	os.Setenv("KILNX_SMTP_PORT", "587")
	os.Setenv("KILNX_SMTP_USER", "user")
	os.Setenv("KILNX_SMTP_PASS", "secret")
	os.Setenv("KILNX_SMTP_FROM", "from@example.com")
	defer func() {
		os.Unsetenv("KILNX_SMTP_HOST")
		os.Unsetenv("KILNX_SMTP_PORT")
		os.Unsetenv("KILNX_SMTP_USER")
		os.Unsetenv("KILNX_SMTP_PASS")
		os.Unsetenv("KILNX_SMTP_FROM")
	}()

	cfg := loadEmailConfig()
	if cfg.Host != "smtp.example.com" {
		t.Errorf("Host = %q, want smtp.example.com", cfg.Host)
	}
	if cfg.Port != "587" {
		t.Errorf("Port = %q, want 587", cfg.Port)
	}
	if cfg.User != "user" {
		t.Errorf("User = %q, want user", cfg.User)
	}
	if cfg.Password != "secret" {
		t.Errorf("Password = %q, want secret", cfg.Password)
	}
	if cfg.From != "from@example.com" {
		t.Errorf("From = %q, want from@example.com", cfg.From)
	}
}

func TestLoadEmailConfig_Defaults(t *testing.T) {
	os.Unsetenv("KILNX_SMTP_HOST")
	os.Unsetenv("KILNX_SMTP_PORT")
	os.Unsetenv("KILNX_SMTP_FROM")

	cfg := loadEmailConfig()
	if cfg.Host != "localhost" {
		t.Errorf("Host default = %q, want localhost", cfg.Host)
	}
	if cfg.Port != "25" {
		t.Errorf("Port default = %q, want 25", cfg.Port)
	}
	if cfg.From != "noreply@localhost" {
		t.Errorf("From default = %q, want noreply@localhost", cfg.From)
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_KILNX_VAR", "value")
	defer os.Unsetenv("TEST_KILNX_VAR")

	if got := getEnv("TEST_KILNX_VAR", "fallback"); got != "value" {
		t.Errorf("getEnv(existing) = %q, want value", got)
	}
	if got := getEnv("TEST_KILNX_MISSING", "fallback"); got != "fallback" {
		t.Errorf("getEnv(missing) = %q, want fallback", got)
	}
}

// ---------- scheduler.go ----------

func TestNextCronOccurrence_Invalid(t *testing.T) {
	got := nextCronOccurrence("not a cron")
	if !got.IsZero() {
		t.Errorf("expected zero time for invalid expr, got %v", got)
	}
}

func TestNextCronOccurrence_EveryDay(t *testing.T) {
	got := nextCronOccurrence("every day at 23:59")
	now := time.Now()
	if got.IsZero() {
		t.Fatal("expected non-zero time")
	}
	if got.Hour() != 23 || got.Minute() != 59 {
		t.Errorf("expected 23:59, got %02d:%02d", got.Hour(), got.Minute())
	}
	if got.Before(now) {
		t.Error("expected future time")
	}
}

func TestNextCronOccurrence_Weekday(t *testing.T) {
	got := nextCronOccurrence("every monday at 9:00")
	if got.IsZero() {
		t.Fatal("expected non-zero time")
	}
	if got.Weekday() != time.Monday {
		t.Errorf("expected Monday, got %v", got.Weekday())
	}
	if got.Hour() != 9 || got.Minute() != 0 {
		t.Errorf("expected 09:00, got %02d:%02d", got.Hour(), got.Minute())
	}
}

func TestNextCronOccurrence_TodayPassed(t *testing.T) {
	// Use a time far in the past so "today" has definitely passed
	got := nextCronOccurrence("every monday at 0:00")
	if got.IsZero() {
		t.Fatal("expected non-zero time")
	}
	if got.Weekday() != time.Monday {
		t.Errorf("expected Monday, got %v", got.Weekday())
	}
	// If today is Monday, the time should be next Monday (7 days later)
	if time.Now().Weekday() == time.Monday {
		expected := time.Now().AddDate(0, 0, 7)
		if got.Day() != expected.Day() {
			t.Errorf("expected next Monday (day %d), got day %d", expected.Day(), got.Day())
		}
	}
}

func TestNextCronOccurrence_NoMinutes(t *testing.T) {
	got := nextCronOccurrence("every day at 5")
	if got.IsZero() {
		t.Fatal("expected non-zero time")
	}
	if got.Hour() != 5 || got.Minute() != 0 {
		t.Errorf("expected 05:00, got %02d:%02d", got.Hour(), got.Minute())
	}
}

// ---------- stream.go ----------

func TestRenderSSERows_Empty(t *testing.T) {
	got := renderSSERows(nil)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestRenderSSERows_SingleValue(t *testing.T) {
	rows := []database.Row{{"name": "Alice"}}
	got := renderSSERows(rows)
	if got != "Alice" {
		t.Errorf("expected Alice, got %q", got)
	}
}

func TestRenderSSERows_SingleValueEscaping(t *testing.T) {
	rows := []database.Row{{"html": "<script>"}}
	got := renderSSERows(rows)
	if got != "&lt;script&gt;" {
		t.Errorf("expected escaped HTML, got %q", got)
	}
}

func TestRenderSSERows_Multiple(t *testing.T) {
	rows := []database.Row{
		{"a": "1", "b": "2"},
		{"a": "3", "b": "4"},
	}
	got := renderSSERows(rows)
	if !strings.Contains(got, `<div class="kilnx-sse-item">`) {
		t.Error("expected div wrapper")
	}
	if !strings.Contains(got, "1") || !strings.Contains(got, "2") {
		t.Error("expected values")
	}
}

// ---------- server.go ----------

func TestExpandColumnModeFields(t *testing.T) {
	manifest := &parser.CustomFieldManifest{
		Fields: []parser.CustomFieldDef{
			{Name: "revenue", Mode: parser.CustomFieldModeColumn},
			{Name: "stage", Mode: parser.CustomFieldModeColumn},
			{Name: "notes", Mode: parser.CustomFieldModeJSON},
		},
	}
	rows := []database.Row{
		{"name": "Deal A", "revenue": "100", "stage": "open"},
		{"name": "Deal B", "revenue": "200"},
	}
	got := expandColumnModeFields(rows, manifest)
	if got[0]["custom.revenue"] != "100" {
		t.Errorf("custom.revenue = %q, want 100", got[0]["custom.revenue"])
	}
	if got[0]["custom.stage"] != "open" {
		t.Errorf("custom.stage = %q, want open", got[0]["custom.stage"])
	}
	if _, ok := got[0]["custom.notes"]; ok {
		t.Error("json mode field should not be aliased")
	}
	if _, ok := got[1]["custom.stage"]; ok {
		t.Error("missing column should not be aliased")
	}
}

// ---------- testing.go ----------

func TestEvaluateExpect_Contains(t *testing.T) {
	step := parser.TestStep{Target: "contains", Value: "hello"}
	if !evaluateExpect(step, "hello world", 200, "", nil) {
		t.Error("expected true for contains")
	}
	if evaluateExpect(step, "goodbye", 200, "", nil) {
		t.Error("expected false for missing text")
	}
}

func TestEvaluateExpect_Redirect(t *testing.T) {
	step := parser.TestStep{Target: "redirect to /home", Value: ""}
	if !evaluateExpect(step, "", 302, "/home", nil) {
		t.Error("expected true for correct redirect")
	}
	if evaluateExpect(step, "", 302, "/other", nil) {
		t.Error("expected false for wrong redirect")
	}
}

func TestEvaluateExpect_RedirectValue(t *testing.T) {
	step := parser.TestStep{Target: "redirect", Value: "/dashboard"}
	if !evaluateExpect(step, "", 302, "/dashboard", nil) {
		t.Error("expected true for redirect value")
	}
}

func TestEvaluateExpect_Status(t *testing.T) {
	step := parser.TestStep{Target: "status 200", Value: ""}
	if !evaluateExpect(step, "", 200, "", nil) {
		t.Error("expected true for status 200")
	}
	if evaluateExpect(step, "", 404, "", nil) {
		t.Error("expected false for status 404")
	}
}

func TestEvaluateExpect_Unknown(t *testing.T) {
	step := parser.TestStep{Target: "something else", Value: ""}
	if !evaluateExpect(step, "", 200, "", nil) {
		t.Error("expected true for unknown target (default pass)")
	}
}

func TestExtractFormAction(t *testing.T) {
	html := `<form action="/submit" method="POST">`
	if got := extractFormAction(html); got != "/submit" {
		t.Errorf("extractFormAction = %q, want /submit", got)
	}
	html2 := `<form method='POST' action='/other'>`
	if got := extractFormAction(html2); got != "/other" {
		t.Errorf("extractFormAction = %q, want /other", got)
	}
	if got := extractFormAction("<div>no form</div>"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestExtractCSRFFromHTML(t *testing.T) {
	html := `<input type="hidden" name="_csrf" value="abc123">`
	if got := extractCSRFFromHTML(html); got != "abc123" {
		t.Errorf("extractCSRFFromHTML = %q, want abc123", got)
	}
	if got := extractCSRFFromHTML("no csrf"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// ---------- unfurl.go ----------

func TestRenderUnfurl_Nil(t *testing.T) {
	if got := renderUnfurl(nil); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestRenderUnfurl_Full(t *testing.T) {
	og := &ogData{
		Title:       "Test Title",
		Description: "Test description",
		Image:       "https://example.com/img.png",
		SiteName:    "Example",
		URL:         "https://example.com",
	}
	got := renderUnfurl(og)
	if !strings.Contains(got, "Test Title") {
		t.Error("expected title in output")
	}
	if !strings.Contains(got, "Test description") {
		t.Error("expected description in output")
	}
	if !strings.Contains(got, "img.png") {
		t.Error("expected image in output")
	}
	if !strings.Contains(got, "Example") {
		t.Error("expected site name in output")
	}
	if !strings.Contains(got, `href="https://example.com"`) {
		t.Error("expected URL in output")
	}
}

func TestRenderUnfurl_LongDescription(t *testing.T) {
	og := &ogData{
		Title:       "T",
		Description: strings.Repeat("a", 250),
		URL:         "https://x.com",
	}
	got := renderUnfurl(og)
	if !strings.Contains(got, "...") {
		t.Error("expected truncated description with ...")
	}
}

// ---------- webhook.go ----------

func TestVerifyStripeSignature_Valid(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"id":"evt_123"}`)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	signedPayload := timestamp + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	sig := hex.EncodeToString(mac.Sum(nil))

	header := fmt.Sprintf("t=%s,v1=%s", timestamp, sig)
	if !verifyStripeSignature(payload, header, secret) {
		t.Error("expected valid signature to pass")
	}
}

func TestVerifyStripeSignature_InvalidSecret(t *testing.T) {
	payload := []byte(`{"id":"evt_123"}`)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	header := fmt.Sprintf("t=%s,v1=bad_sig", timestamp)
	if verifyStripeSignature(payload, header, "wrong_secret") {
		t.Error("expected invalid signature to fail")
	}
}

func TestVerifyStripeSignature_MalformedHeader(t *testing.T) {
	if verifyStripeSignature([]byte("{}"), "malformed", "secret") {
		t.Error("expected malformed header to fail")
	}
}

func TestVerifyStripeSignature_OldTimestamp(t *testing.T) {
	oldTs := fmt.Sprintf("%d", time.Now().Unix()-400)
	header := fmt.Sprintf("t=%s,v1=abc", oldTs)
	if verifyStripeSignature([]byte("{}"), header, "secret") {
		t.Error("expected old timestamp to fail")
	}
}

func TestVerifyStripeSignature_InvalidTimestamp(t *testing.T) {
	header := "t=notanumber,v1=abc"
	if verifyStripeSignature([]byte("{}"), header, "secret") {
		t.Error("expected invalid timestamp to fail")
	}
}

func TestVerifyStripeSignature_ExtraParts(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"id":"evt_123"}`)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	signedPayload := timestamp + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	sig := hex.EncodeToString(mac.Sum(nil))

	header := fmt.Sprintf("foo=bar,t=%s,v1=%s", timestamp, sig)
	if !verifyStripeSignature(payload, header, secret) {
		t.Error("expected valid signature with extra parts to pass")
	}
}

// ---------- websocket.go ----------

func TestWriteWSFrame_Small(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = writeWSFrame(client, []byte("hello"))
		client.Close()
	}()

	buf := make([]byte, 7)
	n, err := io.ReadFull(server, buf)
	if err != nil && err != io.EOF && err != io.ErrClosedPipe {
		t.Fatalf("read error: %v", err)
	}
	if n < 2 {
		t.Fatalf("expected at least 2 bytes, got %d", n)
	}
	if buf[0] != 0x81 {
		t.Errorf("opcode = 0x%02x, want 0x81", buf[0])
	}
	if buf[1] != 5 {
		t.Errorf("length = %d, want 5", buf[1])
	}
	if string(buf[2:7]) != "hello" {
		t.Errorf("payload = %q, want hello", string(buf[2:7]))
	}
}

// ---------- server.go ----------

func TestBuildCustomIterRows(t *testing.T) {
	manifest := &parser.CustomFieldManifest{
		ModelName: "deal",
		Fields: []parser.CustomFieldDef{
			{Name: "revenue", Label: "Revenue", Kind: parser.CustomFieldKindNumber, Mode: parser.CustomFieldModeJSON},
			{Name: "stage", Label: "Stage", Kind: parser.CustomFieldKindText, Mode: parser.CustomFieldModeColumn},
		},
	}
	row := database.Row{
		"custom": `{"revenue":5000}`,
		"stage":  "won",
		"name":   "Deal A",
	}
	got := buildCustomIterRows(row, manifest)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	if got[0]["name"] != "revenue" || got[0]["value"] != "5000" || got[0]["label"] != "Revenue" {
		t.Errorf("first row = %v", got[0])
	}
	if got[1]["name"] != "stage" || got[1]["value"] != "won" || got[1]["label"] != "Stage" {
		t.Errorf("second row = %v", got[1])
	}
}

func TestBuildCustomIterRows_EmptyManifest(t *testing.T) {
	manifest := &parser.CustomFieldManifest{ModelName: "x"}
	row := database.Row{"custom": `{"a":"1"}`}
	got := buildCustomIterRows(row, manifest)
	if len(got) != 0 {
		t.Errorf("expected 0 rows, got %d", len(got))
	}
}

func TestBuildCustomIterRows_InvalidJSON(t *testing.T) {
	manifest := &parser.CustomFieldManifest{
		ModelName: "deal",
		Fields:    []parser.CustomFieldDef{{Name: "revenue", Kind: parser.CustomFieldKindNumber, Label: "Revenue"}},
	}
	row := database.Row{"custom": `not json`}
	got := buildCustomIterRows(row, manifest)
	if len(got) != 1 || got[0]["value"] != "" {
		t.Errorf("expected empty value for invalid JSON, got %v", got)
	}
}

func TestBuildCustomIterRows_EmptyLabel(t *testing.T) {
	manifest := &parser.CustomFieldManifest{
		ModelName: "deal",
		Fields:    []parser.CustomFieldDef{{Name: "revenue", Kind: parser.CustomFieldKindNumber}},
	}
	row := database.Row{"custom": `{"revenue":100}`}
	got := buildCustomIterRows(row, manifest)
	if len(got) != 1 || got[0]["label"] != "revenue" {
		t.Errorf("expected label fallback to name, got %v", got)
	}
}

// ---------- email.go ----------

func TestLoadEmailTemplate(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll("templates", 0755)
	os.WriteFile("templates/welcome.html", []byte("Hello {name}!"), 0644)

	got := LoadEmailTemplate("welcome", map[string]string{"name": "Alice"})
	if got != "Hello Alice!" {
		t.Errorf("got %q, want Hello Alice!", got)
	}

	// Escaping
	os.WriteFile("templates/alert.html", []byte("{msg}"), 0644)
	got2 := LoadEmailTemplate("alert", map[string]string{"msg": "<script>"})
	if got2 != "&lt;script&gt;" {
		t.Errorf("got %q, want escaped HTML", got2)
	}
}

func TestLoadEmailTemplate_Missing(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	got := LoadEmailTemplate("nonexistent", nil)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// ---------- ratelimit.go ----------

func TestMatchRateLimitPath(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"/api/*", "/api/v1/users", true},
		{"/api/*", "/api", true},
		{"/api/*", "/other", false},
		{"/api", "/api", true},
		{"/api", "/api/v1", false},
	}
	for _, tc := range tests {
		got := matchRateLimitPath(tc.pattern, tc.path)
		if got != tc.want {
			t.Errorf("matchRateLimitPath(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter([]parser.RateLimit{
		{PathPattern: "/api", Requests: 2, Window: "minute", Per: "ip"},
	})
	req := httptest.NewRequest("GET", "/api", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	for i := 0; i < 3; i++ {
		rl.Check(req, nil)
	}
	if len(rl.entries) == 0 {
		t.Fatal("expected entries")
	}
	// Make entries expired
	for k, e := range rl.entries {
		e.expiresAt = time.Now().Add(-time.Minute)
		rl.entries[k] = e
	}
	rl.cleanup()
	if len(rl.entries) != 0 {
		t.Errorf("expected 0 entries after cleanup, got %d", len(rl.entries))
	}
}

func TestRateLimiter_Cleanup_Eviction(t *testing.T) {
	rl := NewRateLimiter(nil)
	// Fill beyond max
	for i := 0; i < maxRateLimitEntries+20; i++ {
		rl.entries[fmt.Sprintf("key%d", i)] = &rateLimitEntry{
			count:     1,
			expiresAt: time.Now().Add(time.Hour),
		}
	}
	rl.cleanup()
	if len(rl.entries) >= maxRateLimitEntries+20 {
		t.Errorf("expected eviction, got %d entries", len(rl.entries))
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	if got := clientIP(req); got != "192.168.1.1" {
		t.Errorf("clientIP = %q, want 192.168.1.1", got)
	}

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "127.0.0.1:1234"
	req2.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
	if got := clientIP(req2); got != "10.0.0.1" {
		t.Errorf("clientIP XFF = %q, want 10.0.0.1", got)
	}

	req3 := httptest.NewRequest("GET", "/", nil)
	req3.RemoteAddr = "127.0.0.1:1234"
	req3.Header.Set("X-Real-IP", "10.0.0.2")
	if got := clientIP(req3); got != "10.0.0.2" {
		t.Errorf("clientIP XRI = %q, want 10.0.0.2", got)
	}
}

// ---------- logger.go ----------

func TestNewLogger_NilConfig(t *testing.T) {
	l := NewLogger(nil)
	if l.config.Level != "info" {
		t.Errorf("default level = %q, want info", l.config.Level)
	}
	if l.config.SlowQueryMs != 100 {
		t.Errorf("default slow query = %d, want 100", l.config.SlowQueryMs)
	}
	if l.config.LogRequests {
		t.Error("default LogRequests should be false")
	}
	if !l.config.LogErrors {
		t.Error("default LogErrors should be true")
	}
}

func TestNewLogger_WithConfig(t *testing.T) {
	cfg := &parser.LogConfig{Level: "debug", SlowQueryMs: 50, LogRequests: true, LogErrors: false}
	l := NewLogger(cfg)
	if l.config.Level != "debug" {
		t.Errorf("level = %q, want debug", l.config.Level)
	}
}

func TestLogger_LogError_Nil(t *testing.T) {
	var l *Logger
	l.LogError("test", fmt.Errorf("err")) // should not panic
}

func TestLogger_LogError_Disabled(t *testing.T) {
	l := NewLogger(&parser.LogConfig{LogErrors: false})
	l.LogError("test", fmt.Errorf("err")) // should not print
}

func TestLoggingResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	lw := &loggingResponseWriter{ResponseWriter: rec, statusCode: 200}
	lw.WriteHeader(404)
	if lw.statusCode != 404 {
		t.Errorf("statusCode = %d, want 404", lw.statusCode)
	}
	if rec.Code != 404 {
		t.Errorf("recorder code = %d, want 404", rec.Code)
	}

	// Hijack on non-hijackable writer
	_, _, err := lw.Hijack()
	if err == nil {
		t.Error("expected error for non-hijackable writer")
	}

	// Flush on non-flusher writer
	lw.Flush() // should not panic
}

// ---------- layout.go ----------

func TestRenderDefaultLayout(t *testing.T) {
	got := renderDefaultLayout("Home", "<nav></nav>", "<h1>Hello</h1>")
	if !strings.Contains(got, "<title>Home</title>") {
		t.Error("expected title")
	}
	if !strings.Contains(got, "<h1>Hello</h1>") {
		t.Error("expected content")
	}
	if !strings.Contains(got, "htmx.min.js") {
		t.Error("expected htmx script")
	}
}

func TestRenderDefaultLayout_Escaping(t *testing.T) {
	got := renderDefaultLayout("<script>", "", "")
	if strings.Contains(got, "<script>") {
		t.Error("title should be escaped")
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Error("expected escaped title")
	}
}

func TestRenderWithLayout(t *testing.T) {
	layout := parser.Layout{
		HTMLContent: `<html><head><title>{page.title}</title>{kilnx.js}</head><body>{nav}<main>{page.content}</main></body></html>`,
	}
	got := renderWithLayout(layout, "About", "<nav>Nav</nav>", "<p>Content</p>", nil)
	if !strings.Contains(got, "<title>About</title>") {
		t.Error("expected title")
	}
	if !strings.Contains(got, "<p>Content</p>") {
		t.Error("expected content")
	}
	if !strings.Contains(got, "<nav>Nav</nav>") {
		t.Error("expected nav")
	}
	if !strings.Contains(got, "htmx.min.js") {
		t.Error("expected htmx script")
	}
}

func TestRenderWithLayout_WithQueries(t *testing.T) {
	layout := parser.Layout{
		HTMLContent: `<html><body>{page.content}</body></html>`,
	}
	ctx := &renderContext{
		queries: map[string][]database.Row{
			"nav": {{"name": "Home"}},
		},
	}
	got := renderWithLayout(layout, "T", "", "<h1>X</h1>", ctx)
	if !strings.Contains(got, "<h1>X</h1>") {
		t.Error("expected content")
	}
}

// ---------- server.go: resolveManifest ----------

func TestResolveManifest_NoFields(t *testing.T) {
	s := &Server{}
	model := &parser.Model{Name: "user"}
	app := &parser.App{}
	if got := s.resolveManifest(model, app, nil, nil, nil); got != nil {
		t.Error("expected nil when no custom fields")
	}
}

func TestResolveManifest_StaticPath(t *testing.T) {
	manifest := &parser.CustomFieldManifest{ModelName: "deal"}
	s := &Server{}
	model := &parser.Model{Name: "deal", CustomFieldsFile: "fields.kilnx"}
	app := &parser.App{
		CustomManifests: map[string]*parser.CustomFieldManifest{
			"deal": manifest,
		},
	}
	if got := s.resolveManifest(model, app, nil, nil, nil); got != manifest {
		t.Error("expected static manifest")
	}
}

func TestResolveManifest_DynamicPath(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.WriteFile("tenant_acme.kilnx", []byte(`field revenue: number`), 0644)

	s := &Server{}
	model := &parser.Model{Name: "deal", CustomFieldsFile: "tenant_{:org}.kilnx"}
	app := &parser.App{}
	got := s.resolveManifest(model, app, nil, map[string]string{"org": "acme"}, nil)
	if got == nil {
		t.Fatal("expected manifest")
	}
	if len(got.Fields) != 1 || got.Fields[0].Name != "revenue" {
		t.Errorf("unexpected fields: %v", got.Fields)
	}
}

func TestResolveManifest_DynamicPath_Unresolved(t *testing.T) {
	manifest := &parser.CustomFieldManifest{ModelName: "deal"}
	s := &Server{}
	model := &parser.Model{
		Name:                 "deal",
		CustomFieldsFile:     "tenant_{:org}.kilnx",
		CustomFieldsFallback: "static",
	}
	app := &parser.App{
		CustomManifests: map[string]*parser.CustomFieldManifest{
			"deal": manifest,
		},
	}
	// No path params, placeholder unresolved → fallback to static
	got := s.resolveManifest(model, app, nil, nil, nil)
	if got != manifest {
		t.Error("expected fallback manifest")
	}
}

func TestResolveManifest_DynamicPath_MissingFile(t *testing.T) {
	manifest := &parser.CustomFieldManifest{ModelName: "deal"}
	s := &Server{}
	model := &parser.Model{
		Name:                 "deal",
		CustomFieldsFile:     "tenant_{:org}.kilnx",
		CustomFieldsFallback: "static",
	}
	app := &parser.App{
		CustomManifests: map[string]*parser.CustomFieldManifest{
			"deal": manifest,
		},
	}
	got := s.resolveManifest(model, app, nil, map[string]string{"org": "missing"}, nil)
	if got != manifest {
		t.Error("expected fallback manifest for missing file")
	}
}

func TestResolveManifest_DynamicPath_Cached(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.WriteFile("cached.kilnx", []byte(`field name: text`), 0644)

	s := &Server{}
	model := &parser.Model{Name: "deal", CustomFieldsFile: "cached.kilnx"}
	app := &parser.App{}
	// Primeira chamada
	got1 := s.resolveManifest(model, app, nil, nil, nil)
	// Segunda chamada deve usar cache
	got2 := s.resolveManifest(model, app, nil, nil, nil)
	if got1 != got2 {
		t.Error("expected cached manifest")
	}
}

// ---------- logger.go ----------

func TestLogger_LogSecurity(t *testing.T) {
	// Redirect stderr temporarily
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	l := NewLogger(nil)
	l.LogSecurity("test event", fmt.Errorf("bad thing"))

	w.Close()
	os.Stderr = oldStderr

	buf := make([]byte, 256)
	n, _ := r.Read(buf)
	output := string(buf[:n])
	if !strings.Contains(output, "SECURITY") {
		t.Error("expected SECURITY in output")
	}
	if !strings.Contains(output, "test event") {
		t.Error("expected event message")
	}
}

// ---------- unfurl.go ----------

func TestUnfurlURLs_NoURLs(t *testing.T) {
	got := unfurlURLs("just plain text")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// ---------- logger.go ----------

func TestLogger_LogRequest_Enabled(t *testing.T) {
	l := NewLogger(&parser.LogConfig{LogRequests: true})
	req := httptest.NewRequest("GET", "/test", nil)
	l.LogRequest(req, 200, 5*time.Millisecond) // should print
}

func TestLogger_LogError_WithStacktrace(t *testing.T) {
	l := NewLogger(&parser.LogConfig{LogErrors: true, Stacktrace: true})
	l.LogError("crash", fmt.Errorf("boom")) // should print with stacktrace
}

func TestLogger_LogSlowQuery(t *testing.T) {
	l := NewLogger(&parser.LogConfig{SlowQueryMs: 10})
	l.LogSlowQuery("SELECT * FROM users", 5*time.Millisecond)  // under threshold
	l.LogSlowQuery("SELECT * FROM users", 50*time.Millisecond) // over threshold
}

// ---------- auth.go ----------

func TestGetSession_NilStore(t *testing.T) {
	s := &Server{sessions: nil}
	req := httptest.NewRequest("GET", "/", nil)
	if got := s.getSession(req); got != nil {
		t.Error("expected nil when no session store")
	}
}

func TestGetSession_NoCookie(t *testing.T) {
	store := NewSessionStore("")
	s := &Server{sessions: store}
	req := httptest.NewRequest("GET", "/", nil)
	if got := s.getSession(req); got != nil {
		t.Error("expected nil when no cookie")
	}
}

func TestGetSession_InvalidCookie(t *testing.T) {
	store := NewSessionStore("")
	s := &Server{sessions: store}
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: "bad"})
	if got := s.getSession(req); got != nil {
		t.Error("expected nil for invalid cookie")
	}
}

// ---------- permissions.go ----------

func TestBuildPermissionMap_All(t *testing.T) {
	app := &parser.App{
		Permissions: []parser.Permission{
			{Role: "admin", Rules: []string{"all"}},
		},
	}
	pm := BuildPermissionMap(app)
	rules := pm["admin"]["*"]
	if len(rules) != 1 || rules[0].Action != "all" {
		t.Errorf("unexpected rules: %v", rules)
	}
}

// ---------- logger.go ----------

func TestLogger_LogSlowQuery_Disabled(t *testing.T) {
	l := NewLogger(&parser.LogConfig{SlowQueryMs: 0})
	l.LogSlowQuery("SELECT *", 500*time.Millisecond) // should not print
}

func TestLoggingMiddleware(t *testing.T) {
	l := NewLogger(&parser.LogConfig{LogRequests: true})
	handler := l.LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("hello"))
	}))
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Errorf("code = %d, want 418", rec.Code)
	}
	if rec.Body.String() != "hello" {
		t.Errorf("body = %q, want hello", rec.Body.String())
	}
}

func TestLoggingResponseWriter_HijackUnsupported(t *testing.T) {
	rec := httptest.NewRecorder()
	lw := &loggingResponseWriter{ResponseWriter: rec, statusCode: 200}
	_, _, err := lw.Hijack()
	if err == nil {
		t.Error("expected error for unsupported hijack")
	}
}

func TestLoggingResponseWriter_FlushUnsupported(t *testing.T) {
	rec := httptest.NewRecorder()
	lw := &loggingResponseWriter{ResponseWriter: rec, statusCode: 200}
	// Should not panic even if underlying writer doesn't support Flusher
	lw.Flush()
}

// ---------- permissions.go ----------

func TestResolvePermissionPlaceholders_Missing(t *testing.T) {
	got := resolvePermissionPlaceholders("status = :current_user.id", map[string]string{})
	if got != "" {
		t.Errorf("expected empty for missing param, got %q", got)
	}
}

func TestResolvePermissionPlaceholders_Present(t *testing.T) {
	got := resolvePermissionPlaceholders("status = :current_user.id", map[string]string{"current_user.id": "42"})
	if got != "status = :current_user.id" {
		t.Errorf("expected unchanged filter, got %q", got)
	}
}

// ---------- ratelimit.go ----------

func TestRateLimiter_CheckWithRule_UserFallback(t *testing.T) {
	rl := NewRateLimiter([]parser.RateLimit{
		{PathPattern: "/api", Requests: 1, Window: "minute", Per: "user"},
	})
	req := httptest.NewRequest("GET", "/api", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	exceeded, rule := rl.CheckWithRule(req, nil)
	if exceeded {
		t.Error("first request should not be exceeded")
	}
	if rule != nil {
		t.Error("expected no rule on first request")
	}
}

func TestRateLimiter_CheckWithRule_Exceeded(t *testing.T) {
	rl := NewRateLimiter([]parser.RateLimit{
		{PathPattern: "/api", Requests: 1, Window: "minute", Per: "ip"},
	})
	req := httptest.NewRequest("GET", "/api", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rl.Check(req, nil)                           // first request
	exceeded, rule := rl.CheckWithRule(req, nil) // second request
	if !exceeded {
		t.Error("expected exceeded")
	}
	if rule == nil {
		t.Error("expected matched rule")
	}
}

func TestNewRateLimiter_WithCustomAuthLimit(t *testing.T) {
	rl := NewRateLimiter([]parser.RateLimit{
		{PathPattern: "/login", Requests: 100, Window: "minute", Per: "ip"},
	})
	// Should not add default /login limit since custom exists
	foundDefault := false
	for _, r := range rl.rules {
		if r.PathPattern == "/login" && r.Requests == 10 {
			foundDefault = true
		}
	}
	if foundDefault {
		t.Error("should not add default auth limit when custom exists")
	}
}

// ---------- fetch.go ----------

func TestFlattenJSON_NilAndFloatAndBool(t *testing.T) {
	obj := map[string]interface{}{
		"null_field": nil,
		"pi":         3.14,
		"count":      42.0,
		"active":     true,
		"deleted":    false,
		"str":        "hello",
	}
	row := flattenJSON(obj, "")
	if row["null_field"] != "" {
		t.Errorf("null = %q, want empty", row["null_field"])
	}
	if row["pi"] != "3.14" {
		t.Errorf("pi = %q, want 3.14", row["pi"])
	}
	if row["count"] != "42" {
		t.Errorf("count = %q, want 42", row["count"])
	}
	if row["active"] != "true" {
		t.Errorf("active = %q, want true", row["active"])
	}
	if row["deleted"] != "false" {
		t.Errorf("deleted = %q, want false", row["deleted"])
	}
	if row["str"] != "hello" {
		t.Errorf("str = %q, want hello", row["str"])
	}
}

func TestFlattenJSON_LargeArray(t *testing.T) {
	arr := make([]interface{}, 15)
	for i := range arr {
		arr[i] = fmt.Sprintf("item%d", i)
	}
	obj := map[string]interface{}{"items": arr}
	row := flattenJSON(obj, "")
	if row["items._count"] != "15" {
		t.Errorf("count = %q, want 15", row["items._count"])
	}
	if row["items.9"] != "item9" {
		t.Errorf("item9 = %q, want item9", row["items.9"])
	}
	if _, ok := row["items.10"]; ok {
		t.Error("expected item10 to be skipped")
	}
}

// ---------- email.go ----------

func TestIsPathWithinAllowedDirs_Outside(t *testing.T) {
	if isPathWithinAllowedDirs("/etc/passwd") {
		t.Error("expected false for outside path")
	}
}

func TestResolveManifest_DynamicFields(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec(`CREATE TABLE "_deal_field_defs" (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		"name" TEXT NOT NULL, "kind" TEXT NOT NULL, "label" TEXT NOT NULL,
		"required" INTEGER NOT NULL DEFAULT 0, "options" TEXT,
		"reference_model" TEXT, "tenant_id" TEXT, "sort_order" INTEGER DEFAULT 0)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec(`INSERT INTO "_deal_field_defs" (name, kind, label, required) VALUES ('revenue', 'number', 'Revenue', 0)`); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	s := &Server{db: db}
	model := &parser.Model{Name: "deal", DynamicFields: true}
	app := &parser.App{}
	got := s.resolveManifest(model, app, nil, nil, nil)
	if got == nil {
		t.Fatal("expected manifest")
	}
	if len(got.Fields) != 1 || got.Fields[0].Name != "revenue" {
		t.Errorf("unexpected fields: %v", got.Fields)
	}
}

func TestResolveManifest_DynamicFieldsCached(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec(`CREATE TABLE "_deal_field_defs" (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		"name" TEXT NOT NULL, "kind" TEXT NOT NULL, "label" TEXT NOT NULL,
		"required" INTEGER NOT NULL DEFAULT 0, "options" TEXT,
		"reference_model" TEXT, "tenant_id" TEXT, "sort_order" INTEGER DEFAULT 0)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec(`INSERT INTO "_deal_field_defs" (name, kind, label, required) VALUES ('revenue', 'number', 'Revenue', 0)`); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	s := &Server{db: db}
	model := &parser.Model{Name: "deal", DynamicFields: true}
	app := &parser.App{}
	got1 := s.resolveManifest(model, app, nil, nil, nil)
	got2 := s.resolveManifest(model, app, nil, nil, nil)
	if got1 != got2 {
		t.Error("expected cached manifest")
	}
}

func TestResolveManifest_ParseError(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.WriteFile("bad.kilnx", []byte(`invalid content!!!`), 0644)

	manifest := &parser.CustomFieldManifest{ModelName: "deal"}
	s := &Server{}
	model := &parser.Model{
		Name:                 "deal",
		CustomFieldsFile:     "bad.kilnx",
		CustomFieldsFallback: "static",
	}
	app := &parser.App{
		CustomManifests: map[string]*parser.CustomFieldManifest{
			"deal": manifest,
		},
	}
	got := s.resolveManifest(model, app, nil, nil, nil)
	if got != manifest {
		t.Error("expected fallback manifest for parse error")
	}
}

func TestResolveManifest_PathTraversal(t *testing.T) {
	manifest := &parser.CustomFieldManifest{ModelName: "deal"}
	s := &Server{}
	model := &parser.Model{
		Name:             "deal",
		CustomFieldsFile: "../../../etc/passwd.kilnx",
	}
	app := &parser.App{
		CustomManifests: map[string]*parser.CustomFieldManifest{
			"deal": manifest,
		},
	}
	got := s.resolveManifest(model, app, nil, nil, nil)
	if got != manifest {
		t.Error("expected fallback manifest for path traversal")
	}
}

func TestResolveManifest_DynamicPathNoFallback(t *testing.T) {
	s := &Server{}
	model := &parser.Model{
		Name:             "deal",
		CustomFieldsFile: "tenant_{:org}.kilnx",
	}
	app := &parser.App{}
	// No fallback, missing file → nil
	got := s.resolveManifest(model, app, nil, map[string]string{"org": "missing"}, nil)
	if got != nil {
		t.Error("expected nil when file missing and no fallback")
	}
}

func TestResolveManifest_DynamicPathNonKilnxExtension(t *testing.T) {
	manifest := &parser.CustomFieldManifest{ModelName: "deal"}
	s := &Server{}
	model := &parser.Model{
		Name:                 "deal",
		CustomFieldsFile:     "tenant_{:org}.json",
		CustomFieldsFallback: "static",
	}
	app := &parser.App{
		CustomManifests: map[string]*parser.CustomFieldManifest{
			"deal": manifest,
		},
	}
	// Resolved path does not end in .kilnx and dynamic fields false → nil
	got := s.resolveManifest(model, app, nil, map[string]string{"org": "acme"}, nil)
	if got != nil {
		t.Errorf("expected nil for non-.kilnx resolved path without dynamic fields, got %v", got)
	}
}
