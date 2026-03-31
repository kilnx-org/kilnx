package runtime

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestE2E_Stream_Connect(t *testing.T) {
	src := `
model counter
  value: int

stream /events
  every 1s
  query: SELECT 1 as alive
  event: update
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/events", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected text/event-stream content type, got %q", ct)
	}

	// Read at least one SSE event
	scanner := bufio.NewScanner(resp.Body)
	gotEvent := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "data:") {
			gotEvent = true
			break
		}
	}

	if !gotEvent {
		t.Error("expected to receive at least one SSE event")
	}
}

func TestE2E_Stream_AuthRequired(t *testing.T) {
	src := `
model user
  name: text
  email: email unique
  password: password

auth
  table: user
  identity: email
  password: password

stream /private/events requires auth
  every 1s
  query: SELECT 1 as n
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	status, _ := httpGet(t, baseURL+"/private/events")
	if status != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated stream access, got %d", status)
	}
}
