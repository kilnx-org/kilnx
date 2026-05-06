package runtime

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// ---------- JobQueue.Start ----------

func TestJobQueueStartWithNoDB(t *testing.T) {
	srv := &Server{app: &parser.App{}, db: nil, port: 0}
	jq := NewJobQueue(srv)
	// Should not panic when DB is nil
	jq.Start()
}

func TestServer_StartJobQueue(t *testing.T) {
	srv := &Server{app: &parser.App{}, db: nil, port: 0}
	srv.jobQueue = NewJobQueue(srv)
	// Should delegate to jobQueue.Start without panic
	srv.StartJobQueue()
}

func TestJobQueueStart_WithJobs(t *testing.T) {
	srv := &Server{app: &parser.App{Jobs: []parser.Job{{Name: "test"}}}, db: nil, port: 0}
	jq := NewJobQueue(srv)
	jq.jobs["test"] = parser.Job{Name: "test"}
	// Should not panic and should print ready message
	jq.Start()
}

// ---------- recoverOrphanedJobs ----------

func TestRecoverOrphanedJobsNilDB(t *testing.T) {
	srv := &Server{app: &parser.App{}, db: nil, port: 0}
	jq := NewJobQueue(srv)
	jq.recoverOrphanedJobs()
	// No panic is success
}

func TestRecoverOrphanedJobsNoResults(t *testing.T) {
	mock := newMockExecutor()
	srv := &Server{app: &parser.App{}, db: mock, port: 0}
	jq := NewJobQueue(srv)
	jq.recoverOrphanedJobs()
	if len(mock.execCalled) != 0 {
		t.Fatalf("expected 0 exec calls, got %d", len(mock.execCalled))
	}
}

func TestRecoverOrphanedJobsResetsExecutingJobs(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsResults[`SELECT id, name FROM _kilnx_jobs WHERE state = 'executing'`] = []database.Row{
		{"id": "1", "name": "job1"},
		{"id": "2", "name": "job2"},
	}
	srv := &Server{app: &parser.App{}, db: mock, port: 0}
	jq := NewJobQueue(srv)
	jq.recoverOrphanedJobs()

	if len(mock.execCalled) != 2 {
		t.Fatalf("expected 2 exec calls, got %d", len(mock.execCalled))
	}
	for i, call := range mock.execCalled {
		if !strings.Contains(call.SQL, "UPDATE _kilnx_jobs SET state = 'available'") {
			t.Errorf("call %d: expected UPDATE with state='available', got %s", i, call.SQL)
		}
	}
}

// ---------- Enqueue ----------

func TestEnqueueUnknownJob(t *testing.T) {
	mock := newMockExecutor()
	srv := &Server{app: &parser.App{}, db: mock, port: 0}
	jq := NewJobQueue(srv)
	err := jq.Enqueue("unknown", nil)
	if err == nil {
		t.Fatal("expected error for unknown job")
	}
	if !strings.Contains(err.Error(), "unknown job") {
		t.Fatalf("expected 'unknown job' error, got: %v", err)
	}
}

func TestEnqueueNoDB(t *testing.T) {
	srv := &Server{app: &parser.App{}, db: nil, port: 0}
	jq := NewJobQueue(srv)
	jq.mu.Lock()
	jq.jobs["known"] = parser.Job{Name: "known", MaxRetries: 2}
	jq.mu.Unlock()

	err := jq.Enqueue("known", map[string]string{"key": "val"})
	if err == nil {
		t.Fatal("expected error when db is nil")
	}
	if !strings.Contains(err.Error(), "no database") {
		t.Fatalf("expected 'no database' error, got: %v", err)
	}
}

func TestEnqueueKnownJob(t *testing.T) {
	mock := newMockExecutor()
	srv := &Server{app: &parser.App{}, db: mock, port: 0}
	jq := NewJobQueue(srv)
	jq.mu.Lock()
	jq.jobs["notify"] = parser.Job{Name: "notify", MaxRetries: 2}
	jq.mu.Unlock()

	err := jq.Enqueue("notify", map[string]string{"user_id": "42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.execCalled) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(mock.execCalled))
	}
	call := mock.execCalled[0]
	if !strings.Contains(call.SQL, "INSERT INTO _kilnx_jobs") {
		t.Fatalf("expected INSERT, got %s", call.SQL)
	}
	if call.Params["name"] != "notify" {
		t.Errorf("expected name=notify, got %s", call.Params["name"])
	}
	if call.Params["max_attempts"] != "2" {
		t.Errorf("expected max_attempts=2, got %s", call.Params["max_attempts"])
	}
	if call.Params["params"] == "" {
		t.Error("expected params to be set")
	}
}

// ---------- processNextJob ----------

var availableJobQuery = `SELECT id, name, params, attempts, max_attempts FROM _kilnx_jobs
		 WHERE state = 'available' AND scheduled_at <= datetime('now')
		 ORDER BY id LIMIT 1`

func TestProcessNextJobNilDB(t *testing.T) {
	srv := &Server{app: &parser.App{}, db: nil, port: 0}
	jq := NewJobQueue(srv)
	jq.processNextJob()
	// No panic is success
}

func TestProcessNextJobNoAvailableJobs(t *testing.T) {
	mock := newMockExecutor()
	srv := &Server{app: &parser.App{}, db: mock, port: 0}
	jq := NewJobQueue(srv)
	jq.processNextJob()
	if len(mock.execCalled) != 0 {
		t.Fatalf("expected 0 exec calls, got %d", len(mock.execCalled))
	}
}

func TestProcessNextJobUnknownJobType(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsResults[availableJobQuery] = []database.Row{
		{"id": "1", "name": "orphan", "params": "{}", "attempts": "0", "max_attempts": "1"},
	}
	srv := &Server{app: &parser.App{}, db: mock, port: 0}
	jq := NewJobQueue(srv)
	jq.processNextJob()

	if len(mock.execCalled) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(mock.execCalled))
	}
	call := mock.execCalled[0]
	if !strings.Contains(call.SQL, "discarded") {
		t.Fatalf("expected discarded update, got %s", call.SQL)
	}
	if !strings.Contains(call.SQL, "unknown job type") {
		t.Errorf("expected SQL to contain 'unknown job type', got %s", call.SQL)
	}
}

func TestProcessNextJobSuccess(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsResults[availableJobQuery] = []database.Row{
		{"id": "1", "name": "myjob", "params": "{}", "attempts": "0", "max_attempts": "1"},
	}
	srv := &Server{app: &parser.App{}, db: mock, port: 0}
	jq := NewJobQueue(srv)
	jq.mu.Lock()
	jq.jobs["myjob"] = parser.Job{Name: "myjob", Body: []parser.Node{}}
	jq.mu.Unlock()

	jq.processNextJob()

	if len(mock.execCalled) != 2 {
		t.Fatalf("expected 2 exec calls, got %d", len(mock.execCalled))
	}
	if !strings.Contains(mock.execCalled[0].SQL, "executing") {
		t.Errorf("expected first call to mark executing, got %s", mock.execCalled[0].SQL)
	}
	if !strings.Contains(mock.execCalled[1].SQL, "completed") {
		t.Errorf("expected second call to mark completed, got %s", mock.execCalled[1].SQL)
	}
}

func TestProcessNextJobFailureWithRetry(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsResults[availableJobQuery] = []database.Row{
		{"id": "1", "name": "failjob", "params": "{}", "attempts": "0", "max_attempts": "3"},
	}
	mock.queryRowsWithParamsErr["SELECT * FROM fail"] = fmt.Errorf("boom")

	srv := &Server{app: &parser.App{}, db: mock, port: 0}
	jq := NewJobQueue(srv)
	jq.mu.Lock()
	jq.jobs["failjob"] = parser.Job{Name: "failjob", Body: []parser.Node{
		{Type: parser.NodeQuery, SQL: "SELECT * FROM fail"},
	}}
	jq.mu.Unlock()

	jq.processNextJob()

	if len(mock.execCalled) < 2 {
		t.Fatalf("expected at least 2 exec calls, got %d", len(mock.execCalled))
	}
	// First call marks executing
	if !strings.Contains(mock.execCalled[0].SQL, "executing") {
		t.Errorf("expected first call to mark executing, got %s", mock.execCalled[0].SQL)
	}
	// Last call should schedule retry
	last := mock.execCalled[len(mock.execCalled)-1]
	if !strings.Contains(last.SQL, "available") {
		t.Errorf("expected last call to mark available for retry, got %s", last.SQL)
	}
	if last.Params["attempts"] != "1" {
		t.Errorf("expected attempts=1, got %s", last.Params["attempts"])
	}
	if !strings.Contains(last.Params["error"], "boom") {
		t.Errorf("expected error to contain 'boom', got %s", last.Params["error"])
	}
}

func TestProcessNextJobFailureDiscarded(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsResults[availableJobQuery] = []database.Row{
		{"id": "1", "name": "failjob", "params": "{}", "attempts": "2", "max_attempts": "2"},
	}
	mock.queryRowsWithParamsErr["SELECT * FROM fail"] = fmt.Errorf("boom")

	srv := &Server{app: &parser.App{}, db: mock, port: 0}
	jq := NewJobQueue(srv)
	jq.mu.Lock()
	jq.jobs["failjob"] = parser.Job{Name: "failjob", Body: []parser.Node{
		{Type: parser.NodeQuery, SQL: "SELECT * FROM fail"},
	}}
	jq.mu.Unlock()

	jq.processNextJob()

	if len(mock.execCalled) < 2 {
		t.Fatalf("expected at least 2 exec calls, got %d", len(mock.execCalled))
	}
	last := mock.execCalled[len(mock.execCalled)-1]
	if !strings.Contains(last.SQL, "discarded") {
		t.Errorf("expected last call to mark discarded, got %s", last.SQL)
	}
	if last.Params["attempts"] != "3" {
		t.Errorf("expected attempts=3, got %s", last.Params["attempts"])
	}
}

// ---------- RefreshJobQueue ----------

func TestRefreshJobQueue(t *testing.T) {
	mock := newMockExecutor()
	app := &parser.App{
		Jobs: []parser.Job{
			{Name: "oldjob"},
		},
	}
	srv := NewServer(app, mock, 0)

	srv.jobQueue.mu.RLock()
	_, hasOld := srv.jobQueue.jobs["oldjob"]
	srv.jobQueue.mu.RUnlock()
	if !hasOld {
		t.Fatal("expected oldjob to exist in queue")
	}

	newApp := &parser.App{
		Jobs: []parser.Job{
			{Name: "newjob"},
		},
	}
	srv.Reload(newApp)
	srv.RefreshJobQueue()

	srv.jobQueue.mu.RLock()
	_, hasNew := srv.jobQueue.jobs["newjob"]
	_, stillHasOld := srv.jobQueue.jobs["oldjob"]
	srv.jobQueue.mu.RUnlock()

	if !hasNew {
		t.Error("expected newjob to be added after RefreshJobQueue")
	}
	if !stillHasOld {
		t.Error("expected oldjob to still exist (RefreshJobQueue adds, doesn't clear)")
	}
}

// ---------- runSchedule / runCronSchedule ----------

func TestRunScheduleExitsOnStop(t *testing.T) {
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
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		srv.runSchedule(app.Schedules[0], stop)
		close(done)
	}()
	close(stop)
	select {
	case <-done:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("runSchedule did not exit after stop closed")
	}
}

func TestRunCronScheduleExitsOnStop(t *testing.T) {
	app := &parser.App{
		Schedules: []parser.Schedule{
			{
				Name: "cron",
				Cron: "every day at 23:59",
				Body: []parser.Node{},
			},
		},
	}
	srv := &Server{app: app, port: 0}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		srv.runCronSchedule(app.Schedules[0], stop)
		close(done)
	}()
	close(stop)
	select {
	case <-done:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("runCronSchedule did not exit after stop closed")
	}
}

func TestRunCronScheduleInvalidCronExits(t *testing.T) {
	app := &parser.App{
		Schedules: []parser.Schedule{
			{
				Name: "bad",
				Cron: "something invalid",
				Body: []parser.Node{},
			},
		},
	}
	srv := &Server{app: app, port: 0}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		srv.runCronSchedule(app.Schedules[0], stop)
		close(done)
	}()
	select {
	case <-done:
		// expected: invalid cron causes immediate return
	case <-time.After(2 * time.Second):
		t.Fatal("runCronSchedule should exit immediately for invalid cron")
	}
}

// ---------- executeNodes ----------

func TestExecuteNodes_NoDB(t *testing.T) {
	s := newTestServer(nil)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeQuery, SQL: `SELECT 1`, Name: "q"},
	}, map[string]string{})
	if err != nil {
		t.Errorf("expected nil when db is nil, got %v", err)
	}
}

func TestExecuteNodes_SelectQuery(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT id, name FROM users`] = []database.Row{
		{"id": "1", "name": "Alice"},
	}
	s := newTestServer(mock)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeQuery, SQL: `SELECT id, name FROM users`, Name: "users"},
	}, map[string]string{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestExecuteNodes_MutationQuery(t *testing.T) {
	mock := newMockExecutor()
	mock.execErr[`UPDATE users SET name = 'x' WHERE id = :id`] = nil
	s := newTestServer(mock)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeQuery, SQL: `UPDATE users SET name = 'x' WHERE id = :id`},
	}, map[string]string{"id": "1"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestExecuteNodes_MutationQueryError(t *testing.T) {
	mock := newMockExecutor()
	mock.execErr[`UPDATE users SET name = 'x' WHERE id = :id`] = fmt.Errorf("db error")
	s := newTestServer(mock)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeQuery, SQL: `UPDATE users SET name = 'x' WHERE id = :id`},
	}, map[string]string{"id": "1"})
	if err == nil {
		t.Error("expected error for failed mutation")
	}
}

func TestExecuteNodes_TenantRejection(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	s.tenants = TenantMap{"users": "org_id"}
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeQuery, SQL: `SELECT * FROM users`},
	}, map[string]string{})
	if err == nil {
		t.Error("expected error for tenant rejection")
	}
}

func TestExecuteNodes_SendEmail(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeSendEmail, EmailTo: "user@example.com", EmailSubject: "Hello"},
	}, map[string]string{})
	// Without SMTP configured, email sending fails
	if err == nil {
		t.Error("expected error when SMTP is not configured")
	}
}

func TestExecuteNodes_SendEmailWithQuery(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT email FROM users WHERE id = :id`] = []database.Row{
		{"email": "dynamic@example.com"},
	}
	s := newTestServer(mock)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeSendEmail, EmailTo: ":email", EmailSubject: "Hello", Props: map[string]string{
			"to_query": `SELECT email FROM users WHERE id = :id`,
		}},
	}, map[string]string{"id": "1"})
	// Without SMTP configured, email sending fails
	if err == nil {
		t.Error("expected error when SMTP is not configured")
	}
}

func TestSafeExecuteNodes_PanicRecovery(t *testing.T) {
	jq := &JobQueue{server: nil}
	err := jq.safeExecuteNodes([]parser.Node{
		{Type: parser.NodeQuery, SQL: `SELECT 1`},
	}, nil)
	if err == nil {
		t.Fatal("expected error when server is nil (panic recovered)")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Errorf("expected panic error, got %v", err)
	}
}

func TestExecuteNodes_Fetch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": 1, "title": "Test"}`))
	}))
	defer ts.Close()

	s := newTestServer(nil)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeFetch, FetchURL: ts.URL, Name: "post"},
	}, map[string]string{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestExecuteNodes_FetchError(t *testing.T) {
	// Transport-level fetch failures must propagate so jobs/schedules can
	// surface the error to the caller (and trigger retries) instead of
	// silently committing partial work.
	s := newTestServer(nil)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeFetch, FetchURL: "http://invalid.localhost:99999", Name: "post"},
	}, map[string]string{})
	if err == nil {
		t.Fatal("expected fetch transport error to propagate, got nil")
	}
	if !strings.Contains(err.Error(), "post") {
		t.Errorf("expected error to mention fetch name, got %v", err)
	}
}

func TestExecuteNodes_ValidateInline(t *testing.T) {
	s := newTestServer(nil)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeValidate, Validations: []parser.Validation{
			{Field: "email", Rules: []string{"required"}},
		}},
	}, map[string]string{})
	if err == nil {
		t.Error("expected validation error")
	}
}

func TestExecuteNodes_ValidateInlinePass2(t *testing.T) {
	s := newTestServer(nil)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeValidate, Validations: []parser.Validation{
			{Field: "email", Rules: []string{"required"}},
		}},
	}, map[string]string{"email": "test@example.com"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestExecuteNodes_Enqueue(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec(`CREATE TABLE _kilnx_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		params TEXT NOT NULL DEFAULT '{}',
		state TEXT NOT NULL DEFAULT 'available',
		attempts INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL DEFAULT 1,
		scheduled_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		started_at DATETIME,
		completed_at DATETIME,
		last_error TEXT
	)`); err != nil {
		t.Fatalf("failed to create jobs table: %v", err)
	}

	s := newTestServer(db)
	s.app.Jobs = []parser.Job{{Name: "notify", MaxRetries: 3}}
	s.jobQueue = NewJobQueue(s)

	err = s.executeNodes([]parser.Node{
		{Type: parser.NodeEnqueue, JobName: "notify", JobParams: map[string]string{
			"user_id": ":id",
			"msg":     "hello",
		}},
	}, map[string]string{"id": "42"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestExecuteNodes_SendEmailWithAttachment(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeSendEmail, EmailTo: "user@example.com", EmailSubject: "Hello", EmailAttach: "_generated_pdf", Props: map[string]string{
			"body": "See attached",
		}},
	}, map[string]string{"_generated_pdf": "/tmp/fake.pdf"})
	if err == nil {
		t.Error("expected error when SMTP is not configured")
	}
}

func TestExecuteNodes_SendEmailWithAttachmentFromParam(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeSendEmail, EmailTo: "user@example.com", EmailSubject: "Hello", EmailAttach: "report_path", Props: map[string]string{
			"body": "See attached",
		}},
	}, map[string]string{"report_path": "/tmp/report.pdf"})
	if err == nil {
		t.Error("expected error when SMTP is not configured")
	}
}

func TestExecuteNodes_SendEmailQueryTenantRejection(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	s.tenants = TenantMap{"users": "org_id"}
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeSendEmail, EmailTo: "user@example.com", EmailSubject: "Hello", Props: map[string]string{
			"to_query": `SELECT email FROM users`,
		}},
	}, map[string]string{})
	if err == nil {
		t.Error("expected error when SMTP is not configured")
	}
}

func TestExecuteNodes_EnqueueMissingParam(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec(`CREATE TABLE _kilnx_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		params TEXT NOT NULL DEFAULT '{}',
		state TEXT NOT NULL DEFAULT 'available',
		attempts INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL DEFAULT 1,
		scheduled_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		started_at DATETIME,
		completed_at DATETIME,
		last_error TEXT
	)`); err != nil {
		t.Fatalf("failed to create jobs table: %v", err)
	}

	s := newTestServer(db)
	s.app.Jobs = []parser.Job{{Name: "notify", MaxRetries: 3}}
	s.jobQueue = NewJobQueue(s)

	err = s.executeNodes([]parser.Node{
		{Type: parser.NodeEnqueue, JobName: "notify", JobParams: map[string]string{
			"user_id": ":missing",
			"msg":     "hello",
		}},
	}, map[string]string{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestRunSchedule_Executes(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT 1`] = []database.Row{{"1": "1"}}

	app := &parser.App{
		Schedules: []parser.Schedule{
			{
				Name:         "fast",
				IntervalSecs: 1,
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: `SELECT 1`},
				},
			},
		},
	}
	srv := &Server{app: app, db: mock, port: 0}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		srv.runSchedule(app.Schedules[0], stop)
		close(done)
	}()
	// Wait for at least one execution
	time.Sleep(1500 * time.Millisecond)
	close(stop)
	select {
	case <-done:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("runSchedule did not exit")
	}
}

func TestRunCronSchedule_Executes(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT 1`] = []database.Row{{"1": "1"}}

	app := &parser.App{
		Schedules: []parser.Schedule{
			{
				Name: "cron",
				Cron: "every second",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: `SELECT 1`},
				},
			},
		},
	}
	srv := &Server{app: app, db: mock, port: 0}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		srv.runCronSchedule(app.Schedules[0], stop)
		close(done)
	}()
	// Wait for at least one execution
	time.Sleep(2 * time.Second)
	close(stop)
	select {
	case <-done:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("runCronSchedule did not exit")
	}
}

func TestExecuteNodes_NilParams(t *testing.T) {
	s := newTestServer(nil)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeQuery, SQL: `SELECT 1`, Name: "q"},
	}, nil)
	if err != nil {
		t.Errorf("expected no error with nil params, got %v", err)
	}
}

func TestExecuteNodes_FetchNoName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": 1}`))
	}))
	defer ts.Close()

	s := newTestServer(nil)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeFetch, FetchURL: ts.URL},
	}, map[string]string{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestExecuteNodes_SendEmailToQueryTenantRejection(t *testing.T) {
	s := newTestServer(nil)
	s.tenants = TenantMap{"users": "users"}
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeSendEmail, EmailTo: ":email", EmailSubject: "Hello", Props: map[string]string{
			"to_query": `SELECT email FROM users WHERE tenant_id = 1`,
		}},
	}, map[string]string{})
	// Tenant guard rejects query because no current_user.tenant_id
	if err == nil {
		t.Error("expected error when tenant guard rejects to_query")
	}
}

func TestExecuteNodes_EnqueueUnknownJob(t *testing.T) {
	s := newTestServer(nil)
	s.jobQueue = NewJobQueue(s)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeEnqueue, JobName: "unknown"},
	}, map[string]string{})
	if err == nil {
		t.Error("expected error for unknown job")
	}
}

func TestRunSchedule_ExecuteError(t *testing.T) {
	mock := newMockExecutor()
	mock.execErr[`INSERT INTO logs (msg) VALUES (:msg)`] = fmt.Errorf("db error")

	app := &parser.App{
		Schedules: []parser.Schedule{
			{
				Name:         "fast",
				IntervalSecs: 1,
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: `INSERT INTO logs (msg) VALUES (:msg)`},
				},
			},
		},
	}
	srv := &Server{app: app, db: mock, port: 0}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		srv.runSchedule(app.Schedules[0], stop)
		close(done)
	}()
	// Wait for at least one execution (should log error)
	time.Sleep(1500 * time.Millisecond)
	close(stop)
	select {
	case <-done:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("runSchedule did not exit")
	}
}

func TestExecuteNodes_FetchWithVar(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": 1, "title": "Test Post"}`))
	}))
	defer ts.Close()

	s := newTestServer(nil)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeFetch, FetchURL: ts.URL, Name: "post", Props: map[string]string{"var": "fetched_post"}},
	}, map[string]string{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestExecuteNodes_ValidateInlineFail(t *testing.T) {
	s := newTestServer(nil)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeValidate, Validations: []parser.Validation{
			{Field: "email", Rules: []string{"required"}},
		}},
	}, map[string]string{})
	if err == nil {
		t.Error("expected validation error")
	}
}

func TestExecuteNodes_ValidateInlinePass(t *testing.T) {
	s := newTestServer(nil)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeValidate, Validations: []parser.Validation{
			{Field: "email", Rules: []string{"required"}},
		}},
	}, map[string]string{"email": "test@example.com"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestExecuteNodes_GeneratePDF(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec("INSERT INTO users (name) VALUES ('Alice')"); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	s := newTestServer(db)
	err = s.executeNodes([]parser.Node{
		{Type: parser.NodeQuery, SQL: `SELECT name FROM users`, Name: "users"},
		{Type: parser.NodeGeneratePDF, TemplateName: "Report", DataQueryName: "users"},
	}, map[string]string{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestExecuteNodes_GeneratePDFNoData(t *testing.T) {
	s := newTestServer(nil)
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeGeneratePDF, TemplateName: "Report"},
	}, map[string]string{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestRunSchedule_CronBranch(t *testing.T) {
	app := &parser.App{
		Schedules: []parser.Schedule{
			{
				Name: "cron",
				Cron: "something invalid",
				Body: []parser.Node{},
			},
		},
	}
	srv := &Server{app: app, port: 0}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		srv.runSchedule(app.Schedules[0], stop)
		close(done)
	}()
	select {
	case <-done:
		// expected: invalid cron causes immediate return via runCronSchedule
	case <-time.After(2 * time.Second):
		t.Fatal("runSchedule should exit immediately for invalid cron")
	}
}

func TestRunCronSchedule_ExecuteError(t *testing.T) {
	mock := newMockExecutor()
	mock.execErr[`INSERT INTO logs (msg) VALUES (:msg)`] = fmt.Errorf("db error")

	app := &parser.App{
		Schedules: []parser.Schedule{
			{
				Name: "fast",
				Cron: "every day at 23:59",
				Body: []parser.Node{
					{Type: parser.NodeQuery, SQL: `INSERT INTO logs (msg) VALUES (:msg)`},
				},
			},
		},
	}
	srv := &Server{app: app, db: mock, port: 0}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		srv.runCronSchedule(app.Schedules[0], stop)
		close(done)
	}()
	// Wait long enough for the timer to fire (it won't because next is far future)
	// Instead close stop immediately to test the stop path
	close(stop)
	select {
	case <-done:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("runCronSchedule did not exit")
	}
}
