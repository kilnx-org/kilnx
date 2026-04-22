package runtime

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
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

// requireAuth checks if the page requires auth and/or satisfies all requires clauses.
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
		target := loginPath + "?next=" + url.QueryEscape(r.URL.Path)
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", target)
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
		return false
	}

	if len(page.RequiresClauses) > 0 {
		if !s.evalRequiresClauses(page.RequiresClauses, session) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(renderForbidden(app.Pages, session)))
			return false
		}
		return true
	}

	// legacy single-role path
	role := page.RequiresRole
	if role == "" || role == "auth" {
		return true
	}
	if session.Role == role || s.hasPermission(session.Role, role, app.Permissions) {
		return true
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(renderForbidden(app.Pages, session)))
	return false
}

// requireAPIAuth checks auth for API endpoints, returning JSON 401/403 instead of HTML redirect.
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

	if len(page.RequiresClauses) > 0 {
		if !s.evalRequiresClauses(page.RequiresClauses, session) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"forbidden"}`))
			return false
		}
		return true
	}

	// legacy single-role path
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

// evalRequiresClauses returns true when the session satisfies ALL clauses (AND semantics).
// A superuser (matched by identity) bypasses all checks.
func (s *Server) evalRequiresClauses(clauses []parser.RequiresClause, session *Session) bool {
	if s.superuserIdentity != "" && session != nil && session.Identity == s.superuserIdentity {
		return true
	}
	app := s.getApp()
	for _, clause := range clauses {
		switch clause.Kind {
		case parser.RequiresClauseAuth:
			// satisfied by session existing (already checked before calling this)
		case parser.RequiresClauseRole:
			if session.Role != clause.Value && !s.hasPermission(session.Role, clause.Value, app.Permissions) {
				return false
			}
		case parser.RequiresClauseExpr:
			if !evalAuthExpr(clause.Value, session) {
				return false
			}
		case parser.RequiresClauseSuperuser:
			if s.superuserIdentity == "" || session.Identity != s.superuserIdentity {
				return false
			}
		}
	}
	return true
}

// evalAuthExpr evaluates a boolean expression against session data.
// Supports: "field == value", "field != value", "field in ['a','b']",
// numeric comparisons, and "and"/"or" conjunctions.
func evalAuthExpr(expr string, session *Session) bool {
	expr = strings.TrimSpace(expr)
	// split on " or " first (lowest precedence)
	orParts := splitLogical(expr, " or ")
	if len(orParts) > 1 {
		for _, part := range orParts {
			if evalAuthExpr(strings.TrimSpace(part), session) {
				return true
			}
		}
		return false
	}
	// split on " and "
	andParts := splitLogical(expr, " and ")
	if len(andParts) > 1 {
		for _, part := range andParts {
			if !evalAuthExpr(strings.TrimSpace(part), session) {
				return false
			}
		}
		return true
	}
	// single condition
	return evalSingleAuthCondition(expr, session)
}

// evalSingleAuthCondition evaluates one predicate: field op value or field in [...].
func evalSingleAuthCondition(expr string, session *Session) bool {
	expr = strings.TrimSpace(expr)
	// "field in ['a','b','c']"
	if inIdx := strings.Index(strings.ToLower(expr), " in ["); inIdx >= 0 {
		field := strings.TrimSpace(expr[:inIdx])
		listPart := strings.TrimSpace(expr[inIdx+len(" in ["):])
		listPart = strings.TrimSuffix(listPart, "]")
		items := parseInList(listPart)
		val := resolveSessionField(field, session)
		for _, item := range items {
			if val == item {
				return true
			}
		}
		return false
	}
	// comparison operators
	lhs, op, rhs := splitCondition(expr)
	if op == "" {
		return false
	}
	left := resolveSessionField(strings.TrimSpace(lhs), session)
	right := strings.Trim(strings.TrimSpace(rhs), "\"'")
	switch op {
	case "==":
		return left == right
	case "!=":
		return left != right
	case ">":
		return compareNumeric(left, right) > 0
	case "<":
		return compareNumeric(left, right) < 0
	case ">=":
		return compareNumeric(left, right) >= 0
	case "<=":
		return compareNumeric(left, right) <= 0
	}
	return false
}

// resolveSessionField returns the value of a session field referenced as
// "current_user.fieldName" or plain "fieldName".
func resolveSessionField(field string, session *Session) string {
	if session == nil {
		return ""
	}
	name := field
	if strings.HasPrefix(strings.ToLower(field), "current_user.") {
		name = field[len("current_user."):]
	}
	switch name {
	case "role":
		return session.Role
	case "id":
		return session.UserID
	case "identity", "email":
		return session.Identity
	}
	return session.Data[name]
}

// parseInList parses items from a comma-separated list, stripping quotes.
func parseInList(s string) []string {
	var items []string
	depth := 0
	inSingle := false
	inDouble := false
	start := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inDouble {
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if inSingle {
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		switch ch {
		case '"':
			inDouble = true
		case '\'':
			inSingle = true
		case '[':
			depth++
		case ']':
			depth--
		case ',':
			if depth == 0 {
				item := strings.TrimSpace(s[start:i])
				item = strings.Trim(item, "\"'")
				if item != "" {
					items = append(items, item)
				}
				start = i + 1
			}
		}
	}
	if tail := strings.Trim(strings.TrimSpace(s[start:]), "\"'"); tail != "" {
		items = append(items, tail)
	}
	return items
}

func renderForbidden(pages []parser.Page, session *Session) string {
	nav := renderNav(pages, "", session, "", "")
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

	r.ParseForm()
	identity := r.FormValue("identity")
	password := r.FormValue("password")
	csrfToken := r.FormValue("_csrf")

	if !validateCSRFToken(csrfToken) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	if identity == "" || password == "" {
		redirectWithError(w, r, app.Auth.LoginPath, "All fields are required")
		return
	}

	sql := fmt.Sprintf("SELECT * FROM \"%s\" WHERE \"%s\" = :identity",
		sanitizeIdentifier(app.Auth.Table), sanitizeIdentifier(app.Auth.Identity))
	rows, err := s.db.QueryRowsWithParams(sql, map[string]string{"identity": identity})
	if err != nil || len(rows) == 0 {
		redirectWithError(w, r, app.Auth.LoginPath, "Invalid credentials")
		return
	}

	user := rows[0]
	passwordHash := user[app.Auth.Password]

	if !CheckPassword(password, passwordHash) {
		redirectWithError(w, r, app.Auth.LoginPath, "Invalid credentials")
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
		redirectWithError(w, r, app.Auth.RegisterPath, "All fields are required")
		return
	}

	if len(password) < 6 {
		redirectWithError(w, r, app.Auth.RegisterPath, "Password must be at least 6 characters")
		return
	}

	hash, err := HashPassword(password)
	if err != nil {
		redirectWithError(w, r, app.Auth.RegisterPath, "Server error")
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
			redirectWithError(w, r, app.Auth.RegisterPath, "An account with that email already exists")
			return
		}
		redirectWithError(w, r, app.Auth.RegisterPath, "Could not create account")
		return
	}

	http.Redirect(w, r, app.Auth.LoginPath, http.StatusSeeOther)
}

// redirectWithError issues a 303 See Other to `path?error=...` so the
// user-declared page can re-render with the error visible via a query
// parameter (`{error|default:""}`). Keeps POST handlers in Go while
// letting the UI live entirely in .kilnx land.
func redirectWithError(w http.ResponseWriter, r *http.Request, path, msg string) {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	target := path + sep + "error=" + url.QueryEscape(msg)
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// handleForgotPassword processes reset requests. Only POST is served;
// GET is rendered by the user-declared `page /forgot-password` (enforced
// at compile time by the analyzer).
func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	app := s.getApp()
	if app.Auth == nil || s.db == nil {
		http.Error(w, "Password reset is not available", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		redirectWithError(w, r, app.Auth.ForgotPath, "Invalid request")
		return
	}

	csrf := r.FormValue("_csrf")
	if !validateCSRFToken(csrf) {
		redirectWithError(w, r, app.Auth.ForgotPath, "Invalid CSRF token. Please try again.")
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	if email == "" {
		redirectWithError(w, r, app.Auth.ForgotPath, "Email is required")
		return
	}

	// Check if user exists
	table := sanitizeIdentifier(app.Auth.Table)
	identity := sanitizeIdentifier(app.Auth.Identity)
	rows, err := s.db.QueryRowsWithParams(
		fmt.Sprintf(`SELECT id, %s FROM "%s" WHERE %s = :email`, identity, table, identity),
		map[string]string{"email": email},
	)
	if err != nil || len(rows) == 0 {
		// Don't reveal whether the email exists; redirect to the page
		// with ?sent=1 so it can render a generic confirmation.
		http.Redirect(w, r, app.Auth.ForgotPath+"?sent=1", http.StatusSeeOther)
		return
	}

	// Generate reset token
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	// Store token (expires in 1 hour)
	s.db.ExecWithParams(
		`DELETE FROM _kilnx_password_resets WHERE email = :email`,
		map[string]string{"email": email},
	)
	s.db.ExecWithParams(
		`INSERT INTO _kilnx_password_resets (token, email, expires_at) VALUES (:token, :email, datetime('now', '+1 hour'))`,
		map[string]string{"token": token, "email": email},
	)

	// Build reset URL
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	resetURL := fmt.Sprintf("%s://%s%s?token=%s", scheme, r.Host, app.Auth.ResetPath, token)

	// Try to send email, fall back to console
	sent := false
	smtpHost := os.Getenv("KILNX_SMTP_HOST")
	smtpFrom := os.Getenv("KILNX_SMTP_FROM")
	if smtpHost != "" && smtpFrom != "" {
		smtpPort := os.Getenv("KILNX_SMTP_PORT")
		if smtpPort == "" {
			smtpPort = "587"
		}
		smtpUser := os.Getenv("KILNX_SMTP_USER")
		smtpPass := os.Getenv("KILNX_SMTP_PASS")

		msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Password Reset\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n"+
			"<p>Click the link below to reset your password:</p>"+
			"<p><a href=\"%s\">Reset Password</a></p>"+
			"<p>This link expires in 1 hour.</p>",
			smtpFrom, email, resetURL)

		var auth smtp.Auth
		if smtpUser != "" {
			auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
		}
		err := smtp.SendMail(smtpHost+":"+smtpPort, auth, smtpFrom, []string{email}, []byte(msg))
		if err == nil {
			sent = true
		} else {
			fmt.Printf("  password reset email failed: %v\n", err)
		}
	}

	if !sent {
		fmt.Printf("\n  ╔══════════════════════════════════════════╗\n")
		fmt.Printf("  ║  PASSWORD RESET LINK (SMTP not configured) ║\n")
		fmt.Printf("  ╠══════════════════════════════════════════╣\n")
		fmt.Printf("  ║  Email: %s\n", email)
		fmt.Printf("  ║  Link:  %s\n", resetURL)
		fmt.Printf("  ╚══════════════════════════════════════════╝\n\n")
	}

	http.Redirect(w, r, app.Auth.ForgotPath+"?sent=1", http.StatusSeeOther)
}

// handleResetPassword processes the password reset form
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	app := s.getApp()
	if app.Auth == nil || s.db == nil {
		http.Error(w, "Password reset is not available", http.StatusInternalServerError)
		return
	}

	if token == "" {
		http.Redirect(w, r, app.Auth.ForgotPath, http.StatusSeeOther)
		return
	}

	resetURL := app.Auth.ResetPath + "?token=" + url.QueryEscape(token)

	if err := r.ParseForm(); err != nil {
		redirectWithError(w, r, resetURL, "Invalid request")
		return
	}

	csrf := r.FormValue("_csrf")
	if !validateCSRFToken(csrf) {
		redirectWithError(w, r, resetURL, "Invalid CSRF token. Please try again.")
		return
	}

	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")

	if len(password) < 6 {
		redirectWithError(w, r, resetURL, "Password must be at least 6 characters")
		return
	}
	if password != passwordConfirm {
		redirectWithError(w, r, resetURL, "Passwords do not match")
		return
	}

	// Validate token and get email
	rows, err := s.db.QueryRowsWithParams(
		`SELECT email FROM _kilnx_password_resets WHERE token = :token AND expires_at > datetime('now')`,
		map[string]string{"token": token},
	)
	if err != nil || len(rows) == 0 {
		redirectWithError(w, r, resetURL, "This reset link is invalid or has expired.")
		return
	}

	email := rows[0]["email"]

	// Hash new password
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		redirectWithError(w, r, resetURL, "An error occurred. Please try again.")
		return
	}

	// Update password
	table := sanitizeIdentifier(app.Auth.Table)
	identity := sanitizeIdentifier(app.Auth.Identity)
	passField := sanitizeIdentifier(app.Auth.Password)

	err = s.db.ExecWithParams(
		fmt.Sprintf(`UPDATE "%s" SET %s = :password WHERE %s = :email`, table, passField, identity),
		map[string]string{"password": string(hashed), "email": email},
	)
	if err != nil {
		redirectWithError(w, r, resetURL, "Failed to update password. Please try again.")
		return
	}

	// Delete used token
	s.db.ExecWithParams(
		`DELETE FROM _kilnx_password_resets WHERE token = :token`,
		map[string]string{"token": token},
	)

	// Redirect to login with success flag
	http.Redirect(w, r, app.Auth.LoginPath+"?reset=1", http.StatusSeeOther)
}

func sanitizeIdentifier(name string) string {
	var b strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			b.WriteRune(c)
		}
	}
	return b.String()
}
