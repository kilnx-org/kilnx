package runtime

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/lexer"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// startTestServer parses a .kilnx source string, opens a temp DB, migrates,
// starts the server on a random port, and returns the base URL + cleanup func.
func startTestServer(t *testing.T, src string) (string, func()) {
	t.Helper()

	source := lexer.StripComments(src)
	tokens := lexer.Tokenize(source)
	app, err := parser.Parse(tokens, source)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := db.MigrateInternal(); err != nil {
		db.Close()
		t.Fatalf("migrate internal: %v", err)
	}
	if len(app.Models) > 0 {
		if _, err := db.Migrate(app.Models); err != nil {
			db.Close()
			t.Fatalf("migrate: %v", err)
		}
	}

	// Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		db.Close()
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	srv := NewServer(app, db, port)
	go srv.Start()

	// Wait for server to be ready
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	return baseURL, func() { db.Close() }
}

func httpGet(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func httpGetWithHeader(t *testing.T, urlStr string, header, value string) (int, string) {
	t.Helper()
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set(header, value)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", urlStr, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func httpPost(t *testing.T, urlStr string, data url.Values) (int, string) {
	t.Helper()
	resp, err := http.Post(urlStr, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		t.Fatalf("POST %s: %v", urlStr, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func httpPostJSON(t *testing.T, urlStr string, jsonBody string, headers map[string]string) (int, string) {
	t.Helper()
	req, err := http.NewRequest("POST", urlStr, strings.NewReader(jsonBody))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", urlStr, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}
