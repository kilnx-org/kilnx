package runtime

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestAuthOverride_RegisterGETUsesUserPage asserts that declaring a
// `page /register` in the app overrides the built-in dark auth UI on
// GET requests while the POST flow still hits the built-in handler.
func TestAuthOverride_RegisterGETUsesUserPage(t *testing.T) {
	src := `
config
  secret: "test-secret-32-bytes-min-len-padding"

model user
  name: text required
  email: email unique
  password: password required

auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /home

page /register
  html
    <div class="custom-register-marker">Custom Register UI</div>

page /home requires auth
  html
    welcome
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	_, body := httpGet(t, baseURL+"/register")
	if !strings.Contains(body, "custom-register-marker") {
		t.Fatalf("GET /register should render user page, got:\n%s", body)
	}
	if strings.Contains(body, "kilnx-auth-card") {
		t.Errorf("GET /register should NOT render the built-in kilnx-auth-card when a page is declared; body:\n%s", body)
	}
}

// TestAuthOverride_RegisterPOSTStaysBuiltin asserts that POST still hits
// the built-in handler even when a custom page is declared, so bcrypt
// hashing and session creation keep working.
func TestAuthOverride_RegisterPOSTStaysBuiltin(t *testing.T) {
	src := `
config
  secret: "test-secret-32-bytes-min-len-padding"

model user
  name: text required
  email: email unique
  password: password required

auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /home

page /register
  html
    <p>custom</p>

page /home requires auth
  html
    home
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	// POST without CSRF must be rejected by the built-in handler (HTTP 403).
	form := url.Values{}
	form.Set("name", "Alice")
	form.Set("identity", "alice@test.com")
	form.Set("password", "supersecret")
	resp, err := http.Post(baseURL+"/register", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("POST /register: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 (built-in CSRF check) for POST /register without token, got %d", resp.StatusCode)
	}
}

// TestAuthOverride_LoginPathHonorsOverride uses a non-default LoginPath
// (/entrar) to prove the dispatcher reads app.Auth.LoginPath dynamically
// rather than a hardcoded /login string.
func TestAuthOverride_LoginPathHonorsOverride(t *testing.T) {
	src := `
config
  secret: "test-secret-32-bytes-min-len-padding"

model user
  name: text required
  email: email unique
  password: password required

auth
  table: user
  identity: email
  password: password
  login: /entrar
  after login: /home

page /entrar
  html
    <div class="custom-login-marker">Custom Login</div>

page /home requires auth
  html
    home
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	_, body := httpGet(t, baseURL+"/entrar")
	if !strings.Contains(body, "custom-login-marker") {
		t.Fatalf("GET /entrar should render user page when declared, got:\n%s", body)
	}
}

// TestAuthOverride_CustomLoginPathPOSTStaysBuiltin verifies POST on a
// non-default LoginPath still goes to the built-in handler so bcrypt
// comparison and session issuance keep working.
func TestAuthOverride_CustomLoginPathPOSTStaysBuiltin(t *testing.T) {
	src := `
config
  secret: "test-secret-32-bytes-min-len-padding"

model user
  name: text required
  email: email unique
  password: password required

auth
  table: user
  identity: email
  password: password
  login: /entrar
  after login: /home

page /entrar
  html
    custom

page /home requires auth
  html
    home
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	form := url.Values{}
	form.Set("identity", "alice@test.com")
	form.Set("password", "supersecret")
	resp, err := http.Post(baseURL+"/entrar", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("POST /entrar: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 (built-in CSRF check) for POST /entrar without token, got %d", resp.StatusCode)
	}
}

// TestAuthOverride_LogoutGETRendersUserPage covers the /logout override.
// GET goes to the user page (confirmation dialog) when declared; POST
// always invalidates the session via the built-in handler.
func TestAuthOverride_LogoutGETRendersUserPage(t *testing.T) {
	src := `
config
  secret: "test-secret-32-bytes-min-len-padding"

model user
  name: text required
  email: email unique
  password: password required

auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /home

page /logout
  html
    <div class="custom-logout-marker">Confirmar saida</div>

page /home requires auth
  html
    home
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	_, body := httpGet(t, baseURL+"/logout")
	if !strings.Contains(body, "custom-logout-marker") {
		t.Fatalf("GET /logout should render user page when declared, got:\n%s", body)
	}
}

// TestAuthOverride_CustomSlugs proves all four auth paths are
// configurable in the auth block. Apps that want Portuguese (or any
// other language) slugs declare them via `register:`, `forgot:`,
// `reset:` and the runtime + analyzer honor them end to end.
func TestAuthOverride_CustomSlugs(t *testing.T) {
	src := `
config
  secret: "test-secret-32-bytes-min-len-padding"

model user
  name: text required
  email: email unique
  password: password required

auth
  table: user
  identity: email
  password: password
  login: /entrar
  register: /cadastrar
  forgot: /senha/esqueci
  reset: /senha/redefinir
  after login: /inicio

page /entrar
  html
    <div class="marker-entrar">Entrar</div>

page /cadastrar
  html
    <div class="marker-cadastrar">Cadastrar</div>

page /senha/esqueci
  html
    <div class="marker-esqueci">Esqueci</div>

page /senha/redefinir
  html
    <div class="marker-redefinir">Redefinir</div>

page /inicio requires auth
  html
    inicio
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	cases := []struct{ path, marker string }{
		{"/entrar", "marker-entrar"},
		{"/cadastrar", "marker-cadastrar"},
		{"/senha/esqueci", "marker-esqueci"},
		{"/senha/redefinir", "marker-redefinir"},
	}
	for _, c := range cases {
		_, body := httpGet(t, baseURL+c.path)
		if !strings.Contains(body, c.marker) {
			t.Errorf("GET %s should render user page (expected %q), got:\n%s", c.path, c.marker, body)
		}
	}

	// POST on a custom slug still routes to the built-in handler
	// (CSRF-less POST expected to fail with 403).
	form := url.Values{}
	form.Set("name", "Alice")
	form.Set("identity", "alice@test.com")
	form.Set("password", "supersecret")
	resp, err := http.Post(baseURL+"/cadastrar", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("POST /cadastrar: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("POST /cadastrar without CSRF should hit built-in handler (expect 403), got %d", resp.StatusCode)
	}
}

// TestAuthOverride_ForgotAndResetHonorOverride covers the remaining
// auth routes behind the same override rule.
func TestAuthOverride_ForgotAndResetHonorOverride(t *testing.T) {
	src := `
config
  secret: "test-secret-32-bytes-min-len-padding"

model user
  name: text required
  email: email unique
  password: password required

auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /home

page /forgot-password
  html
    <div class="custom-forgot-marker">Forgot</div>

page /reset-password
  html
    <div class="custom-reset-marker">Reset</div>

page /home requires auth
  html
    home
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	for _, p := range []struct{ path, marker string }{
		{"/forgot-password", "custom-forgot-marker"},
		{"/reset-password", "custom-reset-marker"},
	} {
		_, body := httpGet(t, baseURL+p.path)
		if !strings.Contains(body, p.marker) {
			t.Errorf("GET %s should render user page (expected marker %q), got:\n%s", p.path, p.marker, body)
		}
	}
}
