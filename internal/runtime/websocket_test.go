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
	if buf[0] != 0x81 || buf[1] != 126 {
		t.Errorf("expected [0x81, 126], got [0x%02x, %d]", buf[0], buf[1])
	}
	length := binary.BigEndian.Uint16(buf[2:4])
	if int(length) != len(payload) {
		t.Errorf("length = %d, want %d", length, len(payload))
	}
	if !bytes.Equal(buf[4:], payload) {
		t.Errorf("payload mismatch")
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
	if buf[0] != 0x81 || buf[1] != 127 {
		t.Errorf("expected [0x81, 127], got [0x%02x, %d]", buf[0], buf[1])
	}
	length := binary.BigEndian.Uint64(buf[2:10])
	if int(length) != len(payload) {
		t.Errorf("length = %d, want %d", length, len(payload))
	}
	if !bytes.Equal(buf[10:], payload) {
		t.Errorf("payload mismatch")
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

func TestReadWSFrame_ExtLengthError(t *testing.T) {
	// length == 126 but only 1 extra byte available
	frame := []byte{0x81, 126, 0x00}
	reader := bufio.NewReader(bytes.NewReader(frame))
	_, _, err := readWSFrame(reader)
	if err == nil {
		t.Fatal("expected error for incomplete extended length")
	}
}

func TestReadWSFrame_ExtLength8Error(t *testing.T) {
	// length == 127 but only 4 extra bytes available
	frame := []byte{0x81, 127, 0x00, 0x00, 0x00, 0x00}
	reader := bufio.NewReader(bytes.NewReader(frame))
	_, _, err := readWSFrame(reader)
	if err == nil {
		t.Fatal("expected error for incomplete 64-bit extended length")
	}
}

func TestReadWSFrame_MaskKeyError(t *testing.T) {
	// masked frame with only 1 byte of mask key
	frame := []byte{0x81, byte(0x80 | 5), 0x01}
	reader := bufio.NewReader(bytes.NewReader(frame))
	_, _, err := readWSFrame(reader)
	if err == nil {
		t.Fatal("expected error for incomplete mask key")
	}
}

func TestReadWSFrame_PayloadReadError(t *testing.T) {
	// header says 10 bytes but only 3 available
	frame := []byte{0x81, 10, 0x01, 0x02, 0x03}
	reader := bufio.NewReader(bytes.NewReader(frame))
	_, _, err := readWSFrame(reader)
	if err == nil {
		t.Fatal("expected error for incomplete payload")
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

// hijackableWriter is a ResponseWriter that supports Hijack for WebSocket tests.
type hijackableWriter struct {
	http.ResponseWriter
	conn     net.Conn
	bufrw    *bufio.ReadWriter
	hijacked bool
}

func (hw *hijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hw.hijacked = true
	return hw.conn, hw.bufrw, nil
}

func TestHandleSocket_HandshakeSuccess(t *testing.T) {
	// Create a pipe: clientConn is what the test reads, serverConn is what handleSocket writes to.
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	s := newTestServer(nil)
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	hw := &hijackableWriter{
		ResponseWriter: httptest.NewRecorder(),
		conn:           serverConn,
		bufrw:          bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn)),
	}

	sock := parser.Socket{Path: "/ws", Auth: false}

	// Run handleSocket in a goroutine because it blocks in the read loop.
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleSocket(hw, req, sock)
	}()

	// Read the HTTP upgrade response from clientConn.
	reader := bufio.NewReader(clientConn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		t.Fatalf("failed to read upgrade response: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("status = %d, want 101", resp.StatusCode)
	}
	if resp.Header.Get("Sec-WebSocket-Accept") == "" {
		t.Error("expected Sec-WebSocket-Accept header")
	}

	// Send a close frame to exercise the read loop close-path
	closeFrame := []byte{0x88, 0x00}
	clientConn.Write(closeFrame)

	// Give handleSocket a moment to process the frame
	time.Sleep(100 * time.Millisecond)

	// Close the connection to make the read loop exit.
	clientConn.Close()
	select {
	case <-done:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("handleSocket did not exit after connection close")
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

func TestHandleSocket_RequiresClausesForbidden(t *testing.T) {
	s := newTestServer(nil)
	s.app.Permissions = []parser.Permission{
		{Role: "viewer", Rules: []string{"all"}},
	}
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
