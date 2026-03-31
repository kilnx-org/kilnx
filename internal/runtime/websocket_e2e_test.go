package runtime

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestE2E_WebSocket_UpgradeRequired(t *testing.T) {
	src := `
model message
  body: text

socket /ws/chat
  on message
    query: INSERT INTO message (body) VALUES (:message)
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	// Regular HTTP GET to a WebSocket path should return 400
	status, _ := httpGet(t, baseURL+"/ws/chat")
	if status != http.StatusBadRequest {
		t.Errorf("expected 400 for non-WebSocket request to socket path, got %d", status)
	}
}

func TestE2E_WebSocket_Handshake(t *testing.T) {
	src := `
model message
  body: text

socket /ws/chat
  on message
    query: INSERT INTO message (body) VALUES (:message)
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	host := strings.TrimPrefix(baseURL, "http://")

	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	key := base64.StdEncoding.EncodeToString([]byte("test-websocket-key!"))
	req := fmt.Sprintf("GET /ws/chat HTTP/1.1\r\n"+
		"Host: %s\r\n"+
		"Upgrade: websocket\r\n"+
		"Connection: Upgrade\r\n"+
		"Sec-WebSocket-Key: %s\r\n"+
		"Sec-WebSocket-Version: 13\r\n"+
		"Origin: http://%s\r\n"+
		"\r\n", host, key, host)

	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("write: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	reader := bufio.NewReader(conn)

	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}

	if !strings.Contains(statusLine, "101") {
		t.Fatalf("expected 101 Switching Protocols, got %q", strings.TrimSpace(statusLine))
	}

	// Verify Sec-WebSocket-Accept
	expectedAccept := computeWebSocketAccept(key)
	gotAccept := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Sec-WebSocket-Accept:") {
			accept := strings.TrimSpace(strings.TrimPrefix(line, "Sec-WebSocket-Accept:"))
			if accept == expectedAccept {
				gotAccept = true
			} else {
				t.Errorf("wrong Sec-WebSocket-Accept: got %q, want %q", accept, expectedAccept)
			}
		}
	}
	if !gotAccept {
		t.Error("response missing Sec-WebSocket-Accept header")
	}
}

func TestE2E_WebSocket_AuthRequired(t *testing.T) {
	src := `
model user
  name: text
  email: email unique
  password: password

auth
  table: user
  identity: email
  password: password

socket /ws/private requires auth
  on connect
    query: SELECT 1
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	// Without auth, socket should reject
	host := strings.TrimPrefix(baseURL, "http://")
	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	key := base64.StdEncoding.EncodeToString([]byte("test-ws-auth-key!!"))
	req := fmt.Sprintf("GET /ws/private HTTP/1.1\r\n"+
		"Host: %s\r\n"+
		"Upgrade: websocket\r\n"+
		"Connection: Upgrade\r\n"+
		"Sec-WebSocket-Key: %s\r\n"+
		"Sec-WebSocket-Version: 13\r\n"+
		"Origin: http://%s\r\n"+
		"\r\n", host, key, host)

	conn.Write([]byte(req))
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	reader := bufio.NewReader(conn)

	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}

	// Should not get 101 without auth
	if strings.Contains(statusLine, "101") {
		t.Error("expected auth rejection, but got 101 Switching Protocols")
	}
}

func computeWebSocketAccept(key string) string {
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// TestE2E_WebSocket_404 ensures non-socket paths return normal HTTP responses
func TestE2E_WebSocket_NonExistentPath(t *testing.T) {
	src := `
page /
  html
    <h1>Home</h1>
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	status, _ := httpGet(t, baseURL+"/ws/nonexistent")
	if status != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent path, got %d", status)
	}
}
