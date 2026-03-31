package runtime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
)

func TestE2E_Webhook_ValidSignature(t *testing.T) {
	t.Setenv("TEST_WEBHOOK_SECRET", "mysecret")

	src := `
model event
  type: text
  data: text

webhook /hooks/deploy secret env TEST_WEBHOOK_SECRET
  on event *
    query: INSERT INTO event (type, data) VALUES (:event_type, :event_action)
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	payload := `{"type":"push","action":"deploy"}`
	mac := hmac.New(sha256.New, []byte("mysecret"))
	mac.Write([]byte(payload))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	status, body := httpPostJSON(t, baseURL+"/hooks/deploy", payload, map[string]string{
		"X-Signature-256": sig,
	})

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", status, body)
	}
	if !strings.Contains(body, "ok") {
		t.Errorf("expected ok response, got %s", body)
	}
}

func TestE2E_Webhook_InvalidSignature(t *testing.T) {
	t.Setenv("TEST_WEBHOOK_SECRET", "mysecret")

	src := `
model event
  type: text

webhook /hooks/deploy secret env TEST_WEBHOOK_SECRET
  on event *
    query: INSERT INTO event (type) VALUES (:event_type)
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	payload := `{"type":"push"}`
	status, _ := httpPostJSON(t, baseURL+"/hooks/deploy", payload, map[string]string{
		"X-Signature-256": "sha256=invalidsignature",
	})

	if status != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", status)
	}
}

func TestE2E_Webhook_MethodNotAllowed(t *testing.T) {
	t.Setenv("TEST_WEBHOOK_SECRET2", "secret2")

	src := `
model event
  type: text

webhook /hooks/test secret env TEST_WEBHOOK_SECRET2
  on event *
    query: INSERT INTO event (type) VALUES (:event_type)
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	status, _ := httpGet(t, baseURL+"/hooks/test")
	if status != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET on webhook, got %d", status)
	}
}

func TestE2E_Webhook_MissingSignature(t *testing.T) {
	t.Setenv("TEST_WEBHOOK_SECRET3", "secret3")

	src := `
model event
  type: text

webhook /hooks/nosig secret env TEST_WEBHOOK_SECRET3
  on event *
    query: INSERT INTO event (type) VALUES (:event_type)
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	payload := `{"type":"test"}`
	status, _ := httpPostJSON(t, baseURL+"/hooks/nosig", payload, nil)

	if status != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing signature, got %d", status)
	}
}
