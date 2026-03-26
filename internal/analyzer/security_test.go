package analyzer

import (
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestCheckUnauthActions(t *testing.T) {
	app := &parser.App{
		Actions: []parser.Page{
			{Path: "/delete-user", Method: "POST"},
			{Path: "/admin/delete", Method: "DELETE", Auth: true},
			{Path: "/edit-post", Method: "PUT", RequiresRole: "editor"},
		},
	}
	diags := checkUnauthActions(app)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if !strings.Contains(diags[0].Context, "/delete-user") {
		t.Errorf("expected context to mention /delete-user, got %q", diags[0].Context)
	}
	if diags[0].Level != "warning" {
		t.Errorf("expected warning, got %q", diags[0].Level)
	}
}

func TestCheckUnauthActions_NoActions(t *testing.T) {
	app := &parser.App{}
	diags := checkUnauthActions(app)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d", len(diags))
	}
}

func TestCheckUnauthAPIs_WithMutatingSQL(t *testing.T) {
	app := &parser.App{
		APIs: []parser.Page{
			{
				Path: "/api/users",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "INSERT INTO user (name) VALUES (:name)"},
				},
			},
		},
	}
	diags := checkUnauthAPIs(app)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "write queries") {
		t.Errorf("expected message about write queries, got %q", diags[0].Message)
	}
}

func TestCheckUnauthAPIs_ReadOnly(t *testing.T) {
	app := &parser.App{
		APIs: []parser.Page{
			{
				Path: "/api/users",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT * FROM user"},
				},
			},
		},
	}
	diags := checkUnauthAPIs(app)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for read-only API, got %d", len(diags))
	}
}

func TestCheckUnauthAPIs_WithAuth(t *testing.T) {
	app := &parser.App{
		APIs: []parser.Page{
			{
				Path: "/api/users",
				Auth: true,
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "DELETE FROM user WHERE id = :id"},
				},
			},
		},
	}
	diags := checkUnauthAPIs(app)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for authed API, got %d", len(diags))
	}
}

func TestCheckUnauthStreams(t *testing.T) {
	app := &parser.App{
		Streams: []parser.Stream{
			{Path: "/stream/orders", SQL: "SELECT * FROM orders"},
			{Path: "/stream/public", SQL: "SELECT * FROM news", Auth: true},
		},
	}
	diags := checkUnauthStreams(app)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Context, "/stream/orders") {
		t.Errorf("expected context about /stream/orders, got %q", diags[0].Context)
	}
}

func TestCheckUnauthSockets_WithMutatingOnMessage(t *testing.T) {
	app := &parser.App{
		Sockets: []parser.Socket{
			{
				Path: "/ws/chat",
				OnMessage: []parser.Node{
					{Type: parser.NodeQuery, SQL: "INSERT INTO message (text) VALUES (:text)"},
				},
			},
		},
	}
	diags := checkUnauthSockets(app)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
}

func TestCheckUnauthSockets_ReadOnly(t *testing.T) {
	app := &parser.App{
		Sockets: []parser.Socket{
			{
				Path: "/ws/updates",
				OnMessage: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT * FROM updates"},
				},
			},
		},
	}
	diags := checkUnauthSockets(app)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for read-only socket, got %d", len(diags))
	}
}

func TestCheckWebhookSecrets(t *testing.T) {
	app := &parser.App{
		Webhooks: []parser.Webhook{
			{Path: "/webhook/stripe"},
			{Path: "/webhook/github", SecretEnv: "GITHUB_SECRET"},
		},
	}
	diags := checkWebhookSecrets(app)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Context, "/webhook/stripe") {
		t.Errorf("expected context about /webhook/stripe, got %q", diags[0].Context)
	}
}

func TestCheckPasswordExposure_SelectStar(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{
				Name: "user",
				Fields: []parser.Field{
					{Name: "name", Type: parser.FieldText},
					{Name: "password", Type: parser.FieldPassword},
				},
			},
		},
		Pages: []parser.Page{
			{
				Path: "/users",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT * FROM user"},
				},
			},
		},
	}
	schema := BuildSchema(app.Models)
	diags := checkPasswordExposure(app, schema)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if !strings.Contains(diags[0].Message, "SELECT *") {
		t.Errorf("expected message about SELECT *, got %q", diags[0].Message)
	}
}

func TestCheckPasswordExposure_ExplicitPasswordColumn(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{
				Name: "user",
				Fields: []parser.Field{
					{Name: "name", Type: parser.FieldText},
					{Name: "password", Type: parser.FieldPassword},
				},
			},
		},
		APIs: []parser.Page{
			{
				Path: "/api/users",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT name, password FROM user"},
				},
			},
		},
	}
	schema := BuildSchema(app.Models)
	diags := checkPasswordExposure(app, schema)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "password") {
		t.Errorf("expected message about password, got %q", diags[0].Message)
	}
}

func TestCheckPasswordExposure_SafeSelect(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{
				Name: "user",
				Fields: []parser.Field{
					{Name: "name", Type: parser.FieldText},
					{Name: "password", Type: parser.FieldPassword},
				},
			},
		},
		Pages: []parser.Page{
			{
				Path: "/users",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT name FROM user"},
				},
			},
		},
	}
	schema := BuildSchema(app.Models)
	diags := checkPasswordExposure(app, schema)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for safe select, got %d: %v", len(diags), diags)
	}
}

func TestCheckPasswordExposure_NoPasswordModel(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{
				Name: "post",
				Fields: []parser.Field{
					{Name: "title", Type: parser.FieldText},
				},
			},
		},
		Pages: []parser.Page{
			{
				Path: "/posts",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT * FROM post"},
				},
			},
		},
	}
	schema := BuildSchema(app.Models)
	diags := checkPasswordExposure(app, schema)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics when no password fields exist, got %d", len(diags))
	}
}

func TestCheckPasswordExposure_Fragment(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{
				Name: "user",
				Fields: []parser.Field{
					{Name: "name", Type: parser.FieldText},
					{Name: "password", Type: parser.FieldPassword},
				},
			},
		},
		Fragments: []parser.Page{
			{
				Path: "/fragment/user-list",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT * FROM user"},
				},
			},
		},
	}
	schema := BuildSchema(app.Models)
	diags := checkPasswordExposure(app, schema)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for fragment, got %d", len(diags))
	}
}

func TestCheckAuthWithoutPermissions(t *testing.T) {
	app := &parser.App{
		Auth: &parser.AuthConfig{Table: "user"},
	}
	diags := checkAuthWithoutPermissions(app)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "no permissions") {
		t.Errorf("expected message about no permissions, got %q", diags[0].Message)
	}
}

func TestCheckAuthWithoutPermissions_WithPermissions(t *testing.T) {
	app := &parser.App{
		Auth: &parser.AuthConfig{Table: "user"},
		Permissions: []parser.Permission{
			{Role: "admin", Rules: []string{"all"}},
		},
	}
	diags := checkAuthWithoutPermissions(app)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics when permissions exist, got %d", len(diags))
	}
}

func TestCheckAuthWithoutPermissions_NoAuth(t *testing.T) {
	app := &parser.App{}
	diags := checkAuthWithoutPermissions(app)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics when no auth, got %d", len(diags))
	}
}

func TestCheckSecurity_Integration(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{
			{
				Name: "user",
				Fields: []parser.Field{
					{Name: "name", Type: parser.FieldText},
					{Name: "email", Type: parser.FieldEmail},
					{Name: "password", Type: parser.FieldPassword},
					{Name: "role", Type: parser.FieldOption},
				},
			},
		},
		Auth: &parser.AuthConfig{Table: "user"},
		Pages: []parser.Page{
			{
				Path: "/admin/users",
				Auth: true,
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT * FROM user"},
				},
			},
		},
		Actions: []parser.Page{
			{Path: "/users/delete", Method: "POST", Auth: true},
			{Path: "/public/submit", Method: "POST"},
		},
		Webhooks: []parser.Webhook{
			{Path: "/webhook/stripe", SecretEnv: "STRIPE_SECRET"},
			{Path: "/webhook/test"},
		},
	}

	schema := BuildSchema(app.Models)
	diags := checkSecurity(app, schema)

	// Expect:
	// 1. SELECT * from user exposes password (page /admin/users)
	// 2. action /public/submit has no auth
	// 3. webhook /webhook/test has no secret
	// 4. auth without permissions
	if len(diags) != 4 {
		t.Fatalf("expected 4 diagnostics, got %d:", len(diags))
	}

	var contexts []string
	for _, d := range diags {
		contexts = append(contexts, d.Context)
	}

	found := map[string]bool{}
	for _, d := range diags {
		switch {
		case strings.Contains(d.Message, "SELECT *"):
			found["password_exposure"] = true
		case strings.Contains(d.Message, "no authentication"):
			found["unauth_action"] = true
		case strings.Contains(d.Message, "no secret"):
			found["webhook_secret"] = true
		case strings.Contains(d.Message, "no permissions"):
			found["no_permissions"] = true
		}
	}

	for _, key := range []string{"password_exposure", "unauth_action", "webhook_secret", "no_permissions"} {
		if !found[key] {
			t.Errorf("missing expected diagnostic: %s. Got: %v", key, diags)
		}
	}
}

func TestHasMutatingSQL(t *testing.T) {
	tests := []struct {
		nodes []parser.Node
		want  bool
	}{
		{
			nodes: []parser.Node{{Type: parser.NodeQuery, SQL: "SELECT * FROM user"}},
			want:  false,
		},
		{
			nodes: []parser.Node{{Type: parser.NodeQuery, SQL: "INSERT INTO user (name) VALUES ('test')"}},
			want:  true,
		},
		{
			nodes: []parser.Node{{Type: parser.NodeQuery, SQL: "UPDATE user SET name = 'test'"}},
			want:  true,
		},
		{
			nodes: []parser.Node{{Type: parser.NodeQuery, SQL: "DELETE FROM user WHERE id = 1"}},
			want:  true,
		},
		{
			nodes: []parser.Node{
				{Type: parser.NodeOn, Children: []parser.Node{
					{Type: parser.NodeQuery, SQL: "INSERT INTO log (msg) VALUES ('test')"},
				}},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		got := hasMutatingSQL(tt.nodes)
		if got != tt.want {
			t.Errorf("hasMutatingSQL(%v) = %v, want %v", tt.nodes, got, tt.want)
		}
	}
}
