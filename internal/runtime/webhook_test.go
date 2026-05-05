package runtime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestVerifySignature_Empty(t *testing.T) {
	if verifySignature([]byte("test"), "", "secret") {
		t.Error("expected false for empty signature")
	}
}

func TestVerifySignature_GitHubFormat(t *testing.T) {
	payload := []byte(`{"type":"push"}`)
	secret := "mysecret"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))

	if !verifySignature(payload, "sha256="+sig, secret) {
		t.Error("expected true for valid GitHub signature")
	}
	if verifySignature(payload, "sha256="+sig+"bad", secret) {
		t.Error("expected false for invalid signature")
	}
}

func TestVerifySignature_StripeFormat(t *testing.T) {
	payload := []byte(`{"type":"charge.succeeded"}`)
	secret := "whsec_test"
	ts := fmt.Sprintf("%d", time.Now().Unix())
	signedPayload := ts + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	sig := hex.EncodeToString(mac.Sum(nil))
	header := "t=" + ts + ",v1=" + sig

	if !verifySignature(payload, header, secret) {
		t.Error("expected true for valid Stripe signature")
	}

	// Invalid timestamp (too old)
	oldHeader := "t=1000000000,v1=" + sig
	if verifySignature(payload, oldHeader, secret) {
		t.Error("expected false for old timestamp")
	}

	// Missing v1
	noSigHeader := "t=" + ts
	if verifySignature(payload, noSigHeader, secret) {
		t.Error("expected false for missing v1")
	}
}

func TestExtractEventType(t *testing.T) {
	tests := []struct {
		name     string
		payload  map[string]interface{}
		expected string
	}{
		{"type field", map[string]interface{}{"type": "push"}, "push"},
		{"event field", map[string]interface{}{"event": "deploy"}, "deploy"},
		{"event_type field", map[string]interface{}{"event_type": "click"}, "click"},
		{"action field", map[string]interface{}{"action": "submit"}, "submit"},
		{"non-string value", map[string]interface{}{"type": 42}, "unknown"},
		{"no match", map[string]interface{}{"foo": "bar"}, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractEventType(tt.payload)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFlattenMap_DefaultCase(t *testing.T) {
	// Arrays and nil fall into the default case
	payload := map[string]interface{}{
		"items": []interface{}{"a", "b"},
		"empty": nil,
	}
	result := flattenPayload(payload, "event")
	if !strings.Contains(result["event_items"], "a") {
		t.Errorf("expected array to be stringified, got %q", result["event_items"])
	}
	if result["event_empty"] != "<nil>" {
		t.Errorf("expected nil to be <nil>, got %q", result["event_empty"])
	}
}

func TestHandleWebhook_SecretEnvEmpty(t *testing.T) {
	s := newTestServer(nil)
	wh := parser.Webhook{
		Path:      "/hooks/test",
		SecretEnv: "EMPTY_WEBHOOK_SECRET",
		Events:    []parser.WebhookEvent{{Name: "*"}},
	}
	req := httptest.NewRequest(http.MethodPost, "/hooks/test", strings.NewReader(`{"type":"test"}`))
	rr := httptest.NewRecorder()
	s.handleWebhook(rr, req, wh)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for empty secret env, got %d", rr.Code)
	}
}

func TestHandleWebhook_InvalidJSON(t *testing.T) {
	s := newTestServer(nil)
	wh := parser.Webhook{
		Path:   "/hooks/test",
		Events: []parser.WebhookEvent{{Name: "*"}},
	}
	req := httptest.NewRequest(http.MethodPost, "/hooks/test", strings.NewReader(`{invalid`))
	rr := httptest.NewRecorder()
	s.handleWebhook(rr, req, wh)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", rr.Code)
	}
}
