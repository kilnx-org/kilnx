package runtime

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestWriteWSPing(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_ = writeWSPing(client)
		client.Close()
	}()

	buf := make([]byte, 2)
	_, err := io.ReadFull(server, buf)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if buf[0] != 0x89 || buf[1] != 0x00 {
		t.Errorf("expected [0x89, 0x00], got [0x%02x, 0x%02x]", buf[0], buf[1])
	}
}

func TestWriteWSFrame_Medium(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	payload := bytes.Repeat([]byte("a"), 200)

	go func() {
		_ = writeWSFrame(client, payload)
		client.Close()
	}()

	buf := make([]byte, 204)
	_, err := io.ReadFull(server, buf)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if buf[0] != 0x81 {
		t.Errorf("opcode = 0x%02x, want 0x81", buf[0])
	}
	if buf[1] != 126 {
		t.Errorf("length indicator = %d, want 126", buf[1])
	}
	if binary.BigEndian.Uint16(buf[2:4]) != 200 {
		t.Errorf("payload length = %d, want 200", binary.BigEndian.Uint16(buf[2:4]))
	}
	if !bytes.Equal(buf[4:], payload) {
		t.Error("payload mismatch")
	}
}

func TestWriteWSFrame_Large(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	payload := bytes.Repeat([]byte("b"), 70000)

	go func() {
		_ = writeWSFrame(client, payload)
		client.Close()
	}()

	buf := make([]byte, 70010)
	_, err := io.ReadFull(server, buf)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if buf[0] != 0x81 {
		t.Errorf("opcode = 0x%02x, want 0x81", buf[0])
	}
	if buf[1] != 127 {
		t.Errorf("length indicator = %d, want 127", buf[1])
	}
	if binary.BigEndian.Uint64(buf[2:10]) != 70000 {
		t.Errorf("payload length = %d, want 70000", binary.BigEndian.Uint64(buf[2:10]))
	}
	if !bytes.Equal(buf[10:], payload) {
		t.Error("payload mismatch")
	}
}

func TestBroadcast(t *testing.T) {
	room := &Room{clients: make(map[net.Conn]bool)}

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	room.add(client)

	msg := []byte("hello broadcast")
	go room.broadcast(msg)

	buf := make([]byte, 2+len(msg))
	_, err := io.ReadFull(server, buf)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if buf[0] != 0x81 {
		t.Errorf("opcode = 0x%02x, want 0x81", buf[0])
	}
	if buf[1] != byte(len(msg)) {
		t.Errorf("length = %d, want %d", buf[1], len(msg))
	}
	if string(buf[2:]) != string(msg) {
		t.Errorf("payload = %q, want %q", string(buf[2:]), string(msg))
	}
}

func TestReadWSFrame_MediumPayload(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 200)
	frame := []byte{0x81, 126}
	var tmp [2]byte
	binary.BigEndian.PutUint16(tmp[:], uint16(len(payload)))
	frame = append(frame, tmp[:]...)
	frame = append(frame, payload...)

	reader := bufio.NewReader(bytes.NewReader(frame))
	gotPayload, opcode, err := readWSFrame(reader)
	if err != nil {
		t.Fatalf("readWSFrame error: %v", err)
	}
	if opcode != 0x01 {
		t.Errorf("opcode = 0x%02x, want 0x01", opcode)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Errorf("payload mismatch: got %d bytes, want %d", len(gotPayload), len(payload))
	}
}

func TestReadWSFrame_LargePayload(t *testing.T) {
	payload := bytes.Repeat([]byte("y"), 70000)
	frame := []byte{0x81, 127}
	var tmp [8]byte
	binary.BigEndian.PutUint64(tmp[:], uint64(len(payload)))
	frame = append(frame, tmp[:]...)
	frame = append(frame, payload...)

	reader := bufio.NewReader(bytes.NewReader(frame))
	gotPayload, opcode, err := readWSFrame(reader)
	if err != nil {
		t.Fatalf("readWSFrame error: %v", err)
	}
	if opcode != 0x01 {
		t.Errorf("opcode = 0x%02x, want 0x01", opcode)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Errorf("payload mismatch: got %d bytes, want %d", len(gotPayload), len(payload))
	}
}

func TestReadWSFrame_Masked(t *testing.T) {
	payload := []byte("hello")
	maskKey := []byte{0x01, 0x02, 0x03, 0x04}
	maskedPayload := make([]byte, len(payload))
	for i := range payload {
		maskedPayload[i] = payload[i] ^ maskKey[i%4]
	}
	frame := []byte{0x81, byte(0x80 | len(payload))}
	frame = append(frame, maskKey...)
	frame = append(frame, maskedPayload...)

	reader := bufio.NewReader(bytes.NewReader(frame))
	gotPayload, opcode, err := readWSFrame(reader)
	if err != nil {
		t.Fatalf("readWSFrame error: %v", err)
	}
	if opcode != 0x01 {
		t.Errorf("opcode = 0x%02x, want 0x01", opcode)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Errorf("payload mismatch: got %q, want %q", gotPayload, payload)
	}
}

func TestReadWSFrame_TooLarge(t *testing.T) {
	frame := []byte{0x81, 127}
	var tmp [8]byte
	binary.BigEndian.PutUint64(tmp[:], uint64(maxWSFrameSize+1))
	frame = append(frame, tmp[:]...)

	reader := bufio.NewReader(bytes.NewReader(frame))
	_, _, err := readWSFrame(reader)
	if err == nil {
		t.Fatal("expected error for oversized frame")
	}
}

func TestReadWSFrame_ReadError(t *testing.T) {
	frame := []byte{0x81} // incomplete frame
	reader := bufio.NewReader(bytes.NewReader(frame))
	_, _, err := readWSFrame(reader)
	if err == nil {
		t.Fatal("expected error for incomplete frame")
	}
}

func TestBroadcast_RemoveDeadClient(t *testing.T) {
	room := &Room{clients: make(map[net.Conn]bool)}

	client, server := net.Pipe()
	room.add(client)

	// Close server side to simulate dead client
	server.Close()

	// broadcast should not panic and should remove dead client
	room.broadcast([]byte("test"))

	if len(room.clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(room.clients))
	}
	client.Close()
}

func TestHandleSocket_AuthRequired(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("GET", "/ws", nil)
	rec := httptest.NewRecorder()

	sock := parser.Socket{Path: "/ws", Auth: true}
	s.handleSocket(rec, req, sock)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, want 401", rec.Code)
	}
}

func TestHandleSocket_RequiresRole(t *testing.T) {
	s := newTestServer(nil)
	s.app.Permissions = []parser.Permission{
		{Role: "admin", Rules: []string{"all"}},
	}
	// Create a viewer session with future expiry
	session := &Session{UserID: "1", Identity: "user@example.com", Role: "viewer", ExpiresAt: time.Now().Add(time.Hour)}
	s.sessions.sessions["session-viewer"] = session
	cookieVal := s.sessions.signSessionID("session-viewer")

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Cookie", "_kilnx_session="+cookieVal)
	rec := httptest.NewRecorder()

	sock := parser.Socket{Path: "/ws", Auth: true, RequiresRole: "admin"}
	s.handleSocket(rec, req, sock)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestHandleSocket_NoUpgrade(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("GET", "/ws", nil)
	rec := httptest.NewRecorder()

	sock := parser.Socket{Path: "/ws"}
	s.handleSocket(rec, req, sock)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", rec.Code)
	}
}

func TestHandleSocket_OriginMismatch(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Origin", "https://evil.com")
	req.Host = "good.com"
	rec := httptest.NewRecorder()

	sock := parser.Socket{Path: "/ws"}
	s.handleSocket(rec, req, sock)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestHandleSocket_MissingKey(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()

	sock := parser.Socket{Path: "/ws"}
	s.handleSocket(rec, req, sock)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", rec.Code)
	}
}

func TestHandleSocket_RequiresClauses(t *testing.T) {
	s := newTestServer(nil)
	// Create a viewer session with future expiry
	session := &Session{UserID: "1", Identity: "user@example.com", Role: "viewer", Data: database.Row{"plan": "basic"}, ExpiresAt: time.Now().Add(time.Hour)}
	s.sessions.sessions["session-viewer"] = session
	cookieVal := s.sessions.signSessionID("session-viewer")

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Cookie", "_kilnx_session="+cookieVal)
	rec := httptest.NewRecorder()

	sock := parser.Socket{
		Path: "/ws",
		Auth: true,
		RequiresClauses: []parser.RequiresClause{
			{Kind: parser.RequiresClauseExpr, Value: "current_user.plan == 'premium'"},
		},
	}
	s.handleSocket(rec, req, sock)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}
