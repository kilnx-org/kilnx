package runtime

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func newTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func postForm(t *testing.T, rawURL string) *http.Response {
	t.Helper()
	token := generateCSRFToken()
	vals := url.Values{"_csrf": {token}}
	resp, err := http.Post(rawURL, "application/x-www-form-urlencoded", strings.NewReader(vals.Encode()))
	if err != nil {
		t.Fatalf("POST %s: %v", rawURL, err)
	}
	return resp
}

func TestRespondFile_Roundtrip(t *testing.T) {
	tmp := t.TempDir()
	reportsDir := filepath.Join(tmp, "reports")
	target := newTempFile(t, reportsDir, "r.pdf", "PDFBYTES")

	src := fmt.Sprintf(`config
  serve-roots: %s
action /x
  respond file %s
`, reportsDir, target)

	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	resp := postForm(t, baseURL+"/x")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: %d (body: %s)", resp.StatusCode, body)
	}
	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") || !strings.Contains(cd, "r.pdf") {
		t.Errorf("Content-Disposition: %q", cd)
	}
}

func TestRespondFile_OutsideRoot403(t *testing.T) {
	tmp := t.TempDir()
	allowedDir := filepath.Join(tmp, "allowed")
	os.MkdirAll(allowedDir, 0o755)
	outside := newTempFile(t, filepath.Join(tmp, "elsewhere"), "secret.txt", "shh")

	src := fmt.Sprintf(`config
  serve-roots: %s
action /leak
  respond file %s
`, allowedDir, outside)

	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	resp := postForm(t, baseURL+"/leak")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 403, got %d (body: %s)", resp.StatusCode, body)
	}
}

func TestRespondFile_Traversal403(t *testing.T) {
	tmp := t.TempDir()
	allowedDir := filepath.Join(tmp, "allowed")
	os.MkdirAll(allowedDir, 0o755)
	// Create a file outside allowed, then construct a traversal path
	outsideDir := filepath.Join(tmp, "outside")
	outsideFile := newTempFile(t, outsideDir, "secret.txt", "shh")
	// Build a traversal path using raw concat so ".." is NOT pre-resolved.
	// filepath.Join would clean the ".." away before the runtime ever sees it,
	// making the test equivalent to OutsideRoot403. Raw concat preserves the
	// traversal sequence so filepath.Abs+Clean inside the runtime resolves it.
	traversalPath := allowedDir + "/../outside/secret.txt"
	_ = outsideFile

	src := fmt.Sprintf(`config
  serve-roots: %s
action /t
  respond file %s
`, allowedDir, traversalPath)

	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	resp := postForm(t, baseURL+"/t")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 403, got %d (body: %s)", resp.StatusCode, body)
	}
}

func TestRespondFile_Missing404(t *testing.T) {
	tmp := t.TempDir()
	allowedDir := filepath.Join(tmp, "allowed")
	os.MkdirAll(allowedDir, 0o755)
	missingPath := filepath.Join(allowedDir, "nope.pdf")

	src := fmt.Sprintf(`config
  serve-roots: %s
action /m
  respond file %s
`, allowedDir, missingPath)

	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	resp := postForm(t, baseURL+"/m")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 404, got %d (body: %s)", resp.StatusCode, body)
	}
}

func TestRespondFile_InlineHonored(t *testing.T) {
	tmp := t.TempDir()
	uploadsDir := filepath.Join(tmp, "uploads")
	target := newTempFile(t, uploadsDir, "p.png", "PNGBYTES")

	src := fmt.Sprintf(`config
  serve-roots: %s
action /i
  respond file %s inline
`, uploadsDir, target)

	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	resp := postForm(t, baseURL+"/i")
	defer resp.Body.Close()
	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "inline") {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("want inline disposition, got %q (status=%d body=%s)", cd, resp.StatusCode, body)
	}
}

func TestRespondFile_AsOverridesName(t *testing.T) {
	tmp := t.TempDir()
	uploadsDir := filepath.Join(tmp, "uploads")
	target := newTempFile(t, uploadsDir, "long_internal_name.pdf", "X")

	src := fmt.Sprintf(`config
  serve-roots: %s
action /n
  respond file %s as report.pdf
`, uploadsDir, target)

	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	resp := postForm(t, baseURL+"/n")
	defer resp.Body.Close()
	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "report.pdf") {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("want report.pdf in Content-Disposition, got %q (status=%d body=%s)", cd, resp.StatusCode, body)
	}
}

// TestRespondFile_HandleActionNodes exercises the NodeRespondFile case inside
// handleActionNodes (the second switch site, used by on success/error branches).
func TestRespondFile_HandleActionNodes(t *testing.T) {
	tmp := t.TempDir()
	uploadsDir := filepath.Join(tmp, "uploads")
	target := newTempFile(t, uploadsDir, "doc.pdf", "PDFDATA")

	s := newTestServer(nil)
	// Wire serve-roots into app config so pathInsideAnyRoot allows the file.
	s.app.Config = &parser.AppConfig{ServeRoots: []string{uploadsDir}}

	node := parser.Node{
		Type:                  parser.NodeRespondFile,
		RespondFilePath:       target,
		RespondFileName:       "",
		RespondFileDisposition: "attachment",
	}

	req := httptest.NewRequest("POST", "/x", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{node}, map[string]string{}, s.getApp())

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") || !strings.Contains(cd, "doc.pdf") {
		t.Errorf("Content-Disposition: %q", cd)
	}
}
