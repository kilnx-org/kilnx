package runtime

import (
	"net/http"
	"net/http/httptest"
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

func TestSessionSignAndVerify_NoSecret(t *testing.T) {
	ss := &SessionStore{sessions: make(map[string]*Session), secret: ""}
	id := "session-abc"
	if got := ss.signSessionID(id); got != id {
		t.Errorf("signSessionID with empty secret should return id unchanged, got %q", got)
	}
}

func TestSessionSignAndVerify_WithSecret(t *testing.T) {
	ss := NewSessionStore("my-secret")
	id := "session-abc"
	signed := ss.signSessionID(id)
	if signed == id {
		t.Error("signSessionID should produce signed value when secret is set")
	}
	gotID, valid := ss.verifySessionID(signed)
	if !valid {
		t.Error("verifySessionID should return true for valid signature")
	}
	if gotID != id {
		t.Errorf("verifySessionID id = %q, want %q", gotID, id)
	}
}

func TestSessionVerify_InvalidSignature(t *testing.T) {
	ss := NewSessionStore("my-secret")
	_, valid := ss.verifySessionID("tampered.value")
	if valid {
		t.Error("verifySessionID should return false for tampered cookie")
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

// --- Additional validateFormData tests for uncovered field types ---

func TestValidateFormData_URLValidation(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "bookmark",
			Fields: []parser.Field{
				{Name: "link", Type: parser.FieldURL},
			},
		}},
	}
	errs := validateFormData("bookmark", app, map[string]string{"link": "not-a-url"})
	if len(errs) == 0 {
		t.Fatal("expected error for invalid URL")
	}
	errs = validateFormData("bookmark", app, map[string]string{"link": "https://example.com"})
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid URL, got %v", errs)
	}
}

func TestValidateFormData_DateValidation(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "event",
			Fields: []parser.Field{
				{Name: "date", Type: parser.FieldDate},
			},
		}},
	}
	errs := validateFormData("event", app, map[string]string{"date": "bad-date"})
	if len(errs) == 0 {
		t.Fatal("expected error for invalid date")
	}
	errs = validateFormData("event", app, map[string]string{"date": "2024-01-15"})
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid date, got %v", errs)
	}
}

func TestValidateFormData_DecimalValidation(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "product",
			Fields: []parser.Field{
				{Name: "price", Type: parser.FieldDecimal},
			},
		}},
	}
	errs := validateFormData("product", app, map[string]string{"price": "abc"})
	if len(errs) == 0 {
		t.Fatal("expected error for invalid decimal")
	}
	errs = validateFormData("product", app, map[string]string{"price": "19.99"})
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid decimal, got %v", errs)
	}
}

func TestValidateFormData_BigIntValidation(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "counter",
			Fields: []parser.Field{
				{Name: "count", Type: parser.FieldBigInt},
			},
		}},
	}
	errs := validateFormData("counter", app, map[string]string{"count": "not-a-number"})
	if len(errs) == 0 {
		t.Fatal("expected error for invalid big int")
	}
	errs = validateFormData("counter", app, map[string]string{"count": "9223372036854775807"})
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid big int, got %v", errs)
	}
}

func TestValidateFormData_JSONValidation(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "config",
			Fields: []parser.Field{
				{Name: "settings", Type: parser.FieldJSON},
			},
		}},
	}
	errs := validateFormData("config", app, map[string]string{"settings": "not-json"})
	if len(errs) == 0 {
		t.Fatal("expected error for invalid JSON")
	}
	errs = validateFormData("config", app, map[string]string{"settings": `{"key":"value"}`})
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid JSON, got %v", errs)
	}
}

func TestValidateFormData_TagsValidation(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "item",
			Fields: []parser.Field{
				{Name: "tags", Type: parser.FieldTags, Options: []string{"red", "blue", "green"}},
			},
		}},
	}
	errs := validateFormData("item", app, map[string]string{"tags": "red, yellow"})
	if len(errs) == 0 {
		t.Fatal("expected error for invalid tag")
	}
	errs = validateFormData("item", app, map[string]string{"tags": "red, blue"})
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid tags, got %v", errs)
	}
}

func TestValidateFormData_OptionValidation(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name: "status",
			Fields: []parser.Field{
				{Name: "state", Type: parser.FieldOption, Options: []string{"active", "inactive"}},
			},
		}},
	}
	errs := validateFormData("status", app, map[string]string{"state": "unknown"})
	if len(errs) == 0 {
		t.Fatal("expected error for invalid option")
	}
	errs = validateFormData("status", app, map[string]string{"state": "active"})
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid option, got %v", errs)
	}
}

func TestValidateFormData_CustomFields(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name:       "deal",
			CustomFieldsFile: "deal.json",
			Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText, Required: true},
			},
		}},
		CustomManifests: map[string]*parser.CustomFieldManifest{
			"deal": {
				ModelName: "deal",
				Fields: []parser.CustomFieldDef{
					{Name: "revenue", Kind: parser.CustomFieldKindNumber, Required: true},
					{Name: "region", Kind: parser.CustomFieldKindOption, Options: []string{"N", "S"}},
				},
			},
		},
	}
	// Missing required custom field
	errs := validateFormData("deal", app, map[string]string{"title": "Deal A", "custom": `{"region":"N"}`})
	if len(errs) == 0 {
		t.Fatal("expected error for missing required custom field")
	}
	// Valid custom fields
	errs = validateFormData("deal", app, map[string]string{"title": "Deal A", "custom": `{"revenue":"1000","region":"N"}`})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateFormData_CustomFieldsAllTypes(t *testing.T) {
	app := &parser.App{
		Models: []parser.Model{{
			Name:             "contact",
			CustomFieldsFile: "contact.json",
			Fields: []parser.Field{
				{Name: "name", Type: parser.FieldText, Required: true},
			},
		}},
		CustomManifests: map[string]*parser.CustomFieldManifest{
			"contact": {
				ModelName: "contact",
				Fields: []parser.CustomFieldDef{
					{Name: "birthdate", Kind: parser.CustomFieldKindDate},
					{Name: "email2", Kind: parser.CustomFieldKindEmail},
					{Name: "phone2", Kind: parser.CustomFieldKindPhone},
					{Name: "active", Kind: parser.CustomFieldKindBool},
					{Name: "ref", Kind: parser.CustomFieldKindReference},
					{Name: "status", Kind: parser.CustomFieldKindOption, Options: []string{"a", "b"}},
				},
			},
		},
	}

	cases := []struct {
		name   string
		data   map[string]string
		wantOk bool
	}{
		{"invalid date", map[string]string{"name": "X", "custom": `{"birthdate":"bad"}`}, false},
		{"valid date", map[string]string{"name": "X", "custom": `{"birthdate":"2024-01-15"}`}, true},
		{"invalid email", map[string]string{"name": "X", "custom": `{"email2":"bad"}`}, false},
		{"valid email", map[string]string{"name": "X", "custom": `{"email2":"a@b.c"}`}, true},
		{"invalid phone", map[string]string{"name": "X", "custom": `{"phone2":"123"}`}, false},
		{"valid phone", map[string]string{"name": "X", "custom": `{"phone2":"+1-555-123-4567"}`}, true},
		{"invalid bool", map[string]string{"name": "X", "custom": `{"active":"maybe"}`}, false},
		{"valid bool", map[string]string{"name": "X", "custom": `{"active":"true"}`}, true},
		{"invalid ref", map[string]string{"name": "X", "custom": `{"ref":"abc"}`}, false},
		{"valid ref", map[string]string{"name": "X", "custom": `{"ref":"42"}`}, true},
		{"invalid option", map[string]string{"name": "X", "custom": `{"status":"c"}`}, false},
		{"valid option", map[string]string{"name": "X", "custom": `{"status":"a"}`}, true},
		{"invalid json", map[string]string{"name": "X", "custom": `{"bad`}, false},
		{"unknown field", map[string]string{"name": "X", "custom": `{"unknown":"v"}`}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateFormData("contact", app, tc.data)
			if tc.wantOk && len(errs) != 0 {
				t.Errorf("expected no errors, got %v", errs)
			}
			if !tc.wantOk && len(errs) == 0 {
				t.Errorf("expected errors, got none")
			}
		})
	}
}

// ---------- loadFromDB ----------

func TestLoadFromDB_Success(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsResults[`SELECT token, user_id, identity, role, data, expires_at FROM _kilnx_sessions WHERE expires_at > datetime('now')`] = []database.Row{
		{"token": "abc123", "user_id": "1", "identity": "user@test.com", "role": "viewer", "data": `{"name":"User"}`, "expires_at": time.Now().Add(1 * time.Hour).Format(time.RFC3339)},
	}
	ss := NewSessionStore("test-secret")
	ss.SetDB(mock)
	ss.loadFromDB()

	sess := ss.Get("abc123")
	if sess == nil {
		t.Fatal("expected session to be loaded")
	}
	if sess.UserID != "1" {
		t.Errorf("expected user_id 1, got %q", sess.UserID)
	}
	if sess.Data["name"] != "User" {
		t.Errorf("expected data name=User, got %v", sess.Data)
	}
}

func TestLoadFromDB_NoDB(t *testing.T) {
	ss := NewSessionStore("test-secret")
	ss.loadFromDB() // Should not panic
	if len(ss.sessions) != 0 {
		t.Error("expected no sessions when db is nil")
	}
}

func TestLoadFromDB_InvalidDataJSON(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsResults[`SELECT token, user_id, identity, role, data, expires_at FROM _kilnx_sessions WHERE expires_at > datetime('now')`] = []database.Row{
		{"token": "badjson", "user_id": "2", "identity": "u@test.com", "role": "admin", "data": "not-json", "expires_at": time.Now().Add(1 * time.Hour).Format(time.RFC3339)},
	}
	ss := NewSessionStore("test-secret")
	ss.SetDB(mock)
	ss.loadFromDB()

	sess := ss.Get("badjson")
	if sess == nil {
		t.Fatal("expected session to be loaded even with bad JSON")
	}
	if sess.Role != "admin" {
		t.Errorf("expected role admin, got %q", sess.Role)
	}
}

func TestLoadFromDB_ExpiredSessionSkipped(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsResults[`SELECT token, user_id, identity, role, data, expires_at FROM _kilnx_sessions WHERE expires_at > datetime('now')`] = []database.Row{
		// This row simulates a session with unparseable expiry (expired)
		{"token": "expired", "user_id": "3", "identity": "old@test.com", "role": "viewer", "data": `{}`, "expires_at": "invalid-date"},
	}
	ss := NewSessionStore("test-secret")
	ss.SetDB(mock)
	ss.loadFromDB()

	if ss.Get("expired") != nil {
		t.Error("expected expired session to be skipped")
	}
}

func TestLoadFromDB_AlternateDateFormat(t *testing.T) {
	mock := newMockExecutor()
	// Use a far-future date to avoid timezone comparison issues
	mock.queryRowsResults[`SELECT token, user_id, identity, role, data, expires_at FROM _kilnx_sessions WHERE expires_at > datetime('now')`] = []database.Row{
		{"token": "altfmt", "user_id": "4", "identity": "alt@test.com", "role": "viewer", "data": `{}`, "expires_at": "2030-01-01 00:00:00"},
	}
	ss := NewSessionStore("test-secret")
	ss.SetDB(mock)

	if ss.Get("altfmt") == nil {
		t.Error("expected session with alternate date format to be loaded")
	}
}

// ---------- requireAuth ----------

func TestRequireAuth_NoAuthRequired(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("GET", "/public", nil)
	rec := httptest.NewRecorder()
	page := parser.Page{Path: "/public", Auth: false}
	if !s.requireAuth(rec, req, page) {
		t.Error("expected true when auth is not required")
	}
}

func TestRequireAuth_NoAuthConfig(t *testing.T) {
	s := newTestServer(nil)
	s.app.Auth = nil
	req := httptest.NewRequest("GET", "/protected", nil)
	rec := httptest.NewRecorder()
	page := parser.Page{Path: "/protected", Auth: true}
	if !s.requireAuth(rec, req, page) {
		t.Error("expected true when auth config is nil")
	}
}

func TestRequireAuth_NotLoggedIn(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("GET", "/protected", nil)
	rec := httptest.NewRecorder()
	page := parser.Page{Path: "/protected", Auth: true}
	if s.requireAuth(rec, req, page) {
		t.Error("expected false when not logged in")
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestRequireAuth_NotLoggedInHTMX(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	page := parser.Page{Path: "/protected", Auth: true}
	if s.requireAuth(rec, req, page) {
		t.Error("expected false when not logged in")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, want 401", rec.Code)
	}
	if rec.Header().Get("HX-Redirect") == "" {
		t.Error("expected HX-Redirect header")
	}
}

func TestRequireAuth_LoggedIn(t *testing.T) {
	s := newTestServer(nil)
	sessionID := s.sessions.Create(database.Row{"id": "1", "email": "user@test.com", "role": "viewer"}, "email")
	cookieValue := s.sessions.signSessionID(sessionID)
	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: cookieValue})
	rec := httptest.NewRecorder()
	page := parser.Page{Path: "/protected", Auth: true}
	if !s.requireAuth(rec, req, page) {
		t.Error("expected true when logged in")
	}
}

func TestRequireAuth_RequiresRole(t *testing.T) {
	s := newTestServer(nil)
	sessionID := s.sessions.Create(database.Row{"id": "1", "email": "user@test.com", "role": "viewer"}, "email")
	cookieValue := s.sessions.signSessionID(sessionID)
	req := httptest.NewRequest("GET", "/admin", nil)
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: cookieValue})
	rec := httptest.NewRecorder()
	page := parser.Page{Path: "/admin", Auth: true, RequiresRole: "admin"}
	if s.requireAuth(rec, req, page) {
		t.Error("expected false when role does not match")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestRequireAuth_RequiresRoleMatch(t *testing.T) {
	s := newTestServer(nil)
	sessionID := s.sessions.Create(database.Row{"id": "1", "email": "admin@test.com", "role": "admin"}, "email")
	cookieValue := s.sessions.signSessionID(sessionID)
	req := httptest.NewRequest("GET", "/admin", nil)
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: cookieValue})
	rec := httptest.NewRecorder()
	page := parser.Page{Path: "/admin", Auth: true, RequiresRole: "admin"}
	if !s.requireAuth(rec, req, page) {
		t.Error("expected true when role matches")
	}
}

func TestRequireAuth_RequiresRoleAuth(t *testing.T) {
	s := newTestServer(nil)
	sessionID := s.sessions.Create(database.Row{"id": "1", "email": "user@test.com", "role": "viewer"}, "email")
	cookieValue := s.sessions.signSessionID(sessionID)
	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: cookieValue})
	rec := httptest.NewRecorder()
	page := parser.Page{Path: "/protected", Auth: true, RequiresRole: "auth"}
	if !s.requireAuth(rec, req, page) {
		t.Error("expected true when requires_role is auth")
	}
}

// ---------- requireAPIAuth ----------

func TestRequireAPIAuth_NoAuthRequired(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("GET", "/api/public", nil)
	rec := httptest.NewRecorder()
	page := parser.Page{Path: "/api/public", Auth: false}
	if !s.requireAPIAuth(rec, req, page) {
		t.Error("expected true when auth is not required")
	}
}

func TestRequireAPIAuth_NoAuthConfig(t *testing.T) {
	s := newTestServer(nil)
	s.app.Auth = nil
	req := httptest.NewRequest("GET", "/api/protected", nil)
	rec := httptest.NewRecorder()
	page := parser.Page{Path: "/api/protected", Auth: true}
	if !s.requireAPIAuth(rec, req, page) {
		t.Error("expected true when auth config is nil")
	}
}

func TestRequireAPIAuth_NotLoggedIn(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("GET", "/api/protected", nil)
	rec := httptest.NewRecorder()
	page := parser.Page{Path: "/api/protected", Auth: true}
	if s.requireAPIAuth(rec, req, page) {
		t.Error("expected false when not logged in")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, want 401", rec.Code)
	}
}

func TestRequireAPIAuth_LoggedIn(t *testing.T) {
	s := newTestServer(nil)
	sessionID := s.sessions.Create(database.Row{"id": "1", "email": "user@test.com", "role": "viewer"}, "email")
	cookieValue := s.sessions.signSessionID(sessionID)
	req := httptest.NewRequest("GET", "/api/protected", nil)
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: cookieValue})
	rec := httptest.NewRecorder()
	page := parser.Page{Path: "/api/protected", Auth: true}
	if !s.requireAPIAuth(rec, req, page) {
		t.Error("expected true when logged in")
	}
}

func TestRequireAPIAuth_RequiresRole(t *testing.T) {
	s := newTestServer(nil)
	sessionID := s.sessions.Create(database.Row{"id": "1", "email": "user@test.com", "role": "viewer"}, "email")
	cookieValue := s.sessions.signSessionID(sessionID)
	req := httptest.NewRequest("GET", "/api/admin", nil)
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: cookieValue})
	rec := httptest.NewRecorder()
	page := parser.Page{Path: "/api/admin", Auth: true, RequiresRole: "admin"}
	if s.requireAPIAuth(rec, req, page) {
		t.Error("expected false when role does not match")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestRequireAPIAuth_RequiresRoleMatch(t *testing.T) {
	s := newTestServer(nil)
	sessionID := s.sessions.Create(database.Row{"id": "1", "email": "admin@test.com", "role": "admin"}, "email")
	cookieValue := s.sessions.signSessionID(sessionID)
	req := httptest.NewRequest("GET", "/api/admin", nil)
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: cookieValue})
	rec := httptest.NewRecorder()
	page := parser.Page{Path: "/api/admin", Auth: true, RequiresRole: "admin"}
	if !s.requireAPIAuth(rec, req, page) {
		t.Error("expected true when role matches")
	}
}

// ---------- hasPermission ----------

func TestHasPermission_AllRule(t *testing.T) {
	s := newTestServer(nil)
	perms := []parser.Permission{
		{Role: "admin", Rules: []string{"all"}},
	}
	if !s.hasPermission("admin", "editor", perms) {
		t.Error("admin with 'all' rule should have any permission")
	}
}

func TestHasPermission_Hierarchy(t *testing.T) {
	s := newTestServer(nil)
	if !s.hasPermission("admin", "viewer", nil) {
		t.Error("admin should have viewer permission via hierarchy")
	}
	if !s.hasPermission("editor", "viewer", nil) {
		t.Error("editor should have viewer permission via hierarchy")
	}
	if s.hasPermission("viewer", "editor", nil) {
		t.Error("viewer should not have editor permission")
	}
}

func TestHasPermission_UnknownRole(t *testing.T) {
	s := newTestServer(nil)
	if !s.hasPermission("custom", "custom", nil) {
		t.Error("same unknown role should match")
	}
	if s.hasPermission("custom", "other", nil) {
		t.Error("different unknown roles should not match")
	}
}

func TestHasPermission_MixedHierarchyAndUnknown(t *testing.T) {
	s := newTestServer(nil)
	// When requiredRole is unknown and different from userRole, no match
	if s.hasPermission("admin", "unknown", nil) {
		t.Error("admin should not match unknown role via hierarchy")
	}
	if s.hasPermission("unknown", "admin", nil) {
		t.Error("unknown role should not outrank admin")
	}
}

func TestDelete_WithDB(t *testing.T) {
	mock := newMockExecutor()
	ss := NewSessionStore("test-secret")
	ss.SetDB(mock)

	sessionID := ss.Create(database.Row{"id": "1", "email": "user@test.com"}, "email")
	if ss.Get(sessionID) == nil {
		t.Fatal("expected session to exist")
	}

	ss.Delete(sessionID)
	if ss.Get(sessionID) != nil {
		t.Error("expected session to be deleted")
	}
	// Verify DB delete was called
	found := false
	for _, call := range mock.execCalled {
		if call.SQL == `DELETE FROM _kilnx_sessions WHERE token = :token` {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected DB delete to be called")
	}
}
