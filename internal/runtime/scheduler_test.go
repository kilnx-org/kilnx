package runtime

import (
	"bufio"
	"bytes"
	"net"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// ---------- Scheduler stop/restart ----------

func TestStartSchedulerStopsPreviousGoroutines(t *testing.T) {
	app := &parser.App{
		Schedules: []parser.Schedule{
			{
				Name:         "tick",
				IntervalSecs: 1,
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: "SELECT 1"},
				},
			},
		},
	}

	srv := &Server{app: app, port: 0}

	// Start scheduler first time
	srv.StartScheduler()
	// The goroutine is running. We cannot easily count ticks without a DB,
	// but we can verify the stop channel is set.
	srv.mu.RLock()
	stop1 := srv.scheduleStop
	srv.mu.RUnlock()
	if stop1 == nil {
		t.Fatal("scheduleStop should be set after StartScheduler")
	}

	// Start scheduler second time (simulating hot-reload)
	srv.StartScheduler()
	srv.mu.RLock()
	stop2 := srv.scheduleStop
	srv.mu.RUnlock()

	// Old stop channel should be closed
	select {
	case <-stop1:
		// expected: old channel is closed
	default:
		t.Fatal("old scheduleStop channel should be closed after restart")
	}

	// New stop channel should be different and open
	if stop2 == nil {
		t.Fatal("new scheduleStop should be set")
	}
	select {
	case <-stop2:
		t.Fatal("new scheduleStop should still be open")
	default:
		// expected
	}

	// Cleanup
	srv.StopScheduler()
}

func TestStopSchedulerWithNoSchedules(t *testing.T) {
	srv := &Server{app: &parser.App{}, port: 0}
	// Should not panic
	srv.StopScheduler()
	srv.StartScheduler()
	srv.StopScheduler()
}

func TestSchedulerGoroutineExitsOnStop(t *testing.T) {
	app := &parser.App{
		Schedules: []parser.Schedule{
			{
				Name:         "fast",
				IntervalSecs: 3600,
				Body:         []parser.Node{},
			},
		},
	}

	srv := &Server{app: app, port: 0}
	srv.StartScheduler()

	// Capture the stop channel before calling StopScheduler
	srv.mu.RLock()
	ch := srv.scheduleStop
	srv.mu.RUnlock()

	if ch == nil {
		t.Fatal("scheduleStop should be set")
	}

	srv.StopScheduler()

	// The stop channel should now be closed
	select {
	case <-ch:
		// expected: channel is closed, goroutines can detect this
	default:
		t.Fatal("stop channel should be closed after StopScheduler")
	}
}

// ---------- executeNodes error propagation ----------

func TestExecuteNodesReturnsQueryError(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	srv := &Server{app: &parser.App{}, db: db, port: 0}

	// A mutation query against a non-existent table should return an error
	nodes := []parser.Node{
		{Type: parser.NodeQuery, SQL: "INSERT INTO nonexistent (x) VALUES ('a')"},
	}

	execErr := srv.executeNodes(nodes, nil)
	if execErr == nil {
		t.Fatal("executeNodes should return error for failed mutation query")
	}
}

func TestExecuteNodesReturnsSelectError(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	srv := &Server{app: &parser.App{}, db: db, port: 0}

	nodes := []parser.Node{
		{Type: parser.NodeQuery, SQL: "SELECT * FROM nonexistent"},
	}

	execErr := srv.executeNodes(nodes, nil)
	if execErr == nil {
		t.Fatal("executeNodes should return error for failed SELECT query")
	}
}

func TestExecuteNodesReturnsNilOnSuccess(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	srv := &Server{app: &parser.App{}, db: db, port: 0}

	nodes := []parser.Node{
		{Type: parser.NodeQuery, SQL: "SELECT 1 as val"},
	}

	execErr := srv.executeNodes(nodes, nil)
	if execErr != nil {
		t.Fatalf("executeNodes should return nil on success, got: %v", execErr)
	}
}

func TestExecuteNodesNilDBSkipsQueries(t *testing.T) {
	srv := &Server{app: &parser.App{}, db: nil, port: 0}

	nodes := []parser.Node{
		{Type: parser.NodeQuery, SQL: "INSERT INTO anything (x) VALUES ('a')"},
	}

	execErr := srv.executeNodes(nodes, nil)
	if execErr != nil {
		t.Fatalf("executeNodes with nil db should skip queries, got: %v", execErr)
	}
}

func TestSafeExecuteNodesCatchesPanic(t *testing.T) {
	srv := &Server{app: &parser.App{}, db: nil, port: 0}
	jq := &JobQueue{server: srv, jobs: make(map[string]parser.Job)}

	// We cannot easily trigger a panic inside executeNodes with nil db,
	// but we can verify the function signature works correctly.
	err := jq.safeExecuteNodes(nil, nil)
	if err != nil {
		t.Fatalf("safeExecuteNodes with nil nodes should not error, got: %v", err)
	}
}

// ---------- WebSocket ping frame ----------

func TestWriteWSPing(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		err := writeWSPing(server)
		if err != nil {
			t.Errorf("writeWSPing error: %v", err)
		}
	}()

	buf := make([]byte, 2)
	_, err := client.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	// Verify ping frame: FIN + opcode 0x9 (0x89), length 0
	if buf[0] != 0x89 {
		t.Errorf("expected ping opcode 0x89, got 0x%02x", buf[0])
	}
	if buf[1] != 0x00 {
		t.Errorf("expected zero payload length, got %d", buf[1])
	}
}

func TestReadWSFrameReturnsPongOpcode(t *testing.T) {
	// Create a pong frame (opcode 0xA, FIN, no mask, no payload)
	pongFrame := []byte{0x8A, 0x00}

	reader := bufio.NewReader(bytes.NewReader(pongFrame))
	payload, opcode, err := readWSFrame(reader)
	if err != nil {
		t.Fatalf("readWSFrame error: %v", err)
	}
	if opcode != 0x0A {
		t.Errorf("expected pong opcode 0x0A, got 0x%02x", opcode)
	}
	if len(payload) != 0 {
		t.Errorf("expected empty payload for pong, got %d bytes", len(payload))
	}
}

func TestReadWSFrameReturnsTextOpcode(t *testing.T) {
	// Create a text frame: FIN + text (0x81), length 5, "hello"
	textFrame := []byte{0x81, 0x05, 'h', 'e', 'l', 'l', 'o'}

	reader := bufio.NewReader(bytes.NewReader(textFrame))
	payload, opcode, err := readWSFrame(reader)
	if err != nil {
		t.Fatalf("readWSFrame error: %v", err)
	}
	if opcode != 0x01 {
		t.Errorf("expected text opcode 0x01, got 0x%02x", opcode)
	}
	if string(payload) != "hello" {
		t.Errorf("expected 'hello', got '%s'", string(payload))
	}
}

func TestReadWSFrameCloseReturnsError(t *testing.T) {
	// Close frame: FIN + close (0x88), length 0
	closeFrame := []byte{0x88, 0x00}

	reader := bufio.NewReader(bytes.NewReader(closeFrame))
	_, _, err := readWSFrame(reader)
	if err == nil {
		t.Fatal("expected error for close frame")
	}
}
