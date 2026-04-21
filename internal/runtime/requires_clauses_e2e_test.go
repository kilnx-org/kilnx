package runtime

import (
	"io"
	"net/http"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
)

const requiresClausesAppSrc = `
config
  secret: "test-secret-32-bytes-min-len-padding"

model user
  email: email unique
  password: password required
  role: option [admin, editor, viewer] default viewer
  plan: text

auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /dashboard
  superuser: ops@example.com

page /dashboard requires auth
  html
    <div id="dashboard">welcome</div>

page /admin requires admin
  html
    <div id="admin">admin area</div>

page /ops requires superuser
  html
    <div id="ops">ops area</div>

page /pro requires auth, :current_user.plan in ['cad','full']
  html
    <div id="pro">pro area</div>

page /combo requires admin, :current_user.plan in ['full']
  html
    <div id="combo">combo area</div>
`

func makeUserRow(email, role, plan string) database.Row {
	return database.Row{
		"id":    "1",
		"email": email,
		"role":  role,
		"plan":  plan,
	}
}

func getWithSession(t *testing.T, srv *Server, baseURL, path string, user database.Row) (int, string) {
	t.Helper()
	sessionID := srv.sessions.Create(user, "email")
	signed := srv.sessions.signSessionID(sessionID)

	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: signed})
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestRequiresClauses_AuthPass(t *testing.T) {
	baseURL, _, srv, cleanup := startTestServerWithHandles(t, requiresClausesAppSrc)
	defer cleanup()

	code, _ := getWithSession(t, srv, baseURL, "/dashboard", makeUserRow("u@e.com", "viewer", ""))
	if code != 200 {
		t.Errorf("requires auth: logged-in viewer should get 200, got %d", code)
	}
}

func TestRequiresClauses_AuthFailNoSession(t *testing.T) {
	baseURL, _, _, cleanup := startTestServerWithHandles(t, requiresClausesAppSrc)
	defer cleanup()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(baseURL + "/dashboard")
	if err != nil {
		t.Fatalf("GET /dashboard: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 302 && resp.StatusCode != 303 {
		t.Errorf("unauthenticated should redirect (3xx), got %d", resp.StatusCode)
	}
}

func TestRequiresClauses_RoleAdminPass(t *testing.T) {
	baseURL, _, srv, cleanup := startTestServerWithHandles(t, requiresClausesAppSrc)
	defer cleanup()

	code, _ := getWithSession(t, srv, baseURL, "/admin", makeUserRow("a@e.com", "admin", ""))
	if code != 200 {
		t.Errorf("admin should access /admin, got %d", code)
	}
}

func TestRequiresClauses_RoleAdminFailViewer(t *testing.T) {
	baseURL, _, srv, cleanup := startTestServerWithHandles(t, requiresClausesAppSrc)
	defer cleanup()

	code, _ := getWithSession(t, srv, baseURL, "/admin", makeUserRow("v@e.com", "viewer", ""))
	if code != 403 {
		t.Errorf("viewer should get 403 on /admin, got %d", code)
	}
}

func TestRequiresClauses_SuperuserClausePass(t *testing.T) {
	baseURL, _, srv, cleanup := startTestServerWithHandles(t, requiresClausesAppSrc)
	defer cleanup()

	// ops@example.com is configured as superuser
	code, _ := getWithSession(t, srv, baseURL, "/ops", makeUserRow("ops@example.com", "viewer", ""))
	if code != 200 {
		t.Errorf("superuser identity should access /ops, got %d", code)
	}
}

func TestRequiresClauses_SuperuserClauseFail(t *testing.T) {
	baseURL, _, srv, cleanup := startTestServerWithHandles(t, requiresClausesAppSrc)
	defer cleanup()

	code, _ := getWithSession(t, srv, baseURL, "/ops", makeUserRow("normal@e.com", "admin", ""))
	if code != 403 {
		t.Errorf("non-superuser should get 403 on /ops, got %d", code)
	}
}

func TestRequiresClauses_SuperuserBypassesRoleCheck(t *testing.T) {
	baseURL, _, srv, cleanup := startTestServerWithHandles(t, requiresClausesAppSrc)
	defer cleanup()

	// /admin requires admin role; superuser identity should bypass it
	code, _ := getWithSession(t, srv, baseURL, "/admin", makeUserRow("ops@example.com", "viewer", ""))
	if code != 200 {
		t.Errorf("superuser should bypass role check on /admin, got %d", code)
	}
}

func TestRequiresClauses_ExprInListPass(t *testing.T) {
	baseURL, _, srv, cleanup := startTestServerWithHandles(t, requiresClausesAppSrc)
	defer cleanup()

	code, _ := getWithSession(t, srv, baseURL, "/pro", makeUserRow("u@e.com", "viewer", "cad"))
	if code != 200 {
		t.Errorf("user with plan=cad should access /pro, got %d", code)
	}
}

func TestRequiresClauses_ExprInListFail(t *testing.T) {
	baseURL, _, srv, cleanup := startTestServerWithHandles(t, requiresClausesAppSrc)
	defer cleanup()

	code, _ := getWithSession(t, srv, baseURL, "/pro", makeUserRow("u@e.com", "viewer", "basic"))
	if code != 403 {
		t.Errorf("user with plan=basic should get 403 on /pro, got %d", code)
	}
}

func TestRequiresClauses_CombinedRoleAndExprPass(t *testing.T) {
	baseURL, _, srv, cleanup := startTestServerWithHandles(t, requiresClausesAppSrc)
	defer cleanup()

	code, _ := getWithSession(t, srv, baseURL, "/combo", makeUserRow("a@e.com", "admin", "full"))
	if code != 200 {
		t.Errorf("admin with plan=full should access /combo, got %d", code)
	}
}

func TestRequiresClauses_CombinedFailWrongRole(t *testing.T) {
	baseURL, _, srv, cleanup := startTestServerWithHandles(t, requiresClausesAppSrc)
	defer cleanup()

	code, _ := getWithSession(t, srv, baseURL, "/combo", makeUserRow("v@e.com", "viewer", "full"))
	if code != 403 {
		t.Errorf("viewer with plan=full should get 403 on /combo (needs admin), got %d", code)
	}
}

func TestRequiresClauses_CombinedFailWrongPlan(t *testing.T) {
	baseURL, _, srv, cleanup := startTestServerWithHandles(t, requiresClausesAppSrc)
	defer cleanup()

	code, _ := getWithSession(t, srv, baseURL, "/combo", makeUserRow("a@e.com", "admin", "basic"))
	if code != 403 {
		t.Errorf("admin with plan=basic should get 403 on /combo (needs full), got %d", code)
	}
}
