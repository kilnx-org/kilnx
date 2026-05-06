package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// ---------- fake SMTP server ----------

type fakeSMTPServer struct {
	listener      net.Listener
	addr          string
	wg            sync.WaitGroup
	mu            sync.Mutex
	mailFrom      string
	rcptTo        []string
	data          strings.Builder
	authSeen      bool
	advertiseAuth bool
}

func newFakeSMTPServer(advertiseAuth bool) *fakeSMTPServer {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	return &fakeSMTPServer{
		listener:      l,
		addr:          l.Addr().String(),
		advertiseAuth: advertiseAuth,
	}
}

func (s *fakeSMTPServer) start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := bufio.NewReader(conn)
		fmt.Fprintf(conn, "220 fake ESMTP ready\r\n")

		inData := false
		for {
			line, err := buf.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if inData {
				if line == "." {
					inData = false
					fmt.Fprintf(conn, "250 Message accepted\r\n")
					continue
				}
				s.mu.Lock()
				s.data.WriteString(line)
				s.data.WriteString("\r\n")
				s.mu.Unlock()
				continue
			}
			upper := strings.ToUpper(line)
			switch {
			case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
				if s.advertiseAuth {
					fmt.Fprintf(conn, "250-fake\r\n250-AUTH PLAIN\r\n250 OK\r\n")
				} else {
					fmt.Fprintf(conn, "250 fake\r\n")
				}
			case strings.HasPrefix(upper, "AUTH"):
				s.mu.Lock()
				s.authSeen = true
				s.mu.Unlock()
				fmt.Fprintf(conn, "235 Authentication succeeded\r\n")
			case strings.HasPrefix(upper, "MAIL FROM"):
				s.mu.Lock()
				s.mailFrom = line
				s.mu.Unlock()
				fmt.Fprintf(conn, "250 Sender OK\r\n")
			case strings.HasPrefix(upper, "RCPT TO"):
				s.mu.Lock()
				s.rcptTo = append(s.rcptTo, line)
				s.mu.Unlock()
				fmt.Fprintf(conn, "250 Recipient OK\r\n")
			case strings.HasPrefix(upper, "DATA"):
				fmt.Fprintf(conn, "354 Start mail input\r\n")
				inData = true
			case strings.HasPrefix(upper, "QUIT"):
				fmt.Fprintf(conn, "221 Bye\r\n")
				return
			default:
				fmt.Fprintf(conn, "250 OK\r\n")
			}
		}
	}()
}

func (s *fakeSMTPServer) stop() {
	s.listener.Close()
	s.wg.Wait()
}

// ---------- url rewriting transport for LLM mocking ----------

type urlRewritingTransport struct {
	host string
}

func (rt *urlRewritingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "api.anthropic.com" {
		req.URL.Scheme = "http"
		req.URL.Host = rt.host
	}
	return http.DefaultTransport.RoundTrip(req)
}

// ---------- SendEmail ----------

func TestSendEmail_WithoutAuth(t *testing.T) {
	srv := newFakeSMTPServer(false)
	srv.start()
	defer srv.stop()

	host, port, _ := net.SplitHostPort(srv.addr)
	os.Setenv("KILNX_SMTP_HOST", host)
	os.Setenv("KILNX_SMTP_PORT", port)
	os.Setenv("KILNX_SMTP_FROM", "sender@example.com")
	defer func() {
		os.Unsetenv("KILNX_SMTP_HOST")
		os.Unsetenv("KILNX_SMTP_PORT")
		os.Unsetenv("KILNX_SMTP_FROM")
	}()

	err := SendEmail("recipient@example.com", "Test Subject", "<p>Hello</p>")
	if err != nil {
		t.Fatalf("SendEmail failed: %v", err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if !strings.Contains(srv.mailFrom, "sender@example.com") {
		t.Errorf("expected MAIL FROM sender@example.com, got %s", srv.mailFrom)
	}
	if len(srv.rcptTo) != 1 || !strings.Contains(srv.rcptTo[0], "recipient@example.com") {
		t.Errorf("expected RCPT TO recipient@example.com, got %v", srv.rcptTo)
	}
	data := srv.data.String()
	if !strings.Contains(data, "Subject: Test Subject") {
		t.Errorf("expected Subject in data, got %s", data)
	}
	if !strings.Contains(data, "<p>Hello</p>") {
		t.Errorf("expected body in data, got %s", data)
	}
}

func TestSendEmail_WithAuth(t *testing.T) {
	srv := newFakeSMTPServer(true)
	srv.start()
	defer srv.stop()

	host, port, _ := net.SplitHostPort(srv.addr)
	os.Setenv("KILNX_SMTP_HOST", host)
	os.Setenv("KILNX_SMTP_PORT", port)
	os.Setenv("KILNX_SMTP_USER", "user")
	os.Setenv("KILNX_SMTP_PASS", "secret")
	os.Setenv("KILNX_SMTP_FROM", "auth@example.com")
	defer func() {
		os.Unsetenv("KILNX_SMTP_HOST")
		os.Unsetenv("KILNX_SMTP_PORT")
		os.Unsetenv("KILNX_SMTP_USER")
		os.Unsetenv("KILNX_SMTP_PASS")
		os.Unsetenv("KILNX_SMTP_FROM")
	}()

	err := SendEmail("to@example.com", "Auth Subject", "auth body")
	if err != nil {
		t.Fatalf("SendEmail failed: %v", err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if !srv.authSeen {
		t.Error("expected AUTH to be used")
	}
	if !strings.Contains(srv.mailFrom, "auth@example.com") {
		t.Errorf("expected MAIL FROM auth@example.com, got %s", srv.mailFrom)
	}
}

// ---------- SendEmailWithTemplate ----------

func TestSendEmailWithTemplate(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	_ = os.MkdirAll("templates", 0755)
	_ = os.WriteFile("templates/welcome.html", []byte("Hello {name}!"), 0644)

	srv := newFakeSMTPServer(false)
	srv.start()
	defer srv.stop()

	host, port, _ := net.SplitHostPort(srv.addr)
	os.Setenv("KILNX_SMTP_HOST", host)
	os.Setenv("KILNX_SMTP_PORT", port)
	os.Setenv("KILNX_SMTP_FROM", "sender@example.com")
	defer func() {
		os.Unsetenv("KILNX_SMTP_HOST")
		os.Unsetenv("KILNX_SMTP_PORT")
		os.Unsetenv("KILNX_SMTP_FROM")
	}()

	err := SendEmailWithTemplate("user@example.com", "Welcome", "fallback body", "welcome", map[string]string{"name": "Alice"})
	if err != nil {
		t.Fatalf("SendEmailWithTemplate failed: %v", err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if !strings.Contains(srv.data.String(), "Hello Alice!") {
		t.Errorf("expected interpolated template body, got %s", srv.data.String())
	}
}

// ---------- SendEmailWithAttachment ----------

func TestSendEmailWithAttachment_AllowedPath(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	_ = os.MkdirAll("uploads", 0755)
	_ = os.WriteFile("uploads/report.pdf", []byte("fake pdf content"), 0644)

	srv := newFakeSMTPServer(false)
	srv.start()
	defer srv.stop()

	host, port, _ := net.SplitHostPort(srv.addr)
	os.Setenv("KILNX_SMTP_HOST", host)
	os.Setenv("KILNX_SMTP_PORT", port)
	os.Setenv("KILNX_SMTP_FROM", "sender@example.com")
	defer func() {
		os.Unsetenv("KILNX_SMTP_HOST")
		os.Unsetenv("KILNX_SMTP_PORT")
		os.Unsetenv("KILNX_SMTP_FROM")
	}()

	err := SendEmailWithAttachment("user@example.com", "Report", "See attached", "uploads/report.pdf")
	if err != nil {
		t.Fatalf("SendEmailWithAttachment failed: %v", err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	data := srv.data.String()
	if !strings.Contains(data, "report.pdf") {
		t.Errorf("expected attachment filename in data")
	}
	if !strings.Contains(data, "multipart/mixed") {
		t.Errorf("expected multipart content type")
	}
}

func TestSendEmailWithAttachment_DisallowedPath(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	_ = os.WriteFile("/tmp/secret.txt", []byte("secret"), 0644)
	defer os.Remove("/tmp/secret.txt")

	err := SendEmailWithAttachment("user@example.com", "Bad", "body", "/tmp/secret.txt")
	if err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
		t.Fatalf("expected disallowed path error, got %v", err)
	}
}

func TestLoadEmailTemplate_NotFound(t *testing.T) {
	result := LoadEmailTemplate("nonexistent", map[string]string{"name": "Alice"})
	if result != "" {
		t.Errorf("expected empty string for missing template, got %q", result)
	}
}

// ---------- executeFetch ----------

func TestExecuteFetch_GetJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("User-Agent") != "Kilnx/1.0" {
			t.Errorf("expected User-Agent Kilnx/1.0, got %s", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","name":"Alice"}`))
	}))
	defer srv.Close()

	node := parser.Node{
		Type:        parser.NodeFetch,
		FetchURL:    srv.URL,
		FetchMethod: "GET",
	}
	rows, _, err := executeFetch(node, nil)
	if err != nil {
		t.Fatalf("executeFetch failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["id"] != "1" || rows[0]["name"] != "Alice" {
		t.Errorf("unexpected row: %v", rows[0])
	}
}

func TestExecuteFetch_PostWithBodyAndHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("X-Custom") != "testval" {
			t.Errorf("expected X-Custom testval, got %s", r.Header.Get("X-Custom"))
		}
		_ = r.ParseForm()
		if r.FormValue("key") != "value" {
			t.Errorf("expected key=value, got %s", r.FormValue("key"))
		}
		_, _ = w.Write([]byte(`[{"status":"ok"}]`))
	}))
	defer srv.Close()

	node := parser.Node{
		Type:         parser.NodeFetch,
		FetchURL:     srv.URL,
		FetchMethod:  "POST",
		FetchBody:    map[string]string{"key": "value"},
		FetchHeaders: map[string]string{"X-Custom": "testval"},
	}
	rows, _, err := executeFetch(node, nil)
	if err != nil {
		t.Fatalf("executeFetch failed: %v", err)
	}
	if len(rows) != 1 || rows[0]["status"] != "ok" {
		t.Errorf("unexpected rows: %v", rows)
	}
}

func TestExecuteFetch_UrlParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/42" {
			t.Errorf("expected path /users/42, got %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"found":"true"}`))
	}))
	defer srv.Close()

	node := parser.Node{
		Type:        parser.NodeFetch,
		FetchURL:    srv.URL + "/users/:id",
		FetchMethod: "GET",
	}
	rows, _, err := executeFetch(node, map[string]string{"id": "42"})
	if err != nil {
		t.Fatalf("executeFetch failed: %v", err)
	}
	if len(rows) != 1 || rows[0]["found"] != "true" {
		t.Errorf("unexpected rows: %v", rows)
	}
}

func TestExecuteFetch_NonJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("plain text"))
	}))
	defer srv.Close()

	node := parser.Node{
		Type:        parser.NodeFetch,
		FetchURL:    srv.URL,
		FetchMethod: "GET",
	}
	rows, _, err := executeFetch(node, nil)
	if err != nil {
		t.Fatalf("executeFetch failed: %v", err)
	}
	if len(rows) != 1 || rows[0]["_body"] != "plain text" {
		t.Errorf("unexpected rows: %v", rows)
	}
}

// ---------- executeLLM ----------

func TestExecuteLLM_MissingAPIKey(t *testing.T) {
	os.Unsetenv("ANTHROPIC_API_KEY")
	node := parser.Node{Type: parser.NodeLLM}
	_, err := executeLLM(node, newMockExecutor(), nil)
	if err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY not set") {
		t.Fatalf("expected missing API key error, got %v", err)
	}
}

func TestExecuteLLM_EmptyHistory(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	node := parser.Node{Type: parser.NodeLLM}
	result, err := executeLLM(node, newMockExecutor(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Olá! Como posso ajudar?" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExecuteLLM_WithHistory(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key test-key, got %s", r.Header.Get("x-api-key"))
		}

		var req anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if len(req.Messages) == 0 {
			t.Error("expected non-empty messages")
		}
		if req.System != "You are a test assistant" {
			t.Errorf("expected system prompt, got %q", req.System)
		}

		resp := anthropicResponse{
			Content: []struct {
				Text string `json:"text"`
			}{{Text: "  Resposta do assistente  "}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	origClient := llmClient
	llmClient = &http.Client{
		Timeout:   60 * time.Second,
		Transport: &urlRewritingTransport{host: u.Host},
	}
	defer func() { llmClient = origClient }()

	db := newMockExecutor()
	db.queryRowsWithParamsResults["SELECT papel, conteudo FROM messages"] = []database.Row{
		{"papel": "user", "conteudo": "Oi"},
		{"papel": "assistente", "conteudo": "Olá!"},
	}

	node := parser.Node{
		Type:          parser.NodeLLM,
		LLMModel:      "claude-test",
		LLMSystem:     "You are a test assistant",
		LLMHistorySQL: "SELECT papel, conteudo FROM messages",
	}

	result, err := executeLLM(node, db, nil)
	if err != nil {
		t.Fatalf("executeLLM failed: %v", err)
	}
	if result != "Resposta do assistente" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExecuteLLM_APIError(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(anthropicResponse{
			Error: &struct {
				Message string `json:"message"`
			}{Message: "invalid model"},
		})
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	origClient := llmClient
	llmClient = &http.Client{
		Timeout:   60 * time.Second,
		Transport: &urlRewritingTransport{host: u.Host},
	}
	defer func() { llmClient = origClient }()

	db := newMockExecutor()
	db.queryRowsWithParamsResults["SELECT papel, conteudo FROM messages"] = []database.Row{
		{"papel": "user", "conteudo": "Oi"},
	}

	node := parser.Node{
		Type:          parser.NodeLLM,
		LLMHistorySQL: "SELECT papel, conteudo FROM messages",
	}

	_, err := executeLLM(node, db, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid model") {
		t.Fatalf("expected anthropic error, got %v", err)
	}
}

func TestExecuteLLM_EmptyContent(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(anthropicResponse{
			Content: []struct {
				Text string `json:"text"`
			}{},
		})
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	origClient := llmClient
	llmClient = &http.Client{
		Timeout:   60 * time.Second,
		Transport: &urlRewritingTransport{host: u.Host},
	}
	defer func() { llmClient = origClient }()

	db := newMockExecutor()
	db.queryRowsWithParamsResults["SELECT papel, conteudo FROM messages"] = []database.Row{
		{"papel": "user", "conteudo": "Oi"},
	}

	node := parser.Node{
		Type:          parser.NodeLLM,
		LLMHistorySQL: "SELECT papel, conteudo FROM messages",
	}

	_, err := executeLLM(node, db, nil)
	if err == nil || !strings.Contains(err.Error(), "empty response") {
		t.Fatalf("expected empty response error, got %v", err)
	}
}

func TestExecuteLLM_HistoryQueryError(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	db := newMockExecutor()
	db.queryRowsWithParamsErr["SELECT papel, conteudo FROM messages"] = fmt.Errorf("db error")

	node := parser.Node{
		Type:          parser.NodeLLM,
		LLMHistorySQL: "SELECT papel, conteudo FROM messages",
	}

	_, err := executeLLM(node, db, nil)
	if err == nil || !strings.Contains(err.Error(), "llm history query") {
		t.Fatalf("expected history query error, got %v", err)
	}
}

func TestExecuteLLM_HistorySkipEmptyAndSystem(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req anthropicRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.Messages) != 1 {
			t.Errorf("expected 1 message after skipping empty and system, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "user" || req.Messages[0].Content != "system msg" {
			t.Errorf("expected user role with system msg, got %+v", req.Messages[0])
		}
		_ = json.NewEncoder(w).Encode(anthropicResponse{
			Content: []struct {
				Text string `json:"text"`
			}{{Text: "ok"}},
		})
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	origClient := llmClient
	llmClient = &http.Client{Transport: &urlRewritingTransport{host: u.Host}}
	defer func() { llmClient = origClient }()

	db := newMockExecutor()
	db.queryRowsWithParamsResults["SELECT papel, conteudo FROM messages"] = []database.Row{
		{"papel": "user", "conteudo": ""},
		{"papel": "sistema", "conteudo": "system msg"},
	}

	node := parser.Node{
		Type:          parser.NodeLLM,
		LLMHistorySQL: "SELECT papel, conteudo FROM messages",
	}

	result, err := executeLLM(node, db, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExecuteLLM_NetworkError(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	origClient := llmClient
	llmClient = &http.Client{
		Transport: &failingTransport{},
	}
	defer func() { llmClient = origClient }()

	db := newMockExecutor()
	db.queryRowsWithParamsResults["SELECT papel, conteudo FROM messages"] = []database.Row{
		{"papel": "user", "conteudo": "Oi"},
	}

	node := parser.Node{
		Type:          parser.NodeLLM,
		LLMHistorySQL: "SELECT papel, conteudo FROM messages",
	}

	_, err := executeLLM(node, db, nil)
	if err == nil || !strings.Contains(err.Error(), "llm http") {
		t.Fatalf("expected network error, got %v", err)
	}
}

func TestExecuteLLM_InvalidJSONResponse(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	origClient := llmClient
	llmClient = &http.Client{
		Transport: &urlRewritingTransport{host: u.Host},
	}
	defer func() { llmClient = origClient }()

	db := newMockExecutor()
	db.queryRowsWithParamsResults["SELECT papel, conteudo FROM messages"] = []database.Row{
		{"papel": "user", "conteudo": "Oi"},
	}

	node := parser.Node{
		Type:          parser.NodeLLM,
		LLMHistorySQL: "SELECT papel, conteudo FROM messages",
	}

	_, err := executeLLM(node, db, nil)
	if err == nil || !strings.Contains(err.Error(), "llm parse") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

type failingTransport struct{}

func (f *failingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("network down")
}
