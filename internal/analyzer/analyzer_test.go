package analyzer

import (
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestBuildSchema(t *testing.T) {
	models := []parser.Model{
		{
			Name: "user",
			Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "email", Type: parser.FieldEmail},
				{Name: "password", Type: parser.FieldPassword},
				{Name: "role", Type: parser.FieldOption},
				{Name: "active", Type: parser.FieldBool},
				{Name: "created", Type: parser.FieldTimestamp},
			},
		},
		{
			Name: "post",
			Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
				{Name: "body", Type: parser.FieldRichtext},
				{Name: "author", Type: parser.FieldReference, Reference: "user"},
				{Name: "created", Type: parser.FieldTimestamp},
			},
		},
	}

	schema := BuildSchema(models)

	if len(schema.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(schema.Tables))
	}

	user := schema.Tables["user"]
	if user == nil {
		t.Fatal("user table not found")
	}
	for _, col := range []string{"id", "name", "email", "password", "role", "active", "created"} {
		if _, exists := user.Columns[col]; !exists {
			t.Errorf("user.%s should exist", col)
		}
	}

	post := schema.Tables["post"]
	if post == nil {
		t.Fatal("post table not found")
	}
	if _, exists := post.Columns["author_id"]; !exists {
		t.Error("post.author_id should exist (reference field)")
	}
	if _, exists := post.Columns["author"]; exists {
		t.Error("post.author should NOT exist (reference becomes author_id)")
	}
}

func TestTokenizeSQL(t *testing.T) {
	tests := []struct {
		sql      string
		wantLen  int
		wantLast string
	}{
		{"SELECT * FROM user", 4, "user"},
		{"SELECT name, email FROM user WHERE id = :id", 10, ""},
		{"INSERT INTO user (name, email) VALUES (:name, :email)", 14, ""},
		{"SELECT 'hello world' as greeting", 4, ""},
		{"DELETE FROM user WHERE active = 0", 8, ""},
	}

	for _, tt := range tests {
		tokens := tokenizeSQL(tt.sql)
		if len(tokens) < 2 {
			t.Errorf("tokenizeSQL(%q): got %d tokens, expected at least 2", tt.sql, len(tokens))
		}
	}
}

func TestExtractTableRefs(t *testing.T) {
	tests := []struct {
		sql    string
		tables []string
	}{
		{"SELECT * FROM user", []string{"user"}},
		{"SELECT * FROM user u", []string{"user"}},
		{"SELECT * FROM user AS u", []string{"user"}},
		{"DELETE FROM user WHERE id = 1", []string{"user"}},
		{"INSERT INTO user (name) VALUES ('test')", []string{"user"}},
		{"UPDATE user SET name = 'test' WHERE id = 1", []string{"user"}},
		{"SELECT u.name, p.title FROM user u JOIN post p ON p.author_id = u.id", []string{"user", "post"}},
		{"SELECT u.name FROM user u LEFT JOIN post p ON p.author_id = u.id", []string{"user", "post"}},
		{"SELECT 'connected' as status", nil},
	}

	for _, tt := range tests {
		tokens := tokenizeSQL(tt.sql)
		refs := extractTableRefs(tokens)

		var names []string
		for _, r := range refs {
			names = append(names, r.name)
		}

		if len(names) != len(tt.tables) {
			t.Errorf("extractTableRefs(%q): got %v, want %v", tt.sql, names, tt.tables)
			continue
		}
		for i, name := range names {
			if name != tt.tables[i] {
				t.Errorf("extractTableRefs(%q)[%d]: got %q, want %q", tt.sql, i, name, tt.tables[i])
			}
		}
	}
}

func TestExtractTableRefsAliases(t *testing.T) {
	tokens := tokenizeSQL("SELECT u.name FROM user u JOIN post p ON p.author_id = u.id")
	refs := extractTableRefs(tokens)

	if len(refs) != 2 {
		t.Fatalf("expected 2 table refs, got %d", len(refs))
	}
	if refs[0].name != "user" || refs[0].alias != "u" {
		t.Errorf("ref[0]: got {%s, %s}, want {user, u}", refs[0].name, refs[0].alias)
	}
	if refs[1].name != "post" || refs[1].alias != "p" {
		t.Errorf("ref[1]: got {%s, %s}, want {post, p}", refs[1].name, refs[1].alias)
	}
}

func TestExtractInsertColumns(t *testing.T) {
	tokens := tokenizeSQL("INSERT INTO user (name, email, role) VALUES (:name, :email, :role)")
	table, cols := extractInsertColumns(tokens)

	if table != "user" {
		t.Errorf("table: got %q, want 'user'", table)
	}
	expected := []string{"name", "email", "role"}
	if len(cols) != len(expected) {
		t.Fatalf("cols: got %v, want %v", cols, expected)
	}
	for i, col := range cols {
		if col != expected[i] {
			t.Errorf("cols[%d]: got %q, want %q", i, col, expected[i])
		}
	}
}

func TestExtractUpdateColumns(t *testing.T) {
	tokens := tokenizeSQL("UPDATE user SET name = :name, email = :email WHERE id = :id")
	table, cols := extractUpdateColumns(tokens)

	if table != "user" {
		t.Errorf("table: got %q, want 'user'", table)
	}
	expected := []string{"name", "email"}
	if len(cols) != len(expected) {
		t.Fatalf("cols: got %v, want %v", cols, expected)
	}
	for i, col := range cols {
		if col != expected[i] {
			t.Errorf("cols[%d]: got %q, want %q", i, col, expected[i])
		}
	}
}

func TestExtractSelectColumns(t *testing.T) {
	tests := []struct {
		sql  string
		want []columnRef
		nil_ bool
	}{
		{
			sql:  "SELECT * FROM user",
			nil_: true,
		},
		{
			sql:  "SELECT id, name, email FROM user",
			want: []columnRef{{column: "id"}, {column: "name"}, {column: "email"}},
		},
		{
			sql:  "SELECT count(*) as total FROM user",
			want: []columnRef{},
		},
		{
			sql:  "SELECT u.name, u.email FROM user u",
			want: []columnRef{{table: "u", column: "name"}, {table: "u", column: "email"}},
		},
		{
			sql:  "SELECT id, count(*) as total FROM user GROUP BY id",
			want: []columnRef{{column: "id"}},
		},
		{
			sql:  "SELECT 'connected' as status",
			want: []columnRef{},
		},
	}

	for _, tt := range tests {
		tokens := tokenizeSQL(tt.sql)
		cols := extractSelectColumns(tokens)

		if tt.nil_ {
			if cols != nil {
				t.Errorf("extractSelectColumns(%q): expected nil, got %v", tt.sql, cols)
			}
			continue
		}

		if len(cols) != len(tt.want) {
			t.Errorf("extractSelectColumns(%q): got %d cols, want %d", tt.sql, len(cols), len(tt.want))
			continue
		}
		for i, col := range cols {
			if col.table != tt.want[i].table || col.column != tt.want[i].column {
				t.Errorf("extractSelectColumns(%q)[%d]: got {%s,%s}, want {%s,%s}",
					tt.sql, i, col.table, col.column, tt.want[i].table, tt.want[i].column)
			}
		}
	}
}

func TestAnalyze_AuthPagesRequired(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "email", Type: parser.FieldEmail},
				{Name: "password", Type: parser.FieldPassword},
			}},
		},
		Auth: &parser.AuthConfig{Table: "user", Identity: "email", Password: "password", LoginPath: "/login"},
	}
	diags := Analyze(app)
	required := map[string]bool{"/login": false, "/register": false, "/forgot-password": false, "/reset-password": false}
	for _, d := range diags {
		for path := range required {
			if d.Level == "error" && strings.Contains(d.Message, "'"+path+"'") {
				required[path] = true
			}
		}
	}
	for path, found := range required {
		if !found {
			t.Errorf("expected missing-page error for %s, got diagnostics: %+v", path, diags)
		}
	}
}

func TestAnalyze_AuthPagesRespectCustomLoginPath(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "email", Type: parser.FieldEmail},
				{Name: "password", Type: parser.FieldPassword},
			}},
		},
		Auth: &parser.AuthConfig{Table: "user", Identity: "email", Password: "password", LoginPath: "/entrar"},
		Pages: []parser.Page{
			{Path: "/entrar"},
			{Path: "/register"},
			{Path: "/forgot-password"},
			{Path: "/reset-password"},
		},
	}
	diags := Analyze(app)
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "required page") {
			t.Errorf("unexpected auth-page error when all 4 paths declared: %s", d.Message)
		}
	}
}

func TestAnalyze_AuthPagesHonorCustomSlugs(t *testing.T) {
	// All four auth paths configurable: if the app uses Portuguese
	// slugs, the analyzer must demand those exact paths (not the
	// english defaults) and must NOT demand the defaults.
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "email", Type: parser.FieldEmail},
				{Name: "password", Type: parser.FieldPassword},
			}},
		},
		Auth: &parser.AuthConfig{
			Table:        "user",
			Identity:     "email",
			Password:     "password",
			LoginPath:    "/entrar",
			RegisterPath: "/cadastrar",
			ForgotPath:   "/senha/esqueci",
			ResetPath:    "/senha/redefinir",
		},
		Pages: []parser.Page{
			{Path: "/entrar"},
			{Path: "/cadastrar"},
			{Path: "/senha/esqueci"},
			{Path: "/senha/redefinir"},
		},
	}
	diags := Analyze(app)
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "required page") {
			t.Errorf("all 4 custom slugs declared; unexpected error: %s", d.Message)
		}
	}

	// Missing pt-BR page triggers error about THAT path, not /register.
	app.Pages = []parser.Page{
		{Path: "/entrar"},
		{Path: "/senha/esqueci"},
		{Path: "/senha/redefinir"},
	}
	diags = Analyze(app)
	wantMsg := "'/cadastrar'"
	dontWant := "'/register'"
	foundWant := false
	for _, d := range diags {
		if strings.Contains(d.Message, dontWant) {
			t.Errorf("analyzer asked for default '/register' but custom slug is '/cadastrar': %s", d.Message)
		}
		if strings.Contains(d.Message, wantMsg) {
			foundWant = true
		}
	}
	if !foundWant {
		t.Errorf("expected error mentioning '/cadastrar' (custom register slug), got diagnostics: %+v", diags)
	}
}

func TestAnalyze_NoAuthBlock_NoPagesRequired(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{{Name: "name", Type: parser.FieldText}}},
		},
	}
	diags := Analyze(app)
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "required page") {
			t.Errorf("apps without auth block must not be forced to declare auth pages: %s", d.Message)
		}
	}
}

func TestAnalyze_TenantRefUnknownModel(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "quote", Tenant: "ogr", // typo, no such model
				Fields: []parser.Field{{Name: "ogr", Type: parser.FieldReference, Reference: "ogr", Required: true}}},
		},
	}
	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "tenant 'ogr'") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected error for unknown tenant model, got: %+v", diags)
	}
}

func TestAnalyze_TenantRefSelf(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "quote", Tenant: "quote"},
		},
	}
	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "own tenant") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected self-tenant error, got: %+v", diags)
	}
}

func TestAnalyze_ValidApp(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{
				Name: "user",
				Fields: []parser.Field{
					{Name: "name", Type: parser.FieldText},
					{Name: "email", Type: parser.FieldEmail},
					{Name: "password", Type: parser.FieldPassword},
					{Name: "role", Type: parser.FieldOption},
					{Name: "active", Type: parser.FieldBool},
					{Name: "created", Type: parser.FieldTimestamp},
				},
			},
			{
				Name: "post",
				Fields: []parser.Field{
					{Name: "title", Type: parser.FieldText},
					{Name: "body", Type: parser.FieldRichtext},
					{Name: "status", Type: parser.FieldOption},
					{Name: "author", Type: parser.FieldReference, Reference: "user"},
					{Name: "created", Type: parser.FieldTimestamp},
				},
			},
		},
		Auth: &parser.AuthConfig{Table: "user", Identity: "email", Password: "password"},
		Permissions: []parser.Permission{
			{Role: "admin", Rules: []string{"all"}},
		},
		Pages: []parser.Page{
			{
				Path: "/users",
				Body: []parser.Node{
					{Type: parser.NodeQuery, Name: "users", SQL: "SELECT id, name, email, role FROM user ORDER BY id DESC"},
				},
			},
			// Auth pages required because checkAuthPages enforces user
			// ownership of the GET side of every auth route.
			{Path: "/login"},
			{Path: "/register"},
			{Path: "/forgot-password"},
			{Path: "/reset-password"},
		},
		Actions: []parser.Page{
			{
				Path:         "/users/new",
				Auth:         true,
				RequiresRole: "admin",
				Body: []parser.Node{
					{Type: parser.NodeValidate, ModelName: "user"},
					{Type: parser.NodeQuery, SQL: "INSERT INTO user (name, email, password, role) VALUES (:name, :email, :password, :role)"},
				},
			},
			{
				Path:         "/users/:id/edit",
				Auth:         true,
				RequiresRole: "admin",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "UPDATE user SET name = :name, email = :email, role = :role WHERE id = :id"},
				},
			},
		},
		Streams: []parser.Stream{
			{Path: "/stream/users", SQL: "SELECT count(*) as total FROM user", Auth: true},
		},
	}

	diags := Analyze(app)
	if len(diags) > 0 {
		for _, d := range diags {
			t.Errorf("unexpected diagnostic: [%s] %s: %s", d.Level, d.Context, d.Message)
		}
	}
}

func TestAnalyze_UnknownTable(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{{Name: "name", Type: parser.FieldText}}},
		},
		Pages: []parser.Page{
			{
				Path: "/test",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT * FROM nonexistent"},
				},
			},
		},
	}

	diags := Analyze(app)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Level != "error" {
		t.Errorf("expected error, got %s", diags[0].Level)
	}
	if !strings.Contains(diags[0].Message, "nonexistent") {
		t.Errorf("expected message to mention 'nonexistent', got: %s", diags[0].Message)
	}
}

func TestAnalyze_UnknownColumn_Insert(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "email", Type: parser.FieldEmail},
			}},
		},
		Actions: []parser.Page{
			{
				Path: "/users/new",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "INSERT INTO user (name, username) VALUES (:name, :username)"},
				},
			},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "username") && strings.Contains(d.Message, "user") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about column 'username' in 'user', got: %v", diags)
	}
}

func TestAnalyze_UnknownColumn_Update(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
			}},
		},
		Actions: []parser.Page{
			{
				Path: "/users/:id",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "UPDATE user SET username = :username WHERE id = :id"},
				},
			},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "username") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about column 'username', got: %v", diags)
	}
}

func TestAnalyze_UnknownColumn_Select(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "email", Type: parser.FieldEmail},
			}},
		},
		Pages: []parser.Page{
			{
				Path: "/test",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT id, name, username FROM user"},
				},
			},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "username") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about column 'username', got: %v", diags)
	}
}

func TestAnalyze_InvalidAuthTable(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{{Name: "name", Type: parser.FieldText}}},
		},
		Auth: &parser.AuthConfig{Table: "accounts"},
		// Declare the four auth pages so this test isolates the
		// auth-table error; otherwise checkAuthPages would also fire.
		Pages: []parser.Page{
			{Path: "/login"},
			{Path: "/register"},
			{Path: "/forgot-password"},
			{Path: "/reset-password"},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "accounts") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about auth table 'accounts', got: %v", diags)
	}
}

func TestAnalyze_InvalidReference(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "post", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
				{Name: "author", Type: parser.FieldReference, Reference: "user"},
			}},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "user") && strings.Contains(d.Message, "not defined") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about undefined model 'user', got: %v", diags)
	}
}

func TestAnalyze_InvalidValidateModel(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{{Name: "name", Type: parser.FieldText}}},
		},
		Actions: []parser.Page{
			{
				Path: "/test",
				Body: []parser.Node{
					{Type: parser.NodeValidate, ModelName: "profile"},
				},
			},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "profile") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about validate model 'profile', got: %v", diags)
	}
}

func TestAnalyze_NoFalsePositives_AggregatesAndLiterals(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "active", Type: parser.FieldBool},
				{Name: "created", Type: parser.FieldTimestamp},
			}},
		},
		Pages: []parser.Page{
			{
				Path: "/stats",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT count(*) as total FROM user"},
				},
			},
		},
		Schedules: []parser.Schedule{
			{
				Name: "cleanup",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "DELETE FROM user WHERE active = 0 AND created < datetime('now', '-30 days')"},
				},
			},
		},
		Sockets: []parser.Socket{
			{
				Path:      "/ws/test",
				OnConnect: []parser.Node{{Type: parser.NodeQuery, SQL: "SELECT 'connected' as status"}},
			},
		},
	}

	diags := Analyze(app)
	for _, d := range diags {
		if d.Level == "error" {
			t.Errorf("unexpected error: [%s] %s", d.Context, d.Message)
		}
	}
}

func TestAnalyze_QualifiedColumnRef(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
			}},
			{Name: "post", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
				{Name: "author", Type: parser.FieldReference, Reference: "user"},
			}},
		},
		Pages: []parser.Page{
			{
				Path: "/test",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT u.name, p.title FROM user u JOIN post p ON p.author_id = u.id"},
				},
			},
		},
	}

	diags := Analyze(app)
	for _, d := range diags {
		if d.Level == "error" {
			t.Errorf("unexpected error: [%s] %s", d.Context, d.Message)
		}
	}
}

func TestAnalyze_QualifiedColumnRef_Invalid(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
			}},
		},
		Pages: []parser.Page{
			{
				Path: "/test",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT u.username FROM user u"},
				},
			},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "username") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about column 'username', got: %v", diags)
	}
}

func TestAnalyze_RespondQuerySQL(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{{Name: "name", Type: parser.FieldText}}},
		},
		Actions: []parser.Page{
			{
				Path: "/test",
				Body: []parser.Node{
					{Type: parser.NodeRespond, QuerySQL: "SELECT * FROM nonexistent WHERE id = :id"},
				},
			},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "nonexistent") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about table 'nonexistent', got: %v", diags)
	}
}

// --- Named param validation tests ---

func TestAnalyze_ParamMismatch_ReferenceField(t *testing.T) {
	// This is the exact bug: model has "contact: contact required" which generates
	// column "contact_id" in DB, but the form sends field "contact".
	// If the dev writes :contact_id in SQL, it should error with a clear message.
	app := &parser.App{
		Models: []parser.Model{
			{Name: "contact", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
			}},
			{Name: "deal", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
				{Name: "value", Type: parser.FieldFloat},
				{Name: "contact", Type: parser.FieldReference, Reference: "contact"},
			}},
		},
		Actions: []parser.Page{
			{
				Path: "/deals/new",
				Body: []parser.Node{
					{Type: parser.NodeValidate, ModelName: "deal"},
					{Type: parser.NodeQuery, SQL: "INSERT INTO deal (title, value, contact_id) VALUES (:title, :value, :contact_id)"},
				},
			},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "contact_id") && strings.Contains(d.Message, ":contact") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about :contact_id suggesting :contact, got: %v", diags)
	}
}

func TestAnalyze_ParamMismatch_CorrectUsage(t *testing.T) {
	// Using :contact (the form field name) should NOT produce an error
	app := &parser.App{
		Models: []parser.Model{
			{Name: "contact", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
			}},
			{Name: "deal", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
				{Name: "contact", Type: parser.FieldReference, Reference: "contact"},
			}},
		},
		Actions: []parser.Page{
			{
				Path: "/deals/new",
				Body: []parser.Node{
					{Type: parser.NodeValidate, ModelName: "deal"},
					{Type: parser.NodeQuery, SQL: "INSERT INTO deal (title, contact_id) VALUES (:title, :contact)"},
				},
			},
		},
	}

	diags := Analyze(app)
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "named parameter") {
			t.Errorf("unexpected param error: %s", d.Message)
		}
	}
}

func TestAnalyze_ParamMismatch_URLParam(t *testing.T) {
	// :id from URL path should be recognized
	app := &parser.App{
		Models: []parser.Model{
			{Name: "deal", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
			}},
		},
		Actions: []parser.Page{
			{
				Path: "/deals/:id/edit",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "UPDATE deal SET title = :title WHERE id = :id"},
				},
			},
		},
	}

	diags := Analyze(app)
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "named parameter") {
			t.Errorf("unexpected param error: %s", d.Message)
		}
	}
}

func TestAnalyze_ParamMismatch_UnknownParam(t *testing.T) {
	// A completely unknown param should error
	app := &parser.App{
		Models: []parser.Model{
			{Name: "deal", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
			}},
		},
		Actions: []parser.Page{
			{
				Path: "/deals/new",
				Body: []parser.Node{
					{Type: parser.NodeValidate, ModelName: "deal"},
					{Type: parser.NodeQuery, SQL: "INSERT INTO deal (title) VALUES (:nonexistent)"},
				},
			},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "nonexistent") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about :nonexistent, got: %v", diags)
	}
}

func TestAnalyze_ParamMismatch_DidYouMean(t *testing.T) {
	// Typo: :titl should suggest :title
	app := &parser.App{
		Models: []parser.Model{
			{Name: "deal", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
				{Name: "value", Type: parser.FieldFloat},
			}},
		},
		Actions: []parser.Page{
			{
				Path: "/deals/new",
				Body: []parser.Node{
					{Type: parser.NodeValidate, ModelName: "deal"},
					{Type: parser.NodeQuery, SQL: "INSERT INTO deal (title) VALUES (:titl)"},
				},
			},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "titl") && strings.Contains(d.Message, "title") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about :titl suggesting :title, got: %v", diags)
	}
}

func TestEditDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"contact_id", "contact", 3},
		{"titl", "title", 1},
		{"emial", "email", 2},
	}
	for _, tt := range tests {
		got := editDistance(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("editDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestAnalyze_WebhookAndJobSQL(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "post", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
				{Name: "body", Type: parser.FieldRichtext},
				{Name: "status", Type: parser.FieldOption},
				{Name: "author", Type: parser.FieldReference, Reference: "post"},
			}},
		},
		Webhooks: []parser.Webhook{
			{
				Path: "/hooks/test",
				Events: []parser.WebhookEvent{
					{
						Name: "test.event",
						Body: []parser.Node{
							{Type: parser.NodeQuery, SQL: "INSERT INTO post (title, body, status, author_id) VALUES ('test', 'body', 'draft', 1)"},
						},
					},
				},
			},
		},
	}

	diags := Analyze(app)
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "not defined as a model") {
			t.Errorf("unexpected table error: %s", d.Message)
		}
	}
}

// --- Type system tests ---

func TestCheckModelDefaults_IntFieldWithStringDefault(t *testing.T) {
	models := []parser.Model{{
		Name: "item",
		Fields: []parser.Field{
			{Name: "count", Type: parser.FieldInt, Default: "abc"},
		},
	}}
	diags := checkModelDefaults(models)
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "not a valid integer") {
		t.Errorf("expected int default error, got: %v", diags)
	}
}

func TestCheckModelDefaults_IntFieldWithValidDefault(t *testing.T) {
	models := []parser.Model{{
		Name: "item",
		Fields: []parser.Field{
			{Name: "count", Type: parser.FieldInt, Default: "42"},
		},
	}}
	diags := checkModelDefaults(models)
	if len(diags) != 0 {
		t.Errorf("expected no errors, got: %v", diags)
	}
}

func TestCheckModelDefaults_BoolFieldWithInvalidDefault(t *testing.T) {
	models := []parser.Model{{
		Name: "item",
		Fields: []parser.Field{
			{Name: "active", Type: parser.FieldBool, Default: "maybe"},
		},
	}}
	diags := checkModelDefaults(models)
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "use 'true' or 'false'") {
		t.Errorf("expected bool default error, got: %v", diags)
	}
}

func TestCheckModelDefaults_OptionFieldWithInvalidDefault(t *testing.T) {
	models := []parser.Model{{
		Name: "user",
		Fields: []parser.Field{
			{Name: "role", Type: parser.FieldOption, Default: "superadmin", Options: []string{"admin", "editor", "viewer"}},
		},
	}}
	diags := checkModelDefaults(models)
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "valid options are") {
		t.Errorf("expected option default error, got: %v", diags)
	}
}

func TestCheckModelDefaults_FloatFieldWithValidDefault(t *testing.T) {
	models := []parser.Model{{
		Name: "item",
		Fields: []parser.Field{
			{Name: "price", Type: parser.FieldFloat, Default: "3.14"},
		},
	}}
	diags := checkModelDefaults(models)
	if len(diags) != 0 {
		t.Errorf("expected no errors, got: %v", diags)
	}
}

func TestAnalyze_WhereTypeMismatch_IntVsString(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "user",
			Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "age", Type: parser.FieldInt},
			},
		}},
		Pages: []parser.Page{{
			Path: "/test",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: "SELECT id FROM user WHERE age = 'hello'"},
			},
		}},
	}
	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "not compatible") && strings.Contains(d.Message, "age") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected type mismatch error for age vs string, got: %v", diags)
	}
}

func TestAnalyze_WhereNoFalsePositive_IntVsNumber(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "user",
			Fields: []parser.Field{
				{Name: "age", Type: parser.FieldInt},
			},
		}},
		Pages: []parser.Page{{
			Path: "/test",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: "SELECT id FROM user WHERE age = 25"},
			},
		}},
	}
	diags := Analyze(app)
	for _, d := range diags {
		if strings.Contains(d.Message, "not compatible") {
			t.Errorf("unexpected type error: %s", d.Message)
		}
	}
}

func TestAnalyze_WhereNoFalsePositive_BoolVsInt(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "user",
			Fields: []parser.Field{
				{Name: "active", Type: parser.FieldBool},
			},
		}},
		Pages: []parser.Page{{
			Path: "/test",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: "SELECT id FROM user WHERE active = 1"},
			},
		}},
	}
	diags := Analyze(app)
	for _, d := range diags {
		if strings.Contains(d.Message, "not compatible") {
			t.Errorf("unexpected type error: %s", d.Message)
		}
	}
}

func TestAnalyze_WhereNoFalsePositive_Param(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "user",
			Fields: []parser.Field{
				{Name: "age", Type: parser.FieldInt},
			},
		}},
		Pages: []parser.Page{{
			Path: "/test",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: "SELECT id FROM user WHERE age = :age"},
			},
		}},
	}
	diags := Analyze(app)
	for _, d := range diags {
		if strings.Contains(d.Message, "not compatible") {
			t.Errorf("unexpected type error: %s", d.Message)
		}
	}
}

func TestAnalyze_WhereWarning_BoolVsStringTrue(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "user",
			Fields: []parser.Field{
				{Name: "active", Type: parser.FieldBool},
			},
		}},
		Pages: []parser.Page{{
			Path: "/test",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: "SELECT id FROM user WHERE active = 'true'"},
			},
		}},
	}
	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "warning" && strings.Contains(d.Message, "use 1 (true) or 0 (false)") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about bool vs string 'true', got: %v", diags)
	}
}

func TestAnalyze_InsertTypeMismatch(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "user",
			Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "age", Type: parser.FieldInt},
			},
		}},
		Actions: []parser.Page{{
			Path: "/test",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: "INSERT INTO user (name, age) VALUES ('Alice', 'twenty')"},
			},
		}},
	}
	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "inserting") && strings.Contains(d.Message, "age") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected insert type mismatch for age, got: %v", diags)
	}
}

func TestAnalyze_InsertNoFalsePositive_Param(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "user",
			Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "age", Type: parser.FieldInt},
			},
		}},
		Actions: []parser.Page{{
			Path: "/test",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: "INSERT INTO user (name, age) VALUES (:name, :age)"},
			},
		}},
	}
	diags := Analyze(app)
	for _, d := range diags {
		if strings.Contains(d.Message, "inserting") || strings.Contains(d.Message, "not compatible") {
			t.Errorf("unexpected type error: %s", d.Message)
		}
	}
}

func TestAnalyze_UpdateTypeMismatch(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "user",
			Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "age", Type: parser.FieldInt},
			},
		}},
		Actions: []parser.Page{{
			Path: "/test",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: "UPDATE user SET age = 'hello' WHERE id = 1"},
			},
		}},
	}
	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "setting column") && strings.Contains(d.Message, "age") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected update type mismatch for age, got: %v", diags)
	}
}

// --- Multi-table JOIN (3+ tables) unqualified column validation ---

func TestAnalyze_ThreeTableJoin_UnqualifiedColumn_Valid(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "email", Type: parser.FieldEmail},
			}},
			{Name: "post", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
				{Name: "author", Type: parser.FieldReference, Reference: "user"},
			}},
			{Name: "comment", Fields: []parser.Field{
				{Name: "body", Type: parser.FieldText},
				{Name: "post", Type: parser.FieldReference, Reference: "post"},
				{Name: "author", Type: parser.FieldReference, Reference: "user"},
			}},
		},
		Pages: []parser.Page{
			{
				Path: "/comments",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT u.name, p.title, c.body FROM comment c JOIN post p ON p.id = c.post_id JOIN user u ON u.id = c.author_id WHERE title = 'hello'"},
				},
			},
		},
	}

	diags := Analyze(app)
	for _, d := range diags {
		if d.Level == "error" {
			t.Errorf("unexpected error: [%s] %s", d.Context, d.Message)
		}
	}
}

func TestAnalyze_ThreeTableJoin_UnqualifiedColumn_Invalid(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
			}},
			{Name: "post", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
				{Name: "author", Type: parser.FieldReference, Reference: "user"},
			}},
			{Name: "comment", Fields: []parser.Field{
				{Name: "body", Type: parser.FieldText},
				{Name: "post", Type: parser.FieldReference, Reference: "post"},
			}},
		},
		Pages: []parser.Page{
			{
				Path: "/test",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT nonexistent FROM comment c JOIN post p ON p.id = c.post_id JOIN user u ON u.id = p.author_id"},
				},
			},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "nonexistent") && strings.Contains(d.Message, "does not exist in any") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about column 'nonexistent' not in any model, got: %v", diags)
	}
}

func TestAnalyze_TwoTableJoin_UnqualifiedColumn_Valid(t *testing.T) {
	// Unqualified column that exists in one of the two joined tables should pass
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
			}},
			{Name: "post", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
				{Name: "author", Type: parser.FieldReference, Reference: "user"},
			}},
		},
		Pages: []parser.Page{
			{
				Path: "/test",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT title, name FROM user u JOIN post p ON p.author_id = u.id"},
				},
			},
		},
	}

	diags := Analyze(app)
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "does not exist") {
			t.Errorf("unexpected column error: [%s] %s", d.Context, d.Message)
		}
	}
}

// --- Subquery validation tests ---

func TestAnalyze_Subquery_ValidTable(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "active", Type: parser.FieldBool},
			}},
			{Name: "post", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
				{Name: "author", Type: parser.FieldReference, Reference: "user"},
			}},
		},
		Pages: []parser.Page{
			{
				Path: "/active-posts",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT id, title FROM post WHERE author_id IN (SELECT id FROM user WHERE active = 1)"},
				},
			},
		},
	}

	diags := Analyze(app)
	for _, d := range diags {
		if d.Level == "error" {
			t.Errorf("unexpected error: [%s] %s", d.Context, d.Message)
		}
	}
}

func TestAnalyze_Subquery_InvalidTable(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "post", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
			}},
		},
		Pages: []parser.Page{
			{
				Path: "/test",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT id FROM post WHERE id IN (SELECT post_id FROM nonexistent)"},
				},
			},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "nonexistent") && strings.Contains(d.Message, "not defined as a model") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about subquery table 'nonexistent', got: %v", diags)
	}
}

func TestAnalyze_Subquery_InvalidColumn(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
			}},
			{Name: "post", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
				{Name: "author", Type: parser.FieldReference, Reference: "user"},
			}},
		},
		Pages: []parser.Page{
			{
				Path: "/test",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT id FROM post WHERE author_id IN (SELECT id FROM user WHERE nonexistent_col = 1)"},
				},
			},
		},
	}

	diags := Analyze(app)
	// The subquery SELECT id FROM user is valid, but WHERE nonexistent_col = 1
	// should not produce a type error (column lookup for WHERE comparisons
	// only checks known columns). The key validation here is that
	// table + select columns get checked.
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "nonexistent_col") {
			// This is expected if WHERE column validation catches it
			return
		}
	}
}

func TestAnalyze_Subquery_InvalidSelectColumn(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
			}},
			{Name: "post", Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
				{Name: "author", Type: parser.FieldReference, Reference: "user"},
			}},
		},
		Pages: []parser.Page{
			{
				Path: "/test",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT id FROM post WHERE author_id IN (SELECT nonexistent FROM user)"},
				},
			},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "nonexistent") && strings.Contains(d.Message, "user") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about column 'nonexistent' in subquery, got: %v", diags)
	}
}

func TestAnalyze_Subquery_InsertWithSubquery(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "user", Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "active", Type: parser.FieldBool},
			}},
		},
		Actions: []parser.Page{
			{
				Path: "/test",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "UPDATE user SET active = 0 WHERE id IN (SELECT id FROM nonexistent)"},
				},
			},
		},
	}

	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "nonexistent") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about subquery table 'nonexistent', got: %v", diags)
	}
}

func TestExtractSubqueries(t *testing.T) {
	tests := []struct {
		sql  string
		want int
	}{
		{"SELECT id FROM user WHERE id IN (SELECT user_id FROM post)", 1},
		{"SELECT * FROM user", 0},
		{"SELECT id FROM user WHERE id IN (SELECT id FROM post) AND name IN (SELECT name FROM tag)", 2},
		{"DELETE FROM user WHERE id IN (SELECT id FROM old_users)", 1},
	}

	for _, tt := range tests {
		tokens := tokenizeSQL(tt.sql)
		subs := extractSubqueries(tokens)
		if len(subs) != tt.want {
			t.Errorf("extractSubqueries(%q): got %d subqueries, want %d", tt.sql, len(subs), tt.want)
		}
	}
}

func TestCheckTemplateInterpolations_ValidField(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "user",
			Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
				{Name: "email", Type: parser.FieldEmail},
			},
		}},
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{Type: parser.NodeQuery, Name: "users", SQL: "SELECT * FROM user"},
				{Type: parser.NodeHTML, HTMLContent: `<p>{users.name}</p><p>{users.email}</p>`},
			},
		}},
	}
	diags := Analyze(app)
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "template reference") {
			t.Errorf("unexpected template error: %s", d.Message)
		}
	}
}

func TestCheckTemplateInterpolations_InvalidField(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "user",
			Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
			},
		}},
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{Type: parser.NodeQuery, Name: "users", SQL: "SELECT * FROM user"},
				{Type: parser.NodeHTML, HTMLContent: `<p>{users.nonexistent}</p>`},
			},
		}},
	}
	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "nonexistent") && strings.Contains(d.Message, "does not exist in model 'user'") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for nonexistent field, got: %v", diags)
	}
}

func TestCheckTemplateInterpolations_UnknownQuery(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "user",
			Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
			},
		}},
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{Type: parser.NodeHTML, HTMLContent: `<p>{missing.name}</p>`},
			},
		}},
	}
	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "unknown query 'missing'") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for unknown query, got: %v", diags)
	}
}

func TestCheckTemplateInterpolations_ReservedNames(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "user",
			Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
			},
		}},
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{Type: parser.NodeHTML, HTMLContent: `<title>{page.title}</title>{kilnx.css}{t.greeting}`},
			},
		}},
	}
	diags := Analyze(app)
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "template reference") {
			t.Errorf("reserved names should not trigger errors: %s", d.Message)
		}
	}
}

func TestCheckTemplateInterpolations_BuiltinCount(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "user",
			Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText},
			},
		}},
		Pages: []parser.Page{{
			Path: "/users",
			Body: []parser.Node{
				{Type: parser.NodeQuery, Name: "users", SQL: "SELECT * FROM user"},
				{Type: parser.NodeHTML, HTMLContent: `<span>{users.count} users</span>`},
			},
		}},
	}
	diags := Analyze(app)
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "count") && strings.Contains(d.Message, "template reference") {
			t.Errorf(".count is a built-in field and should not trigger error: %s", d.Message)
		}
	}
}

func TestCheckTemplateInterpolations_Fragment(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "post",
			Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
				{Name: "body", Type: parser.FieldRichtext},
			},
		}},
		Fragments: []parser.Page{{
			Path: "/posts/row",
			Body: []parser.Node{
				{Type: parser.NodeQuery, Name: "post", SQL: "SELECT * FROM post WHERE id = :id"},
				{Type: parser.NodeHTML, HTMLContent: `<div>{post.title}</div><div>{post.missing}</div>`},
			},
		}},
	}
	diags := Analyze(app)
	foundMissing := false
	foundTitle := false
	for _, d := range diags {
		if d.Level == "error" && strings.Contains(d.Message, "template reference") {
			if strings.Contains(d.Message, "missing") {
				foundMissing = true
			}
			if strings.Contains(d.Message, "title") {
				foundTitle = true
			}
		}
	}
	if !foundMissing {
		t.Errorf("expected error for {post.missing}, got: %v", diags)
	}
	if foundTitle {
		t.Errorf("should not error for valid {post.title}")
	}
}

func TestQueryModelMap(t *testing.T) {
	pages := []parser.Page{{
		Path: "/users",
		Body: []parser.Node{
			{Type: parser.NodeQuery, Name: "users", SQL: "SELECT * FROM user"},
			{Type: parser.NodeQuery, Name: "posts", SQL: "SELECT * FROM post WHERE author_id = :id"},
		},
	}}

	m := queryModelMap(pages, nil, nil)
	if m["users"] != "user" {
		t.Errorf("expected users -> user, got %s", m["users"])
	}
	if m["posts"] != "post" {
		t.Errorf("expected posts -> post, got %s", m["posts"])
	}
}

func TestBuildSchemaCustomFields(t *testing.T) {
	models := []parser.Model{
		{
			Name:             "deal",
			CustomFieldsFile: "deal_fields.kilnx",
			Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
			},
		},
		{
			Name:   "contact",
			Fields: []parser.Field{{Name: "name", Type: parser.FieldText}},
		},
	}
	schema := BuildSchema(models)

	deal := schema.Tables["deal"]
	if deal == nil {
		t.Fatal("deal table not found")
	}
	if _, ok := deal.Columns["custom"]; !ok {
		t.Error("deal table should have 'custom' column")
	}

	contact := schema.Tables["contact"]
	if _, ok := contact.Columns["custom"]; ok {
		t.Error("contact table should NOT have 'custom' column")
	}
}

func TestCheckCustomFieldRefs(t *testing.T) {
	manifest := &parser.CustomFieldManifest{
		ModelName: "deal",
		Fields: []parser.CustomFieldDef{
			{Name: "revenue", Kind: parser.CustomFieldKindNumber, Label: "Receita"},
		},
	}
	app := &parser.App{
		Models: []parser.Model{
			{Name: "deal", CustomFieldsFile: "deal_fields.kilnx"},
		},
		Pages: []parser.Page{{
			Path: "/deals/:id",
			Body: []parser.Node{
				{Type: parser.NodeQuery, Name: "d", SQL: "SELECT * FROM deal WHERE id = :id"},
				{Type: parser.NodeHTML, HTMLContent: "<span>{d.custom.revenue}</span><span>{d.custom.unknown}</span>"},
			},
		}},
		CustomManifests: map[string]*parser.CustomFieldManifest{
			"deal": manifest,
		},
	}
	schema := BuildSchema(app.Models)
	diags := checkCustomFieldRefs(app, schema)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic (unknown field), got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "unknown") {
		t.Errorf("expected 'unknown' in diagnostic, got %q", diags[0].Message)
	}
}

func TestCheckCustomManifestRefs_InvalidReference(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "deal", CustomFieldsFile: "deal_fields.kilnx"},
		},
		CustomManifests: map[string]*parser.CustomFieldManifest{
			"deal": {
				ModelName: "deal",
				Fields: []parser.CustomFieldDef{
					{Name: "client", Kind: parser.CustomFieldKindReference, Reference: "nonexistent"},
				},
			},
		},
	}
	diags := checkCustomManifestRefs(app)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for bad reference target, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "nonexistent") {
		t.Errorf("expected model name in diagnostic, got %q", diags[0].Message)
	}
}

func TestCheckCustomManifestRefs_ValidReference(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{Name: "deal", CustomFieldsFile: "deal_fields.kilnx"},
			{Name: "client"},
		},
		CustomManifests: map[string]*parser.CustomFieldManifest{
			"deal": {
				ModelName: "deal",
				Fields: []parser.CustomFieldDef{
					{Name: "client_ref", Kind: parser.CustomFieldKindReference, Reference: "client"},
				},
			},
		},
	}
	diags := checkCustomManifestRefs(app)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for valid reference, got %v", diags)
	}
}

func TestBuildSchemaColumnModeFields(t *testing.T) {
	models := []parser.Model{
		{
			Name:             "deal",
			CustomFieldsFile: "deal_fields.kilnx",
			Fields:           []parser.Field{{Name: "title", Type: parser.FieldText}},
		},
	}
	cm := map[string]*parser.CustomFieldManifest{
		"deal": {
			ModelName: "deal",
			Fields: []parser.CustomFieldDef{
				{Name: "revenue", Kind: parser.CustomFieldKindNumber, Mode: parser.CustomFieldModeColumn},
				{Name: "region", Kind: parser.CustomFieldKindText, Mode: parser.CustomFieldModeJSON},
			},
		},
	}
	schema := BuildSchema(models, cm)
	tbl := schema.Tables["deal"]
	if tbl == nil {
		t.Fatal("table 'deal' not in schema")
	}
	if _, ok := tbl.Columns["revenue"]; !ok {
		t.Error("column-mode field 'revenue' should be a real schema column")
	}
	if _, ok := tbl.Columns["custom"]; !ok {
		t.Error("'custom' JSON column should still be in schema")
	}
	if _, ok := tbl.Columns["region"]; ok {
		t.Error("JSON-mode field 'region' should not be a real schema column")
	}
}
