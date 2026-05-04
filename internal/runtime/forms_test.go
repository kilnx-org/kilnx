package runtime

import (
	"bytes"
	"mime/multipart"
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
	if !strings.HasPrefix(data["document"], "/_uploads/") {
		t.Errorf("expected uploaded document path to start with /_uploads/, got %q", data["document"])
	}
	// Verify file exists
	filePath := filepath.Join(uploadDir, strings.TrimPrefix(data["document"], "/_uploads/"))
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
	if data["document"] != "" {
		t.Errorf("expected disallowed extension to be rejected, got %q", data["document"])
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
