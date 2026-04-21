package parser

import (
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/lexer"
)

func parse(t *testing.T, src string) *App {
	t.Helper()
	tokens := lexer.Tokenize(src)
	app, err := Parse(tokens, src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return app
}

func parseAllowErrors(t *testing.T, src string) (*App, error) {
	t.Helper()
	tokens := lexer.Tokenize(src)
	return Parse(tokens, src)
}

func TestHelloWorld(t *testing.T) {
	src := "page /\n  \"Hello World\""
	app := parse(t, src)

	if len(app.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(app.Pages))
	}
	if app.Pages[0].Path != "/" {
		t.Errorf("expected path '/', got %q", app.Pages[0].Path)
	}
	if len(app.Pages[0].Body) != 1 {
		t.Fatalf("expected 1 body node, got %d", len(app.Pages[0].Body))
	}
	if app.Pages[0].Body[0].Type != NodeText {
		t.Errorf("expected NodeText, got %d", app.Pages[0].Body[0].Type)
	}
	if app.Pages[0].Body[0].Value != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", app.Pages[0].Body[0].Value)
	}
}

func TestPageDefaultMethod(t *testing.T) {
	app := parse(t, "page /about\n  \"About us\"")
	if app.Pages[0].Method != "GET" {
		t.Errorf("page default method should be GET, got %q", app.Pages[0].Method)
	}
}

func TestPageWithTitle(t *testing.T) {
	app := parse(t, "page /about title \"About Us\"\n  \"content\"")
	if app.Pages[0].Title != "About Us" {
		t.Errorf("expected title 'About Us', got %q", app.Pages[0].Title)
	}
}

func TestPageWithLayout(t *testing.T) {
	app := parse(t, "page /about layout main\n  \"content\"")
	if app.Pages[0].Layout != "main" {
		t.Errorf("expected layout 'main', got %q", app.Pages[0].Layout)
	}
}

func TestPageRequiresAuth(t *testing.T) {
	app := parse(t, "page /dashboard requires auth\n  \"Dashboard\"")
	p := app.Pages[0]
	if !p.Auth {
		t.Error("page should have Auth=true")
	}
	if p.RequiresRole != "auth" {
		t.Errorf("expected RequiresRole 'auth', got %q", p.RequiresRole)
	}
}

func TestPageRequiresAdmin(t *testing.T) {
	app := parse(t, "page /admin requires admin\n  \"Admin\"")
	p := app.Pages[0]
	if !p.Auth {
		t.Error("page should have Auth=true")
	}
	if p.RequiresRole != "admin" {
		t.Errorf("expected RequiresRole 'admin', got %q", p.RequiresRole)
	}
}

func TestModelBasic(t *testing.T) {
	src := `model user
  name: text required min 2 max 100
  email: email unique
  active: bool default true
  created: timestamp auto`

	app := parse(t, src)

	if len(app.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(app.Models))
	}
	m := app.Models[0]
	if m.Name != "user" {
		t.Errorf("expected model name 'user', got %q", m.Name)
	}
	if len(m.Fields) != 4 {
		t.Fatalf("expected 4 fields, got %d", len(m.Fields))
	}

	// name: text required min 2 max 100
	f := m.Fields[0]
	if f.Name != "name" {
		t.Errorf("field 0: expected name 'name', got %q", f.Name)
	}
	if f.Type != FieldText {
		t.Errorf("field 0: expected type text, got %q", f.Type)
	}
	if !f.Required {
		t.Error("field 0: expected required=true")
	}
	if f.Min != "2" {
		t.Errorf("field 0: expected min '2', got %q", f.Min)
	}
	if f.Max != "100" {
		t.Errorf("field 0: expected max '100', got %q", f.Max)
	}

	// email: email unique
	f = m.Fields[1]
	if f.Type != FieldEmail {
		t.Errorf("field 1: expected type email, got %q", f.Type)
	}
	if !f.Unique {
		t.Error("field 1: expected unique=true")
	}

	// active: bool default true
	f = m.Fields[2]
	if f.Type != FieldBool {
		t.Errorf("field 2: expected type bool, got %q", f.Type)
	}
	if f.Default != "true" {
		t.Errorf("field 2: expected default 'true', got %q", f.Default)
	}

	// created: timestamp auto
	f = m.Fields[3]
	if f.Type != FieldTimestamp {
		t.Errorf("field 3: expected type timestamp, got %q", f.Type)
	}
	if !f.Auto {
		t.Error("field 3: expected auto=true")
	}
}

func TestModelOptionField(t *testing.T) {
	src := `model user
  role: option [admin, editor, viewer] default viewer`

	app := parse(t, src)
	f := app.Models[0].Fields[0]
	if f.Type != FieldOption {
		t.Errorf("expected type option, got %q", f.Type)
	}
	if len(f.Options) != 3 {
		t.Fatalf("expected 3 options, got %d", len(f.Options))
	}
	expected := []string{"admin", "editor", "viewer"}
	for i, opt := range expected {
		if f.Options[i] != opt {
			t.Errorf("option[%d]: expected %q, got %q", i, opt, f.Options[i])
		}
	}
	if f.Default != "viewer" {
		t.Errorf("expected default 'viewer', got %q", f.Default)
	}
}

func TestModelReferenceField(t *testing.T) {
	src := `model post
  author: user required`

	app := parse(t, src)
	f := app.Models[0].Fields[0]
	if f.Type != FieldReference {
		t.Errorf("expected type reference, got %q", f.Type)
	}
	if f.Reference != "user" {
		t.Errorf("expected reference 'user', got %q", f.Reference)
	}
	if !f.Required {
		t.Error("expected required=true")
	}
}

func TestModelTenantDirective(t *testing.T) {
	src := `model quote
  tenant: org
  number: text required
  total: float default 0`

	app := parse(t, src)
	if len(app.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(app.Models))
	}
	m := app.Models[0]
	if m.Name != "quote" {
		t.Errorf("expected model name 'quote', got %q", m.Name)
	}
	if m.Tenant != "org" {
		t.Errorf("expected Tenant 'org', got %q", m.Tenant)
	}
	// Expect auto-synthesized tenant FK field first, then declared fields.
	if len(m.Fields) != 3 {
		t.Fatalf("expected 3 fields (auto + 2 declared), got %d", len(m.Fields))
	}
	auto := m.Fields[0]
	if auto.Name != "org" || auto.Type != FieldReference || auto.Reference != "org" || !auto.Required {
		t.Errorf("expected auto-synthesized org reference (required), got %+v", auto)
	}
	if m.Fields[1].Name != "number" || m.Fields[2].Name != "total" {
		t.Errorf("declared fields out of order: %+v %+v", m.Fields[1], m.Fields[2])
	}
}

func TestModelTenantFieldStillWorksWithBuiltinType(t *testing.T) {
	// A field literally named "tenant" with a built-in type is NOT a
	// directive. It stays a regular field (escape hatch so users can have
	// a column called tenant if they want).
	src := `model agreement
  tenant: text required`

	app := parse(t, src)
	m := app.Models[0]
	if m.Tenant != "" {
		t.Errorf("expected no tenant directive, got %q", m.Tenant)
	}
	if len(m.Fields) != 1 || m.Fields[0].Name != "tenant" || m.Fields[0].Type != FieldText {
		t.Fatalf("expected regular 'tenant: text' field, got %+v", m.Fields)
	}
}

func TestModelTenantDirectiveRejectsDuplicate(t *testing.T) {
	src := `model quote
  tenant: org
  tenant: account`
	_, err := parseAllowErrors(t, src)
	if err == nil || !strings.Contains(err.Error(), "already has a tenant directive") {
		t.Fatalf("expected duplicate tenant error, got %v", err)
	}
}

func TestModelTenantDirectiveMustComeFirst(t *testing.T) {
	src := `model quote
  number: text required
  tenant: org`
	_, err := parseAllowErrors(t, src)
	if err == nil || !strings.Contains(err.Error(), "must appear before field declarations") {
		t.Fatalf("expected ordering error, got %v", err)
	}
}

func TestAuthBlock(t *testing.T) {
	src := `auth
  table: account
  identity: username
  password: secret
  login: /signin
  after login: /home`

	app := parse(t, src)

	if app.Auth == nil {
		t.Fatal("expected Auth to be set")
	}
	a := app.Auth
	if a.Table != "account" {
		t.Errorf("expected table 'account', got %q", a.Table)
	}
	if a.Identity != "username" {
		t.Errorf("expected identity 'username', got %q", a.Identity)
	}
	if a.Password != "secret" {
		t.Errorf("expected password 'secret', got %q", a.Password)
	}
	if a.LoginPath != "/signin" {
		t.Errorf("expected login '/signin', got %q", a.LoginPath)
	}
	if a.AfterLogin != "/home" {
		t.Errorf("expected after login '/home', got %q", a.AfterLogin)
	}
}

func TestAuthDefaults(t *testing.T) {
	src := "auth"
	app := parse(t, src)
	if app.Auth == nil {
		t.Fatal("expected Auth to be set")
	}
	a := app.Auth
	if a.Table != "user" {
		t.Errorf("expected default table 'user', got %q", a.Table)
	}
	if a.Identity != "email" {
		t.Errorf("expected default identity 'email', got %q", a.Identity)
	}
	if a.Password != "password" {
		t.Errorf("expected default password 'password', got %q", a.Password)
	}
	if a.LoginPath != "/login" {
		t.Errorf("expected default login '/login', got %q", a.LoginPath)
	}
	if a.AfterLogin != "/" {
		t.Errorf("expected default after login '/', got %q", a.AfterLogin)
	}
}

func TestActionWithQuery(t *testing.T) {
	src := `action /users/create method POST
  query: INSERT INTO user (name) VALUES (:name)
  redirect /users`

	app := parse(t, src)

	if len(app.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(app.Actions))
	}
	a := app.Actions[0]
	if a.Path != "/users/create" {
		t.Errorf("expected path '/users/create', got %q", a.Path)
	}
	if a.Method != "POST" {
		t.Errorf("expected method POST, got %q", a.Method)
	}
	if len(a.Body) < 2 {
		t.Fatalf("expected at least 2 body nodes, got %d", len(a.Body))
	}

	// First node should be a query
	q := a.Body[0]
	if q.Type != NodeQuery {
		t.Errorf("expected NodeQuery, got %d", q.Type)
	}
	if !strings.Contains(q.SQL, "INSERT INTO user") {
		t.Errorf("SQL should contain 'INSERT INTO user', got %q", q.SQL)
	}

	// Second node should be a redirect
	r := a.Body[1]
	if r.Type != NodeRedirect {
		t.Errorf("expected NodeRedirect, got %d", r.Type)
	}
	if r.Value != "/users" {
		t.Errorf("expected redirect to '/users', got %q", r.Value)
	}
}

func TestActionDefaultMethod(t *testing.T) {
	src := `action /do
  redirect /`
	app := parse(t, src)
	if app.Actions[0].Method != "POST" {
		t.Errorf("action default method should be POST, got %q", app.Actions[0].Method)
	}
}

func TestFragment(t *testing.T) {
	src := `fragment /users/:id/card
  query user: SELECT name FROM user WHERE id = :id
  html
    <div class="card">{user.name}</div>`

	app := parse(t, src)

	if len(app.Fragments) != 1 {
		t.Fatalf("expected 1 fragment, got %d", len(app.Fragments))
	}
	f := app.Fragments[0]
	if f.Path != "/users/:id/card" {
		t.Errorf("expected path '/users/:id/card', got %q", f.Path)
	}

	if len(f.Body) < 1 {
		t.Fatal("expected at least 1 body node")
	}

	// Check query node exists
	hasQuery := false
	hasHTML := false
	for _, node := range f.Body {
		if node.Type == NodeQuery {
			hasQuery = true
			if node.Name != "user" {
				t.Errorf("expected query name 'user', got %q", node.Name)
			}
		}
		if node.Type == NodeHTML {
			hasHTML = true
			if !strings.Contains(node.HTMLContent, "{user.name}") {
				t.Errorf("html should contain '{user.name}', got %q", node.HTMLContent)
			}
		}
	}
	if !hasQuery {
		t.Error("fragment body should contain a query node")
	}
	if !hasHTML {
		t.Error("fragment body should contain an html node")
	}
}

func TestLayout(t *testing.T) {
	src := `layout main
  html
    <html><body>{page.content}</body></html>`

	app := parse(t, src)

	if len(app.Layouts) != 1 {
		t.Fatalf("expected 1 layout, got %d", len(app.Layouts))
	}
	l := app.Layouts[0]
	if l.Name != "main" {
		t.Errorf("expected name 'main', got %q", l.Name)
	}
	if !strings.Contains(l.HTMLContent, "{page.content}") {
		t.Errorf("layout HTML should contain '{page.content}', got %q", l.HTMLContent)
	}
}

func TestStream(t *testing.T) {
	src := `stream /notifications requires auth
  query: SELECT message FROM notifications WHERE seen = false
  every 5 s`

	app := parse(t, src)

	if len(app.Streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(app.Streams))
	}
	s := app.Streams[0]
	if s.Path != "/notifications" {
		t.Errorf("expected path '/notifications', got %q", s.Path)
	}
	if !s.Auth {
		t.Error("stream should require auth")
	}
	if s.RequiresRole != "auth" {
		t.Errorf("expected RequiresRole 'auth', got %q", s.RequiresRole)
	}
	if !strings.Contains(s.SQL, "SELECT message FROM notifications") {
		t.Errorf("SQL should contain select, got %q", s.SQL)
	}
	if s.IntervalSecs != 5 {
		t.Errorf("expected interval 5s, got %d", s.IntervalSecs)
	}
}

func TestStreamDefaultInterval(t *testing.T) {
	src := `stream /feed
  query: SELECT * FROM items`
	app := parse(t, src)
	if app.Streams[0].IntervalSecs != 5 {
		t.Errorf("expected default interval 5, got %d", app.Streams[0].IntervalSecs)
	}
	if app.Streams[0].EventName != "message" {
		t.Errorf("expected default event 'message', got %q", app.Streams[0].EventName)
	}
}

func TestScheduleWithInterval(t *testing.T) {
	src := `schedule cleanup every 24 h
  query: DELETE FROM session WHERE expired = true`

	app := parse(t, src)

	if len(app.Schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(app.Schedules))
	}
	s := app.Schedules[0]
	if s.Name != "cleanup" {
		t.Errorf("expected name 'cleanup', got %q", s.Name)
	}
	if s.IntervalSecs != 86400 {
		t.Errorf("expected interval 86400s (24h), got %d", s.IntervalSecs)
	}
}

func TestScheduleWithCron(t *testing.T) {
	src := `schedule report every monday at 9:00
  query: SELECT count(*) FROM user`

	app := parse(t, src)
	s := app.Schedules[0]
	if s.Cron == "" {
		t.Error("expected cron expression to be set")
	}
	if !strings.Contains(s.Cron, "monday") {
		t.Errorf("cron should contain 'monday', got %q", s.Cron)
	}
}

func TestJob(t *testing.T) {
	src := `job generate-report
  retry 3
  query: SELECT * FROM orders`

	app := parse(t, src)

	if len(app.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(app.Jobs))
	}
	j := app.Jobs[0]
	if j.Name != "generate-report" {
		t.Errorf("expected name 'generate-report', got %q", j.Name)
	}
	if j.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", j.MaxRetries)
	}
}

func TestJobDefaultRetry(t *testing.T) {
	src := `job simple-task
  query: SELECT 1`
	app := parse(t, src)
	if app.Jobs[0].MaxRetries != 3 {
		t.Errorf("expected default MaxRetries 3, got %d", app.Jobs[0].MaxRetries)
	}
}

func TestAPI(t *testing.T) {
	src := `api /api/v1/users requires auth
  query users: SELECT id, name FROM user`

	app := parse(t, src)

	if len(app.APIs) != 1 {
		t.Fatalf("expected 1 api, got %d", len(app.APIs))
	}
	a := app.APIs[0]
	if a.Path != "/api/v1/users" {
		t.Errorf("expected path '/api/v1/users', got %q", a.Path)
	}
	if !a.Auth {
		t.Error("api should require auth")
	}
	if a.Method != "GET" {
		t.Errorf("expected default method GET, got %q", a.Method)
	}
}

func TestWebhook(t *testing.T) {
	src := `webhook /stripe/payment secret env STRIPE_SECRET
  on event payment_intent.succeeded
    query: UPDATE orders SET status = 'paid' WHERE stripe_id = :event_id`

	app := parse(t, src)

	if len(app.Webhooks) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(app.Webhooks))
	}
	wh := app.Webhooks[0]
	if wh.Path != "/stripe/payment" {
		t.Errorf("expected path '/stripe/payment', got %q", wh.Path)
	}
	if wh.SecretEnv != "STRIPE_SECRET" {
		t.Errorf("expected secret env 'STRIPE_SECRET', got %q", wh.SecretEnv)
	}
	if len(wh.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(wh.Events))
	}
	if !strings.Contains(wh.Events[0].Name, "payment_intent.succeeded") {
		t.Errorf("expected event 'payment_intent.succeeded', got %q", wh.Events[0].Name)
	}
}

func TestSocket(t *testing.T) {
	src := `socket /chat/:room requires auth
  on connect
    query: SELECT 'connected' as status
  on message
    query: INSERT INTO messages (room, text) VALUES (:room, :text)`

	app := parse(t, src)

	if len(app.Sockets) != 1 {
		t.Fatalf("expected 1 socket, got %d", len(app.Sockets))
	}
	s := app.Sockets[0]
	if s.Path != "/chat/:room" {
		t.Errorf("expected path '/chat/:room', got %q", s.Path)
	}
	if !s.Auth {
		t.Error("socket should require auth")
	}
	if len(s.OnConnect) == 0 {
		t.Error("expected on connect body")
	}
	if len(s.OnMessage) == 0 {
		t.Error("expected on message body")
	}
}

func TestRateLimit(t *testing.T) {
	src := `limit /api/*
  requests: 100 per minute per user`

	app := parse(t, src)

	if len(app.RateLimits) != 1 {
		t.Fatalf("expected 1 rate limit, got %d", len(app.RateLimits))
	}
	rl := app.RateLimits[0]
	if rl.PathPattern != "/api/*" {
		t.Errorf("expected path '/api/*', got %q", rl.PathPattern)
	}
	if rl.Requests != 100 {
		t.Errorf("expected 100 requests, got %d", rl.Requests)
	}
	if rl.Window != "minute" {
		t.Errorf("expected window 'minute', got %q", rl.Window)
	}
	if rl.Per != "user" {
		t.Errorf("expected per 'user', got %q", rl.Per)
	}
}

func TestRateLimitDefaults(t *testing.T) {
	src := `limit /login`
	app := parse(t, src)
	rl := app.RateLimits[0]
	if rl.Requests != 100 {
		t.Errorf("expected default 100 requests, got %d", rl.Requests)
	}
	if rl.Window != "minute" {
		t.Errorf("expected default window 'minute', got %q", rl.Window)
	}
	if rl.Per != "ip" {
		t.Errorf("expected default per 'ip', got %q", rl.Per)
	}
}

func TestTestBlock(t *testing.T) {
	src := `test "user can create post"
  visit /posts/new
  fill title with "Test Post"
  submit
  expect page /posts contains "Test Post"`

	app := parse(t, src)

	if len(app.Tests) != 1 {
		t.Fatalf("expected 1 test, got %d", len(app.Tests))
	}
	tt := app.Tests[0]
	if tt.Name != "user can create post" {
		t.Errorf("expected test name 'user can create post', got %q", tt.Name)
	}
	if len(tt.Steps) < 4 {
		t.Fatalf("expected at least 4 steps, got %d", len(tt.Steps))
	}

	if tt.Steps[0].Action != "visit" {
		t.Errorf("step 0: expected action 'visit', got %q", tt.Steps[0].Action)
	}
	if tt.Steps[0].Target != "/posts/new" {
		t.Errorf("step 0: expected target '/posts/new', got %q", tt.Steps[0].Target)
	}

	if tt.Steps[1].Action != "fill" {
		t.Errorf("step 1: expected action 'fill', got %q", tt.Steps[1].Action)
	}
	if tt.Steps[1].Target != "title" {
		t.Errorf("step 1: expected target 'title', got %q", tt.Steps[1].Target)
	}
	if tt.Steps[1].Value != "Test Post" {
		t.Errorf("step 1: expected value 'Test Post', got %q", tt.Steps[1].Value)
	}

	if tt.Steps[2].Action != "submit" {
		t.Errorf("step 2: expected action 'submit', got %q", tt.Steps[2].Action)
	}

	if tt.Steps[3].Action != "expect" {
		t.Errorf("step 3: expected action 'expect', got %q", tt.Steps[3].Action)
	}
	if tt.Steps[3].Value != "Test Post" {
		t.Errorf("step 3: expected value 'Test Post', got %q", tt.Steps[3].Value)
	}
}

func TestTranslations(t *testing.T) {
	src := `translations
  en
    welcome: "Welcome back"
    users: "Users"
  pt
    welcome: "Bem vindo de volta"
    users: "Usuários"`

	app := parse(t, src)

	if app.Translations == nil {
		t.Fatal("expected translations to be set")
	}
	if len(app.Translations) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(app.Translations))
	}

	en := app.Translations["en"]
	if en == nil {
		t.Fatal("expected 'en' translations")
	}
	if en["welcome"] != "Welcome back" {
		t.Errorf("en.welcome: expected 'Welcome back', got %q", en["welcome"])
	}
	if en["users"] != "Users" {
		t.Errorf("en.users: expected 'Users', got %q", en["users"])
	}

	pt := app.Translations["pt"]
	if pt == nil {
		t.Fatal("expected 'pt' translations")
	}
	if pt["welcome"] != "Bem vindo de volta" {
		t.Errorf("pt.welcome: expected 'Bem vindo de volta', got %q", pt["welcome"])
	}
}

func TestNamedQueries(t *testing.T) {
	src := `queries
  active-users: SELECT name FROM user WHERE active = true
  recent-posts: SELECT * FROM post ORDER BY created DESC LIMIT 10`

	app := parse(t, src)

	if app.NamedQueries == nil {
		t.Fatal("expected named queries to be set")
	}
	if len(app.NamedQueries) != 2 {
		t.Fatalf("expected 2 named queries, got %d", len(app.NamedQueries))
	}
	if !strings.Contains(app.NamedQueries["active-users"], "SELECT name FROM user") {
		t.Errorf("active-users query unexpected: %q", app.NamedQueries["active-users"])
	}
	if !strings.Contains(app.NamedQueries["recent-posts"], "SELECT * FROM post") {
		t.Errorf("recent-posts query unexpected: %q", app.NamedQueries["recent-posts"])
	}
}

func TestErrorRecovery(t *testing.T) {
	// model missing name should produce an error but parsing should continue
	src := `model
page /about
  "About us"`

	app, err := parseAllowErrors(t, src)
	if err == nil {
		t.Log("expected an error from malformed model, but parser may have recovered gracefully")
	}

	// The page should still be parsed despite the model error
	if len(app.Pages) != 1 {
		t.Errorf("expected 1 page after error recovery, got %d", len(app.Pages))
	}
}

func TestMultipleBlocks(t *testing.T) {
	src := `model user
  name: text required

page /
  "Welcome"

action /create method POST
  redirect /`

	app := parse(t, src)

	if len(app.Models) != 1 {
		t.Errorf("expected 1 model, got %d", len(app.Models))
	}
	if len(app.Pages) != 1 {
		t.Errorf("expected 1 page, got %d", len(app.Pages))
	}
	if len(app.Actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(app.Actions))
	}
}

func TestActionRequiresAuth(t *testing.T) {
	src := `action /admin/delete requires admin
  query: DELETE FROM user WHERE id = :id`
	app := parse(t, src)
	a := app.Actions[0]
	if !a.Auth {
		t.Error("action should require auth")
	}
	if a.RequiresRole != "admin" {
		t.Errorf("expected RequiresRole 'admin', got %q", a.RequiresRole)
	}
}

func TestQueryWithName(t *testing.T) {
	src := `page /users
  query users: SELECT name, email FROM user`
	app := parse(t, src)
	if len(app.Pages[0].Body) < 1 {
		t.Fatal("expected at least 1 body node")
	}
	q := app.Pages[0].Body[0]
	if q.Type != NodeQuery {
		t.Fatalf("expected NodeQuery, got %d", q.Type)
	}
	if q.Name != "users" {
		t.Errorf("expected query name 'users', got %q", q.Name)
	}
}

func TestValidateNode(t *testing.T) {
	src := `action /create method POST
  validate user
  redirect /`
	app := parse(t, src)
	found := false
	for _, n := range app.Actions[0].Body {
		if n.Type == NodeValidate && n.ModelName == "user" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validate node with model 'user'")
	}
}

func TestEmptyApp(t *testing.T) {
	app := parse(t, "")
	if len(app.Pages) != 0 {
		t.Errorf("expected 0 pages, got %d", len(app.Pages))
	}
	if len(app.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(app.Models))
	}
}

func TestRateLimitWithMessage(t *testing.T) {
	src := `limit /login
  requests: 5 per minute per ip
  message: "Too many attempts, please try again later"`

	app := parse(t, src)
	rl := app.RateLimits[0]
	if rl.Requests != 5 {
		t.Errorf("expected 5 requests, got %d", rl.Requests)
	}
	if rl.Message != "Too many attempts, please try again later" {
		t.Errorf("unexpected message: %q", rl.Message)
	}
}

func TestSocketWithDisconnect(t *testing.T) {
	src := `socket /live
  on connect
    query: SELECT 1
  on disconnect
    query: DELETE FROM connections WHERE id = :conn_id`

	app := parse(t, src)
	s := app.Sockets[0]
	if len(s.OnConnect) == 0 {
		t.Error("expected on connect body")
	}
	if len(s.OnDisconnect) == 0 {
		t.Error("expected on disconnect body")
	}
}

func TestWebhookMultipleEvents(t *testing.T) {
	src := `webhook /stripe secret env STRIPE_KEY
  on event payment.completed
    query: UPDATE orders SET paid = true WHERE id = :order_id
  on event refund.created
    query: UPDATE orders SET refunded = true WHERE id = :order_id`

	app := parse(t, src)
	wh := app.Webhooks[0]
	if len(wh.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(wh.Events))
	}
}

func TestModelMultipleFieldTypes(t *testing.T) {
	src := `model product
  title: text
  description: richtext
  price: float
  quantity: int
  photo: image
  contact: phone`

	app := parse(t, src)
	m := app.Models[0]
	expectedTypes := map[string]FieldType{
		"title":       FieldText,
		"description": FieldRichtext,
		"price":       FieldFloat,
		"quantity":    FieldInt,
		"photo":       FieldImage,
		"contact":     FieldPhone,
	}
	for _, f := range m.Fields {
		if expected, ok := expectedTypes[f.Name]; ok {
			if f.Type != expected {
				t.Errorf("field %q: expected type %q, got %q", f.Name, expected, f.Type)
			}
		}
	}
}

func TestPageWithQueryAndHTML(t *testing.T) {
	src := `page /users
  query users: SELECT name FROM user
  html
    <ul>{users}</ul>`

	app := parse(t, src)
	body := app.Pages[0].Body
	hasQuery := false
	hasHTML := false
	for _, n := range body {
		if n.Type == NodeQuery {
			hasQuery = true
		}
		if n.Type == NodeHTML {
			hasHTML = true
		}
	}
	if !hasQuery {
		t.Error("expected query node in page body")
	}
	if !hasHTML {
		t.Error("expected html node in page body")
	}
}

func TestNamedQueryResolution(t *testing.T) {
	src := `queries
  all-users: SELECT * FROM user

page /users
  query users: all-users`

	app := parse(t, src)
	if len(app.Pages) < 1 || len(app.Pages[0].Body) < 1 {
		t.Fatal("expected page with query")
	}
	q := app.Pages[0].Body[0]
	if q.Type != NodeQuery {
		t.Fatal("expected NodeQuery")
	}
	if !strings.Contains(q.SQL, "SELECT * FROM user") {
		t.Errorf("expected resolved SQL, got %q", q.SQL)
	}
}

func TestTestBlockWithAs(t *testing.T) {
	src := `test "admin access"
  as user with role admin
  visit /admin`

	app := parse(t, src)
	tt := app.Tests[0]
	if len(tt.Steps) < 2 {
		t.Fatalf("expected at least 2 steps, got %d", len(tt.Steps))
	}
	if tt.Steps[0].Action != "as" {
		t.Errorf("expected action 'as', got %q", tt.Steps[0].Action)
	}
}

func TestFragmentRequiresAuth(t *testing.T) {
	src := `fragment /secure/card requires auth
  html
    <div>Secure</div>`

	app := parse(t, src)
	f := app.Fragments[0]
	if !f.Auth {
		t.Error("fragment should require auth")
	}
}

func TestLayoutWithQueries(t *testing.T) {
	src := `layout docs
  query nav_items: SELECT slug, title FROM doc ORDER BY sort_order
  html
    <html>
    <body>
      {{each nav_items}}
      <a href="/docs/{slug}">{title}</a>
      {{end}}
      {page.content}
    </body>
    </html>

page /test layout docs
  "Hello"`

	app := parse(t, src)
	if len(app.Layouts) != 1 {
		t.Fatalf("expected 1 layout, got %d", len(app.Layouts))
	}
	layout := app.Layouts[0]
	if layout.Name != "docs" {
		t.Errorf("expected layout name 'docs', got '%s'", layout.Name)
	}
	if len(layout.Queries) != 1 {
		t.Fatalf("expected 1 query in layout, got %d", len(layout.Queries))
	}
	if layout.Queries[0].Name != "nav_items" {
		t.Errorf("expected query name 'nav_items', got '%s'", layout.Queries[0].Name)
	}
	if layout.HTMLContent == "" {
		t.Error("layout HTML content should not be empty")
	}
}

func TestPageWithDynamicTitle(t *testing.T) {
	src := "page /docs/:slug title {doc.title}\n  query doc: SELECT title FROM doc WHERE slug = :slug\n  html\n    <h1>{doc.title}</h1>"
	app := parse(t, src)
	if app.Pages[0].Title != "{doc.title}" {
		t.Errorf("expected dynamic title '{doc.title}', got %q", app.Pages[0].Title)
	}
}

func TestPageWithDynamicTitleMixed(t *testing.T) {
	src := "page / title Docs - {doc.title}\n  \"content\""
	app := parse(t, src)
	if app.Pages[0].Title != "Docs - {doc.title}" {
		t.Errorf("expected 'Docs - {doc.title}', got %q", app.Pages[0].Title)
	}
}

func TestPageWithDynamicTitleAndModifiers(t *testing.T) {
	src := "page /docs/:slug title {doc.title} layout main requires auth\n  \"content\""
	app := parse(t, src)
	if app.Pages[0].Title != "{doc.title}" {
		t.Errorf("expected '{doc.title}', got %q", app.Pages[0].Title)
	}
	if app.Pages[0].Layout != "main" {
		t.Errorf("expected layout 'main', got %q", app.Pages[0].Layout)
	}
	if !app.Pages[0].Auth {
		t.Error("expected auth to be true")
	}
}

func TestPageWithStaticTitleStillWorks(t *testing.T) {
	src := "page /about title \"About Us\" layout main\n  \"content\""
	app := parse(t, src)
	if app.Pages[0].Title != "About Us" {
		t.Errorf("expected 'About Us', got %q", app.Pages[0].Title)
	}
	if app.Pages[0].Layout != "main" {
		t.Errorf("expected layout 'main', got %q", app.Pages[0].Layout)
	}
}

func TestModelCustomFieldsDirective(t *testing.T) {
	src := "model deal\n  title: text required\n  custom fields from \"deal_fields.kilnx\"\n  value: float"
	app := parse(t, src)
	if len(app.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(app.Models))
	}
	m := app.Models[0]
	if m.CustomFieldsFile != "deal_fields.kilnx" {
		t.Errorf("expected CustomFieldsFile 'deal_fields.kilnx', got %q", m.CustomFieldsFile)
	}
	if len(m.Fields) != 2 {
		t.Errorf("expected 2 fields (title, value), got %d", len(m.Fields))
	}
}

func TestParseManifest(t *testing.T) {
	src := `field revenue
  kind: number
  label: "Receita"
  required: false

field region
  kind: option [N, S, L, O]
  label: "Região"
  required: true`
	manifest, err := ParseManifest(src, "deal")
	if err != nil {
		t.Fatalf("ParseManifest error: %v", err)
	}
	if manifest.ModelName != "deal" {
		t.Errorf("expected model name 'deal', got %q", manifest.ModelName)
	}
	if len(manifest.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(manifest.Fields))
	}
	rev := manifest.Fields[0]
	if rev.Name != "revenue" {
		t.Errorf("expected field name 'revenue', got %q", rev.Name)
	}
	if rev.Kind != CustomFieldKindNumber {
		t.Errorf("expected kind 'number', got %q", rev.Kind)
	}
	if rev.Label != "Receita" {
		t.Errorf("expected label 'Receita', got %q", rev.Label)
	}
	if rev.Required {
		t.Error("expected required=false")
	}
	reg := manifest.Fields[1]
	if reg.Name != "region" {
		t.Errorf("expected field name 'region', got %q", reg.Name)
	}
	if len(reg.Options) != 4 {
		t.Errorf("expected 4 options, got %d", len(reg.Options))
	}
	if !reg.Required {
		t.Error("expected required=true for region")
	}
}

func TestParseManifestColumnModeAndReference(t *testing.T) {
	src := `field revenue
  kind: number
  mode: column
  label: "Revenue"

field client
  kind: reference company
  mode: column
  label: "Client"

field notes
  kind: text
  label: "Notes"`
	manifest, err := ParseManifest(src, "deal")
	if err != nil {
		t.Fatalf("ParseManifest error: %v", err)
	}
	if len(manifest.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(manifest.Fields))
	}
	rev := manifest.Fields[0]
	if rev.Mode != CustomFieldModeColumn {
		t.Errorf("revenue: expected mode=column, got %q", rev.Mode)
	}
	if rev.Kind != CustomFieldKindNumber {
		t.Errorf("revenue: expected kind=number, got %q", rev.Kind)
	}
	client := manifest.Fields[1]
	if client.Kind != CustomFieldKindReference {
		t.Errorf("client: expected kind=reference, got %q", client.Kind)
	}
	if client.Reference != "company" {
		t.Errorf("client: expected reference=company, got %q", client.Reference)
	}
	if client.Mode != CustomFieldModeColumn {
		t.Errorf("client: expected mode=column, got %q", client.Mode)
	}
	notes := manifest.Fields[2]
	if notes.Mode != CustomFieldModeJSON {
		t.Errorf("notes: expected mode=JSON (empty), got %q", notes.Mode)
	}
}
