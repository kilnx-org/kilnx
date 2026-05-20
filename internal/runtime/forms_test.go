package runtime

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestExtractFormData_Multipart(t *testing.T) {
	// Create a multipart request with a form field and a file
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	w.WriteField("name", "Alice")
	w.WriteField("age", "30")
	w.Close()

	req := httptest.NewRequest("POST", "/upload", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())

	data := extractFormData(req, nil)
	if data["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %q", data["name"])
	}
	if data["age"] != "30" {
		t.Errorf("expected age=30, got %q", data["age"])
	}
}

func TestExtractFormData_MultipartWithFileUpload(t *testing.T) {
	// Create a temp file to upload
	tmpDir := t.TempDir()
	uploadDir := filepath.Join(tmpDir, "uploads")

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	w.WriteField("title", "My Doc")

	// Add a file part
	fw, _ := w.CreateFormFile("document", "report.pdf")
	fw.Write([]byte("fake pdf content"))
	w.Close()

	req := httptest.NewRequest("POST", "/upload", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())

	cfg := &parser.AppConfig{UploadsDir: uploadDir, UploadsMaxMB: 10}
	data := extractFormData(req, cfg)

	if data["title"] != "My Doc" {
		t.Errorf("expected title=My Doc, got %q", data["title"])
	}
	if data["document"] == "" {
		t.Fatal("expected document path in data")
	}
	var docPaths []string
	if err := json.Unmarshal([]byte(data["document"]), &docPaths); err != nil {
		t.Fatalf("expected JSON array for document, got %q: %v", data["document"], err)
	}
	if len(docPaths) != 1 || !strings.HasPrefix(docPaths[0], "/_uploads/") {
		t.Errorf("expected 1 /_uploads/ path, got %v", docPaths)
	}
	// Verify file exists
	filePath := filepath.Join(uploadDir, strings.TrimPrefix(docPaths[0], "/_uploads/"))
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("uploaded file does not exist at %s", filePath)
	}
}

func TestExtractFormData_MultipartDisallowedExt(t *testing.T) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, _ := w.CreateFormFile("document", "malware.exe")
	fw.Write([]byte("evil"))
	w.Close()

	req := httptest.NewRequest("POST", "/upload", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())

	data := extractFormData(req, nil)
	var paths []string
	if err := json.Unmarshal([]byte(data["document"]), &paths); err != nil {
		t.Fatalf("expected JSON array, got %q: %v", data["document"], err)
	}
	if len(paths) != 0 {
		t.Errorf("expected disallowed extension to be rejected (empty array), got %v", paths)
	}
}

func TestExtractFormData_URLEncoded(t *testing.T) {
	req := httptest.NewRequest("POST", "/form", strings.NewReader("name=Bob&email=bob%40test.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	data := extractFormData(req, nil)
	if data["name"] != "Bob" {
		t.Errorf("expected name=Bob, got %q", data["name"])
	}
	if data["email"] != "bob@test.com" {
		t.Errorf("expected email=bob@test.com, got %q", data["email"])
	}
}

func TestExtractFormData_CustomBrackets(t *testing.T) {
	req := httptest.NewRequest("POST", "/form", strings.NewReader("title=Deal&custom[revenue]=500&custom[region]=S"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	data := extractFormData(req, nil)
	if data["title"] != "Deal" {
		t.Errorf("expected title=Deal, got %q", data["title"])
	}
	if data["custom"] == "" {
		t.Fatal("expected custom JSON field")
	}
	if !strings.Contains(data["custom"], `"revenue"`) {
		t.Errorf("expected revenue in custom JSON, got %q", data["custom"])
	}
	if data["revenue"] != "500" {
		t.Errorf("expected revenue promoted to top level, got %q", data["revenue"])
	}
	if data["region"] != "S" {
		t.Errorf("expected region promoted to top level, got %q", data["region"])
	}
}

func TestExtractFormData_MultipartParseError(t *testing.T) {
	// invalid multipart body
	req := httptest.NewRequest("POST", "/upload", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=----fake")

	data := extractFormData(req, nil)
	if data == nil {
		t.Error("expected empty map, not nil")
	}
}

func TestExtractFormData_SingleFile_StillJSONArray(t *testing.T) {
	tmp := t.TempDir()
	cfg := &parser.AppConfig{UploadsDir: tmp, UploadsMaxMB: 32}
	req := newMultipartRequest(t, map[string][]string{
		"avatar": {"me.png|content"},
	})
	data := extractFormData(req, cfg)
	raw := data["avatar"]
	var paths []string
	if err := json.Unmarshal([]byte(raw), &paths); err != nil {
		t.Fatalf("expected JSON array for single file, got %q", raw)
	}
	if len(paths) != 1 {
		t.Fatalf("want 1 path, got %d", len(paths))
	}
}

func newMultipartRequest(t *testing.T, files map[string][]string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for field, entries := range files {
		for _, entry := range entries {
			// entry is "<filename>|<content>"
			parts := strings.SplitN(entry, "|", 2)
			fw, err := w.CreateFormFile(field, parts[0])
			if err != nil {
				t.Fatalf("CreateFormFile: %v", err)
			}
			fw.Write([]byte(parts[1]))
		}
	}
	w.Close()
	req, _ := http.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestExtractFormData_MultiFile_ReturnsJSONArray(t *testing.T) {
	tmp := t.TempDir()
	cfg := &parser.AppConfig{UploadsDir: tmp, UploadsMaxMB: 32}
	req := newMultipartRequest(t, map[string][]string{
		"files": {"a.pdf|alpha", "b.pdf|beta", "c.pdf|gamma"},
	})
	data := extractFormData(req, cfg)
	raw, ok := data["files"]
	if !ok {
		t.Fatalf("expected files binding, got %#v", data)
	}
	var paths []string
	if err := json.Unmarshal([]byte(raw), &paths); err != nil {
		t.Fatalf("expected JSON array, got %q (err %v)", raw, err)
	}
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d (%v)", len(paths), paths)
	}
	for _, p := range paths {
		if !strings.HasPrefix(p, "/_uploads/") {
			t.Fatalf("expected /_uploads/ prefix, got %q", p)
		}
	}
}
