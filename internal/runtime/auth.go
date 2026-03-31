package runtime

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
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

// SessionStore manages sessions with in-memory fast path and SQLite persistence
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	db       *database.DB
	secret   string // used for HMAC signing of session cookie values
}

func NewSessionStore(secret string) *SessionStore {
	ss := &SessionStore{sessions: make(map[string]*Session), secret: secret}
	go ss.cleanupLoop()
	return ss
}

// signSessionID creates an HMAC-signed cookie value: "id.signature"
func (ss *SessionStore) signSessionID(id string) string {
	if ss.secret == "" {
		return id
	}
	mac := hmac.New(sha256.New, []byte(ss.secret))
	mac.Write([]byte(id))
	sig := hex.EncodeToString(mac.Sum(nil))
	return id + "." + sig
}

// verifySessionID verifies and extracts the session ID from a signed cookie value
func (ss *SessionStore) verifySessionID(signed string) (string, bool) {
	if ss.secret == "" {
		return signed, true
	}
	parts := strings.SplitN(signed, ".", 2)
	if len(parts) != 2 {
		return "", false
	}
	id, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, []byte(ss.secret))
	mac.Write([]byte(id))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", false
	}
	return id, true
}

// SetDB attaches a database for session persistence and loads existing sessions
func (ss *SessionStore) SetDB(db *database.DB) {
	ss.db = db
	ss.loadFromDB()
}

func (ss *SessionStore) Create(user database.Row, identityField string) string {
	id := generateSessionID()
	expiresAt := time.Now().Add(24 * time.Hour)
	sess := &Session{
		UserID:    user["id"],
		Identity:  user[identityField],
		Role:      user["role"],
		ExpiresAt: expiresAt,
		Data:      user,
	}

	ss.mu.Lock()
	ss.sessions[id] = sess
	ss.mu.Unlock()

	// Persist to SQLite
	if ss.db != nil {
		dataJSON, _ := json.Marshal(user)
		ss.db.ExecWithParams(
			`INSERT OR REPLACE INTO _kilnx_sessions (token, user_id, identity, role, data, expires_at)
			 VALUES (:token, :user_id, :identity, :role, :data, :expires_at)`,
			map[string]string{
				"token":      id,
				"user_id":    sess.UserID,
				"identity":   sess.Identity,
				"role":       sess.Role,
				"data":       string(dataJSON),
				"expires_at": expiresAt.Format(time.RFC3339),
			})
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
	delete(ss.sessions, id)
	ss.mu.Unlock()

	// Remove from SQLite
	if ss.db != nil {
		ss.db.ExecWithParams(
			`DELETE FROM _kilnx_sessions WHERE token = :token`,
			map[string]string{"token": id})
	}
}

// loadFromDB restores non-expired sessions from SQLite on startup
func (ss *SessionStore) loadFromDB() {
	if ss.db == nil {
		return
	}
	rows, err := ss.db.QueryRows(
		`SELECT token, user_id, identity, role, data, expires_at FROM _kilnx_sessions WHERE expires_at > datetime('now')`)
	if err != nil {
		return
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()

	loaded := 0
	for _, row := range rows {
		expiresAt, err := time.Parse(time.RFC3339, row["expires_at"])
		if err != nil {
			expiresAt, err = time.Parse("2006-01-02 15:04:05", row["expires_at"])
			if err != nil {
				continue
			}
		}

		var data database.Row
		if err := json.Unmarshal([]byte(row["data"]), &data); err != nil {
			data = database.Row{
				"id":   row["user_id"],
				"role": row["role"],
			}
		}

		ss.sessions[row["token"]] = &Session{
			UserID:    row["user_id"],
			Identity:  row["identity"],
			Role:      row["role"],
			ExpiresAt: expiresAt,
			Data:      data,
		}
		loaded++
	}
	if loaded > 0 {
		fmt.Printf("Restored %d session(s) from database\n", loaded)
	}
}

// cleanupLoop periodically removes expired sessions
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

		// Cleanup SQLite too
		if ss.db != nil {
			ss.db.ExecWithParams(
				`DELETE FROM _kilnx_sessions WHERE expires_at < :now`,
				map[string]string{"now": time.Now().Format(time.RFC3339)})
		}
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

// getSession extracts the session from the request cookie, verifying HMAC signature
func (s *Server) getSession(r *http.Request) *Session {
	if s.sessions == nil {
		return nil
	}
	cookie, err := r.Cookie("_kilnx_session")
	if err != nil {
		return nil
	}
	id, valid := s.sessions.verifySessionID(cookie.Value)
	if !valid {
		return nil
	}
	return s.sessions.Get(id)
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
		http.Redirect(w, r, loginPath+"?next="+url.QueryEscape(r.URL.Path), http.StatusSeeOther)
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

// requireAPIAuth checks auth for API endpoints, returning JSON 401 instead of HTML redirect.
func (s *Server) requireAPIAuth(w http.ResponseWriter, r *http.Request, page parser.Page) bool {
	if !page.Auth {
		return true
	}
	app := s.getApp()
	if app.Auth == nil {
		return true
	}

	session := s.getSession(r)
	if session == nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
		return false
	}

	role := page.RequiresRole
	if role == "" || role == "auth" {
		return true
	}

	if session.Role == role || s.hasPermission(session.Role, role, app.Permissions) {
		return true
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"error":"forbidden"}`))
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

func renderForbidden(pages []parser.Page, session *Session) string {
	nav := renderNav(pages, "", session, "")
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
	signedID := s.sessions.signSessionID(sessionID)
	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     "_kilnx_session",
		Value:    signedID,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
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

// handleLogout clears the session (POST only, CSRF validated)
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.ParseForm()
	csrfToken := r.FormValue("_csrf")
	if !validateCSRFToken(csrfToken) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	cookie, err := r.Cookie("_kilnx_session")
	if err == nil {
		if id, valid := s.sessions.verifySessionID(cookie.Value); valid {
			s.sessions.Delete(id)
		}
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

	body := fmt.Sprintf(`    <p class="kilnx-auth-sub">Sign in to your account</p>
%s    <form method="POST" class="kilnx-form">
      <input type="hidden" name="_csrf" value="%s">
      <div class="kilnx-field">
        <label for="identity">%s</label>
        <input type="%s" id="identity" name="identity" placeholder="%s" required>
      </div>
      <div class="kilnx-field">
        <label for="password">Password</label>
        <input type="password" id="password" name="password" placeholder="Enter your password" required>
      </div>
      <button type="submit" class="kilnx-btn">Log in</button>
    </form>
    <p style="margin-top:1.25rem;font-size:0.85rem;text-align:center">Don't have an account? <a href="/register">Register</a></p>
`, errorHTML, csrf, identityLabel, identityType, strings.ToLower(identityLabel)+"@example.com")

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

	body := fmt.Sprintf(`    <p class="kilnx-auth-sub">Create your account</p>
%s    <form method="POST" class="kilnx-form">
      <input type="hidden" name="_csrf" value="%s">
      <div class="kilnx-field">
        <label for="name">Name</label>
        <input type="text" id="name" name="name" placeholder="Your name" required>
      </div>
      <div class="kilnx-field">
        <label for="identity">%s</label>
        <input type="%s" id="identity" name="identity" placeholder="%s" required>
      </div>
      <div class="kilnx-field">
        <label for="password">Password</label>
        <input type="password" id="password" name="password" placeholder="Min 6 characters" required minlength="6">
      </div>
      <button type="submit" class="kilnx-btn">Create account</button>
    </form>
    <p style="margin-top:1.25rem;font-size:0.85rem;text-align:center">Already have an account? <a href="%s">Log in</a></p>
`, errorHTML, csrf, identityLabel, identityType, strings.ToLower(identityLabel)+"@example.com", app.Auth.LoginPath)

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
  <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 470 469'><g transform='translate(0,470) scale(0.1,-0.1)' fill='%%23e64a19'><path d='M360 2850 l0-1850 111 0 111 0 240 233c132 127 256 245 274 260l34 28 0 530 0 529 1220 0 1220 0 0-522-1-523 273-267 273-267 113-1 112 0 0 1850 0 1850-1990 0-1990 0 0-1850z M1410 1877l0-473-349-347-349-347-356 0-356 0 2-352c2-194 6-352 11-350 4 1 333 2 732 2l725 0 0 288 0 288 116 215c64 117 156 288 205 379l89 164 0 503 0 503-235 0-235 0 0-473z M2120 2243c0-60-1-322-2-583l-2-475-178-374-178-374 0-213 0-214 590 0 590 0 0 214 0 214-111 233c-61 129-139 290-173 359l-61 125-5 595-5 595-232 3-233 2 0-107z M2820 1848l0-502 92-170c51-94 143-265 205-381l113-210 0-287 0-288 735 0 735 0 0 350 0 350-355 0-355 0-350 346-350 347 0 473 0 474-235 0-235 0 0-502z'/></g></svg>">
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: system-ui, -apple-system, sans-serif; line-height: 1.6; color: #e4e4e7; background: #09090b; min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 1rem; -webkit-font-smoothing: antialiased; }
    .kilnx-auth-card { width: 100%%; max-width: 400px; background: #18181b; border: 1px solid #27272a; border-radius: 12px; padding: 2rem; box-shadow: 0 8px 30px rgba(0,0,0,0.4); }
    .kilnx-auth-logo { display: flex; align-items: center; gap: 0.5rem; margin-bottom: 1.5rem; }
    .kilnx-auth-logo svg { width: 28px; height: 28px; }
    .kilnx-auth-logo span { font-size: 1.1rem; font-weight: 700; color: #ff6e40; letter-spacing: -0.02em; }
    h2 { margin-bottom: 0.25rem; font-size: 1.5rem; font-weight: 700; color: #fafafa; letter-spacing: -0.02em; }
    .kilnx-auth-sub { font-size: 0.85rem; color: #71717a; margin-bottom: 1.5rem; }
    .kilnx-form { display: flex; flex-direction: column; gap: 0.875rem; }
    .kilnx-field { display: flex; flex-direction: column; gap: 0.3rem; }
    .kilnx-field label { font-size: 0.8rem; font-weight: 500; color: #a1a1aa; text-transform: uppercase; letter-spacing: 0.04em; }
    .kilnx-field input { padding: 0.625rem 0.75rem; background: #09090b; border: 1px solid #3f3f46; border-radius: 8px; font-size: 0.9rem; font-family: inherit; color: #fafafa; outline: none; transition: border-color 0.2s, box-shadow 0.2s; }
    .kilnx-field input:hover { border-color: #52525b; }
    .kilnx-field input:focus { border-color: #e64a19; box-shadow: 0 0 0 3px rgba(230,74,25,0.2); }
    .kilnx-field input::placeholder { color: #52525b; }
    .kilnx-btn { padding: 0.625rem 1.25rem; background: #e64a19; color: white; border: none; border-radius: 8px; font-size: 0.9rem; font-weight: 600; cursor: pointer; font-family: inherit; transition: background 0.2s, box-shadow 0.2s, transform 0.1s; margin-top: 0.25rem; }
    .kilnx-btn:hover { background: #ff6e40; box-shadow: 0 0 20px rgba(230,74,25,0.25); }
    .kilnx-btn:active { transform: scale(0.98); }
    .kilnx-alert { padding: 0.75rem 1rem; border-radius: 8px; margin-bottom: 1rem; font-size: 0.85rem; }
    .kilnx-alert-error { background: rgba(239,68,68,0.1); color: #fca5a5; border: 1px solid rgba(239,68,68,0.2); }
    a { color: #ff6e40; text-decoration: none; transition: color 0.2s; }
    a:hover { color: #ffab91; }
    p { color: #71717a; }
  </style>
</head>
<body>
  <div class="kilnx-auth-card">
  <div class="kilnx-auth-logo">
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 470 469"><g transform="translate(0,470) scale(0.1,-0.1)" fill="#e64a19" stroke="none"><path d="M360 2850 l0-1850 111 0 111 0 240 233c132 127 256 245 274 260l34 28 0 530 0 529 1220 0 1220 0 0-522-1-523 273-267 273-267 113-1 112 0 0 1850 0 1850-1990 0-1990 0 0-1850z M1410 1877l0-473-349-347-349-347-356 0-356 0 2-352c2-194 6-352 11-350 4 1 333 2 732 2l725 0 0 288 0 288 116 215c64 117 156 288 205 379l89 164 0 503 0 503-235 0-235 0 0-473z M2120 2243c0-60-1-322-2-583l-2-475-178-374-178-374 0-213 0-214 590 0 590 0 0 214 0 214-111 233c-61 129-139 290-173 359l-61 125-5 595-5 595-232 3-233 2 0-107z M2820 1848l0-502 92-170c51-94 143-265 205-381l113-210 0-287 0-288 735 0 735 0 0 350 0 350-355 0-355 0-350 346-350 347 0 473 0 474-235 0-235 0 0-502z"/></g></svg>
    <span>kilnx</span>
  </div>
  <h2>%s</h2>
%s  </div>
</body>
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
