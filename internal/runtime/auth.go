package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"io"
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
	ss := &SessionStore{sessions: make(map[string]*Session)}
	go ss.cleanupLoop()
	return ss
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

// cleanupLoop periodically removes expired sessions (#3 fix)
func (ss *SessionStore) cleanupLoop() {
	for {
		time.Sleep(5 * time.Minute)
		ss.mu.Lock()
		now := time.Now()
		for id, sess := range ss.sessions {
			if now.After(sess.ExpiresAt) {
				delete(ss.sessions, id)
			}
		}
		ss.mu.Unlock()
	}
}

func generateSessionID() string {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic("kilnx: failed to generate session ID: " + err.Error())
	}
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
		loginPath := app.Auth.LoginPath
		http.Redirect(w, r, loginPath+"?next="+r.URL.Path, http.StatusSeeOther)
		return false
	}

	role := page.RequiresRole
	if role == "" || role == "auth" {
		return true
	}

	userRole := session.Role
	if userRole == role {
		return true
	}

	if s.hasPermission(userRole, role, app.Permissions) {
		return true
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(renderForbidden(app.Pages, session)))
	return false
}

// hasPermission checks if userRole has the required access level
func (s *Server) hasPermission(userRole, requiredRole string, perms []parser.Permission) bool {
	for _, p := range perms {
		if p.Role == userRole {
			for _, rule := range p.Rules {
				if rule == "all" {
					return true
				}
			}
		}
	}

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

	return userRole == requiredRole
}

// hasPermissionForRequest checks if a user role has permission for a specific HTTP request
// Supports rules like "read post", "write post", "all"
func (s *Server) hasPermissionForRequest(userRole string, r *http.Request, perms []parser.Permission) bool {
	if len(perms) == 0 {
		return true
	}

	// Determine access type from HTTP method
	accessType := "read"
	switch r.Method {
	case "POST", "PUT", "DELETE", "PATCH":
		accessType = "write"
	}

	for _, p := range perms {
		if p.Role != userRole {
			continue
		}
		for _, rule := range p.Rules {
			if rule == "all" {
				return true
			}
			parts := strings.Fields(rule)
			if len(parts) >= 2 {
				ruleAccess := parts[0]   // "read" or "write"
				ruleResource := parts[1] // "post", "user", etc.

				// Check if the access type matches
				if ruleAccess != accessType {
					continue
				}

				// Check if the URL path contains the resource name
				if strings.Contains(r.URL.Path, ruleResource) || strings.Contains(r.URL.Path, ruleResource+"s") {
					return true
				}
			}
		}
	}

	return false
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

// isLocalPath validates that a redirect path is local (not an open redirect) (#1 fix)
func isLocalPath(path string) bool {
	if path == "" {
		return false
	}
	if strings.HasPrefix(path, "//") || strings.Contains(path, "://") {
		return false
	}
	if !strings.HasPrefix(path, "/") {
		return false
	}
	return true
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

	sql := fmt.Sprintf("SELECT * FROM \"%s\" WHERE \"%s\" = :identity",
		sanitizeIdentifier(app.Auth.Table), sanitizeIdentifier(app.Auth.Identity))
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

	sessionID := s.sessions.Create(user, app.Auth.Identity)
	http.SetCookie(w, &http.Cookie{
		Name:     "_kilnx_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	// Validate redirect target (#1 fix: prevent open redirect)
	next := r.URL.Query().Get("next")
	if !isLocalPath(next) {
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

	if name == "" {
		name = strings.Split(identity, "@")[0]
	}

	sql := fmt.Sprintf("INSERT INTO \"%s\" (name, \"%s\", \"%s\") VALUES (:name, :identity, :password)",
		sanitizeIdentifier(app.Auth.Table), sanitizeIdentifier(app.Auth.Identity), sanitizeIdentifier(app.Auth.Password))
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

// sanitizeIdentifier ensures a SQL identifier contains only safe characters
func sanitizeIdentifier(name string) string {
	var b strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			b.WriteRune(c)
		}
	}
	return b.String()
}
