package runtime

import (
	"strings"
	"testing"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// ---------- HashPassword / CheckPassword ----------

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("hash should not be empty")
	}
	if hash == "secret123" {
		t.Fatal("hash should differ from plaintext")
	}
	if !CheckPassword("secret123", hash) {
		t.Error("CheckPassword should return true for correct password")
	}
}

func TestCheckPasswordWrongPassword(t *testing.T) {
	hash, _ := HashPassword("correct")
	if CheckPassword("wrong", hash) {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestCheckPasswordInvalidHash(t *testing.T) {
	if CheckPassword("anything", "not-a-bcrypt-hash") {
		t.Error("CheckPassword should return false for invalid hash")
	}
}

func TestHashPasswordDifferentHashes(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Error("bcrypt should produce different hashes for the same input (different salts)")
	}
}

// ---------- SessionStore lifecycle ----------

func TestSessionCreateGetDelete(t *testing.T) {
	ss := &SessionStore{sessions: make(map[string]*Session)}

	user := database.Row{
		"id":    "1",
		"email": "alice@test.com",
		"role":  "admin",
	}

	token := ss.Create(user, "email")
	if token == "" {
		t.Fatal("Create should return a non-empty token")
	}

	sess := ss.Get(token)
	if sess == nil {
		t.Fatal("Get should return the session")
	}
	if sess.UserID != "1" {
		t.Errorf("expected UserID=1, got %s", sess.UserID)
	}
	if sess.Identity != "alice@test.com" {
		t.Errorf("expected Identity=alice@test.com, got %s", sess.Identity)
	}
	if sess.Role != "admin" {
		t.Errorf("expected Role=admin, got %s", sess.Role)
	}

	ss.Delete(token)
	if ss.Get(token) != nil {
		t.Error("Get should return nil after Delete")
	}
}

func TestSessionGetNonexistent(t *testing.T) {
	ss := &SessionStore{sessions: make(map[string]*Session)}
	if ss.Get("nonexistent-token") != nil {
		t.Error("Get should return nil for unknown token")
	}
}

// ---------- Session expiry ----------

func TestSessionExpiry(t *testing.T) {
	ss := &SessionStore{sessions: make(map[string]*Session)}

	// Manually insert an already-expired session
	ss.mu.Lock()
	ss.sessions["expired-token"] = &Session{
		UserID:    "99",
		Identity:  "old@test.com",
		Role:      "",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		Data:      database.Row{"id": "99"},
	}
	ss.mu.Unlock()

	if ss.Get("expired-token") != nil {
		t.Error("expired session should return nil from Get")
	}
}

func TestSessionNotExpired(t *testing.T) {
	ss := &SessionStore{sessions: make(map[string]*Session)}

	ss.mu.Lock()
	ss.sessions["valid-token"] = &Session{
		UserID:    "1",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	ss.mu.Unlock()

	if ss.Get("valid-token") == nil {
		t.Error("non-expired session should be returned")
	}
}

// ---------- generateSessionID ----------

func TestGenerateSessionID_Length(t *testing.T) {
	id := generateSessionID()
	if len(id) != 64 {
		t.Errorf("expected 64-char hex string, got %d chars: %s", len(id), id)
	}
}

func TestGenerateSessionID_HexChars(t *testing.T) {
	id := generateSessionID()
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex character %c in session ID: %s", c, id)
			break
		}
	}
}

func TestGenerateSessionID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateSessionID()
		if ids[id] {
			t.Fatalf("duplicate session ID generated: %s", id)
		}
		ids[id] = true
	}
}

// ---------- sanitizeIdentifier ----------

func TestSanitizeIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"users", "users"},
		{"user_name", "user_name"},
		{"CamelCase", "CamelCase"},
		{"has spaces", "hasspaces"},
		{"drop;table", "droptable"},
		{`"quoted"`, "quoted"},
		{"special!@#chars", "specialchars"},
		{"123numbers", "_123numbers"},
		{"", ""},
		{"a-b-c", "abc"},
	}
	for _, tc := range tests {
		got := sanitizeIdentifier(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeIdentifier(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// ---------- isLocalPath ----------

func TestIsLocalPath(t *testing.T) {
	tests := []struct {
		path  string
		valid bool
	}{
		{"/foo", true},
		{"/bar/baz", true},
		{"/", true},
		{"/a/b/c/d", true},
		{"", false},
		{"//evil.com", false},
		{"http://evil.com", false},
		{"https://evil.com", false},
		{"ftp://evil.com", false},
		{"relative/path", false},
		{"javascript://alert(1)", false},
		{"//", false},
	}
	for _, tc := range tests {
		got := isLocalPath(tc.path)
		if got != tc.valid {
			t.Errorf("isLocalPath(%q) = %v, want %v", tc.path, got, tc.valid)
		}
	}
}

// ---------- CSRF tokens ----------

func TestGenerateCSRFToken_Format(t *testing.T) {
	token := generateCSRFToken()
	if len(token) != 64 {
		t.Errorf("CSRF token should be 64 chars, got %d: %s", len(token), token)
	}
	for _, c := range token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex char %c in CSRF token", c)
			break
		}
	}
}

func TestGenerateCSRFToken_Unique(t *testing.T) {
	t1 := generateCSRFToken()
	t2 := generateCSRFToken()
	if t1 == t2 {
		t.Error("two CSRF tokens should not be identical")
	}
}

func TestValidateCSRFToken_SingleUse(t *testing.T) {
	token := generateCSRFToken()

	if !validateCSRFToken(token) {
		t.Error("first validation should succeed")
	}
	if validateCSRFToken(token) {
		t.Error("second validation should fail (single-use)")
	}
}

func TestValidateCSRFToken_UnknownRejected(t *testing.T) {
	if validateCSRFToken("not-a-real-token") {
		t.Error("unknown token should be rejected")
	}
}

func TestValidateCSRFToken_ExpiredRejected(t *testing.T) {
	// Manually insert an expired token
	csrfTokensMu.Lock()
	csrfTokens["expired-csrf"] = csrfEntry{createdAt: time.Now().Add(-1 * time.Hour)}
	csrfTokensMu.Unlock()

	if validateCSRFToken("expired-csrf") {
		t.Error("expired CSRF token should be rejected")
	}
}

// ---------- validateFormData ----------

func TestValidateFormData_RequiredFields(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "post",
			Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText, Required: true},
				{Name: "body", Type: parser.FieldText},
			},
		}},
	}

	// Missing required field
	errs := validateFormData("post", app, map[string]string{"body": "content"})
	if len(errs) == 0 {
		t.Fatal("expected validation error for missing title")
	}
	if !strings.Contains(errs[0], "Title") || !strings.Contains(errs[0], "required") {
		t.Errorf("unexpected error message: %s", errs[0])
	}

	// All fields present
	errs = validateFormData("post", app, map[string]string{"title": "Hello", "body": "World"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateFormData_EmailValidation(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "contact",
			Fields: []parser.Field{
				{Name: "email", Type: parser.FieldEmail},
			},
		}},
	}

	errs := validateFormData("contact", app, map[string]string{"email": "invalid"})
	if len(errs) == 0 {
		t.Fatal("expected error for invalid email")
	}

	errs = validateFormData("contact", app, map[string]string{"email": "user@example.com"})
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid email, got %v", errs)
	}

	// Empty email should not trigger email format error (not required)
	errs = validateFormData("contact", app, map[string]string{"email": ""})
	if len(errs) != 0 {
		t.Errorf("empty non-required email should pass, got %v", errs)
	}
}

func TestValidateFormData_MinMaxText(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "profile",
			Fields: []parser.Field{
				{Name: "bio", Type: parser.FieldText, Min: "5", Max: "10"},
			},
		}},
	}

	// Too short
	errs := validateFormData("profile", app, map[string]string{"bio": "hi"})
	if len(errs) == 0 {
		t.Fatal("expected min length error")
	}
	if !strings.Contains(errs[0], "at least 5 characters") {
		t.Errorf("unexpected error: %s", errs[0])
	}

	// Too long
	errs = validateFormData("profile", app, map[string]string{"bio": "this is way too long"})
	if len(errs) == 0 {
		t.Fatal("expected max length error")
	}

	// Just right
	errs = validateFormData("profile", app, map[string]string{"bio": "hello!"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateFormData_MinMaxNumeric(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "product",
			Fields: []parser.Field{
				{Name: "price", Type: parser.FieldFloat, Min: "1", Max: "1000"},
				{Name: "quantity", Type: parser.FieldInt, Min: "0", Max: "100"},
			},
		}},
	}

	// Price below min
	errs := validateFormData("product", app, map[string]string{"price": "0.5", "quantity": "10"})
	if len(errs) == 0 {
		t.Fatal("expected min value error for price")
	}

	// Quantity above max
	errs = validateFormData("product", app, map[string]string{"price": "50", "quantity": "200"})
	if len(errs) == 0 {
		t.Fatal("expected max value error for quantity")
	}

	// Valid values
	errs = validateFormData("product", app, map[string]string{"price": "50", "quantity": "10"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateFormData_UnknownModel(t *testing.T) {
	app := &parser.App{Models: []parser.Model{}}
	errs := validateFormData("nonexistent", app, map[string]string{})
	if len(errs) != 1 || !strings.Contains(errs[0], "Unknown model") {
		t.Errorf("expected unknown model error, got %v", errs)
	}
}

func TestValidateFormData_AutoFieldsSkipped(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "event",
			Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText, Required: true},
				{Name: "created_at", Type: parser.FieldTimestamp, Auto: true, Required: true},
			},
		}},
	}

	// Auto fields should not be validated even if required
	errs := validateFormData("event", app, map[string]string{"name": "Party"})
	if len(errs) != 0 {
		t.Errorf("auto fields should be skipped, got %v", errs)
	}
}

func TestValidateFormData_ReferenceFieldsSkipped(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "comment",
			Fields: []parser.Field{
				{Name: "body", Type: parser.FieldText, Required: true},
				{Name: "post", Type: parser.FieldReference, Reference: "post", Required: true},
			},
		}},
	}

	errs := validateFormData("comment", app, map[string]string{"body": "Nice post"})
	if len(errs) != 0 {
		t.Errorf("reference fields should be skipped, got %v", errs)
	}
}

// ---------- validateInlineRules ----------

func TestValidateInlineRules_Required(t *testing.T) {
	validations := []parser.Validation{
		{Field: "name", Rules: []string{"required"}},
	}

	errs := validateInlineRules(validations, map[string]string{"name": ""})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "Name") || !strings.Contains(errs[0], "required") {
		t.Errorf("unexpected error: %s", errs[0])
	}

	errs = validateInlineRules(validations, map[string]string{"name": "Alice"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateInlineRules_IsEmail(t *testing.T) {
	validations := []parser.Validation{
		{Field: "email", Rules: []string{"is", "email"}},
	}

	errs := validateInlineRules(validations, map[string]string{"email": "bad"})
	if len(errs) == 0 {
		t.Fatal("expected email validation error")
	}

	errs = validateInlineRules(validations, map[string]string{"email": "user@example.com"})
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid email, got %v", errs)
	}

	// Empty value should pass (not required)
	errs = validateInlineRules(validations, map[string]string{"email": ""})
	if len(errs) != 0 {
		t.Errorf("empty non-required email should pass, got %v", errs)
	}
}

func TestValidateInlineRules_IsDate(t *testing.T) {
	validations := []parser.Validation{
		{Field: "birthday", Rules: []string{"is", "date"}},
	}

	errs := validateInlineRules(validations, map[string]string{"birthday": "not-a-date"})
	if len(errs) == 0 {
		t.Fatal("expected date validation error")
	}

	errs = validateInlineRules(validations, map[string]string{"birthday": "2024-01-15"})
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid date, got %v", errs)
	}
}

func TestValidateInlineRules_Min(t *testing.T) {
	validations := []parser.Validation{
		{Field: "password", Rules: []string{"min", "8"}},
	}

	errs := validateInlineRules(validations, map[string]string{"password": "short"})
	if len(errs) == 0 {
		t.Fatal("expected min length error")
	}
	if !strings.Contains(errs[0], "at least 8 characters") {
		t.Errorf("unexpected error: %s", errs[0])
	}

	errs = validateInlineRules(validations, map[string]string{"password": "longenough"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateInlineRules_Max(t *testing.T) {
	validations := []parser.Validation{
		{Field: "tag", Rules: []string{"max", "5"}},
	}

	errs := validateInlineRules(validations, map[string]string{"tag": "toolong"})
	if len(errs) == 0 {
		t.Fatal("expected max length error")
	}

	errs = validateInlineRules(validations, map[string]string{"tag": "ok"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateInlineRules_CombinedRules(t *testing.T) {
	validations := []parser.Validation{
		{Field: "email", Rules: []string{"required", "is", "email"}},
		{Field: "name", Rules: []string{"required", "min", "2", "max", "50"}},
	}

	// Both empty
	errs := validateInlineRules(validations, map[string]string{"email": "", "name": ""})
	if len(errs) < 2 {
		t.Errorf("expected at least 2 errors, got %d: %v", len(errs), errs)
	}

	// Valid data
	errs = validateInlineRules(validations, map[string]string{"email": "a@b.com", "name": "Alice"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateInlineRules_EmptyValueSkipsMinMax(t *testing.T) {
	validations := []parser.Validation{
		{Field: "optional", Rules: []string{"min", "3", "max", "10"}},
	}

	errs := validateInlineRules(validations, map[string]string{"optional": ""})
	if len(errs) != 0 {
		t.Errorf("empty value should skip min/max, got %v", errs)
	}
}
