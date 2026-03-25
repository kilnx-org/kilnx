package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
	"golang.org/x/crypto/bcrypt"
)

// Session stores authenticated user data
type Session struct {
	UserID    string
	Identity  string
	Role      string
	ExpiresAt time.Time
	Data      database.Row // full user row
}

// SessionStore manages sessions in memory
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

func (ss *SessionStore) Create(user database.Row, identityField string) string {
	id := generateSessionID()
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.sessions[id] = &Session{
		UserID:    user["id"],
		Identity:  user[identityField],
		Role:      user["role"],
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Data:      user,
	}
	return id
}

func (ss *SessionStore) Get(id string) *Session {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	sess, ok := ss.sessions[id]
	if !ok || time.Now().After(sess.ExpiresAt) {
		return nil
	}
	return sess
}

func (ss *SessionStore) Delete(id string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.sessions, id)
}

func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// HashPassword hashes a plaintext password with bcrypt
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password with a bcrypt hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// getSession extracts the session from the request cookie
func (s *Server) getSession(r *http.Request) *Session {
	if s.sessions == nil {
		return nil
	}
	cookie, err := r.Cookie("_kilnx_session")
	if err != nil {
		return nil
	}
	return s.sessions.Get(cookie.Value)
}

// requireAuth checks if the page requires auth and/or a specific role.
// Returns true if the request should proceed, false if redirected or forbidden.
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request, page parser.Page) bool {
	if !page.Auth {
		return true
	}
	app := s.getApp()
	if app.Auth == nil {
		return true
	}

	session := s.getSession(r)
	if session == nil {
		// Not logged in: redirect to login
		loginPath := app.Auth.LoginPath
		http.Redirect(w, r, loginPath+"?next="+r.URL.Path, http.StatusSeeOther)
		return false
	}

	// Check role if specified
	role := page.RequiresRole
	if role == "" || role == "auth" {
		return true // any authenticated user
	}

	// Check if user's role matches or if user's role has "all" permission
	userRole := session.Role
	if userRole == role {
		return true
	}

	// Check permissions table: does the user's role have access?
	if s.hasPermission(userRole, role, app.Permissions) {
		return true
	}

	// Forbidden
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(renderForbidden(app.Pages, session)))
	return false
}

// hasPermission checks if userRole has the required access level
func (s *Server) hasPermission(userRole, requiredRole string, perms []parser.Permission) bool {
	// Find the user's permission entry
	for _, p := range perms {
		if p.Role == userRole {
			for _, rule := range p.Rules {
				if rule == "all" {
					return true
				}
			}
		}
	}

	// Check role hierarchy: if requiredRole is defined in permissions,
	// check if userRole is "higher" (has more rules or has "all")
	// Simple approach: admin > editor > viewer by convention
	roleHierarchy := map[string]int{
		"admin":  100,
		"editor": 50,
		"viewer": 10,
	}

	userLevel, userOk := roleHierarchy[userRole]
	requiredLevel, reqOk := roleHierarchy[requiredRole]

	if userOk && reqOk {
		return userLevel >= requiredLevel
	}

	// Direct match only
	return userRole == requiredRole
}

func renderForbidden(pages []parser.Page, session *Session) string {
	nav := renderNav(pages, "", session)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>403 - Forbidden</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: system-ui, -apple-system, sans-serif; line-height: 1.6; color: #1a1a1a; max-width: 800px; margin: 0 auto; padding: 1rem; }
    nav { display: flex; gap: 1rem; padding: 0.75rem 0; border-bottom: 1px solid #e0e0e0; margin-bottom: 1.5rem; flex-wrap: wrap; }
    nav a { text-decoration: none; color: #555; font-size: 0.9rem; }
    nav a:hover { color: #1a1a1a; }
    main { padding: 2rem 0; text-align: center; }
    h1 { font-size: 3rem; color: #ccc; margin-bottom: 0.5rem; }
    p { color: #888; }
  </style>
</head>
<body>
%s  <main>
    <h1>403</h1>
    <p>You don't have permission to access this page.</p>
  </main>
</body>
</html>
`, nav)
}

// handleLogin processes the login form POST
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	app := s.getApp()
	if app.Auth == nil {
		http.Error(w, "Auth not configured", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		s.renderLoginPage(w, r, "")
		return
	}

	// POST: process login
	r.ParseForm()
	identity := r.FormValue("identity")
	password := r.FormValue("password")
	csrfToken := r.FormValue("_csrf")

	if !validateCSRFToken(csrfToken) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	if identity == "" || password == "" {
		s.renderLoginPage(w, r, "All fields are required")
		return
	}

	// Look up user
	sql := fmt.Sprintf("SELECT * FROM %s WHERE %s = :identity",
		app.Auth.Table, app.Auth.Identity)
	rows, err := s.db.QueryRowsWithParams(sql, map[string]string{"identity": identity})
	if err != nil || len(rows) == 0 {
		s.renderLoginPage(w, r, "Invalid credentials")
		return
	}

	user := rows[0]
	passwordHash := user[app.Auth.Password]

	if !CheckPassword(password, passwordHash) {
		s.renderLoginPage(w, r, "Invalid credentials")
		return
	}

	// Create session
	sessionID := s.sessions.Create(user, app.Auth.Identity)
	http.SetCookie(w, &http.Cookie{
		Name:     "_kilnx_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	// Redirect to "next" or after login path
	next := r.URL.Query().Get("next")
	if next == "" {
		next = app.Auth.AfterLogin
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

// handleLogout clears the session
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("_kilnx_session")
	if err == nil {
		s.sessions.Delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "_kilnx_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	app := s.getApp()
	loginPath := "/login"
	if app.Auth != nil {
		loginPath = app.Auth.LoginPath
	}
	http.Redirect(w, r, loginPath, http.StatusSeeOther)
}

// handleRegister handles user registration
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	app := s.getApp()
	if app.Auth == nil {
		http.Error(w, "Auth not configured", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		s.renderRegisterPage(w, r, "")
		return
	}

	// POST: process registration
	r.ParseForm()
	identity := r.FormValue("identity")
	password := r.FormValue("password")
	name := r.FormValue("name")
	csrfToken := r.FormValue("_csrf")

	if !validateCSRFToken(csrfToken) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	if identity == "" || password == "" {
		s.renderRegisterPage(w, r, "All fields are required")
		return
	}

	if len(password) < 6 {
		s.renderRegisterPage(w, r, "Password must be at least 6 characters")
		return
	}

	hash, err := HashPassword(password)
	if err != nil {
		s.renderRegisterPage(w, r, "Server error")
		return
	}

	// Insert user
	if name == "" {
		name = strings.Split(identity, "@")[0]
	}

	sql := fmt.Sprintf("INSERT INTO %s (name, %s, %s) VALUES (:name, :identity, :password)",
		app.Auth.Table, app.Auth.Identity, app.Auth.Password)
	err = s.db.ExecWithParams(sql, map[string]string{
		"name":     name,
		"identity": identity,
		"password": hash,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			s.renderRegisterPage(w, r, "An account with that email already exists")
			return
		}
		s.renderRegisterPage(w, r, "Could not create account")
		return
	}

	http.Redirect(w, r, app.Auth.LoginPath, http.StatusSeeOther)
}

func (s *Server) renderLoginPage(w http.ResponseWriter, r *http.Request, errorMsg string) {
	app := s.getApp()
	csrf := generateCSRFToken()

	errorHTML := ""
	if errorMsg != "" {
		errorHTML = fmt.Sprintf("    <div class=\"kilnx-alert kilnx-alert-error\">%s</div>\n", html.EscapeString(errorMsg))
	}

	identityLabel := "Email"
	if app.Auth != nil && app.Auth.Identity != "email" {
		identityLabel = strings.ToUpper(app.Auth.Identity[:1]) + app.Auth.Identity[1:]
	}

	identityType := "email"
	if app.Auth != nil && app.Auth.Identity != "email" {
		identityType = "text"
	}

	body := fmt.Sprintf(`%s    <form method="POST" class="kilnx-form">
      <input type="hidden" name="_csrf" value="%s">
      <div class="kilnx-field">
        <label for="identity">%s</label>
        <input type="%s" id="identity" name="identity" required>
      </div>
      <div class="kilnx-field">
        <label for="password">Password</label>
        <input type="password" id="password" name="password" required>
      </div>
      <button type="submit" class="kilnx-btn">Log in</button>
    </form>
    <p style="margin-top:1rem;font-size:0.85rem;color:#888">Don't have an account? <a href="/register">Register</a></p>
`, errorHTML, csrf, identityLabel, identityType)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(renderAuthPage("Log in", body, app.Pages)))
}

func (s *Server) renderRegisterPage(w http.ResponseWriter, r *http.Request, errorMsg string) {
	app := s.getApp()
	csrf := generateCSRFToken()

	errorHTML := ""
	if errorMsg != "" {
		errorHTML = fmt.Sprintf("    <div class=\"kilnx-alert kilnx-alert-error\">%s</div>\n", html.EscapeString(errorMsg))
	}

	identityLabel := "Email"
	identityType := "email"
	if app.Auth != nil && app.Auth.Identity != "email" {
		identityLabel = strings.ToUpper(app.Auth.Identity[:1]) + app.Auth.Identity[1:]
		identityType = "text"
	}

	body := fmt.Sprintf(`%s    <form method="POST" class="kilnx-form">
      <input type="hidden" name="_csrf" value="%s">
      <div class="kilnx-field">
        <label for="name">Name</label>
        <input type="text" id="name" name="name" required>
      </div>
      <div class="kilnx-field">
        <label for="identity">%s</label>
        <input type="%s" id="identity" name="identity" required>
      </div>
      <div class="kilnx-field">
        <label for="password">Password</label>
        <input type="password" id="password" name="password" required minlength="6">
      </div>
      <button type="submit" class="kilnx-btn">Register</button>
    </form>
    <p style="margin-top:1rem;font-size:0.85rem;color:#888">Already have an account? <a href="%s">Log in</a></p>
`, errorHTML, csrf, identityLabel, identityType, app.Auth.LoginPath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(renderAuthPage("Register", body, app.Pages)))
}

func renderAuthPage(title, body string, pages []parser.Page) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: system-ui, -apple-system, sans-serif; line-height: 1.6; color: #1a1a1a; max-width: 400px; margin: 4rem auto; padding: 1rem; }
    h2 { margin-bottom: 1.5rem; font-size: 1.5rem; }
    .kilnx-form { display: flex; flex-direction: column; gap: 0.75rem; }
    .kilnx-field { display: flex; flex-direction: column; gap: 0.25rem; }
    .kilnx-field label { font-size: 0.85rem; font-weight: 500; color: #555; }
    .kilnx-field input { padding: 0.5rem; border: 1px solid #ddd; border-radius: 4px; font-size: 0.9rem; font-family: inherit; }
    .kilnx-field input:focus { outline: none; border-color: #4a7aba; box-shadow: 0 0 0 2px rgba(74,122,186,0.15); }
    .kilnx-btn { padding: 0.5rem 1.25rem; background: #1a1a1a; color: white; border: none; border-radius: 4px; font-size: 0.9rem; cursor: pointer; }
    .kilnx-btn:hover { background: #333; }
    .kilnx-alert { padding: 0.75rem 1rem; border-radius: 4px; margin-bottom: 1rem; font-size: 0.9rem; }
    .kilnx-alert-error { background: #fef2f2; color: #991b1b; border: 1px solid #fecaca; }
    a { color: #4a7aba; }
  </style>
</head>
<body>
  <h2>%s</h2>
%s</body>
</html>
`, html.EscapeString(title), html.EscapeString(title), body)
}
